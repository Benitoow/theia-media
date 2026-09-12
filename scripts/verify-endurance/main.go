// Command verify-endurance exercises the real binary and pinned FFmpeg across
// repeated cancelled streams and one complete long-duration stream.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	managedffmpeg "github.com/Benitoow/theia-media/internal/ffmpeg"
)

type movie struct {
	ID       int64  `json:"id"`
	FileName string `json:"file_name"`
}

func main() {
	theia := flag.String("theia", "theia.exe", "Theia binary to exercise")
	ffmpegPath := flag.String("ffmpeg", os.Getenv("THEIA_TEST_FFMPEG"), "pinned FFmpeg binary; omitted to download Theia's verified runtime")
	cycles := flag.Int("cycles", 100, "number of open/read/cancel cycles")
	longMinutes := flag.Int("long-minutes", 30, "duration encoded into the fully consumed stream")
	port := flag.Int("port", 8395, "throwaway server port")
	flag.Parse()
	if *cycles < 1 || *longMinutes < 1 {
		fmt.Fprintln(os.Stderr, "verify-endurance: positive -cycles and positive -long-minutes are required")
		os.Exit(2)
	}
	if *ffmpegPath == "" {
		bootstrap, err := os.MkdirTemp("", "theia-endurance-runtime-*")
		if err != nil {
			fmt.Fprintln(os.Stderr, "verify-endurance:", err)
			os.Exit(1)
		}
		defer os.RemoveAll(bootstrap)
		*ffmpegPath, err = managedffmpeg.New(bootstrap, slog.Default()).Path(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, "verify-endurance: preparing the pinned ffmpeg:", err)
			os.Exit(1)
		}
	}
	if err := run(*theia, *ffmpegPath, *cycles, *longMinutes, *port); err != nil {
		fmt.Fprintln(os.Stderr, "verify-endurance:", err)
		os.Exit(1)
	}
}

func run(theiaPath, ffmpegPath string, cycles, longMinutes, port int) error {
	temporary, err := os.MkdirTemp("", "theia-endurance-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	dataDir, mediaDir := filepath.Join(temporary, "data"), filepath.Join(temporary, "media")
	binDir := filepath.Join(dataDir, "bin")
	for _, dir := range []string{dataDir, mediaDir, binDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	ffmpegTarget := filepath.Join(binDir, "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpegTarget += ".exe"
	}
	if err := copyFile(ffmpegPath, ffmpegTarget); err != nil {
		return fmt.Errorf("copying pinned ffmpeg: %w", err)
	}
	config, _ := json.Marshal(map[string]any{
		"port": port, "hostname": "theia-endurance", "library_paths": []string{mediaDir},
	})
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), config, 0o600); err != nil {
		return err
	}

	cycleFilm := filepath.Join(mediaDir, "Endurance.Cycles.2026.mkv")
	longFilm := filepath.Join(mediaDir, "Endurance.Long.2026.mkv")
	fmt.Printf("==> generating cycle media and a %d-minute long-duration stream\n", longMinutes)
	if err := encode(ffmpegPath, cycleFilm, "testsrc2=size=640x360:rate=24", 60, "mpeg2video"); err != nil {
		return err
	}
	if err := encode(ffmpegPath, longFilm, "color=c=0x18202b:size=320x180:rate=24", longMinutes*60, "libx264"); err != nil {
		return err
	}
	old := time.Now().Add(-time.Hour)
	for _, path := range []string{cycleFilm, longFilm} {
		if err := os.Chtimes(path, old, old); err != nil {
			return err
		}
	}

	theiaAbs, err := filepath.Abs(theiaPath)
	if err != nil {
		return err
	}
	command := exec.Command(theiaAbs, "-data-dir", dataDir, "-port", strconv.Itoa(port))
	command.Dir, command.Stdout, command.Stderr = dataDir, os.Stdout, os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	defer command.Process.Kill()
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{Timeout: 30 * time.Second}
	movies, err := waitForMovies(client, baseURL, 2, 45*time.Second)
	if err != nil {
		return err
	}
	cycleID, longID := findMovies(movies)
	if cycleID == 0 || longID == 0 {
		return fmt.Errorf("generated films were not both indexed: %+v", movies)
	}
	baselineFFmpeg, err := ffmpegProcessCount()
	if err != nil {
		return err
	}
	baselineMemory, err := processMemory(command.Process.Pid)
	if err != nil {
		return err
	}
	fmt.Printf("==> baseline: %.1f MiB, %d ffmpeg processes on the machine\n", mib(baselineMemory), baselineFFmpeg)

	samples := []int64{baselineMemory}
	streamClient := &http.Client{Timeout: 30 * time.Second}
	for index := 0; index < cycles; index++ {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		url := fmt.Sprintf("%s/api/stream/%d/remux?t=%d", baseURL, cycleID, index%45)
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		response, requestErr := streamClient.Do(request)
		if requestErr != nil {
			cancel()
			return fmt.Errorf("cycle %d opening stream: %w", index+1, requestErr)
		}
		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			cancel()
			return fmt.Errorf("cycle %d status %d: %s", index+1, response.StatusCode, body)
		}
		read, _ := io.CopyN(io.Discard, response.Body, 256<<10)
		response.Body.Close()
		cancel()
		if read == 0 {
			return fmt.Errorf("cycle %d returned no stream bytes", index+1)
		}
		if !waitFor(5*time.Second, func() bool { n, _ := ffmpegProcessCount(); return n <= baselineFFmpeg }) {
			return fmt.Errorf("cycle %d left an ffmpeg process alive", index+1)
		}
		if (index+1)%10 == 0 {
			memory, err := processMemory(command.Process.Pid)
			if err != nil {
				return err
			}
			samples = append(samples, memory)
			fmt.Printf("    %3d/%d cycles, RSS %.1f MiB\n", index+1, cycles, mib(memory))
		}
	}

	fmt.Printf("==> consuming the complete %d-minute stream\n", longMinutes)
	started := time.Now()
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Get(
		fmt.Sprintf("%s/api/stream/%d/remux", baseURL, longID),
	)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return fmt.Errorf("long stream status %d", response.StatusCode)
	}
	bytesRead, copyErr := io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if copyErr != nil || bytesRead == 0 {
		return fmt.Errorf("long stream read %d bytes: %w", bytesRead, copyErr)
	}
	if !waitFor(10*time.Second, func() bool { n, _ := ffmpegProcessCount(); return n <= baselineFFmpeg }) {
		return fmt.Errorf("the complete stream left an ffmpeg process alive")
	}
	finalMemory, err := processMemory(command.Process.Pid)
	if err != nil {
		return err
	}
	samples = append(samples, finalMemory)
	peak := baselineMemory
	for _, sample := range samples {
		if sample > peak {
			peak = sample
		}
	}
	const finalAllowance = int64(192 << 20)
	const peakAllowance = int64(384 << 20)
	if finalMemory-baselineMemory > finalAllowance {
		return fmt.Errorf("RSS retained %.1f MiB after the run (allowance %.0f MiB)", mib(finalMemory-baselineMemory), mib(finalAllowance))
	}
	if peak-baselineMemory > peakAllowance {
		return fmt.Errorf("RSS peaked %.1f MiB above baseline (allowance %.0f MiB)", mib(peak-baselineMemory), mib(peakAllowance))
	}
	fmt.Printf("==> long stream: %.1f MiB in %s\n", mib(bytesRead), time.Since(started).Round(time.Millisecond))
	fmt.Printf("==> RSS final %.1f MiB, peak %.1f MiB; no ffmpeg above baseline\n", mib(finalMemory), mib(peak))
	fmt.Println("PASS")
	return nil
}

func encode(ffmpeg, output, video string, seconds int, codec string) error {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", video,
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", strconv.Itoa(seconds), "-c:v", codec,
	}
	if codec == "libx264" {
		args = append(args, "-preset", "ultrafast", "-pix_fmt", "yuv420p")
	}
	args = append(args, "-c:a", "ac3", output)
	if out, err := exec.Command(ffmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("encoding %s: %w: %s", filepath.Base(output), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitForMovies(client *http.Client, baseURL string, count int, timeout time.Duration) ([]movie, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, err := client.Get(baseURL + "/api/library/movies?limit=10")
		if err == nil {
			var page struct {
				Movies []movie `json:"movies"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&page)
			response.Body.Close()
			if decodeErr == nil && len(page.Movies) >= count {
				return page.Movies, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("the generated library was not indexed within %s", timeout)
}

func findMovies(movies []movie) (cycleID, longID int64) {
	for _, item := range movies {
		if strings.Contains(item.FileName, "Cycles") {
			cycleID = item.ID
		}
		if strings.Contains(item.FileName, "Long") {
			longID = item.ID
		}
	}
	return cycleID, longID
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func ffmpegProcessCount() (int, error) {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq ffmpeg.exe", "/FO", "CSV", "/NH").Output()
		if err != nil {
			return 0, err
		}
		return strings.Count(strings.ToLower(string(out)), "ffmpeg.exe"), nil
	}
	out, err := exec.Command("sh", "-c", "pgrep -x ffmpeg || true").Output()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Fields(string(out)) {
		if line != "" {
			count++
		}
	}
	return count, nil
}

func processMemory(pid int) (int64, error) {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return 0, err
		}
		records, err := csv.NewReader(strings.NewReader(string(out))).ReadAll()
		if err != nil || len(records) == 0 || len(records[0]) < 5 {
			return 0, fmt.Errorf("cannot read RSS for pid %d from %q", pid, out)
		}
		digits := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, records[0][4])
		kilobytes, err := strconv.ParseInt(digits, 10, 64)
		return kilobytes * 1024, err
	}
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			return 0, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				kilobytes, err := strconv.ParseInt(fields[1], 10, 64)
				return kilobytes * 1024, err
			}
		}
	}
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	kilobytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return kilobytes * 1024, err
}

func waitFor(timeout time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func mib(bytes int64) float64 { return float64(bytes) / (1024 * 1024) }

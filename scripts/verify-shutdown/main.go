//go:build windows

// Command verify-shutdown runs the shutdown acceptance of the playback
// backend redesign (docs/plan-refonte-lecture.md, tranche 3) against a real
// binary: hold live converted streams, ask the server to stop the way the
// console does, and prove that no ffmpeg is left running beside a dead
// server.
//
// It is deliberately end-to-end: a real binary, a real generated film, real
// ffmpeg processes counted in the process list -- the failure this checks for
// is one the unit tests cannot see, because it happens between the process
// and its children.
//
// Usage:
//
//	go run ./scripts/verify-shutdown -theia <binary> -ffmpeg <pinned ffmpeg> [-count 3] [-port 8395]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	theia := flag.String("theia", "theia.exe", "the Theia binary to verify")
	ffmpeg := flag.String("ffmpeg", os.Getenv("THEIA_TEST_FFMPEG"), "the pinned ffmpeg, used to build the film")
	count := flag.Int("count", 3, "how many live streams to hold during the shutdown")
	port := flag.Int("port", 8395, "port for the throwaway server")
	flag.Parse()

	if *ffmpeg == "" {
		fmt.Fprintln(os.Stderr, "verify-shutdown: -ffmpeg is required")
		os.Exit(2)
	}

	if err := run(*theia, *ffmpeg, *count, *port); err != nil {
		fmt.Fprintln(os.Stderr, "verify-shutdown:", err)
		os.Exit(1)
	}
}

func run(theiaPath, ffmpegPath string, count, port int) error {
	// A throwaway everything: data directory, library, database.
	dir, err := os.MkdirTemp("", "theia-shutdown-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	dataDir := filepath.Join(dir, "data")
	mediaDir := filepath.Join(dir, "media")
	for _, d := range []string{dataDir, mediaDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"),
		[]byte(fmt.Sprintf(`{"port": %d, "library_paths": [%q]}`, port, mediaDir)), 0o644); err != nil {
		return err
	}

	film := filepath.Join(mediaDir, "Long Film (2020).mkv")
	fmt.Println("==> building the film")
	build := exec.Command(ffmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=1920x1080:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", "600",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k",
		film)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("building the film: %w: %s", err, oneLine(string(out)))
	}

	// The baseline is what this machine already runs: other ffmpeg processes
	// belong to other software, and the verdict counts only the delta.
	baseline, err := ffmpegProcessCount()
	if err != nil {
		return err
	}

	fmt.Printf("==> starting %s on port %d\n", theiaPath, port)
	// cmd.Dir would otherwise resolve a relative path against the throwaway
	// directory rather than the caller's, which is not what the argument
	// promised.
	theiaAbs, err := filepath.Abs(theiaPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(theiaAbs, "--data-dir", dataDir, "--port", strconv.Itoa(port))
	cmd.Dir = dataDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// A new process group is what makes the console interrupt below reach
	// this child and nothing else on the machine.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return err
	}
	running := true
	defer func() {
		if running {
			_ = cmd.Process.Kill()
		}
	}()

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 10 * time.Second}
	if err := waitHealthy(client, base, 30*time.Second); err != nil {
		return err
	}

	fmt.Println("==> waiting for the scan")
	filmID, err := waitForFilm(client, base, 150*time.Second)
	if err != nil {
		return err
	}
	if code := statusOf(client, base+fmt.Sprintf("/api/stream/%d/files/1/info", filmID)); code != http.StatusOK {
		return fmt.Errorf("info = %d", code)
	}

	fmt.Printf("==> holding %d live streams\n", count)
	live := make([]*http.Response, 0, count)
	defer func() {
		for _, res := range live {
			if res != nil && res.Body != nil {
				res.Body.Close()
			}
		}
	}()
	streamClient := &http.Client{Timeout: 10 * time.Minute}
	for i := 0; i < count; i++ {
		res, err := streamClient.Get(base + fmt.Sprintf("/api/stream/%d/files/1/remux", filmID))
		if err != nil {
			return fmt.Errorf("stream %d failed to start: %w", i, err)
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			return fmt.Errorf("stream %d = %d, want 200", i, res.StatusCode)
		}
		live = append(live, res)
		// A viewer drains what it is given -- the browser reads its stream
		// continuously. A client that holds the body open without reading
		// fills its TCP window and parks the server's copy loop on the write
		// side, which would measure the stall, not the shutdown.
		go func() { _, _ = io.Copy(io.Discard, res.Body) }()
	}

	// The streams are live only when their ffmpeg processes exist.
	if !waitFor(60*time.Second, func() bool {
		n, err := ffmpegProcessCount()
		return err == nil && n >= baseline+count
	}) {
		return fmt.Errorf("only %d ffmpeg processes appeared (baseline %d, want +%d)",
			mustCount(), baseline, count)
	}
	now, _ := ffmpegProcessCount()
	fmt.Printf("==> %d ffmpeg processes live (baseline %d)\n", now, baseline)

	fmt.Println("==> sending the console interrupt")
	start := time.Now()
	if err := interrupt(cmd.Process.Pid); err != nil {
		return fmt.Errorf("sending the interrupt: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		running = false
		fmt.Printf("==> the server exited in %s\n", time.Since(start).Round(time.Millisecond))
	case <-time.After(60 * time.Second):
		return fmt.Errorf("the server did not exit within 60s of the interrupt")
	}

	// Every stream's encoder must be gone with it. A process that ignores a
	// kill is dead in practice only when the process list says so, which is
	// why this is checked in the list and not in the registry.
	var remaining int
	if !waitFor(15*time.Second, func() bool {
		n, err := ffmpegProcessCount()
		if err != nil {
			return false
		}
		remaining = n
		return n <= baseline
	}) {
		return fmt.Errorf("%d ffmpeg processes outlived the server (baseline %d) -- orphans",
			remaining-baseline, baseline)
	}
	fmt.Printf("==> no orphan ffmpeg (baseline %d, now %d)\n", baseline, remaining)
	fmt.Println("PASS")
	return nil
}

// interrupt asks the child's console for a Ctrl+Break, which Go delivers as
// os.Interrupt -- the same signal the console and a service manager send.
func interrupt(pid int) error {
	const ctrlBreakEvent = 1
	generate := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")
	// A child started with CREATE_NEW_PROCESS_GROUP has its own pid as the
	// process-group id, which is what the call wants.
	ret, _, callErr := generate.Call(ctrlBreakEvent, uintptr(pid))
	if ret == 0 {
		// The child joined this console's own group rather than a fresh one;
		// break the whole console, which is what closing a terminal does.
		ret, _, callErr = generate.Call(ctrlBreakEvent, 0)
		if ret == 0 {
			return callErr
		}
	}
	return nil
}

func waitHealthy(client *http.Client, base string, within time.Duration) error {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		res, err := client.Get(base + "/api/health")
		if err == nil {
			var health struct{ Status string }
			err = json.NewDecoder(res.Body).Decode(&health)
			res.Body.Close()
			if err == nil && health.Status == "ok" {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("the server never reported healthy within %s", within)
}

func waitForFilm(client *http.Client, base string, within time.Duration) (int64, error) {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		res, err := client.Get(base + "/api/library/movies")
		if err == nil {
			var page struct {
				Movies []struct {
					ID int64 `json:"id"`
				} `json:"movies"`
			}
			err = json.NewDecoder(res.Body).Decode(&page)
			res.Body.Close()
			if err == nil && len(page.Movies) > 0 {
				return page.Movies[0].ID, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0, fmt.Errorf("the scan never found the film within %s", within)
}

func statusOf(client *http.Client, url string) int {
	res, err := client.Get(url)
	if err != nil {
		return 0
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

// ffmpegProcessCount counts ffmpeg.exe in the process list. It is the
// acceptance's own instrument: the orphan has to be visible where a human
// would look for it.
func ffmpegProcessCount() (int, error) {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq ffmpeg.exe", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "ffmpeg.exe") {
			count++
		}
	}
	return count, nil
}

func mustCount() int {
	n, _ := ffmpegProcessCount()
	return n
}

func waitFor(within time.Duration, condition func() bool) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func oneLine(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
	if len(value) > 300 {
		return value[len(value)-300:]
	}
	return value
}

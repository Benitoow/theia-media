// Command create-proof-media makes a repeatable test pattern for native player
// checks using Theia's own pinned and digest-verified FFmpeg runtime.
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Benitoow/theia-media/internal/ffmpeg"
)

func main() {
	output := flag.String("out", "", "MP4 fixture to create")
	seconds := flag.Int("seconds", 20, "duration in seconds (3-120)")
	flag.Parse()
	if *output == "" || *seconds < 3 || *seconds > 120 {
		fmt.Fprintln(os.Stderr, "usage: create-proof-media -out <path.mp4> [-seconds 3..120]")
		os.Exit(2)
	}
	if err := create(*output, *seconds); err != nil {
		fmt.Fprintln(os.Stderr, "create-proof-media:", err)
		os.Exit(1)
	}
}

func create(output string, seconds int) error {
	bootstrap, err := os.MkdirTemp("", "theia-proof-ffmpeg-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(bootstrap)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	binary, err := ffmpeg.New(bootstrap, slog.Default()).Path(ctx)
	if err != nil {
		return fmt.Errorf("prepare pinned ffmpeg: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	args := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
		"-t", strconv.Itoa(seconds),
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-movflags", "+faststart", output,
	}
	command := exec.CommandContext(ctx, binary, args...)
	if body, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("encoding fixture: %w: %s", err, body)
	}
	film, err := os.ReadFile(output)
	if err != nil {
		return err
	}
	if len(film) == 0 {
		return fmt.Errorf("fixture is empty: %s", output)
	}
	fmt.Printf("proof film: %s (%d bytes, sha256:%x)\n", output, len(film), sha256.Sum256(film))
	return nil
}

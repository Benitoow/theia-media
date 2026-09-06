// Command testfixture creates synthetic, playable media in a new test directory.
// It is development tooling and is never linked into the Theia executable.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Benitoow/theia-media/internal/db"
	"github.com/Benitoow/theia-media/internal/ffmpeg"
	"github.com/Benitoow/theia-media/internal/library"
)

func main() {
	dir := flag.String("data-dir", "", "new disposable directory")
	cached := flag.String("ffmpeg", "", "optional previously downloaded pinned FFmpeg")
	flag.Parse()
	if *dir == "" {
		panic("data-dir is required")
	}
	if _, err := os.Stat(filepath.Join(*dir, db.FileName)); !os.IsNotExist(err) {
		panic("refusing to replace an existing database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	check(os.MkdirAll(filepath.Join(*dir, "bin"), 0700))
	if *cached != "" {
		b, err := os.ReadFile(*cached)
		check(err)
		check(os.WriteFile(filepath.Join(*dir, "bin", filepath.Base(*cached)), b, 0700))
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	manager := ffmpeg.New(filepath.Join(*dir, "bin"), log)
	binary, err := manager.Path(ctx)
	check(err)
	media := filepath.Join(*dir, "media")
	check(os.MkdirAll(media, 0700))
	for _, item := range []struct{ name, video, audio string }{
		{"Guard.Direct.2026.mp4", "libx264", "aac"},
		{"Guard.Converted.2026.mkv", "mpeg2video", "ac3"},
		{"Guard.Remux.2026.mkv", "libx264", "ac3"},
	} {
		args := []string{"-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-f", "lavfi", "-i", "sine=frequency=880:sample_rate=48000", "-map", "0:v", "-map", "1:a", "-map", "2:a", "-t", "45", "-c:v", item.video, "-g", "48", "-c:a", item.audio, "-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=fra", filepath.Join(media, item.name)}
		if out, err := exec.CommandContext(ctx, binary, args...).CombinedOutput(); err != nil {
			panic(fmt.Sprintf("fixture encoding: %v: %s", err, out))
		}
	}
	b, err := os.ReadFile(filepath.Join(media, "Guard.Converted.2026.mkv"))
	check(err)
	check(os.WriteFile(filepath.Join(media, "Guard.Series.S01E01.mkv"), b, 0600))
	check(os.WriteFile(filepath.Join(media, "Guard.Remux.2026.fr.srt"), []byte("1\n00:00:00,000 --> 00:00:44,000\nTheia playback guard\n"), 0600))
	entries, err := os.ReadDir(media)
	check(err)
	old := time.Now().Add(-time.Hour)
	for _, entry := range entries {
		check(os.Chtimes(filepath.Join(media, entry.Name()), old, old))
	}
	config, err := json.Marshal(map[string]any{"library_paths": []string{media}, "port": 8397, "hostname": "theia-playback-guard"})
	check(err)
	check(os.WriteFile(filepath.Join(*dir, "config.json"), config, 0600))
	database, err := db.Open(ctx, filepath.Join(*dir, db.FileName))
	check(err)
	defer database.Close()
	service := library.NewService(library.NewStore(database), nil, log)
	_, err = service.Scan(ctx, []string{media})
	check(err)
	// Stamp synthetic metadata as current (schema version 2). A published binary
	// has a TMDB key and would otherwise replace the fixture during its first scan.
	_, err = database.ExecContext(ctx, `UPDATE movies SET tmdb_id=id, tmdb_title=title, metadata_status='ok', metadata_fetched_at=?, metadata_version=2, runtime_minutes=CASE WHEN title LIKE '%Direct%' THEN 85 ELSE 135 END, overview='Synthetic media generated locally for the playback guard.', director='Theia Test Studio', genres_json='["Test"]'`, time.Now().Unix())
	check(err)
	for _, table := range []string{"series", "seasons", "episodes"} {
		_, err = database.ExecContext(ctx, "UPDATE "+table+" SET metadata_status='ok', metadata_fetched_at=?", time.Now().Unix())
		check(err)
	}
	_, err = database.ExecContext(ctx, "UPDATE series SET metadata_version=2")
	check(err)
	fmt.Println("Playable fixture ready")
}
func check(err error) {
	if err != nil {
		panic(err)
	}
}

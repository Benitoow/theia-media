package library

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	theiadb "github.com/Benitoow/theia-media/internal/db"
)

const benchmarkMovieCount = 10_000

func benchmarkStore(b *testing.B) *Store {
	b.Helper()
	database, err := theiadb.Open(context.Background(), filepath.Join(b.TempDir(), "benchmark.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = database.Close() })

	tx, err := database.Begin()
	if err != nil {
		b.Fatal(err)
	}
	statement, err := tx.Prepare(`
		INSERT INTO movies (
			path, file_name, size_bytes, modified_at, title, year,
			first_seen_scan, last_seen_scan, added_at, updated_at,
			tmdb_title, overview, poster_path, backdrop_path, director,
			genres_json, runtime_minutes, vote_average, metadata_status
		) VALUES (?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'ok')`)
	if err != nil {
		_ = tx.Rollback()
		b.Fatal(err)
	}
	for i := 0; i < benchmarkMovieCount; i++ {
		title := fmt.Sprintf("Film %05d", i)
		if i%31 == 0 {
			title = fmt.Sprintf("Heat %05d", i)
		}
		path := fmt.Sprintf("C:/media/%05d.mkv", i)
		if _, err := statement.Exec(
			path, filepath.Base(path), int64(4_000_000_000+i), int64(1_700_000_000+i),
			title, 1980+i%45, int64(1_700_000_000+i), int64(1_700_000_000+i),
			title, "A synopsis used by library cards.", "/poster.jpg", "/backdrop.jpg",
			"Director", `["Drama","Science Fiction"]`, 120, 7.5,
		); err != nil {
			_ = statement.Close()
			_ = tx.Rollback()
			b.Fatal(err)
		}
	}
	if err := statement.Close(); err != nil {
		_ = tx.Rollback()
		b.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		b.Fatal(err)
	}
	return NewStore(database)
}

func BenchmarkList500From10000(b *testing.B) {
	store := benchmarkStore(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		movies, err := store.List(context.Background(), 1, 500, 0)
		if err != nil {
			b.Fatal(err)
		}
		if len(movies) != 500 {
			b.Fatalf("got %d movies, want 500", len(movies))
		}
	}
}

func BenchmarkSearchMissFrom10000(b *testing.B) {
	store := benchmarkStore(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results, err := store.Search(context.Background(), 1, "zzzz-no-match-zzzz")
		if err != nil {
			b.Fatal(err)
		}
		if len(results.Movies) != 0 || len(results.Series) != 0 {
			b.Fatal("impossible query unexpectedly matched")
		}
	}
}

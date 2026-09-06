package imagecache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCachedArtworkSurvivesMissingMetadataClient(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "w1280"), 0700); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "w1280", "cached.jpg")
	if err := os.WriteFile(want, []byte("cached artwork"), 0600); err != nil {
		t.Fatal(err)
	}
	cache, err := New(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cache.Path(t.Context(), "w1280", "/cached.jpg")
	if err != nil || got != want {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := cache.Path(t.Context(), "w1280", "/missing.jpg"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("miss=%v", err)
	}
	if _, err := cache.Path(t.Context(), "w1280", "../cached.jpg"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("traversal=%v", err)
	}
}

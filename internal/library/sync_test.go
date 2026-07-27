package library

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Benitoow/theia-media/internal/db"
)

// newTestService builds a service over a real migrated database in a temporary
// directory, and a library root to put files in.
func newTestService(t *testing.T) (*Service, string) {
	t.Helper()

	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	root := t.TempDir()
	return NewService(NewStore(database), slog.New(slog.DiscardHandler)), root
}

// writeFile creates a file of a plausible size at a path relative to root.
func writeFile(t *testing.T, root, rel string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1024)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanIndexesVideoFiles(t *testing.T) {
	service, root := newTestService(t)

	writeFile(t, root, "The.Matrix.1999.1080p.BluRay.x264.mkv")
	writeFile(t, root, "Amelie/Amelie (2001).mp4")
	writeFile(t, root, "Le Salaire de la peur (1953).avi")

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatalf("Scan returned an unexpected error: %v", err)
	}

	if report.Found != 3 {
		t.Errorf("found = %d, want 3 (problems: %v)", report.Found, report.Problems)
	}
	if report.Added != 3 {
		t.Errorf("added = %d, want 3", report.Added)
	}
	if len(report.Problems) != 0 {
		t.Errorf("problems = %v, want none", report.Problems)
	}

	movies, err := service.List(t.Context(), 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 3 {
		t.Fatalf("listed %d films, want 3", len(movies))
	}

	// Sorted by title, so the order is deterministic.
	want := []struct {
		title string
		year  int
	}{
		{"Amelie", 2001},
		{"Le Salaire de la peur", 1953},
		{"The Matrix", 1999},
	}
	for i, w := range want {
		if movies[i].Title != w.title || movies[i].Year != w.year {
			t.Errorf("film %d = %q (%d), want %q (%d)",
				i, movies[i].Title, movies[i].Year, w.title, w.year)
		}
	}
}

func TestScanIgnoresWhatIsNotAFilm(t *testing.T) {
	service, root := newTestService(t)

	writeFile(t, root, "Real Movie (2010).mkv")
	writeFile(t, root, "poster.jpg")
	writeFile(t, root, "Real Movie (2010).srt")
	writeFile(t, root, "notes.txt")
	writeFile(t, root, ".hidden.mkv")
	writeFile(t, root, "Extras/Making Of.mkv")
	writeFile(t, root, "Sample/whatever.mkv")
	writeFile(t, root, "Real.Movie.2010.sample.mkv")
	writeFile(t, root, "Real.Movie.2010.trailer.mkv")

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatalf("Scan returned an unexpected error: %v", err)
	}
	if report.Found != 1 {
		movies, _ := service.List(t.Context(), 100, 0)
		var got []string
		for _, m := range movies {
			got = append(got, m.FileName)
		}
		t.Errorf("found = %d, want 1; indexed %v", report.Found, got)
	}
}

func TestRescanUpdatesRatherThanDuplicates(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Alien (1979).mkv")

	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	second, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}

	if second.Added != 0 || second.Updated != 1 {
		t.Errorf("second scan added=%d updated=%d, want added=0 updated=1", second.Added, second.Updated)
	}
	count, err := service.Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("library holds %d films after rescanning, want 1", count)
	}
}

func TestDeletedFilesLeaveTheLibrary(t *testing.T) {
	service, root := newTestService(t)
	writeFile(t, root, "Keeper (1980).mkv")
	gone := writeFile(t, root, "Goner (1990).mkv")

	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if report.Removed != 1 {
		t.Errorf("removed = %d, want 1", report.Removed)
	}

	movies, err := service.List(t.Context(), 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 || movies[0].Title != "Keeper" {
		t.Errorf("library = %+v, want only Keeper", movies)
	}
}

func TestAnUnreadableRootDoesNotEmptyTheLibrary(t *testing.T) {
	// The scenario this guards against: the library lives on an external drive,
	// the drive is unplugged, a scan runs, finds nothing, and helpfully deletes
	// everything the user owns. Losing the index to a loose USB cable is not a
	// recoverable mistake, so pruning is skipped whenever a root was unreadable.
	service, root := newTestService(t)
	writeFile(t, root, "Solaris (1972).mkv")

	if _, err := service.Scan(t.Context(), []string{root}); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(t.TempDir(), "unplugged-drive")
	report, err := service.Scan(t.Context(), []string{missing})
	if err != nil {
		t.Fatalf("a missing root should be reported, not returned as an error: %v", err)
	}
	if len(report.Problems) == 0 {
		t.Error("a missing root produced no problem report")
	}
	if report.Removed != 0 {
		t.Errorf("removed = %d, want 0: nothing may be pruned after a failed walk", report.Removed)
	}

	count, err := service.Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("library holds %d films, want the 1 it had before the drive vanished", count)
	}
}

func TestScanWithNoConfiguredDirectories(t *testing.T) {
	service, _ := newTestService(t)

	report, err := service.Scan(t.Context(), nil)
	if err != nil {
		t.Fatalf("Scan returned an unexpected error: %v", err)
	}
	if report.Found != 0 || report.Added != 0 {
		t.Errorf("report = %+v, want an empty one", report)
	}
	if service.LastScan() == nil {
		t.Error("LastScan() is nil after a scan with nothing to do")
	}
}

func TestBadlyNamedFilesStillLand(t *testing.T) {
	// A real library always has one of these. It must produce a row with a
	// usable title, not a constraint violation.
	service, root := newTestService(t)
	writeFile(t, root, "___.mkv")
	writeFile(t, root, "1080p.mkv")
	writeFile(t, root, "é.mkv")

	report, err := service.Scan(t.Context(), []string{root})
	if err != nil {
		t.Fatalf("Scan returned an unexpected error: %v", err)
	}
	if report.Found != 3 || report.Added != 3 {
		t.Errorf("found=%d added=%d, want 3 and 3 (problems: %v)",
			report.Found, report.Added, report.Problems)
	}

	movies, err := service.List(t.Context(), 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range movies {
		if m.Title == "" {
			t.Errorf("%s produced an empty title", m.FileName)
		}
	}
}

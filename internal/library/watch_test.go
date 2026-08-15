package library

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

// settle backdates a file so that the watcher considers it finished being
// written. Tests that want a file *indexed* have to do this, which is the
// stability rule showing through: a file touched a moment ago is not yet a film.
func settle(t *testing.T, path string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func newTestWatcher(t *testing.T) (*Watcher, *Service, string) {
	t.Helper()
	service, root := newTestService(t)
	return NewWatcher(service, []string{root}, slog.New(slog.DiscardHandler)), service, root
}

// count reads the library size straight from the database, so that a test can
// tell "the watcher scanned again" from "the watcher left things alone".
func count(t *testing.T, service *Service) int {
	t.Helper()
	n, err := service.Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAnUnchangedLibraryIsNotReconciledTwice(t *testing.T) {
	watcher, service, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "The.Matrix.1999.mkv"))

	watcher.pass(t.Context(), true)
	if got := count(t, service); got != 1 {
		t.Fatalf("after the first pass, films = %d, want 1", got)
	}

	// Emptying the table behind the watcher's back gives the next pass
	// something to restore. If it runs at all, the film comes back.
	if _, err := service.store.db.ExecContext(t.Context(), `DELETE FROM movies`); err != nil {
		t.Fatal(err)
	}

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 0 {
		t.Errorf("the watcher reconciled an unchanged library: films = %d, want 0", got)
	}
}

func TestAFileAppearingIsPickedUpWithoutAnybodyAsking(t *testing.T) {
	watcher, service, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "The.Matrix.1999.mkv"))
	watcher.pass(t.Context(), true)

	settle(t, writeFile(t, root, "Heat (1995).mkv"))

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 2 {
		t.Errorf("films = %d, want 2: the new file was not indexed", got)
	}
}

func TestAFileStillBeingWrittenIsLeftAloneUntilItSettles(t *testing.T) {
	watcher, service, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "The.Matrix.1999.mkv"))
	watcher.pass(t.Context(), true)

	// Written just now: this is what a copy in progress looks like.
	arriving := writeFile(t, root, "Heat (1995).mkv")

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 1 {
		t.Fatalf("films = %d, want 1: a file still being copied was indexed", got)
	}

	settle(t, arriving)

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 2 {
		t.Errorf("films = %d, want 2: the finished copy was never picked up", got)
	}
}

func TestAFileDisappearingIsNoticed(t *testing.T) {
	watcher, service, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "The.Matrix.1999.mkv"))
	gone := writeFile(t, root, "Heat (1995).mkv")
	settle(t, gone)
	watcher.pass(t.Context(), true)
	if got := count(t, service); got != 2 {
		t.Fatalf("films = %d, want 2", got)
	}

	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 1 {
		t.Errorf("films = %d, want 1: the deleted file was not removed", got)
	}
}

// A root that cannot be read must not read as "the library is empty now".
// scanner.Scan reports it as a problem and Service.Scan refuses to prune on a
// problem; this pins the watcher end of that contract.
func TestTheWatcherDoesNotEmptyTheLibraryWhenARootGoesAway(t *testing.T) {
	watcher, service, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "The.Matrix.1999.mkv"))
	watcher.pass(t.Context(), true)

	watcher.SetRoots([]string{root + "-unplugged"})

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 1 {
		t.Errorf("films = %d, want 1: an unreadable root deleted the library", got)
	}
}

func TestChangingTheFoldersTakesEffectWithoutARestart(t *testing.T) {
	watcher, service, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "The.Matrix.1999.mkv"))
	watcher.pass(t.Context(), true)

	second := t.TempDir()
	settle(t, writeFile(t, second, "Heat (1995).mkv"))
	watcher.SetRoots([]string{root, second})

	watcher.pass(t.Context(), false)
	if got := count(t, service); got != 2 {
		t.Errorf("films = %d, want 2: the added folder was not scanned", got)
	}
}

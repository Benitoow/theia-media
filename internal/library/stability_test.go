package library

import (
	"testing"
)

func TestFreshFileDeferredDuringStartup(t *testing.T) {
	w, s, root := newTestWatcher(t)
	writeFile(t, root, "AuditFresh.2026.mkv")
	w.pass(t.Context(), true)
	if got := count(t, s); got != 0 {
		t.Fatalf("startup indexed %d fresh file(s) inside the stability window", got)
	}
}
func TestFreshFileDeferredAlongsideSettledChange(t *testing.T) {
	w, s, root := newTestWatcher(t)
	settle(t, writeFile(t, root, "AuditOld.2024.mkv"))
	w.pass(t.Context(), true)
	settle(t, writeFile(t, root, "AuditSettled.2025.mkv"))
	writeFile(t, root, "AuditFresh.2026.mkv")
	w.pass(t.Context(), false)
	if got := count(t, s); got != 2 {
		t.Fatalf("settled change also indexed fresh file; got %d want 2", got)
	}
}

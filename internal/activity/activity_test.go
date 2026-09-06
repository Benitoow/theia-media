package activity

import "testing"

func TestInstallationAndPlaybackCannotOverlap(t *testing.T) {
	a := New()
	release, ok := a.TryInstall()
	if !ok {
		t.Fatal("idle installation refused")
	}
	if end, ok := a.TryBegin(); ok {
		end()
		t.Fatal("playback entered installation")
	}
	release()
	end, ok := a.TryBegin()
	if !ok {
		t.Fatal("failed installation did not reopen playback")
	}
	if _, ok := a.TryInstall(); ok {
		t.Fatal("installation entered playback")
	}
	end()
	end()
	if !a.Busy() {
		t.Fatal("buffered playback lost its grace period")
	}
}

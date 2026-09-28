package layout

import "testing"

// The macOS answer is the reason this package exists, so it is asserted here
// rather than only where it is used: the launcher that looked for a loose
// `theia-player` on a Mac passed every test it had, because every test it had
// ran on a platform where that is the right answer.
func TestAMacPlayerIsTheFileInsideTheBundle(t *testing.T) {
	if got, want := PlayerExecutable("darwin"), "Theia.app/Contents/MacOS/theia-player"; got != want {
		t.Errorf("the macOS player is %q, want %q", got, want)
	}
	if got, want := PlayerExecutable("windows"), "theia-player.exe"; got != want {
		t.Errorf("the Windows player is %q, want %q", got, want)
	}
	if got, want := PlayerExecutable("linux"), "theia-player"; got != want {
		t.Errorf("the Linux player is %q, want %q", got, want)
	}
}

func TestTheServerHasNoBundleOnAnyPlatform(t *testing.T) {
	if got, want := ServerExecutable("windows"), "theia-server.exe"; got != want {
		t.Errorf("the Windows server is %q, want %q", got, want)
	}
	for _, goos := range []string{"darwin", "linux", "freebsd"} {
		if got, want := ServerExecutable(goos), "theia-server"; got != want {
			t.Errorf("the %s server is %q, want %q", goos, got, want)
		}
	}
}

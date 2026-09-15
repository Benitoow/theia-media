package setup

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTheProgressBarSaysWhatItIsDoing(t *testing.T) {
	// The screen exists because a download with no sign of life cannot be told
	// from a hang. What it says is a catalogue sentence, and the program is named
	// rather than described so one sentence serves both of them.
	french, _ := CatalogueFor("fr")
	model := &progressModel{
		language: french,
		phase:    PhaseDownloading,
		detail:   "server",
		done:     24 << 20,
		total:    43 << 20,
	}
	view := model.View()
	if !strings.Contains(view, "Téléchargement : le serveur") {
		t.Errorf("the screen does not say what it is downloading:\n%s", view)
	}
	if !strings.Contains(view, "24 Mo / 43 Mo") {
		t.Errorf("the screen does not say how far along it is:\n%s", view)
	}
	if !strings.Contains(view, "55 %") {
		t.Errorf("the screen does not show the percentage:\n%s", view)
	}
	if !strings.Contains(view, french["brand"]) {
		t.Errorf("the screen does not carry the product's name:\n%s", view)
	}

	// The bar itself is the width it says it is, so a narrow console is a
	// decision rather than an accident.
	drawn := strings.Count(view, "█") + strings.Count(view, "░")
	if drawn != barWidth {
		t.Errorf("the bar is %d cells wide, want %d:\n%s", drawn, barWidth, view)
	}

	english, _ := CatalogueFor("en")
	model.language = english
	if view := model.View(); !strings.Contains(view, "Downloading the server") {
		t.Errorf("the English screen did not use the English catalogue:\n%s", view)
	}
}

func TestAPhaseWithNoCountStillSaysSomething(t *testing.T) {
	// Extraction and installation have no bytes to count, and the release page
	// answers before a size is known. None of those may leave an empty screen.
	french, _ := CatalogueFor("fr")
	for _, model := range []*progressModel{
		{language: french, phase: PhaseChecking},
		{language: french, phase: PhaseExtracting, detail: "player"},
		{language: french, phase: PhaseInstalling, detail: `C:\Users\starx\Programs\Theia`},
		{language: french, phase: PhaseDone},
		{language: french, phase: PhaseDownloading, detail: "player"},
	} {
		view := strings.TrimSpace(model.View())
		if len(view) <= len(french["brand"]) {
			t.Errorf("phase %q drew nothing but its title:\n%s", model.phase, view)
		}
	}
	// And a size that is not known yet says so rather than drawing 100 %.
	waiting := &progressModel{language: french, phase: PhaseDownloading, detail: "player"}
	if view := waiting.View(); !strings.Contains(view, french["progressWaiting"]) {
		t.Errorf("a download of unknown size did not say it was connecting:\n%s", view)
	}
}

func TestANewPhaseStartsItsOwnCount(t *testing.T) {
	// Carrying the server's bytes into the player's download would draw a bar
	// that begins nearly full and then goes backwards.
	french, _ := CatalogueFor("fr")
	model := &progressModel{language: french, phase: PhaseDownloading, detail: "server", done: 18 << 20, total: 18 << 20}
	model.Update(phaseMsg{phase: PhaseDownloading, detail: "player"})
	if model.done != 0 || model.total != 0 {
		t.Errorf("the second download started at %d of %d bytes", model.done, model.total)
	}
}

func TestAnUnknownProgramIsShownRatherThanSwallowed(t *testing.T) {
	// A missing catalogue entry must be visible, not blank: a sentence with a
	// hole in it is how a translation goes missing without anybody noticing.
	french, _ := CatalogueFor("fr")
	model := &progressModel{language: french, phase: PhaseDownloading, detail: "something-new"}
	if view := model.View(); !strings.Contains(view, "something-new") {
		t.Errorf("an unknown program name vanished from the screen:\n%s", view)
	}
}

func TestSizesAreWrittenInTheCatalogueLanguage(t *testing.T) {
	french, _ := CatalogueFor("fr")
	english, _ := CatalogueFor("en")
	cases := []struct {
		bytes   int64
		french  string
		english string
	}{
		{0, "0 Mo", "0 MB"},
		{1 << 20, "1 Mo", "1 MB"},
		{43 << 20, "43 Mo", "43 MB"},
		{2 << 30, "2,0 Go", "2.0 GB"},
		{3*(1<<30) + 512*(1<<20), "3,5 Go", "3.5 GB"},
	}
	for _, want := range cases {
		if got := humanSize(want.bytes, french); got != want.french {
			t.Errorf("humanSize(%d) = %q, want %q", want.bytes, got, want.french)
		}
		if got := humanSize(want.bytes, english); got != want.english {
			t.Errorf("humanSize(%d) in English = %q, want %q", want.bytes, got, want.english)
		}
	}
}

func TestAPipeGetsLinesInsteadOfABar(t *testing.T) {
	// Somebody redirecting the installer into a file is entitled to a file, not
	// to four hundred lines of control characters.
	french, _ := CatalogueFor("fr")
	var out bytes.Buffer
	boom := errors.New("the download failed")
	err := RunProgress(&out, french, func(_ context.Context, report Reporter) error {
		report.Phase(PhaseChecking, "server")
		report.Phase(PhaseDownloading, "server")
		report.Progress(10, 100)
		report.Phase(PhaseDownloading, "server") // the same phase and program twice
		report.Phase(PhaseDownloading, "player") // the other program: a second line
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("RunProgress returned %v, want the work's own error", err)
	}
	text := out.String()
	if strings.Contains(text, "\x1b[") {
		t.Errorf("a pipe was written control characters:\n%q", text)
	}
	if lines := strings.Count(strings.TrimSpace(text), "\n") + 1; lines != 3 {
		t.Errorf("the log has %d lines, want one per phase and per program:\n%s", lines, text)
	}
	if !strings.Contains(text, "Téléchargement : le serveur") || !strings.Contains(text, "Téléchargement : le lecteur") {
		t.Errorf("the log does not name both programs it fetched:\n%s", text)
	}
}

func TestControlCStopsTheScreen(t *testing.T) {
	french, _ := CatalogueFor("fr")
	model := &progressModel{language: french, phase: PhaseDownloading, detail: "player", done: 1 << 20, total: 43 << 20}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !model.cancel {
		t.Error("control-C did not ask for the work to stop")
	}
	if cmd == nil {
		t.Fatal("control-C left the screen up")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("control-C did not stop the program")
	}
	// Any other key is ignored: this screen has one job.
	model.cancel = false
	if _, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd != nil || model.cancel {
		t.Error("a random key did something on the progress screen")
	}
}

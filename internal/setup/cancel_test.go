package setup

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The three ways somebody leaves this installer without installing anything.
//
// The form has been tested at the model level since it was written - escape sets
// `aborted`, "no" leaves `Confirmed` false - but that is a statement about a
// field in a struct. What somebody is promised is a statement about a disk: the
// installer prints "cancelled", and the whole content of that sentence is that
// nothing was written. A sentence like that is not a message, it is a promise.
//
// So these tests call RunInteractive - the real entry point, the one main calls -
// and then count what appeared. Nothing here mocks the installer; the isolation
// is the point, and it is the same isolation the other installer tests use: a
// temporary data directory, a temporary installation directory and a redirected
// APPDATA, so the machine's own Start Menu, Desktop, logon folder and
// applications list are never in reach.

// captured is the Output RunInteractive is given.
//
// It is deliberately not a file: formSize then falls back to the width it uses
// when nobody knows better, and RunProgress takes its plain-text path, so both
// the form and the bar run with no terminal attached. That is the same shape as
// the choice on a real screen - it is the answers that decide, not the size of
// the window.
type captured struct{ bytes.Buffer }

// isolatedFor returns the options for one run and the directories that must
// still be empty afterwards.
//
// APPDATA is redirected because the installer writes the Start Menu folder, the
// Desktop entry, the logon folder entry and the applications-list registration
// under it, and a guard that touches the real ones is a guard nobody can run
// twice - or run at all without changing somebody's machine.
func isolatedFor(t *testing.T) (FormOptions, string, string, string) {
	t.Helper()
	root := t.TempDir()
	appData := filepath.Join(root, "appdata")
	if err := os.MkdirAll(appData, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", appData)
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "localappdata"))

	dataDir := filepath.Join(root, "data")
	installDir := filepath.Join(root, "programs")
	films := filepath.Join(root, "films")
	if err := os.MkdirAll(films, 0o755); err != nil {
		t.Fatal(err)
	}

	options := FormOptions{
		Output: &captured{},
		// Escape, as a terminal sends it. Every test here leaves the form the
		// same way a person does; a nil Input would leave Bubble Tea reading
		// this process's own stdin, which hangs a test suite rather than
		// cancelling anything.
		Input:      bytes.NewReader([]byte{0x1b}),
		Language:   "fr",
		DataDir:    dataDir,
		InstallDir: installDir,
		Library:    []string{films},
		Service:    true,
	}
	return options, dataDir, installDir, appData
}

// written reports everything under the given directories, so a failure can say
// what appeared rather than only that something did.
func written(t *testing.T, dirs ...string) []string {
	t.Helper()
	var found []string
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || path == dir {
				return nil
			}
			found = append(found, path)
			return nil
		})
	}
	return found
}

// TestControlCAndEscapeAreTheWayOut pins the binding the two paths below rely on.
//
// This is the fault that made the screen lie: Huh binds ctrl+c and nothing else,
// while the previous version of the screen printed "esc quitter" under the
// question - because SetHelp changes the label and not the binding. The label is
// only worth printing if the key is bound, so the binding is asserted here rather
// than assumed from the copy.
func TestControlCAndEscapeAreTheWayOut(t *testing.T) {
	keys := keyMap()
	bound := keys.Quit.Keys()
	if len(bound) == 0 {
		t.Fatal("the quit binding names no key, so neither key can leave the form")
	}
	var hasEsc, hasCtrlC bool
	for _, key := range bound {
		switch key {
		case "esc":
			hasEsc = true
		case "ctrl+c":
			hasCtrlC = true
		}
	}
	if !hasEsc {
		t.Errorf("escape is printed under the question but is not bound; bound keys are %v", bound)
	}
	if !hasCtrlC {
		t.Errorf("control-C is not bound; bound keys are %v", bound)
	}
}

// TestAnAbortedRunLeavesTheDiskAlone is the state the guard exists for.
//
// `Confirmed` starts at true - it has to, because the confirmation page proposes
// installing and a hidden default of "no" would turn a habit of pressing enter
// into a silent refusal. The consequence is that an escape on the first page
// leaves `Confirmed` true, and the only thing standing between that state and an
// installation nobody asked for is the `aborted` flag.
func TestAnAbortedRunLeavesTheDiskAlone(t *testing.T) {
	options, dataDir, installDir, appData := isolatedFor(t)
	language, _ := CatalogueFor(options.Language)

	result, err := formDefaults(options)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Confirmed {
		t.Fatal("the form no longer starts on yes, so this test would prove nothing")
	}

	model := &formModel{form: buildForm(&result, language, maxFormWidth, 14), text: func(key string) string { return language[key] }}
	if _, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc}); !model.aborted {
		t.Fatal("escape did not abort the form")
	}
	if !result.Confirmed {
		t.Fatal("escape changed the answer, so this test would prove nothing")
	}
	if found := written(t, dataDir, installDir, appData); len(found) != 0 {
		t.Errorf("an aborted form wrote %d paths, first %s", len(found), found[0])
	}
}

// TestEscapeThroughTheRealEntryPointWritesNothing runs the whole path.
//
// The test above proves the guard; this one proves the entry point reaches it,
// with the input and output a launcher would give it. Escape arrives as a byte
// on the reader, which is what a terminal sends.
func TestEscapeThroughTheRealEntryPointWritesNothing(t *testing.T) {
	options, dataDir, installDir, appData := isolatedFor(t)

	_, err := RunInteractive(options)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("RunInteractive returned %v, want ErrCancelled", err)
	}
	if found := written(t, dataDir, installDir, appData); len(found) != 0 {
		t.Errorf("escaping wrote %d paths, first %s", len(found), found[0])
	}
}

// TestTheTwoDirectoriesAreNotEvenCreated is the order the first page promises.
//
// The form says the programs come after the confirmation and never before it. A
// data directory is not a program, but it is still a trace of somebody who
// changed their mind, and an installer has no business leaving one behind.
//
// `countWritten` above would pass on a directory that was created empty, and an
// empty directory is still a directory somebody has to explain to themselves
// later. This asks the stricter question.
func TestTheTwoDirectoriesAreNotEvenCreated(t *testing.T) {
	options, dataDir, installDir, _ := isolatedFor(t)

	if _, err := RunInteractive(options); err != nil && !errors.Is(err, ErrCancelled) {
		t.Fatalf("RunInteractive: %v", err)
	}
	for _, dir := range []string{dataDir, installDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("%s exists after a cancellation (stat error %v)", dir, err)
		}
	}
}

// TestNothingIsWrittenUnderAppDataCoversTheEntries covers the four things a
// person would otherwise have to remove by hand.
//
// The Start Menu folder, the Desktop shortcut, the logon entry and the
// applications-list registration all live under APPDATA, and they are written by
// the install path rather than by the plan. A cancellation that reached them
// would be a cancellation that needed cleaning up after - and the shortcuts are
// the ones somebody actually sees.
func TestNothingIsWrittenUnderAppDataCoversTheEntries(t *testing.T) {
	options, _, _, appData := isolatedFor(t)

	if _, err := RunInteractive(options); err != nil && !errors.Is(err, ErrCancelled) {
		t.Fatalf("RunInteractive: %v", err)
	}
	if found := written(t, appData); len(found) != 0 {
		t.Errorf("a cancellation wrote %d paths under APPDATA, first %s", len(found), found[0])
	}
}

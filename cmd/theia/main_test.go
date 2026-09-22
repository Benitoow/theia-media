package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// What this program decides, and in which order, with the three things it does
// to the machine replaced by a recording. The order is the contract - a server
// started after the player has been opened is a search that cannot succeed - and
// opening real windows in a test would prove nothing anybody could read.

// event is one thing the program did, in the order it did it.
type event struct {
	kind string // "reachable", "spawn", "wait" or "open"
	name string
}

// watching replaces what this program does to the machine, and puts it back when
// the test ends. answering is what the pre-flight check finds: false means
// nothing is listening yet, which is the case that starts a server.
type watching struct {
	events    []event
	answering bool
	// openFails is the machine with no graphical session: the opener refuses,
	// which must not be an error for a command whose promise was to start the
	// server.
	openFails bool
}

func watch(t *testing.T, answering bool) *watching {
	t.Helper()
	w := &watching{answering: answering}
	previousSpawn, previousReachable, previousWait := spawn, reachable, waitReady
	previousOpen := openBrowser
	t.Cleanup(func() {
		spawn, reachable, waitReady = previousSpawn, previousReachable, previousWait
		openBrowser = previousOpen
	})

	spawn = func(path, _ string) error {
		w.events = append(w.events, event{kind: "spawn", name: filepath.Base(path)})
		return nil
	}
	reachable = func(address string) bool {
		w.events = append(w.events, event{kind: "reachable", name: address})
		return w.answering
	}
	waitReady = func(address string, _ time.Duration) bool {
		w.events = append(w.events, event{kind: "wait", name: address})
		return true
	}
	openBrowser = func(url string) error {
		w.events = append(w.events, event{kind: "open", name: url})
		if w.openFails {
			return errors.New("no graphical session")
		}
		return nil
	}
	return w
}

// order is what happened, as one readable line per step.
func (w *watching) order() []string {
	lines := make([]string, 0, len(w.events))
	for _, one := range w.events {
		lines = append(lines, one.kind+":"+one.name)
	}
	return lines
}

func (w *watching) equal(want ...string) bool {
	got := w.order()
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

// offTheMachine points this process at a throwaway data directory, so that the
// address a test sees is the default one, and so that no test reads - or
// creates - the data directory of whoever is running the suite.
func offTheMachine(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("THEIA_DATA_DIR", dir)
	return dir
}

// installation writes the programs a case needs into a directory of its own,
// which is the folder run is told to start them from.
func installation(t *testing.T, programs ...string) string {
	t.Helper()
	offTheMachine(t)
	dir := t.TempDir()
	for _, base := range programs {
		if err := os.WriteFile(filepath.Join(dir, programName(base)), []byte("MZ"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runIn(dir string, args ...string) (int, string) {
	var out bytes.Buffer
	code := run(dir, args, &out)
	return code, out.String()
}

func TestTheBareCommandStartsTheServerThenOpensThePlayer(t *testing.T) {
	dir := installation(t, "theia-server", "theia-player")
	w := watch(t, false)

	if code, output := runIn(dir); code != 0 {
		t.Fatalf("theia exited %d: %s", code, output)
	}
	// The wait belongs between the two spawns: it is what makes the player's
	// first discovery attempt find a server.
	want := []string{
		"reachable:127.0.0.1:8383",
		"spawn:" + programName("theia-server"),
		"wait:127.0.0.1:8383",
		"spawn:" + programName("theia-player"),
	}
	if !w.equal(want...) {
		t.Errorf("the order was %v, want %v", w.order(), want)
	}
}

func TestAServerThatIsAlreadyAnsweringIsNotStartedAgain(t *testing.T) {
	dir := installation(t, "theia-server", "theia-player")
	w := watch(t, true)

	if code, output := runIn(dir); code != 0 {
		t.Fatalf("theia exited %d: %s", code, output)
	}
	// One reachable check, and then only the player: a second server exits on
	// the busy port anyway, but it would do it in front of somebody.
	want := []string{"reachable:127.0.0.1:8383", "spawn:" + programName("theia-player")}
	if !w.equal(want...) {
		t.Errorf("the order was %v, want %v", w.order(), want)
	}
}

func TestTheServerCommandStartsTheServerAlone(t *testing.T) {
	dir := installation(t, "theia-server", "theia-player")
	w := watch(t, false)

	if code, output := runIn(dir, "server"); code != 0 {
		t.Fatalf("theia server exited %d: %s", code, output)
	}
	want := []string{
		"reachable:127.0.0.1:8383",
		"spawn:" + programName("theia-server"),
		"wait:127.0.0.1:8383",
	}
	if !w.equal(want...) {
		t.Errorf("the order was %v, want %v", w.order(), want)
	}
}

func TestThePlayerCommandLooksForNoServerAtAll(t *testing.T) {
	dir := installation(t, "theia-server", "theia-player")
	w := watch(t, true)

	if code, output := runIn(dir, "player"); code != 0 {
		t.Fatalf("theia player exited %d: %s", code, output)
	}
	// Nothing is probed and nothing is waited for: `player` is one window, and a
	// player-only machine has no local server to find.
	if want := []string{"spawn:" + programName("theia-player")}; !w.equal(want...) {
		t.Errorf("the order was %v, want %v", w.order(), want)
	}
}

func TestABareCommandOnAOneProgramMachineOpensThatOne(t *testing.T) {
	for _, only := range []string{"theia-server", "theia-player"} {
		dir := installation(t, only)
		w := watch(t, false)

		if code, output := runIn(dir); code != 0 {
			t.Fatalf("theia with only %s exited %d: %s", only, code, output)
		}
		spawned := 0
		for _, one := range w.events {
			if one.kind != "spawn" {
				continue
			}
			spawned++
			if one.name != programName(only) {
				t.Errorf("theia with only %s also started %s", only, one.name)
			}
		}
		if spawned != 1 {
			t.Errorf("theia with only %s started %d programs: %v", only, spawned, w.order())
		}
	}
}

// A machine with a server and no player is the cupboard, the NAS, and the Mac
// whose player has not been installed yet. Starting the server and saying
// nothing was a command that looked like it had done nothing; the web interface
// is what a browser can reach, and the address is printed before anything is
// opened because on a machine with no graphical session the opener is what
// fails.
func TestAServerWithNoPlayerOpensTheWebInterface(t *testing.T) {
	dir := installation(t, "theia-server")
	w := watch(t, false)

	code, output := runIn(dir)
	if code != 0 {
		t.Fatalf("theia exited %d: %s", code, output)
	}
	want := []string{
		"reachable:127.0.0.1:8383",
		"spawn:" + programName("theia-server"),
		"wait:127.0.0.1:8383",
		"open:http://127.0.0.1:8383/",
	}
	if !w.equal(want...) {
		t.Errorf("the order was %v, want %v", w.order(), want)
	}
	if !strings.Contains(output, "http://127.0.0.1:8383/") {
		t.Errorf("the address was not printed: %q", output)
	}
}

// And an opener that refuses is not a failure: the server is up, which is what
// the command promised, and the printed address is then the whole answer.
func TestACommandWhoseBrowserRefusesStillSucceeds(t *testing.T) {
	dir := installation(t, "theia-server")
	w := watch(t, true)
	w.openFails = true

	code, output := runIn(dir)
	if code != 0 {
		t.Fatalf("theia exited %d with no browser available: %s", code, output)
	}
	if !strings.Contains(output, "http://127.0.0.1:8383/") {
		t.Errorf("the address was not printed: %q", output)
	}
}

// A machine that has the player opens the player and nothing else: a browser
// beside it would be a second window nobody asked for.
func TestAMachineWithAPlayerOpensNoBrowser(t *testing.T) {
	for _, args := range [][]string{nil, {"player"}} {
		dir := installation(t, "theia-server", "theia-player")
		w := watch(t, true)

		if code, output := runIn(dir, args...); code != 0 {
			t.Fatalf("theia %v exited %d: %s", args, code, output)
		}
		for _, one := range w.events {
			if one.kind == "open" {
				t.Errorf("theia %v opened a browser at %s", args, one.name)
			}
		}
	}
}

// `theia server` is the headless form: whoever typed it is either in a terminal
// or starting a service, and neither wants a browser on that machine.
func TestTheServerVerbOpensNoBrowser(t *testing.T) {
	dir := installation(t, "theia-server")
	w := watch(t, true)

	if code, output := runIn(dir, "server"); code != 0 {
		t.Fatalf("theia server exited %d: %s", code, output)
	}
	for _, one := range w.events {
		if one.kind == "open" {
			t.Errorf("theia server opened a browser at %s", one.name)
		}
	}
}

func TestAMachineWithNeitherProgramSaysSoRatherThanDoingNothing(t *testing.T) {
	dir := installation(t)
	w := watch(t, false)

	code, output := runIn(dir)
	if code != 1 {
		t.Errorf("theia on a machine with nothing installed exited %d, want 1", code)
	}
	for _, want := range []string{programName("theia-server"), programName("theia-player")} {
		if !strings.Contains(output, want) {
			t.Errorf("the explanation does not name %s: %q", want, output)
		}
	}
	if len(w.events) != 0 {
		t.Errorf("nothing was installed, yet the program did %v", w.order())
	}
}

func TestAVerbForAProgramThatIsNotInstalledIsRefused(t *testing.T) {
	for _, one := range []struct {
		verb    string
		missing string
	}{
		{"server", "theia-server"},
		{"player", "theia-player"},
	} {
		dir := installation(t) // neither program is installed
		w := watch(t, false)

		code, output := runIn(dir, one.verb)
		if code != 1 {
			t.Errorf("theia %s exited %d, want 1", one.verb, code)
		}
		if !strings.Contains(output, programName(one.missing)) {
			t.Errorf("the refusal of %s does not name it: %q", one.verb, output)
		}
		if len(w.events) != 0 {
			t.Errorf("theia %s did something anyway: %v", one.verb, w.order())
		}
	}
}

func TestACommandThatDoesNotExistIsRefusedWithTheUsage(t *testing.T) {
	dir := installation(t, "theia-server")
	for _, args := range [][]string{{"install"}, {"server", "player"}} {
		code, output := runIn(dir, args...)
		if code != 2 {
			t.Errorf("theia %v exited %d, want 2", args, code)
		}
		if !strings.Contains(output, "theia server") || !strings.Contains(output, "theia player") {
			t.Errorf("the refusal of %v does not print the usage: %q", args, output)
		}
	}
}

func TestTheVersionIsTheBuildThisProgramCameFrom(t *testing.T) {
	offTheMachine(t)
	code, output := runIn(t.TempDir(), "-version")
	if code != 0 {
		t.Fatalf("theia -version exited %d", code)
	}
	if strings.TrimSpace(output) != "theia "+version {
		t.Errorf("theia -version printed %q, want %q", output, "theia "+version)
	}
}

func TestTheHelpPrintsWhatTheCommandsAre(t *testing.T) {
	offTheMachine(t)
	code, output := runIn(t.TempDir(), "-help")
	if code != 0 {
		t.Fatalf("theia -help exited %d", code)
	}
	for _, want := range []string{"theia server", "theia player", "theia -version"} {
		if !strings.Contains(output, want) {
			t.Errorf("the usage does not mention %q: %q", want, output)
		}
	}
}

func TestAServerThatDoesNotComeUpIsSaidOutLoudWithoutStoppingThePlayer(t *testing.T) {
	dir := installation(t, "theia-server", "theia-player")
	w := watch(t, false)
	waitReady = func(address string, _ time.Duration) bool {
		w.events = append(w.events, event{kind: "wait", name: address})
		return false
	}

	code, output := runIn(dir)
	if code != 0 {
		t.Fatalf("theia exited %d: %s", code, output)
	}
	if !strings.Contains(output, "not answering") {
		t.Errorf("the timeout was swallowed: %q", output)
	}
	want := []string{
		"reachable:127.0.0.1:8383",
		"spawn:" + programName("theia-server"),
		"wait:127.0.0.1:8383",
		"spawn:" + programName("theia-player"),
	}
	if !w.equal(want...) {
		t.Errorf("the player was not opened after the server failed: %v", w.order())
	}
}

func TestAProgramThatRefusesToStartIsReported(t *testing.T) {
	dir := installation(t, "theia-server")
	w := watch(t, false)
	spawn = func(path, _ string) error {
		w.events = append(w.events, event{kind: "spawn", name: filepath.Base(path)})
		return errors.New("access is denied")
	}

	code, output := runIn(dir, "server")
	if code != 1 {
		t.Errorf("a server that could not start exited %d, want 1", code)
	}
	if !strings.Contains(output, "access is denied") {
		t.Errorf("the failure was not explained: %q", output)
	}
}

func TestTheAddressIsThePortTheServerWasConfiguredWith(t *testing.T) {
	dir := installation(t, "theia-server")
	if err := os.WriteFile(filepath.Join(os.Getenv("THEIA_DATA_DIR"), "config.json"), []byte(`{"port": 9001}`), 0o644); err != nil {
		t.Fatal(err)
	}
	w := watch(t, false)

	if code, output := runIn(dir, "server"); code != 0 {
		t.Fatalf("theia server exited %d: %s", code, output)
	}
	want := []string{"reachable:127.0.0.1:9001", "spawn:" + programName("theia-server"), "wait:127.0.0.1:9001"}
	if !w.equal(want...) {
		t.Errorf("the order was %v, want %v", w.order(), want)
	}
}

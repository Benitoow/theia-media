package setup

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The startup entries on macOS and Linux, as far as a Windows machine can check
// them.
//
// service_unix.go is compiled on Unix only, so the suite that runs here cannot
// call writeLaunchAgent or writeSystemdUnit at all - and a claim about a launchd
// agent that nothing on this machine can reach is a claim nobody has checked.
// What it *can* check is the half that decides whether the agent works: which
// path it is written to, and what is in it. Those live in autostart.go for
// exactly this reason, and the two things left in service_unix.go are the file
// write (shared, and covered here) and `launchctl`/`systemctl`, which only a Mac
// and a Linux machine can confirm.

// plist is enough of a property list to read the agent back: keys and their
// values in document order, plus the nested dictionary KeepAlive is.
type plist struct {
	XMLName xml.Name  `xml:"plist"`
	Dict    plistDict `xml:"dict"`
}

type plistDict struct {
	Keys    []string    `xml:"key"`
	Strings []string    `xml:"string"`
	Trues   []string    `xml:"true"`
	Arrays  []plistArr  `xml:"array"`
	Nested  []plistDict `xml:"dict"`
}

type plistArr struct {
	Values []string `xml:"string"`
}

// TestTheLaunchdAgentSaysWhatTheInstallationDoes reads the agent as XML rather
// than as a string: what matters is not the whitespace but that launchd would
// find a Label, an argument vector ending at the server with this installation's
// data directory, and the two keys that make it start and stay started.
func TestTheLaunchdAgentSaysWhatTheInstallationDoes(t *testing.T) {
	home := t.TempDir()
	install := filepath.Join(home, ".local", "lib", "theia")
	plan := Plan{Role: RoleServer, DataDir: filepath.Join(home, "Theia"), InstallDir: install}
	server := filepath.Join(install, "theia-server")

	path := launchAgentPathIn(home)
	if want := filepath.Join(home, "Library", "LaunchAgents", "media.theia.server.plist"); path != want {
		t.Errorf("the agent is written to %q, want %q", path, want)
	}

	var agent plist
	if err := xml.Unmarshal([]byte(launchAgentPlist(launchCommand(plan, server))), &agent); err != nil {
		t.Fatalf("the agent is not a property list: %v", err)
	}
	keys := strings.Join(agent.Dict.Keys, ",")
	if keys != "Label,ProgramArguments,RunAtLoad,KeepAlive" {
		t.Errorf("the agent's keys are %q", keys)
	}
	// The Label is the first value, and the arguments are the array beside it.
	if len(agent.Dict.Strings) == 0 || agent.Dict.Strings[0] != launchAgentLabel {
		t.Errorf("the agent's label is %v", agent.Dict.Strings)
	}
	if len(agent.Dict.Arrays) != 1 {
		t.Fatalf("the agent has %d argument arrays: %+v", len(agent.Dict.Arrays), agent.Dict)
	}
	args := agent.Dict.Arrays[0].Values
	want := launchCommand(plan, server)
	if len(args) != len(want) {
		t.Fatalf("the agent starts %v, want %v", args, want)
	}
	for index := range want {
		if args[index] != want[index] {
			t.Fatalf("the agent starts %v, want %v", args, want)
		}
	}
	// A plist `<true/>` has no content, so it reads back as an empty value; what
	// is asserted is that the key is there with a true beside it.
	if len(agent.Dict.Trues) != 1 {
		t.Errorf("RunAtLoad is not true in %+v", agent.Dict)
	}
	// KeepAlive with SuccessfulExit false: restarted after a crash, left alone
	// after a clean exit, which is the difference between a service and a loop.
	if len(agent.Dict.Nested) != 1 || strings.Join(agent.Dict.Nested[0].Keys, ",") != "SuccessfulExit" {
		t.Errorf("KeepAlive is %+v", agent.Dict.Nested)
	}
}

// TestTheSystemdUnitSaysTheSameThingAsTheAgent pins the two Unix entries to one
// argument vector: a unit that started something else would be a second machine
// configuration nobody wrote down.
func TestTheSystemdUnitSaysTheSameThingAsTheAgent(t *testing.T) {
	home := t.TempDir()
	plan := Plan{Role: RoleServer, DataDir: filepath.Join(home, "Theia"), InstallDir: filepath.Join(home, "progs")}
	server := filepath.Join(plan.InstallDir, "theia-server")

	if want := filepath.Join(home, ".config", "systemd", "user", "theia.service"); systemdUnitPathIn(home) != want {
		t.Errorf("the unit is written to %q, want %q", systemdUnitPathIn(home), want)
	}
	unit := systemdUnit(launchCommand(plan, server))
	for _, line := range []string{
		"ExecStart=" + server + " --data-dir " + plan.DataDir,
		"Restart=on-failure",
		"WantedBy=default.target",
	} {
		if !strings.Contains(unit, line) {
			t.Errorf("the unit does not say %q:\n%s", line, unit)
		}
	}
}

// TestTheStartupEntryLandsWhereTheSystemReadsIt writes the agent the way
// service_unix.go writes it, under a home directory that is thrown away: the path
// and the content are what the platform half of `launchctl load` depends on.
func TestTheStartupEntryLandsWhereTheSystemReadsIt(t *testing.T) {
	home := t.TempDir()
	plan := Plan{Role: RoleServer, DataDir: filepath.Join(home, "Theia")}
	server := filepath.Join(home, ".local", "lib", "theia", "theia-server")
	path := launchAgentPathIn(home)

	if err := writeAutostartFile(path, launchAgentPlist(launchCommand(plan, server))); err != nil {
		t.Fatalf("writing the agent: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the agent was not written where launchd looks for it: %v", err)
	}
	if !strings.Contains(string(body), "<string>"+server+"</string>") {
		t.Errorf("the agent does not start %s:\n%s", server, body)
	}
	// The folder between the home directory and the file is created by the
	// writer, because on a machine that has never had one it does not exist.
	if !isDirectory(filepath.Join(home, "Library", "LaunchAgents")) {
		t.Error("the folder launchd reads was not created")
	}
}

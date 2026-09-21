// Command theia starts the product: the server, the player, or both.
//
// The three programs live side by side in one directory, and this is the name a
// person types: `theia` brings up what the machine is for, `theia server` and
// `theia player` bring up one half of it. The raw executables stay reachable
// under their own names - the server prints its log in the console somebody
// opens deliberately - and this program deliberately has no console at all.
//
// **Why it exists.** On a machine that has both halves, opening the player while
// the server is down shows a search that cannot succeed: the player finds a
// server, it does not own one. So the product's own entry point starts the server
// first, waits for it to answer, and only then opens the player. On a machine
// that has one half, it opens that half - what is installed decides, rather than
// a role somebody has to remember.
//
// **Why it is built windowed.** With `-H=windowsgui` a program started from the
// Start Menu or the Desktop opens no window of its own; a console build would
// flash a black rectangle every time somebody clicked Theia, and internal/setup
// points the product's entry here. It borrows the terminal it was started from
// when there is one, so `theia -version` prints where somebody typed it, and says
// nothing at all when there is not - which is what a double-click deserves.
package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Benitoow/theia-media/internal/config"
)

// version is overwritten at build time with -ldflags "-X main.version=v1.2.3",
// exactly like the server's and the installer's.
var version = "dev"

// usage is what -help prints, and what an unknown command prints instead of
// leaving somebody with a cold prompt. English, like every other internal
// string: the two interfaces own the words a viewer reads (CLAUDE.md).
const usage = `theia starts Theia.

  theia           start the server if it is not answering, then open the player
  theia server    start the server alone
  theia player    open the player alone
  theia -version  print the version and exit

The programs are started from the folder this command was installed in.
`

// serverWait is how long `theia` waits for a server it started to answer before
// opening the player anyway. A server that is already up costs nothing: the
// check in ensureServer answers on the first try and nothing is spawned.
const serverWait = 8 * time.Second

// The three things this program does to the machine, as variables so that a test
// can watch them without starting anything. They are the whole of its behaviour
// besides deciding what to start and in which order.
var (
	spawn     = spawnDetached
	reachable = portAnswers
	waitReady = waitForPort
)

func main() {
	out := console()
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(out, "theia: locating this command: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(filepath.Dir(self), os.Args[1:], out))
}

// run is the whole command line.
//
// The directory is an argument rather than something looked up in here, so that
// a test can drive the real decisions against an installation it built in a
// temporary directory, and so that the one place this program decides where its
// siblings are is its own main.
func run(dir string, args []string, out io.Writer) int {
	if len(args) > 1 {
		return refuse(out, "expected at most one command")
	}
	verb := ""
	if len(args) == 1 {
		verb = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch verb {
	case "":
		return launch(dir, request{bare: true}, out)
	case "server":
		return launch(dir, request{server: true}, out)
	case "player":
		return launch(dir, request{player: true}, out)
	case "-version", "--version", "version":
		fmt.Fprintln(out, "theia", version)
		return 0
	case "-help", "--help", "-h", "help":
		fmt.Fprint(out, usage)
		return 0
	default:
		return refuse(out, fmt.Sprintf("unknown command %q", args[0]))
	}
}

// request is what the command line asked for. A bare `theia` starts whatever
// this machine has; a verb starts that half and refuses when it is not there.
type request struct {
	bare   bool
	server bool
	player bool
}

// launch starts what was asked for, from the folder this command lives in -
// which is where the installer put its siblings.
func launch(dir string, want request, out io.Writer) int {
	server := filepath.Join(dir, programName("theia-server"))
	player := filepath.Join(dir, programName("theia-player"))
	hasServer, hasPlayer := isFile(server), isFile(player)

	switch {
	case want.bare && !hasServer && !hasPlayer:
		return fail(out, fmt.Errorf("neither %s nor %s is installed beside this command",
			filepath.Base(server), filepath.Base(player)))
	case want.server && !hasServer:
		return fail(out, fmt.Errorf("%s is not installed beside this command", filepath.Base(server)))
	case want.player && !hasPlayer:
		return fail(out, fmt.Errorf("%s is not installed beside this command", filepath.Base(player)))
	}

	if hasServer && (want.bare || want.server) {
		if err := ensureServer(server, out); err != nil {
			return fail(out, err)
		}
	}
	if hasPlayer && (want.bare || want.player) {
		if err := spawn(player, dir); err != nil {
			return fail(out, fmt.Errorf("starting %s: %w", filepath.Base(player), err))
		}
	}
	return 0
}

// ensureServer starts the server unless something is already answering on its
// port, and comes back once it answers - or once it is clear that it will not.
func ensureServer(server string, out io.Writer) error {
	address := serverAddress()
	if reachable(address) {
		return nil
	}
	if err := spawn(server, filepath.Dir(server)); err != nil {
		return fmt.Errorf("starting %s: %w", filepath.Base(server), err)
	}
	// The player is opened next, and a moment spent here is the difference
	// between its first discovery attempt finding the server and showing a
	// search box. The wait is bounded: a server that cannot start is not this
	// program's failure to report twice - the player says what it finds.
	if !waitReady(address, serverWait) {
		fmt.Fprintf(out, "theia: %s is not answering on %s yet\n", filepath.Base(server), address)
	}
	return nil
}

// serverAddress is where the server listens: the port it was configured with, on
// the loopback address. The server binds every interface, so the loopback
// address answers whenever it is up, whatever the machine's hostname is.
func serverAddress() string {
	port := config.DefaultPort
	if dir, err := config.DataDir(); err == nil {
		// Load creates the data directory when it is absent. That is what the
		// server would do a moment later anyway, and reading the port from
		// somewhere else would be a second way to decide where the port lives.
		if cfg, err := config.Load(dir); err == nil && cfg.Port != 0 {
			port = cfg.Port
		}
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
}

// portAnswers reports whether something accepts a connection there.
func portAnswers(address string) bool {
	connection, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		return false
	}
	connection.Close()
	return true
}

// waitForPort polls the address until it answers or the time runs out.
func waitForPort(address string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if portAnswers(address) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// programName is what a program is called on this platform.
func programName(base string) string { return base + exeSuffix }

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// refuse answers a command line that asks for something this program does not
// do. The usage goes with it: a person who typed the wrong word is one line away
// from the right one.
func refuse(out io.Writer, reason string) int {
	fmt.Fprintf(out, "theia: %s\n\n%s", reason, usage)
	return 2
}

func fail(out io.Writer, err error) int {
	fmt.Fprintf(out, "theia: %v\n", err)
	return 1
}

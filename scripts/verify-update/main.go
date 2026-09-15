// Command verify-update drives two real Theia binaries through install,
// restart, health commit and automatic rollback against a local release stub.
//
// It also verifies the rename in decision 119, which is the reason the release
// stub advertises *both* asset names. An installed v3.2 carries the old updater
// and looks for `theia-<os>-<arch>`; V3.3 looks for `theia-server-<os>-<arch>`.
// Give it a real v3.2 binary and it walks the transitional path:
//
//	go run ./scripts/verify-update -from <a v3.2.0 executable>
//
// Without `-from` it builds this tree as the starting point, which is the
// ordinary update every later release performs.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	fromVersion = "3.2.0"
	toVersion   = "3.2.1"
)

type processLog struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	pid    int
}

var pidPattern = regexp.MustCompile(`\bpid=(\d+)`)

func (l *processLog) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = os.Stdout.Write(data)
	_, _ = l.buffer.Write(data)
	for _, match := range pidPattern.FindAllSubmatch(l.buffer.Bytes(), -1) {
		if value, err := strconv.Atoi(string(match[1])); err == nil {
			l.pid = value
		}
	}
	return len(data), nil
}

func (l *processLog) latestPID() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pid
}

func (l *processLog) contains(fragment string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Contains(l.buffer.String(), fragment)
}

func main() {
	fromBinary := flag.String("from", "", "a v3.2-era executable to update from, instead of building this tree")
	noTransitional := flag.Bool("no-transitional", false,
		"publish only the renamed asset, as the release after the first V3.3 will: a v3.2 installation must then fail to find anything")
	flag.Parse()

	root, err := os.Getwd()
	check(err)
	for _, path := range []string{
		filepath.Join(root, "go.mod"),
		filepath.Join(root, "cmd", "theia-server", "main.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			check(fmt.Errorf("run from the repository root: %w", err))
		}
	}

	temporary, err := os.MkdirTemp("", "theia-update-e2e-*")
	check(err)
	defer os.RemoveAll(temporary)

	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goBinary += ".exe"
	}
	for _, scenario := range []struct {
		name      string
		unhealthy bool
	}{
		{name: "healthy restart"},
		{name: "unhealthy restart rollback", unhealthy: true},
	} {
		fmt.Printf("\n== %s ==\n", scenario.name)
		check(runScenario(root, goBinary, filepath.Join(temporary, strings.ReplaceAll(scenario.name, " ", "-")), scenario.unhealthy, *fromBinary, *noTransitional))
	}
	fmt.Println("\nPASS: verified install, real restart, health commit and automatic rollback")
}

func runScenario(root, goBinary, dir string, unhealthy bool, fromBinary string, noTransitional bool) error {
	installDir := filepath.Join(dir, "install")
	dataDir := filepath.Join(dir, "data")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	executable := filepath.Join(installDir, "theia")
	target := filepath.Join(dir, "target")
	if runtime.GOOS == "windows" {
		executable += ".exe"
		target += ".exe"
	}
	if fromBinary != "" {
		// A real installation of the previous line: copied in, not rebuilt, so
		// the updater doing the looking is the one that shipped.
		data, err := os.ReadFile(fromBinary)
		if err != nil {
			return fmt.Errorf("reading the binary to update from: %w", err)
		}
		if err := os.WriteFile(executable, data, 0o755); err != nil {
			return err
		}
		fmt.Printf("updating from %s (built elsewhere)\n", fromBinary)
	} else if err := build(root, goBinary, executable, fromVersion, ""); err != nil {
		return err
	}
	override := ""
	if unhealthy {
		override = "deliberate-e2e-health-mismatch"
	}
	if err := build(root, goBinary, target, toVersion, override); err != nil {
		return err
	}

	digest, size, err := fileDigest(target)
	if err != nil {
		return err
	}
	var releaseServer *httptest.Server
	releaseServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			// Both names, as the first V3.3 release publishes them: the artifact
			// renamed by decision 119, and a byte-identical transitional copy
			// under the name an installed v3.2 asks for. GitHub reports a digest
			// per asset, and both carry the same one because they are the same
			// bytes.
			assets := []map[string]any{}
			names := []string{assetName()}
			if !noTransitional {
				names = append(names, legacyAssetName())
			}
			for _, name := range names {
				assets = append(assets, map[string]any{
					"name": name, "size": size,
					"browser_download_url": releaseServer.URL + "/download",
					"digest":               "sha256:" + digest,
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v" + toVersion,
				"html_url": releaseServer.URL + "/release",
				"assets":   assets,
			})
		case r.URL.Path == "/download":
			http.ServeFile(w, r, target)
		default:
			http.NotFound(w, r)
		}
	}))
	defer releaseServer.Close()

	port, err := freePort()
	if err != nil {
		return err
	}
	logs := &processLog{}
	command := exec.Command(executable, "-data-dir", dataDir, "-port", strconv.Itoa(port))
	command.Dir = installDir
	command.Env = append(os.Environ(), "THEIA_UPDATE_API="+releaseServer.URL)
	command.Stdout = logs
	command.Stderr = logs
	if err := command.Start(); err != nil {
		return err
	}
	defer stopProcesses(command.Process, logs)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	if err := waitForVersion(baseURL, fromVersion, 20*time.Second); err != nil {
		return fmt.Errorf("initial server: %w", err)
	}
	response, err := http.Post(baseURL+"/api/update/apply", "application/json", nil)
	if err != nil {
		return fmt.Errorf("applying update: %w", err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		return readErr
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("apply status %d: %s", response.StatusCode, body)
	}

	want := toVersion
	if unhealthy {
		// The apply response precedes the intentional 1.5 s restart delay. Do not
		// mistake the still-running source process for a successful rollback: the
		// unhealthy target must have started before the old version is acceptable.
		if err := waitForLog(logs, "version="+toVersion, 20*time.Second); err != nil {
			return err
		}
		want = fromVersion
	}
	if err := waitForVersion(baseURL, want, 35*time.Second); err != nil {
		return fmt.Errorf("replacement server: %w", err)
	}
	if err := assertInstalledVersion(executable, want); err != nil {
		return err
	}
	if _, err := os.Stat(executable + ".old"); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("previous executable still present after final health state: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "update-recovery", "pending.json")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("recovery journal still present after final health state: %v", err)
	}
	if unhealthy {
		if _, err := os.Stat(executable + ".failed-update"); err != nil {
			return fmt.Errorf("failed target was not preserved for diagnosis: %w", err)
		}
	}
	fmt.Printf("verified final version %s on port %d\n", want, port)
	return nil
}

func waitForLog(logs *processLog, fragment string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if logs.contains(fragment) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("process log never contained %q", fragment)
}

func build(root, goBinary, output, version, healthOverride string) error {
	flags := "-X main.version=" + version
	if healthOverride != "" {
		flags += " -X main.healthExpectationOverride=" + healthOverride
	}
	command := exec.Command(goBinary, "build", "-trimpath", "-ldflags", flags, "-o", output, "./cmd/theia-server")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("building fixture: %w\n%s", err, output)
	}
	return nil
}

func fileDigest(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), size, err
}

// assetName is what the release workflow names the artifact for a platform. It
// has to match internal/updater's own copy exactly, or this verifies an update
// nobody's binary can perform.
func assetName() string {
	return releaseName("theia-server")
}

// legacyAssetName is the pre-V3.3 name. The release workflow publishes it beside
// the new one for one release - decision 119 - so that an installed v3.2, whose
// updater cannot be changed, still has something to find.
func legacyAssetName() string {
	return releaseName("theia")
}

func releaseName(prefix string) string {
	name := prefix + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForVersion(baseURL, version string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 750 * time.Millisecond}
	var last string
	for time.Now().Before(deadline) {
		response, err := client.Get(baseURL + "/api/health")
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			last = string(body)
			var health struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			}
			if response.StatusCode == http.StatusOK && json.Unmarshal(body, &health) == nil &&
				health.Status == "ok" && strings.TrimPrefix(health.Version, "v") == version {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("version %s did not become healthy; last response %q", version, last)
}

func assertInstalledVersion(path, want string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("running installed binary: %w", err)
	}
	if !strings.Contains(string(output), want) {
		return fmt.Errorf("installed binary reports %q, want %s", output, want)
	}
	return nil
}

func stopProcesses(initial *os.Process, logs *processLog) {
	if pid := logs.latestPID(); pid > 0 {
		if process, err := os.FindProcess(pid); err == nil {
			_ = process.Kill()
			_, _ = process.Wait()
		}
	}
	if initial != nil {
		_ = initial.Kill()
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

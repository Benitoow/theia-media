package setup

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Benitoow/theia-media/internal/updater"
)

// UpdateStatusView is what this tool can say about updates, flattened.
//
// It mirrors the updater's own status rather than embedding it, because the
// setup tool reports to a terminal and to scripts, and neither should have to
// know the server's internal type to read one field.
type UpdateStatusView struct {
	State     string `json:"state"`
	Current   string `json:"current_version"`
	Latest    string `json:"latest_version,omitempty"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Message   string `json:"message,omitempty"`
	ExecPath  string `json:"exec_path,omitempty"`
}

// UpdateTarget is the installation an update would replace.
type UpdateTarget struct {
	// ExecPath is the server binary on disk, resolved the same way the autostart
	// entry resolves it.
	ExecPath string

	// Version is what the running installation reports. "dev" is honest for a
	// build made from a working tree: every release is newer than it.
	Version string
}

// ResolveUpdateTarget finds the server binary to update, and asks it for the
// version it carries rather than assuming one.
//
// Asking the binary is the point: an installation directory can hold a binary
// from any build, and a tool that reported the version it *expected* would be
// wrong exactly when somebody needs it to be right.
func ResolveUpdateTarget() (UpdateTarget, error) {
	path, err := serverExecutable()
	if err != nil {
		return UpdateTarget{}, err
	}
	version, err := binaryVersion(path)
	if err != nil {
		return UpdateTarget{}, err
	}
	return UpdateTarget{ExecPath: path, Version: version}, nil
}

// CheckForUpdate asks GitHub Releases what the current version is. It downloads
// nothing.
func CheckForUpdate(ctx context.Context, target UpdateTarget) (UpdateStatusView, error) {
	instance := newUpdater(target)
	status, err := instance.Check(ctx)
	if err != nil {
		return UpdateStatusView{}, err
	}
	return view(status, target.ExecPath), nil
}

// ApplyUpdate downloads the release for this platform, verifies the digest
// GitHub reports for it, smoke-tests it and swaps it into place.
//
// This is the server's own updater, called by a tool that is not the server: the
// verification is therefore the same code that has been driven end to end by
// scripts/verify-update, rather than a second implementation that would have to
// earn that trust again.
func ApplyUpdate(ctx context.Context, target UpdateTarget) (UpdateStatusView, error) {
	instance := newUpdater(target)
	if err := instance.Apply(ctx); err != nil {
		return view(instance.Status(), target.ExecPath), err
	}
	return view(instance.Status(), target.ExecPath), nil
}

func newUpdater(target UpdateTarget) *updater.Updater {
	return updater.New(updater.Options{
		Repo:     updater.DefaultRepo,
		Version:  target.Version,
		ExecPath: target.ExecPath,
		Logger:   slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		// The same hook the server uses, so a mirror can be pointed at, and so
		// the whole cycle can be exercised against a local stub.
		APIBase: os.Getenv("THEIA_UPDATE_API"),
		// Nothing to restart: this tool is not the server. The new binary is in
		// place, and whatever starts the server - a service entry, a terminal, a
		// person - will pick it up. Saying so is the caller's job.
		Restart: func() {},
	})
}

// binaryVersion asks a Theia binary what version it carries.
//
// `-version` prints "theia <version>": the first word is the product, which is
// not the answer to the question, so it is dropped rather than reported as part
// of a version string.
func binaryVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := runBinary(ctx, path, "-version")
	if err != nil {
		return "", fmt.Errorf("asking %s for its version: %w", filepath.Base(path), err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("%s printed no version", filepath.Base(path))
	}
	if len(fields) == 1 {
		return fields[0], nil
	}
	return fields[1], nil
}

// runBinary runs an installation's own binary and returns its standard output.
// Used to ask a binary what it is rather than to trust what a directory says.
func runBinary(ctx context.Context, path string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("%w: %s", err, message)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func view(status updater.Status, execPath string) UpdateStatusView {
	return UpdateStatusView{
		State:     string(status.State),
		Current:   status.CurrentVersion,
		Latest:    status.LatestVersion,
		Available: status.Available,
		Reason:    string(status.Reason),
		Message:   status.Message,
		ExecPath:  execPath,
	}
}

// platformLabel names this machine the way the release assets do, which is how
// the updater selects a binary. Exported for the status output, where "which
// build would I get" is a fair question.
func platformLabel() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

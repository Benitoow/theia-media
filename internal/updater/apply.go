package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// oldSuffix marks the outgoing binary. Windows will not let a running
// executable be deleted, but it will happily let it be renamed, so the previous
// version sits beside the new one until the next start clears it away. It is
// also the manual way back if a new version turns out to be broken.
const oldSuffix = ".old"

// Apply downloads the latest release and puts it in place.
//
// Ordering is the whole design. Everything that can fail happens first, against
// a temporary file; the running executable is not touched until a verified,
// checksummed, actually-executed binary is sitting next to it. Any failure
// before that point leaves the installation exactly as it was.
func (u *Updater) Apply(ctx context.Context) error {
	if !u.Supported() {
		return fmt.Errorf("updater: this build does not update itself")
	}
	if err := u.claim(); err != nil {
		return err
	}
	defer u.release()

	// Rule two: never mid-film. Checked here rather than only at the caller,
	// because another device may have started watching since the button was
	// pressed.
	if u.activity != nil && u.activity.Busy() {
		u.setStatus(func(s *Status) {
			s.State = StateDeferred
			s.Message = "held back: something is playing"
		})
		return ErrPlaybackInProgress
	}

	rel, err := u.latestRelease(ctx)
	if err != nil {
		return u.fail(err, "could not read the latest release")
	}
	if newer, ok := isNewer(rel.TagName, u.current); !ok || !newer {
		u.setStatus(func(s *Status) {
			s.State = StateIdle
			s.Message = "this is the latest version"
		})
		return nil
	}

	goos, goarch := currentPlatform()
	target, digest, err := assetFor(rel, goos, goarch)
	if err != nil {
		return u.fail(err, "this release has no verifiable binary for this platform")
	}

	u.setStatus(func(s *Status) {
		s.State = StateDownloading
		s.LatestVersion = rel.TagName
		s.Message = ""
	})

	// Staged beside the executable, not in the data directory: a rename across
	// filesystems is a copy, and a copy is not atomic. The two have to live on
	// the same volume for the swap to be a single instant.
	dir := filepath.Dir(u.execPath)
	staged, err := u.download(ctx, target.BrowserDownloadURL, dir, digest)
	if err != nil {
		return u.fail(err, "the download could not be verified")
	}
	defer os.Remove(staged) // no-op once the rename below succeeds

	if err := u.smokeTest(ctx, staged, rel.TagName); err != nil {
		return u.fail(err, "the downloaded binary did not run correctly")
	}

	if err := u.swap(staged); err != nil {
		return u.fail(err, "the binary could not be replaced")
	}

	u.setStatus(func(s *Status) {
		s.State = StateReady
		s.CurrentVersion = rel.TagName
		s.Available = false
		s.Message = "installed; restarting"
	})
	u.log.Info("update installed", "version", rel.TagName)

	if u.restart != nil {
		// Given to main, which closes the listener before starting the
		// replacement -- otherwise the new process finds the port still taken.
		go u.restart()
	}
	return nil
}

// download fetches a URL into a temporary file in dir and verifies its digest.
//
// The file is written without an executable bit and only made executable once
// the hash matches. An unverified binary must never be runnable, however
// briefly.
func (u *Updater) download(ctx context.Context, url, dir, wantDigest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building the download request: %w", err)
	}
	res, err := u.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading: unexpected status %d", res.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, ".theia-update-*")
	if err != nil {
		return "", fmt.Errorf("creating a staging file: %w", err)
	}
	name := tmp.Name()

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), res.Body); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", fmt.Errorf("downloading: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("writing the download: %w", err)
	}

	if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, wantDigest) {
		os.Remove(name)
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", wantDigest, got)
	}
	if err := os.Chmod(name, 0o755); err != nil {
		os.Remove(name)
		return "", fmt.Errorf("making the new binary executable: %w", err)
	}
	return name, nil
}

// smokeTest runs the downloaded binary and checks it reports the version it is
// supposed to be.
//
// This is the step that separates "the bytes arrived intact" from "this thing
// works". A correct checksum only proves the file matches what was published;
// it says nothing about whether that file runs on this machine -- wrong
// architecture, missing loader, a release built from a broken commit. Finding
// that out before the swap is the difference between a failed update and a dead
// installation.
func (u *Updater) smokeTest(ctx context.Context, path, wantVersion string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("running %s -version: %w", filepath.Base(path), err)
	}

	reported := strings.TrimSpace(string(out))
	if !strings.Contains(reported, strings.TrimPrefix(wantVersion, "v")) {
		return fmt.Errorf("the new binary reports %q, expected version %s", reported, wantVersion)
	}
	return nil
}

// swap puts the staged binary in place of the running one.
//
// Two renames rather than a copy, because a rename is atomic and a copy has a
// window in which the file on disk is neither version. Windows refuses to
// delete a running executable but allows renaming it, which is what makes this
// work without a helper process -- measured, not assumed.
//
// If the second rename fails, the first is undone. Leaving an installation with
// no executable at all is the one outcome that must not happen.
func (u *Updater) swap(staged string) error {
	previous := u.execPath + oldSuffix

	// A leftover from an earlier update would block the rename on Windows.
	_ = os.Remove(previous)

	if err := os.Rename(u.execPath, previous); err != nil {
		return fmt.Errorf("moving the current binary aside: %w", err)
	}

	if err := os.Rename(staged, u.execPath); err != nil {
		if restoreErr := os.Rename(previous, u.execPath); restoreErr != nil {
			// Both renames failed. Say exactly where the old binary is, since
			// the installation now needs a human.
			return fmt.Errorf(
				"installing the new binary failed (%w) and the previous one could not be "+
					"restored (%v); it is at %s", err, restoreErr, previous)
		}
		return fmt.Errorf("installing the new binary failed, the previous one was restored: %w", err)
	}
	return nil
}

// CleanPrevious removes the binary left behind by a completed update.
//
// Called at startup rather than straight after the swap, because the outgoing
// executable is still running at that point and Windows will not delete a file
// that is in use.
func CleanPrevious(execPath string, log *slog.Logger) {
	previous := execPath + oldSuffix
	if _, err := os.Stat(previous); err != nil {
		return
	}
	if err := os.Remove(previous); err != nil {
		// Not worth failing a start over. It will be retried next time.
		log.Debug("could not remove the previous binary", "path", previous, "error", err)
		return
	}
	log.Info("removed the previous version left by an update", "path", previous)
}

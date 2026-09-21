// Package release is what a Theia release publishes, and how one file from it is
// fetched and then checked.
//
// Two tools ask the same questions: the server, updating itself, and this
// installer, fetching what a machine is still missing. The answers have to
// agree. An asset name that drifts between them is an installation that can
// never find its own release, and a digest that is checked on one path and not
// the other is an unverified binary on somebody's disk - which the founding
// spec forbids outright.
//
// The naming and the verification rule live here. The updater keeps its own
// reader, because replacing a running installation is a different problem from
// fetching a file that is not there yet: it stages, smoke-tests and swaps, and
// its failure wording is already visible through the server's API.
package release

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRepo is where releases are published. The updater's own constant points
// here rather than repeating the string: two copies of a repository name is one
// copy too many, and the day it changes is the day nobody notices the second.
const DefaultRepo = "Benitoow/theia-media"

// ErrNoRelease means the repository has published nothing yet, which is an
// ordinary state for a young project rather than a failure.
var ErrNoRelease = errors.New("release: the repository has no published release")

// ServerName is the asset holding the server for a platform. It has to match the
// release workflow's build step exactly (`theia-server-$GOOS-$GOARCH`), because
// this name is the only thing that connects a released file to the tool that
// wants it.
//
// V3.3 renamed it from `theia-<os>-<arch>` (decision 119); an installed v3.2 asks
// for the old name through the updater it already carries, which is why the
// first V3.3 release also publishes a transitional copy.
func ServerName(goos, goarch string) string {
	return withExtension(fmt.Sprintf("theia-server-%s-%s", goos, goarch), goos)
}

// SetupName is the installer itself, published so a machine can fetch the tool
// that maintains it.
func SetupName(goos, goarch string) string {
	return withExtension(fmt.Sprintf("theia-setup-%s-%s", goos, goarch), goos)
}

// PlayerName is the player **bundle**, not the executable: the native player is
// useless without its engine, and the licences travel with it because shipping
// libmpv without its notice is a licence breach rather than an incomplete
// download.
func PlayerName(goos, goarch string) string {
	return fmt.Sprintf("theia-player-%s-%s.zip", goos, goarch)
}

// LauncherName is the `theia` command: a bare executable, installed under that
// name, whose published name says what it is.
//
// It is deliberately not `theia-<os>-<arch>`. Decision 119 retired that name,
// and it belonged to the *server*: a folder still holding a v3.2 download would
// otherwise offer a server to the installer that asked for a launcher.
func LauncherName(goos, goarch string) string {
	return withExtension(fmt.Sprintf("theia-launcher-%s-%s", goos, goarch), goos)
}

// ArchiveName is the one file a person downloads by hand: everything, in one
// zip, for the platforms that have a player.
func ArchiveName(version, goos, goarch string) string {
	return fmt.Sprintf("theia-%s-%s-%s.zip", version, goos, goarch)
}

func withExtension(name, goos string) string {
	if goos == "windows" {
		return name + ".exe"
	}
	return name
}

// Asset is one file a release publishes.
type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	URL  string `json:"browser_download_url"`

	// Digest is what GitHub reports for the asset, as "sha256:<hex>". It is the
	// whole reason a release needs no separate checksum file, and a release that
	// does not carry one is refused rather than trusted.
	Digest string `json:"digest"`
}

// SHA256 returns the digest without its algorithm prefix, and whether there was
// a usable one at all.
func (a Asset) SHA256() (string, bool) {
	digest, ok := strings.CutPrefix(a.Digest, "sha256:")
	if !ok || len(digest) != 64 {
		return "", false
	}
	return strings.ToLower(digest), true
}

// Release is a published release, as far as this project needs to know.
type Release struct {
	Tag        string  `json:"tag_name"`
	URL        string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Named returns the asset published under exactly this name.
func (r Release) Named(name string) (Asset, error) {
	for _, candidate := range r.Assets {
		if candidate.Name == name {
			return candidate, nil
		}
	}
	return Asset{}, fmt.Errorf("release: %s publishes no asset called %s", r.Tag, name)
}

// Latest asks for the most recent published release.
//
// apiBase exists so a mirror can be pointed at, and so the whole path can be
// driven against a local stub in a test - which is the only way to exercise a
// download without publishing a release first.
func Latest(ctx context.Context, client *http.Client, apiBase, repo string) (Release, error) {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	if repo == "" {
		repo = DefaultRepo
	}
	if client == nil {
		client = DefaultClient()
	}

	url := fmt.Sprintf("%s/repos/%s/releases/latest", strings.TrimSuffix(apiBase, "/"), repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("release: building the request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	res, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("release: contacting the release page: %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusNotFound:
		return Release{}, ErrNoRelease
	case res.StatusCode == http.StatusForbidden:
		// Unauthenticated calls are rate limited per address. A person running
		// an installer twice never gets near the limit, but saying so plainly
		// beats reporting a mystery.
		return Release{}, errors.New("release: the release page declined the request, possibly rate limited")
	case res.StatusCode != http.StatusOK:
		return Release{}, fmt.Errorf("release: the release page answered %d", res.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("release: reading the release: %w", err)
	}
	if rel.Draft {
		return Release{}, ErrNoRelease
	}
	return rel, nil
}

// DefaultClient is the HTTP client both this package and its callers use when
// nobody supplied one. The timeout is generous on purpose: a hundred megabytes
// over a slow line is minutes, not seconds.
func DefaultClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Minute}
}

// Download fetches the asset into dir, checks the SHA-256 the release reports,
// and only then gives the file its real name.
//
// The order matters and is the same discipline the updater applies to a running
// installation: nothing that has not been verified is ever visible under the
// name something else will pick up. A release that advertises no usable digest
// is refused before a single byte is requested - "we could not check it" is not
// a reason to install it.
//
// progress is called with the bytes written so far and the announced size; the
// size is -1 when the release did not say. It may be nil.
func (a Asset) Download(ctx context.Context, client *http.Client, dir string, progress func(done, total int64)) (string, error) {
	want, ok := a.SHA256()
	if !ok {
		return "", fmt.Errorf("release: %s is published without a usable SHA-256 digest, refusing it", a.Name)
	}
	if a.URL == "" {
		return "", fmt.Errorf("release: %s has no download address", a.Name)
	}
	if client == nil {
		client = DefaultClient()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("release: creating %s: %w", dir, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", fmt.Errorf("release: building the request for %s: %w", a.Name, err)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("release: fetching %s: %w", a.Name, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release: fetching %s answered %d", a.Name, res.StatusCode)
	}

	// The announced size is what the bar divides by; the response's own length
	// is the better answer when the release did not carry one.
	total := a.Size
	if res.ContentLength > 0 {
		total = res.ContentLength
	}

	partial := filepath.Join(dir, a.Name+".part")
	file, err := os.Create(partial)
	if err != nil {
		return "", fmt.Errorf("release: creating %s: %w", partial, err)
	}
	hash := sha256.New()
	written, err := copyWithProgress(file, res.Body, hash, total, progress)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(partial)
		return "", fmt.Errorf("release: fetching %s: %w", a.Name, err)
	}

	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		// The file is deleted rather than kept for inspection: an installer has
		// no business leaving a file on disk that it has just refused, under a
		// name that looks like a program.
		os.Remove(partial)
		return "", fmt.Errorf("release: %s arrived with digest %s, expected %s", a.Name, got, want)
	}
	if a.Size > 0 && written != a.Size {
		os.Remove(partial)
		return "", fmt.Errorf("release: %s is %d bytes, the release announced %d", a.Name, written, a.Size)
	}

	final := filepath.Join(dir, a.Name)
	if err := os.Rename(partial, final); err != nil {
		os.Remove(partial)
		return "", fmt.Errorf("release: naming %s: %w", final, err)
	}
	return final, nil
}

// copyWithProgress copies while hashing, so the digest costs no second pass over
// a hundred megabytes.
func copyWithProgress(dst io.Writer, src io.Reader, hash io.Writer, total int64, progress func(int64, int64)) (int64, error) {
	buffer := make([]byte, 256<<10)
	var written int64
	for {
		read, err := src.Read(buffer)
		if read > 0 {
			if _, werr := dst.Write(buffer[:read]); werr != nil {
				return written, werr
			}
			hash.Write(buffer[:read])
			written += int64(read)
			if progress != nil {
				progress(written, total)
			}
		}
		if err == io.EOF {
			return written, nil
		}
		if err != nil {
			return written, err
		}
	}
}

// Extract writes the named entries of a zip into dir and reports what it wrote.
//
// Only the names asked for are taken, and every one of them must be there: a
// player bundle missing its licence is a licence breach, not an incomplete
// download, so half an extraction is a failure rather than a partial success.
//
// The extraction directory is built by this machine, but the archive comes from
// somewhere else. An entry whose name climbs out of the destination is refused
// rather than unpacked over whatever it reaches.
func Extract(archive, dir string, names []string) ([]string, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return nil, fmt.Errorf("release: reading %s: %w", filepath.Base(archive), err)
	}
	defer reader.Close()

	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = true
	}
	written := make([]string, 0, len(names))
	for _, entry := range reader.File {
		if !wanted[entry.Name] {
			continue
		}
		target, err := safeJoin(dir, entry.Name)
		if err != nil {
			return nil, err
		}
		if err := writeEntry(entry, target); err != nil {
			return nil, err
		}
		delete(wanted, entry.Name)
		written = append(written, entry.Name)
	}
	if len(wanted) > 0 {
		missing := make([]string, 0, len(wanted))
		for _, name := range names {
			if wanted[name] {
				missing = append(missing, name)
			}
		}
		return nil, fmt.Errorf("release: %s does not contain %s",
			filepath.Base(archive), strings.Join(missing, ", "))
	}
	return written, nil
}

// safeJoin refuses an entry that would be written outside dir.
func safeJoin(dir, name string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(name))
	relative, err := filepath.Rel(dir, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("release: the archive contains an entry outside the destination: %s", name)
	}
	return target, nil
}

func writeEntry(entry *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("release: creating %s: %w", filepath.Dir(target), err)
	}
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("release: opening %s in the archive: %w", entry.Name, err)
	}
	defer source.Close()

	// 0o755 for a program, 0o644 for the texts beside it: a Windows machine
	// ignores the mode, and a Unix one does not.
	mode := os.FileMode(0o644)
	if strings.HasSuffix(strings.ToLower(entry.Name), ".exe") || entry.Mode()&0o111 != 0 {
		mode = 0o755
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("release: creating %s: %w", target, err)
	}
	_, err = io.Copy(file, source)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("release: writing %s: %w", target, err)
	}
	return nil
}

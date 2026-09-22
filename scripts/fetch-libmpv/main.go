// Command fetch-libmpv obtains the media engine the player ships, at the exact
// version decision 118 pinned, by digest.
//
// It exists because the engine is the one third-party binary this project
// redistributes, and "we downloaded it once" is not a reproducible build step.
// Everything it does is checked against player/libmpv.json:
//
//	the archive's SHA-256    before anything is extracted
//	the library's SHA-256    after extraction, so a bad extractor is caught too
//
// Usage:
//
//	go run ./scripts/fetch-libmpv -out player/vendor
//	go run ./scripts/fetch-libmpv -out player/target/release -print-pin
//
// The mirror is tried first. The upstream project keeps thirty days of builds
// (decision 118), so by the time somebody rebuilds an old tag the original may
// be gone; THEIA_LIBMPV_MIRROR, or a mirror release of this repository, is what
// keeps a pinned version installable. Both URLs carry the same digest, and the
// digest is the only thing that decides whether a download is used.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// manifest is player/libmpv.json, as far as this tool needs it.
type manifest struct {
	Engine    string                     `json:"engine"`
	PinnedAt  string                     `json:"pinned_at"`
	Platforms map[string]platformVersion `json:"platforms"`
}

type platformVersion struct {
	Provider      string `json:"provider"`
	Release       string `json:"release"`
	Asset         string `json:"asset"`
	URL           string `json:"url"`
	ArchiveBytes  int64  `json:"archive_bytes"`
	ArchiveSHA256 string `json:"archive_sha256"`
	Library       string `json:"library"`
	LibrarySHA256 string `json:"library_sha256"`
	LibraryBytes  int64  `json:"library_bytes"`
	Licence       string `json:"licence"`
	LicenceFile   string `json:"licence_file"`
	SourceOffer   string `json:"source_offer"`

	// A macOS engine is a set of dylibs rather than one file: mpv loads
	// libavcodec, libplacebo, libass and their own dependencies from beside
	// itself. The manifest names every one with its digest, because "the archive
	// verified" says nothing about what came out of it - and the interesting
	// failure is an archive that verifies and an extraction that did not.
	EngineLibrary    string            `json:"engine_library"`
	RuntimeLibraries map[string]string `json:"runtime_libraries"`
	RuntimeSymlinks  map[string]string `json:"runtime_symlinks"`
	LicenceDir       string            `json:"licence_dir"`
}

// isSet reports whether this pin is a set of libraries rather than one file.
func (v platformVersion) isSet() bool {
	return len(v.RuntimeLibraries) > 0
}

func main() {
	manifestPath := flag.String("manifest", filepath.Join("player", "libmpv.json"), "the pinned manifest")
	outDir := flag.String("out", filepath.Join("player", "vendor"), "directory to put the library in")
	platform := flag.String("platform", runtime.GOOS+"/"+runtime.GOARCH, "which pinned platform to fetch")
	printPin := flag.Bool("print-pin", false, "print the pinned source and digests, and exit")
	keepArchive := flag.Bool("keep-archive", false, "leave the downloaded archive next to the library")
	flag.Parse()

	file, err := os.ReadFile(*manifestPath)
	if err != nil {
		fail(err)
	}
	var pinned manifest
	if err := json.Unmarshal(file, &pinned); err != nil {
		fail(fmt.Errorf("reading %s: %w", *manifestPath, err))
	}

	version, ok := pinned.Platforms[*platform]
	if !ok {
		// Not a failure of this tool: it is the honest state of the project. Only
		// windows/amd64 has been built, run and verified, and a pin nobody has
		// executed is a claim rather than a pin.
		known := make([]string, 0, len(pinned.Platforms))
		for name := range pinned.Platforms {
			known = append(known, name)
		}
		fail(fmt.Errorf("no engine is pinned for %s; the manifest pins %s", *platform, strings.Join(known, ", ")))
	}

	if *printPin {
		fmt.Printf("engine   %s from %s %s\n", pinned.Engine, version.Provider, version.Release)
		fmt.Printf("asset    %s\n", version.Asset)
		fmt.Printf("archive  sha256:%s\n", version.ArchiveSHA256)
		if version.isSet() {
			fmt.Printf("library  %s sha256:%s\n", version.EngineLibrary, version.RuntimeLibraries[version.EngineLibrary])
			fmt.Printf("         and the %d libraries it loads, each pinned by digest\n", len(version.RuntimeLibraries)-1)
		} else {
			fmt.Printf("library  %s sha256:%s\n", version.Library, version.LibrarySHA256)
		}
		fmt.Printf("licence  %s (%s)\n", version.Licence, version.LicenceFile)
		fmt.Printf("source   %s\n", version.SourceOffer)
		return
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail(err)
	}

	archivePath := filepath.Join(*outDir, version.Asset)
	if err := obtain(version, archivePath); err != nil {
		fail(err)
	}

	libraryPath := filepath.Join(*outDir, version.Library)
	if err := extract(archivePath, version.Library, libraryPath); err != nil {
		fail(err)
	}

	// The second check, and the one that matters most: it is the file that will
	// be loaded by the player, so it is the file that has to be the pinned one.
	got, size, err := digestOf(libraryPath)
	if err != nil {
		fail(err)
	}
	if !strings.EqualFold(got, version.LibrarySHA256) {
		// Removed for the same reason the archive is: whatever is in the build
		// directory is what a build will pick up, and this is not the pinned file.
		_ = os.Remove(libraryPath)
		fail(fmt.Errorf("%s is %s, pinned as %s - refusing to ship it", version.Library, got, version.LibrarySHA256))
	}
	if version.LibraryBytes > 0 && size != version.LibraryBytes {
		fail(fmt.Errorf("%s is %d bytes, pinned at %d", version.Library, size, version.LibraryBytes))
	}

	if !*keepArchive {
		_ = os.Remove(archivePath)
	}
	fmt.Printf("fetched %s (%d bytes, sha256:%s)\n", version.Library, size, got)
	fmt.Printf("licence %s must ship beside it: see %s\n", version.Licence, version.LicenceFile)
}

// obtain downloads the archive unless a file with the right digest is already
// there, and verifies it either way.
func obtain(version platformVersion, path string) error {
	if _, _, err := digestOf(path); err == nil {
		fmt.Printf("%s is already here\n", filepath.Base(path))
	} else {
		if err := download(version, path); err != nil {
			return err
		}
	}

	got, size, err := digestOf(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, version.ArchiveSHA256) {
		// Removed on purpose: a file that is not the pinned one has no business
		// sitting in the build directory where something else might pick it up.
		_ = os.Remove(path)
		return fmt.Errorf("%s is %s, pinned as %s - refusing to extract it", filepath.Base(path), got, version.ArchiveSHA256)
	}
	if version.ArchiveBytes > 0 && size != version.ArchiveBytes {
		return fmt.Errorf("%s is %d bytes, pinned at %d", filepath.Base(path), size, version.ArchiveBytes)
	}
	fmt.Printf("archive verified (%d bytes, sha256:%s)\n", size, got)
	return nil
}

func download(version platformVersion, path string) error {
	urls := make([]string, 0, 2)
	if mirror := os.Getenv("THEIA_LIBMPV_MIRROR"); mirror != "" {
		urls = append(urls, strings.TrimRight(mirror, "/")+"/"+version.Asset)
	}
	urls = append(urls, version.URL)

	var lastErr error
	for _, url := range urls {
		fmt.Printf("downloading %s\n", url)
		if err := fetch(url, path); err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "  %v\n", err)
			continue
		}
		return nil
	}
	return fmt.Errorf("could not download the pinned archive: %w", lastErr)
}

func fetch(url, path string) error {
	client := &http.Client{Timeout: 30 * time.Minute}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", url, response.Status)
	}

	tmp := path + ".part"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	// io.Copy rather than ReadAll: the archive is 27 MB today and the ceiling
	// should be the disk, not the memory.
	if _, err := io.Copy(file, response.Body); err != nil {
		file.Close()
		os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// extract pulls one file out of the archive.
//
// `tar` rather than a 7-Zip library: libarchive reads 7z, and it ships with
// Windows 10+, macOS and every Linux this project targets. The extracted file is
// digested afterwards, so an extractor that produced something wrong is caught
// even if it exits zero.
func extract(archive, member, destination string) error {
	staging, err := os.MkdirTemp(filepath.Dir(destination), ".libmpv-extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	attempts := [][]string{
		{"tar", "-xf", archive, "-C", staging, member},
		// bsdtar on Windows accepts the forward-slash path; 7z is what a
		// developer machine usually has if tar is an old one.
		{"7z", "e", "-y", "-o" + staging, archive, member},
	}
	var lastErr error
	for _, command := range attempts {
		binary, err := exec.LookPath(command[0])
		if err != nil {
			lastErr = err
			continue
		}
		out, err := exec.Command(binary, command[1:]...).CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("%s: %w: %s", command[0], err, strings.TrimSpace(string(out)))
			continue
		}
		if _, err := os.Stat(filepath.Join(staging, member)); err != nil {
			lastErr = fmt.Errorf("%s did not produce %s", command[0], member)
			continue
		}
		if err := os.Rename(filepath.Join(staging, member), destination); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("could not extract %s: %w", member, lastErr)
}

func digestOf(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "fetch-libmpv: %v\n", err)
	os.Exit(1)
}

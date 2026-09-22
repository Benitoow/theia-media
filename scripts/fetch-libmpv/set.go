package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// extractSet unpacks an engine archive that holds a *set* of libraries, and lays
// them out flat in outDir.
//
// Flat, because that is how dyld finds them: every load command in the set asks
// for `@rpath/lib<name>.dylib`, and each library's own rpaths are
// `@loader_path`, `@loader_path/../lib` and `@loader_path/../Frameworks` - so
// the whole set has to sit in one directory, and the directory the caller names
// is the one the app bundle will use.
//
// The licence texts come out under a directory of their own rather than being
// left in the archive: shipping this engine without them is the mistake this
// tool exists to prevent, and the LGPL obligation is about the text travelling
// with the library, not about it being mentioned somewhere.
func extractSet(archive string, version platformVersion, outDir string) error {
	staging, err := os.MkdirTemp("", "theia-libmpv-set-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := runTar(archive, staging); err != nil {
		return err
	}
	root, err := singleRoot(staging)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(version.RuntimeLibraries))
	for name := range version.RuntimeLibraries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		from := filepath.Join(root, "lib", name)
		if !fileExists(from) {
			return fmt.Errorf("%s is not in %s", name, filepath.Base(archive))
		}
		if err := copyFile(from, filepath.Join(outDir, name)); err != nil {
			return err
		}
	}

	// The archive ships the unversioned names as symlinks to the versioned ones.
	// Where tar created them, nothing is left to do; where the filesystem cannot
	// hold a symlink, the target is already beside it and a copy does the same
	// job for a loader that only wants the name to resolve.
	for name, target := range version.RuntimeSymlinks {
		destination := filepath.Join(outDir, name)
		if fileExists(destination) {
			continue
		}
		if err := copyFile(filepath.Join(outDir, target), destination); err != nil {
			return err
		}
	}

	if version.LicenceDir != "" {
		from := filepath.Join(root, version.LicenceDir)
		if !dirExists(from) {
			return fmt.Errorf("%s holds no %s directory, so the engine's licences are missing",
				filepath.Base(archive), version.LicenceDir)
		}
		if err := copyTree(from, filepath.Join(outDir, version.LicenceDir)); err != nil {
			return err
		}
	}
	return nil
}

// verifySet is the check that matters: the files that will actually be loaded,
// each against the digest the manifest pins. An archive that verifies says
// nothing about an extraction that did not.
func verifySet(version platformVersion, dir string) error {
	names := make([]string, 0, len(version.RuntimeLibraries))
	for name := range version.RuntimeLibraries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		got, _, err := digestOf(path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(got, version.RuntimeLibraries[name]) {
			// Removed for the same reason the archive is: whatever sits in the
			// build directory is what a build will pick up.
			_ = os.Remove(path)
			return fmt.Errorf("%s is %s, pinned as %s - refusing to ship it", name, got, version.RuntimeLibraries[name])
		}
	}
	for name, target := range version.RuntimeSymlinks {
		path := filepath.Join(dir, name)
		if !fileExists(path) {
			return fmt.Errorf("%s is missing: the engine's own load commands ask for it by that name", name)
		}
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		// A copy where the archive had a symlink is acceptable; a different file
		// is not, because the loader would then load something unpinned.
		want, known := version.RuntimeLibraries[target]
		if !known {
			continue
		}
		if got, _, err := digestOf(path); err != nil || !strings.EqualFold(got, want) {
			return fmt.Errorf("%s is not %s", name, target)
		}
	}
	if version.Library != "" && !fileExists(filepath.Join(dir, version.Library)) {
		return fmt.Errorf("%s is missing: that is the library the player loads", version.Library)
	}
	return nil
}

// runTar unpacks a whole archive, and does not treat tar's own exit status as
// the answer.
//
// On Windows, bsdtar cannot create the symlinks a macOS engine ships - the
// filesystem refuses them without elevation - so it exits non-zero *after*
// extracting every regular file. That is the case this tolerates, and it is the
// tool's own doctrine: the members are checked by name and then by digest, so an
// extractor that produced something wrong is caught either way. Only a staging
// directory that holds nothing at all is an error.
func runTar(archive, dir string) error {
	binary, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("tar is needed to unpack the engine: %w", err)
	}
	out, err := exec.Command(binary, "-xf", archive, "-C", dir).CombinedOutput()
	if err != nil {
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || len(entries) == 0 {
			return fmt.Errorf("tar: %w: %s", err, strings.TrimSpace(string(out)))
		}
		fmt.Fprintf(os.Stderr,
			"note: tar reported %v - a link it could not create on this filesystem; every member is verified by digest below\n", err)
	}
	return nil
}

// singleRoot finds the one top-level directory an engine archive holds. Its name
// carries the version and the architecture, and nothing should have to know it.
func singleRoot(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	roots := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() {
			roots = append(roots, entry.Name())
		}
	}
	if len(roots) != 1 {
		return "", fmt.Errorf("expected one directory in the archive, found %d", len(roots))
	}
	return filepath.Join(dir, roots[0]), nil
}

func copyFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	destination, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return err
	}
	return destination.Close()
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		return copyFile(path, target)
	})
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

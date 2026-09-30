package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Ubuntu's engine is one library. Codec, driver and desktop libraries belong
// to the distribution; they are neither copied nor silently downloaded here.
func extractDeb(archive string, version platformVersion, out string) error {
	staging, err := os.MkdirTemp(out, ".libmpv-deb-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if output, err := exec.Command("dpkg-deb", "-x", archive, staging).CombinedOutput(); err != nil {
		return fmt.Errorf("extracting the Ubuntu engine: %w: %s", err, output)
	}
	for source, destination := range map[string]string{
		version.ArchiveLibrary:            version.Library,
		"usr/share/doc/libmpv2/copyright": "COPYRIGHT-libmpv.txt",
	} {
		content, err := os.ReadFile(filepath.Join(staging, filepath.FromSlash(source)))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(out, destination), content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

package setup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
)

// selfArchive reports whether the setup executable carries the complete
// platform payload as an appended ZIP. Plain setup executables from older
// releases remain valid and continue to fetch their components from GitHub.
func selfArchive(path, goos string) bool {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer archive.Close()
	want := map[string]bool{
		executablePath("theia-server", goos): false,
		installerExecutable(goos):            false,
	}
	for _, entry := range archive.File {
		name := filepath.ToSlash(entry.Name)
		name = trimArchivePrefix(name)
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	for _, found := range want {
		if !found {
			return false
		}
	}
	return true
}

func trimArchivePrefix(name string) string {
	for len(name) >= 2 && name[:2] == "./" {
		name = name[2:]
	}
	return name
}

// sourceFromSelf gives an explicit --from priority, then the appended payload
// in the downloaded setup. The latter lets one human download install the whole
// product with no separate player or launcher asset on the release page.
func sourceFromSelf(source Source) Source {
	if source.From != "" {
		return source
	}
	self, err := os.Executable()
	if err == nil && selfArchive(self, runtime.GOOS) {
		source.From = filepath.Clean(self)
	}
	return source
}

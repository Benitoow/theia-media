// Package layout is where the product's programs live inside an installation,
// and what an installation remembers about itself.
//
// It exists because that fact was written twice and only one copy knew about
// macOS. `internal/setup` has always known that a Mac player is the file inside
// `Theia.app`; `cmd/theia` looked for a loose `theia-player` beside itself, so on
// macOS the command started the server, found no player, and opened a browser
// instead of the film. Measured on the `macos-15-intel` runner on 28 September
// 2026: `PASS the theia command brought the installed server up by itself`,
// `FAIL the theia command did not open the player`.
//
// The answers are a function of the target platform rather than a build tag, so
// a test on any host can ask what a Mac installation is told (the lesson of
// decision 149). The package is deliberately small, and what it does beyond
// naming files is reading the record the installer leaves beside them - the one
// fact the programs inside an installation cannot work out for themselves.
package layout

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Benitoow/theia-media/internal/config"
)

// PlayerExecutable is the file a shell runs for the player, relative to the
// installation directory.
//
// On Windows and Linux the player is a program beside its engine. On macOS it is
// an application bundle - which is what gives the window an identity and a Dock
// icon - and the file a shell can run is the one inside `Contents/MacOS`.
func PlayerExecutable(goos string) string {
	switch goos {
	case "darwin":
		return path.Join("Theia.app", "Contents", "MacOS", "theia-player")
	case "windows":
		return "theia-player.exe"
	default:
		return "theia-player"
	}
}

// ServerExecutable is the same for the server, which is a loose program on every
// platform that ships one - it has no window, so it has no bundle.
func ServerExecutable(goos string) string {
	if goos == "windows" {
		return "theia-server.exe"
	}
	return "theia-server"
}

// InstallDirOf is the directory of the file that is running, not of the name
// somebody typed.
//
// The two differ where the installer links a command into `~/.local/bin`
// instead of copying it there, because that link is the name a terminal finds.
// macOS hands a process the path it was started from, links unresolved, so the
// directory of that path holds the link - and the link's siblings - while the
// programs are beside the file it points at. Measured on the `macos-15-intel`
// runner on 28 September 2026: the linked command started the installed server
// and then opened a browser, because no player stands beside `~/.local/bin`.
func InstallDirOf(self string) string {
	if strings.TrimSpace(self) == "" {
		// Nowhere to look: the caller could not say where it is, and `.` is not
		// an installation.
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	return filepath.Dir(self)
}

// RecordFile is the file beside the programs that says which data directory this
// installation was made with.
//
// It exists because neither program that needs the answer can work it out.
// `theia` decides which port to probe and which directory to hand the server it
// starts; `theia-setup` keeps and names the library when it removes the
// programs. The installer knows both directories while it runs, and nothing
// remembered the pair afterwards: an installation made with `--data-dir
// <elsewhere>` got a launcher that probed the default port and then started a
// server over the default directory, which is a library nobody installed.
const RecordFile = "installation.json"

// Record is what an installation remembers about itself, for the programs that
// live inside it.
type Record struct {
	// DataDir holds config.json, the database, the cache and the installer's own
	// record of the machine.
	DataDir string `json:"data_dir"`
}

// WriteRecord writes where this installation keeps its data, beside its
// programs.
func WriteRecord(installDir, dataDir string) error {
	data, err := json.MarshalIndent(Record{DataDir: dataDir}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(installDir, RecordFile), append(data, '\n'), 0o644)
}

// ReadRecord reads the record beside the programs in installDir. A missing file
// is reported as absent rather than as an error: an installation made before the
// record existed has none, and says nothing wrong by that.
func ReadRecord(installDir string) (Record, bool, error) {
	data, err := os.ReadFile(filepath.Join(installDir, RecordFile))
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, false, fmt.Errorf("reading %s: %w", RecordFile, err)
	}
	if strings.TrimSpace(record.DataDir) == "" {
		return Record{}, false, fmt.Errorf("%s names no data directory", RecordFile)
	}
	return record, true, nil
}

// DataDir is where the installation in installDir keeps its data.
//
// The order is deliberate, and each answer is somebody different speaking:
//
//  1. `THEIA_DATA_DIR`, which is an operator saying so for this run - what the
//     proof runs and the tests use to keep off the machine's own library.
//  2. The record the installer wrote beside the programs, which is what makes an
//     installation given `--data-dir` elsewhere behave like itself.
//  3. The standard location, which is where an installation from before the
//     record put its data.
//
// A record that exists and cannot be read is an error rather than a quiet fall
// to the standard location: the whole fault this replaced was a program that
// looked in the wrong place and said nothing.
func DataDir(installDir string) (string, error) {
	if fromEnv, ok := config.DataDirFromEnv(); ok {
		return fromEnv, nil
	}
	if strings.TrimSpace(installDir) != "" {
		record, ok, err := ReadRecord(installDir)
		if err != nil {
			return "", err
		}
		if ok {
			return record.DataDir, nil
		}
	}
	return config.DataDir()
}

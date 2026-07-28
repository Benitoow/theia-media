// Package scanner walks the configured library directories and reports the
// video files it finds.
//
// It knows nothing about films, titles or metadata -- it answers "what video
// files exist on disk right now", and internal/library decides what they mean.
//
// Nothing here aborts a scan. A real library lives on external drives, network
// shares and directories the user forgot they had; an unreadable folder is a
// thing to report, not a reason to abandon the other nine.
package scanner

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File is a video file found on disk.
type File struct {
	Path       string
	Name       string
	SizeBytes  int64
	ModifiedAt time.Time
}

// Result is what one pass over the configured roots turned up.
type Result struct {
	Files []File

	// Problems encountered along the way. A scan with problems still returns
	// everything it did manage to read.
	Problems []Problem
}

// videoExtensions is what counts as a film. Deliberately generous: a file the
// browser cannot play is still a file the user has, and M4 decides what to do
// about it.
var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".mpg": true, ".mpeg": true,
	".m2ts": true, ".mts": true, ".ts": true, ".vob": true, ".ogv": true,
	".divx": true, ".3gp": true, ".rmvb": true, ".asf": true, ".m2v": true,
}

// skippedDirectories never contain the feature film itself. Matched
// case-insensitively against each directory name.
var skippedDirectories = map[string]bool{
	"extras": true, "extra": true, "featurettes": true, "featurette": true,
	"behind the scenes": true, "deleted scenes": true, "interviews": true,
	"trailers": true, "trailer": true, "sample": true, "samples": true,
	"subs": true, "subtitles": true, "other": true, "shorts": true,

	// Platform and appliance noise.
	"@eadir": true, "$recycle.bin": true, "system volume information": true,
	"lost+found": true, "#recycle": true,
}

// Scan walks every root and returns the video files beneath them.
//
// Roots that do not exist are reported as problems rather than errors: a drive
// being unplugged is an ordinary Tuesday, not a reason to fail the scan.
func Scan(ctx context.Context, roots []string, log *slog.Logger) (Result, error) {
	var result Result
	seen := make(map[string]bool)

	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		absolute, err := filepath.Abs(root)
		if err != nil {
			log.Warn("a library path could not be resolved", "path", root, "error", err)
			result.Problems = append(result.Problems,
				Problem{Kind: KindDirectoryUnreadable, Path: root})
			continue
		}
		info, err := os.Stat(absolute)
		if err != nil {
			// Overwhelmingly this is an external drive that is not plugged in.
			// The error text says GetFileAttributesEx; the user needs to hear
			// about their drive.
			log.Warn("a library directory is unreadable", "path", root, "error", err)
			result.Problems = append(result.Problems,
				Problem{Kind: KindDirectoryUnreadable, Path: root})
			continue
		}
		if !info.IsDir() {
			result.Problems = append(result.Problems,
				Problem{Kind: KindNotADirectory, Path: root})
			continue
		}

		if err := scanRoot(ctx, absolute, seen, &result, log); err != nil {
			return result, err
		}
	}
	return result, nil
}

func scanRoot(ctx context.Context, root string, seen map[string]bool, result *Result, log *slog.Logger) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		if err != nil {
			// Permission denied on one subtree, a vanished symlink, a network
			// share that dropped. Note it and carry on with the rest.
			log.Warn("a directory could not be read", "path", path, "error", err)
			result.Problems = append(result.Problems,
				Problem{Kind: KindSubdirectoryUnreadable, Path: path})
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		name := entry.Name()

		if entry.IsDir() {
			// Never skip the root itself, however it happens to be named.
			if path == root {
				return nil
			}
			if isHidden(name) || skippedDirectories[strings.ToLower(name)] {
				return fs.SkipDir
			}
			return nil
		}

		if isHidden(name) || !videoExtensions[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if looksLikeSample(name) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			log.Warn("a file could not be read", "path", path, "error", err)
			result.Problems = append(result.Problems,
				Problem{Kind: KindFileUnreadable, Path: path})
			return nil
		}

		// Two roots can overlap, or one can be a symlink into the other. The
		// same file must not become two rows.
		if seen[path] {
			return nil
		}
		seen[path] = true

		result.Files = append(result.Files, File{
			Path:       path,
			Name:       name,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime(),
		})
		return nil
	})
}

// isHidden covers dot-files on every platform. Windows' hidden attribute is not
// consulted: plenty of ordinary media directories carry it by accident, and
// silently ignoring a user's whole library would be worse than listing a file
// they meant to hide.
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// looksLikeSample matches the short teaser file that ships next to a release.
// The check is on a whole word so that a film with "sample" inside a longer
// word is not thrown away.
//
// Only words that describe the *file* belong here. Release group names do not:
// an earlier version of this list included "rarbg", which threw away every film
// whose filename carried that group -- nine of them in a 277-file test library.
// The junk it was meant to catch (RARBG.txt and friends) is not a video file
// and never reaches this function.
func looksLikeSample(name string) bool {
	base := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	for _, field := range strings.FieldsFunc(base, func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == ' ' || r == '(' || r == ')' || r == '[' || r == ']'
	}) {
		if field == "sample" || field == "trailer" {
			return true
		}
	}
	return false
}

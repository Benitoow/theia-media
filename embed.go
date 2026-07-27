// Package theia holds the assets that must be embedded from the module root.
//
// //go:embed cannot reach outside the directory of the file that declares it,
// so the frontend bundle -- which lives in web-dist/ at the repository root --
// has to be embedded here rather than from internal/api where it is consumed.
package theia

import (
	"embed"
	"io/fs"
)

// The all: prefix keeps files whose names start with "." or "_", which matters
// because SvelteKit emits its hashed assets under _app/.
//
//go:embed all:web-dist
var webDist embed.FS

// WebFS returns the compiled frontend, rooted so that "index.html" is at the
// top level. On a fresh clone where the frontend has not been built yet the
// returned filesystem is empty but valid; api.Server serves a placeholder page
// in that case rather than failing.
func WebFS() (fs.FS, error) {
	return fs.Sub(webDist, "web-dist")
}

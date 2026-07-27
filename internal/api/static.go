package api

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// staticHandler serves the embedded SvelteKit bundle.
//
// The frontend is a single-page app, so any path that does not match a real
// file falls through to index.html and lets the client-side router handle it.
// Two deliberate exceptions: /api/ never falls back, because an unknown
// endpoint has to be a JSON 404 rather than a stray HTML page the frontend
// would choke on parsing; and a build that produced no index.html at all gets
// the placeholder below, which is what you see if you run the binary without
// having built web/ first.
func (s *Server) staticHandler() http.Handler {
	files := http.FileServerFS(s.web)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSONError(w, http.StatusNotFound, "unknown API endpoint")
			return
		}

		if name := strings.TrimPrefix(path.Clean(r.URL.Path), "/"); name != "" {
			if info, err := fs.Stat(s.web, name); err == nil && !info.IsDir() {
				// SvelteKit content-hashes everything under _app/immutable, so
				// those may be cached forever. Nothing else here may.
				if strings.HasPrefix(name, "_app/immutable/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}

		s.serveIndex(w, r)
	})
}

// serveIndex writes the SPA entry point.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	f, err := s.web.Open("index.html")
	if err != nil {
		s.servePlaceholder(w)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// index.html names the hashed bundles, so a cached copy that outlives an
	// update points at files that no longer exist. Never cache it.
	w.Header().Set("Cache-Control", "no-cache")

	if _, err := io.Copy(w, f); err != nil {
		s.log.Warn("serving index.html failed", "error", err)
	}
}

// placeholderPage stands in for the frontend when the binary was built without
// one. Only ever seen when building from source, hence English and hence the
// build instruction rather than something reassuring.
const placeholderPage = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Theia &mdash; frontend not built</title>
<style>
  body { background:#0b0b0f; color:#e8e8ef; font:16px/1.6 system-ui, sans-serif;
         display:grid; place-content:center; min-height:100vh; margin:0; padding:2rem; }
  main { max-width:34rem; }
  h1 { font-weight:600; font-size:1.4rem; margin:0 0 .75rem; }
  p { color:#a0a0b0; margin:0 0 1rem; }
  code { background:#1a1a24; padding:.15rem .4rem; border-radius:.25rem; color:#e8e8ef; }
  pre { background:#1a1a24; padding:.9rem 1.1rem; border-radius:.5rem; overflow-x:auto; }
</style>
<main>
  <h1>The server is running, the interface is not built.</h1>
  <p>The Go binary starts and answers <code>/api/health</code>, but
     <code>web-dist/</code> contains no <code>index.html</code> to serve.</p>
  <pre>cd web
npm install
npm run build</pre>
  <p>Then rebuild the binary so the fresh bundle is embedded.</p>
</main>
`

func (s *Server) servePlaceholder(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusServiceUnavailable)
	if _, err := io.WriteString(w, placeholderPage); err != nil {
		s.log.Warn("serving placeholder page failed", "error", err)
	}
}

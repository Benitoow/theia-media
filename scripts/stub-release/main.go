// Command stub-release serves a Theia release from files on this machine.
//
// It exists so the installer's download path can be driven end to end without
// publishing anything: the files are the real ones, the digests are their real
// SHA-256, and the JSON is what GitHub would answer. The updater and the
// installer both honour THEIA_UPDATE_API, so pointing it here is enough to make
// either of them fetch from this server instead of from GitHub.
//
//	go run ./scripts/stub-release -dir <folder> [-addr 127.0.0.1:8397] [-rate 8]
//
// Every file in the folder becomes an asset under its own name, which is how the
// release page names them. -rate slows the transfer to a given number of
// megabytes per second, because a progress bar that only exists for two
// milliseconds cannot be looked at.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	dir := flag.String("dir", "", "folder holding the files to publish as assets")
	addr := flag.String("addr", "127.0.0.1:8397", "address to listen on")
	tag := flag.String("tag", "v0.0.0-stub", "the tag the release claims")
	rate := flag.Int("rate", 0, "megabytes per second to serve at; 0 means as fast as the disk allows")
	flag.Parse()

	if *dir == "" {
		log.Fatal("stub-release: -dir is required")
	}
	assets, err := collect(*dir)
	if err != nil {
		log.Fatalf("stub-release: %v", err)
	}
	if len(assets) == 0 {
		log.Fatalf("stub-release: %s holds no files, so there is nothing to publish", *dir)
	}

	for _, asset := range assets {
		fmt.Printf("%-42s %10d bytes  sha256:%s\n", asset.Name, asset.Size, asset.Digest)
	}

	prefix := "/repos/" + repoFromEnvironment() + "/releases/"
	mux := http.NewServeMux()
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			published := make([]map[string]any, 0, len(assets))
			for _, asset := range assets {
				published = append(published, map[string]any{
					"name":                 asset.Name,
					"size":                 asset.Size,
					"browser_download_url": "http://" + *addr + "/assets/" + asset.Name,
					"digest":               "sha256:" + asset.Digest,
				})
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"tag_name": *tag,
				"html_url": "http://" + *addr + "/tag/" + *tag,
				"assets":   published,
			})
			return
		}
		http.NotFound(w, r)
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/assets/")
		for _, asset := range assets {
			if asset.Name != name {
				continue
			}
			serve(w, asset, *rate)
			return
		}
		http.NotFound(w, r)
	})

	fmt.Printf("\nserving %s on http://%s\n", *tag, *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// asset is one published file: its name, its size and the digest the release
// page advertises for it.
type asset struct {
	Name   string
	Path   string
	Size   int64
	Digest string
}

func collect(dir string) ([]asset, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	assets := make([]asset, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		hash := sha256.New()
		size, err := io.Copy(hash, file)
		file.Close()
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset{
			Name:   entry.Name(),
			Path:   path,
			Size:   size,
			Digest: hex.EncodeToString(hash.Sum(nil)),
		})
	}
	return assets, nil
}

// serve sends one asset, optionally slowed down so the progress bar can be
// watched rather than glimpsed.
func serve(w http.ResponseWriter, a asset, megabytesPerSecond int) {
	file, err := os.Open(a.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Length", fmt.Sprint(a.Size))
	if megabytesPerSecond <= 0 {
		io.Copy(w, file)
		return
	}

	const chunk = 64 << 10
	pause := time.Duration(float64(time.Second) * float64(chunk) / float64(megabytesPerSecond<<20))
	buffer := make([]byte, chunk)
	for {
		read, err := file.Read(buffer)
		if read > 0 {
			if _, werr := w.Write(buffer[:read]); werr != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			time.Sleep(pause)
		}
		if err != nil {
			return
		}
	}
}

// repoFromEnvironment names the repository the fake release belongs to, so the
// path matches what the client asks for.
func repoFromEnvironment() string {
	if repo := os.Getenv("THEIA_STUB_REPO"); repo != "" {
		return repo
	}
	return "Benitoow/theia-media"
}

// A library at scale, without touching a real one.
//
// The standard on this project is to report what was verified against the real
// library rather than what was assumed, and the real library is 274 films. That
// is fine on the maintainer's machine and impossible everywhere else: a fresh
// clone, a CI runner, or the same machine with the drive unplugged has nothing
// to look at, and a grid of eight test files tells you nothing about a grid.
//
// This fills a throwaway database with as many films as there is artwork for,
// so the interface can be judged at the size it is actually used at. It was
// written after building the same thing by hand twice in one afternoon.
//
//	go run ./scripts/bench -data <dir> [-count 200]
//
// The rows point at paths that do not exist, which is deliberate: this builds a
// library to *look* at, not one to play. Nothing here can stream, and the
// watcher is told to leave it alone by clearing the roots.
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Titles worth reading while judging a grid. Real films, so the eye has
// something to recognise, and long enough in places to catch a card that
// cannot hold its own heading.
var titles = []string{
	"Heat", "The Conversation", "Solaris", "Stalker", "Chungking Express",
	"In the Mood for Love", "The Handmaiden", "Burning", "Memories of Murder",
	"Le Samouraï", "L'Armée des ombres", "Le Cercle rouge", "Playtime",
	"Mon Oncle", "La Haine", "Un prophète", "De rouille et d'os",
	"La Vie d'Adèle", "Amour", "Caché", "Le Ruban blanc", "Persona",
	"Les Fraises sauvages", "Le Septième Sceau", "Cris et chuchotements",
	"Andreï Roublev", "Le Miroir", "Nostalghia", "Le Sacrifice",
	"Requiem pour un massacre", "Il est difficile d'être un dieu",
}

var genres = [][]string{
	{"Drame", "Thriller"}, {"Drame"}, {"Science-fiction", "Drame"},
	{"Policier", "Drame"}, {"Romance", "Drame"}, {"Guerre", "Drame"},
}

func main() {
	dataDir := flag.String("data", "", "the data directory of a Theia instance (required)")
	count := flag.Int("count", 0, "how many films to write; 0 means as many as there is artwork for")
	flag.Parse()

	if *dataDir == "" {
		flag.Usage()
		os.Exit(2)
	}

	dbPath := filepath.Join(*dataDir, "theia.db")
	if _, err := os.Stat(dbPath); err != nil {
		log.Fatalf("no database at %s -- start Theia once against this directory first", dbPath)
	}

	backdrops, posters, err := cachedArtwork(filepath.Join(*dataDir, "cache", "images"))
	if err != nil {
		log.Fatal(err)
	}
	if len(backdrops) == 0 {
		log.Fatalf("no artwork cached under %s.\n"+
			"Point Theia at a few real films once and let it fetch, or copy an existing\n"+
			"cache/images directory here. A bench with no pictures is not a bench.",
			filepath.Join(*dataDir, "cache", "images"))
	}

	wanted := len(backdrops)
	if *count > 0 {
		wanted = *count
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	written, generation, err := seed(db, wanted, backdrops, posters)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d films written at scan generation %d, drawing on %d backdrops and %d posters.\n",
		written, generation, len(backdrops), len(posters))
	fmt.Printf("Clear library_paths in %s before starting Theia, or the next scan will\n"+
		"remove every one of them for having no file on disk.\n",
		filepath.Join(*dataDir, "config.json"))
}

// cachedArtwork lists the TMDB image paths already on disk, so the bench needs
// no network and fetches nothing.
func cachedArtwork(dir string) (backdrops, posters []string, err error) {
	read := func(size string) []string {
		entries, err := os.ReadDir(filepath.Join(dir, size))
		if err != nil {
			return nil
		}
		var out []string
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".jpg") {
				out = append(out, "/"+e.Name())
			}
		}
		return out
	}

	backdrops = read("w780")
	posters = read("w342")
	if backdrops == nil && posters == nil {
		if _, statErr := os.Stat(dir); statErr != nil {
			return nil, nil, fmt.Errorf("reading the image cache: %w", statErr)
		}
	}
	if posters == nil {
		posters = backdrops
	}
	if backdrops == nil {
		backdrops = posters
	}
	return backdrops, posters, nil
}

// seed writes the rows and puts them all in the current scan generation, which
// is what makes them visible: every read filters on last_seen_scan, and a row
// left behind the running generation is a film the scan decided has gone.
func seed(db *sql.DB, count int, backdrops, posters []string) (written int, generation int64, err error) {
	if err = db.QueryRow(`SELECT COALESCE(MAX(last_seen_scan), 1) FROM movies`).Scan(&generation); err != nil {
		return 0, 0, fmt.Errorf("reading the scan generation: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().Unix()
	for i := 0; i < count; i++ {
		title := titles[i%len(titles)]
		year := 1958 + (i*7)%64
		// The title carries the year so that two hundred rows drawn from thirty
		// titles are still told apart on screen.
		display := fmt.Sprintf("%s (%d)", title, year)
		path := fmt.Sprintf("/bench/%04d %s.mkv", i, strings.ReplaceAll(title, " ", "."))
		genre := genres[i%len(genres)]

		_, err = tx.Exec(`
			INSERT INTO movies (path, file_name, size_bytes, modified_at, title, year,
				first_seen_scan, last_seen_scan, added_at, updated_at,
				tmdb_id, tmdb_title, overview, release_date, poster_path, backdrop_path,
				runtime_minutes, vote_average, director, genres_json, cast_json,
				metadata_status, metadata_fetched_at)
			VALUES (?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?,?, ?,?,?,?,?, 'ok', ?)
			ON CONFLICT(path) DO UPDATE SET last_seen_scan = excluded.last_seen_scan`,
			path, filepath.Base(path), int64(4)<<30, now, display, year,
			generation, generation, now, now,
			900000+i, display,
			"A bench row carrying real artwork, so that a grid can be judged at the "+
				"size it is actually used at rather than at the size eight test files make it.",
			fmt.Sprintf("%d-06-01", year),
			posters[i%len(posters)], backdrops[i%len(backdrops)],
			88+i%74, 5.5+float64(i%45)/10, "Banc d'essai",
			`["`+strings.Join(genre, `","`)+`"]`,
			`[{"name":"Première actrice"},{"name":"Second rôle"}]`,
			now)
		if err != nil {
			return 0, 0, fmt.Errorf("writing bench film %d: %w", i, err)
		}
		written++
	}

	// Everything already in the table joins the same generation, so a bench run
	// against a database that has been scanned since does not leave half its
	// rows invisible.
	if _, err = tx.Exec(`UPDATE movies SET last_seen_scan = ?`, generation); err != nil {
		return 0, 0, err
	}

	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	if written == 0 {
		return 0, 0, errors.New("nothing was written")
	}
	return written, generation, nil
}

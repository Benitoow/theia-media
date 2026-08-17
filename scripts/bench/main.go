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
	"encoding/json"
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

// Taglines, invented rather than quoted: this repository is public and a real
// tagline is somebody's copy. Lengths vary on purpose -- the short one proves
// the line is not padded, the long one proves it wraps inside its measure
// instead of running the width of a television.
var taglines = []string{
	"Rien ne se répare.",
	"Il savait ce qu'il faisait. C'est bien le problème.",
	"Une ville, deux nuits, et personne pour dire ce qui s'est vraiment passé cette semaine-là.",
	"",
	"Le silence coûte plus cher que l'aveu.",
	"",
}

// Certificates as the boards write them, with the country that issued them.
// The empty pair matters as much as the others: plenty of films carry no
// certificate at all, and the row must not leave a gap where the box was.
var certificates = [][2]string{
	{"12", "FR"}, {"16", "FR"}, {"TP", "FR"}, {"", ""},
	{"18", "FR"}, {"PG-13", "US"}, {"", ""}, {"R", "US"},
}

var writers = []string{"Claire Bénard", "Ivan Sorel", "Marthe Guillou", "Tomas Lind"}
var composers = []string{"Anna Weiss", "Élie Vasseur", "Kenji Oda"}
var photographers = []string{"Paul Restier", "Noor Haddad", "Greta Lindqvist"}

// Cast names long enough to catch a portrait row that cannot hold its own
// second column.
var castNames = []string{
	"Camille Aubert-Ferrand", "Youcef Benali", "Ingrid Sørensen", "Marc Level",
	"Awa Diouf", "Tadeusz Wilczyński", "Hélène Roussel", "Bruno Cazes",
	"Mei Lin Chow", "Olivier Trémaux",
}

var characters = []string{
	"L'inspectrice", "Le pianiste", "La veuve", "Le passeur", "Elle-même",
	"Le juge d'instruction", "La sœur aînée", "Le contremaître", "La traductrice",
	"Le fils",
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
	fmt.Printf("Taglines, certificates, credits and %d sagas are filled too, so the film page\n"+
		"can be judged with its record and not just its poster.\n", (written+6)/7)
	fmt.Println("Artwork is whatever this cache holds: a bench film's poster is not its own,\n" +
		"because the film does not exist. Cast portraits are only shown for people this\n" +
		"database already knows by name -- an invented actor gets the stand-in, never\n" +
		"somebody else's face.")
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

	// Read the real credits before writing anything, so the bench cannot end up
	// reading its own invented rows back on a second run.
	known := knownCredits(db)

	now := time.Now().Unix()
	for i := 0; i < count; i++ {
		title := titles[i%len(titles)]
		year := 1958 + (i*7)%64
		// The title carries the year so that two hundred rows drawn from thirty
		// titles are still told apart on screen.
		display := fmt.Sprintf("%s (%d)", title, year)
		path := fmt.Sprintf("/bench/%04d %s.mkv", i, strings.ReplaceAll(title, " ", "."))
		genre := genres[i%len(genres)]

		certificate := certificates[i%len(certificates)]

		// Sagas, three films at a time, over the first three of every seven
		// rows. Enough of the library to find one immediately, and far from all
		// of it: a collection row under every single film would say nothing
		// about how the page reads without one.
		var (
			collectionID     any
			collectionName   any
			collectionPoster any
		)
		if i%7 < 3 {
			group := i / 7
			collectionID = 7000 + group
			collectionName = fmt.Sprintf("Cycle du banc n° %d", group+1)
			collectionPoster = posters[group%len(posters)]
		}

		_, err = tx.Exec(`
			INSERT INTO movies (path, file_name, size_bytes, modified_at, title, year,
				first_seen_scan, last_seen_scan, added_at, updated_at,
				tmdb_id, tmdb_title, original_title, original_language, tagline,
				overview, release_date, poster_path, backdrop_path,
				runtime_minutes, vote_average, director, genres_json, cast_json, crew_json,
				certification, certification_country,
				collection_id, collection_name, collection_poster_path,
				metadata_status, metadata_fetched_at, metadata_version)
			VALUES (?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?, ?,?,?,?, ?,?,?,?,?,?, ?,?, ?,?,?, 'ok', ?, 2)
			ON CONFLICT(path) DO UPDATE SET last_seen_scan = excluded.last_seen_scan`,
			path, filepath.Base(path), int64(4)<<30, now, display, year,
			generation, generation, now, now,
			900000+i, display,
			// An original title on every third film, so the credits block is
			// seen both with the line and without it.
			originalTitle(i, title), "en", taglines[i%len(taglines)],
			"A bench row carrying real artwork, so that a grid can be judged at the "+
				"size it is actually used at rather than at the size eight test files make it.",
			fmt.Sprintf("%d-06-01", year),
			posters[i%len(posters)], backdrops[i%len(backdrops)],
			88+i%74, 5.5+float64(i%45)/10, "Banc d'essai",
			`["`+strings.Join(genre, `","`)+`"]`,
			castJSON(i, known), crewJSON(i),
			certificate[0], certificate[1],
			collectionID, collectionName, collectionPoster,
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

// originalTitle returns a foreign original for some films and nothing for the
// rest. The credits block hides a row it has no value for, and that is worth
// seeing on a page rather than trusting.
func originalTitle(i int, title string) string {
	if i%3 != 0 {
		return ""
	}
	return title + " (original)"
}

// castJSON builds a billed cast, in the shape the library stores and the
// interface reads.
//
// A portrait is only used where it can be named truthfully. An earlier version
// borrowed cached portraits for invented actors, which put Mark Hamill's face
// under "Awa Diouf" on every film in the bench -- and a bench nobody can stand to
// look at does not do its job, which is to let somebody judge the interface. So:
// people the database already knows keep their own face and their own name, and
// invented people get no photograph at all. The composed stand-in is a case worth
// looking at anyway, because TMDB has no portrait for plenty of real credits.
func castJSON(i int, known []credit) string {
	// Six, eight or ten names: a two-column list with an odd count leaves a gap
	// in the last row, and that is a case the layout has to survive.
	size := 6 + (i%3)*2
	entries := make([]string, 0, size)
	for n := 0; n < size; n++ {
		// Real credits first, so a bench run against a library that has been
		// enriched shows photographs; invented names fill the rest.
		if len(known) > 0 && (i+n)%3 != 2 {
			person := known[(i+n)%len(known)]
			entries = append(entries, fmt.Sprintf(
				`{"name":%q,"character":%q,"profile_path":%q}`,
				person.Name, person.Character, person.ProfilePath))
			continue
		}
		entries = append(entries, fmt.Sprintf(
			`{"name":%q,"character":%q,"profile_path":""}`,
			castNames[(i+n)%len(castNames)], characters[(i+n)%len(characters)]))
	}
	return "[" + strings.Join(entries, ",") + "]"
}

// credit is one real person the database already holds, with the face that
// actually belongs to them.
type credit struct {
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
}

// knownCredits reads the cast of films this database has already enriched, so
// the bench can show a photograph without inventing who is in it. An empty
// result is ordinary -- a throwaway directory has never fetched anything -- and
// means the bench draws stand-ins instead.
func knownCredits(db *sql.DB) []credit {
	rows, err := db.Query(`
		SELECT cast_json FROM movies
		WHERE metadata_status = 'ok' AND cast_json IS NOT NULL AND cast_json != ''`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	seen := map[string]bool{}
	var out []credit
	for rows.Next() {
		var blob string
		if err := rows.Scan(&blob); err != nil {
			return out
		}
		var people []credit
		if err := json.Unmarshal([]byte(blob), &people); err != nil {
			continue
		}
		for _, person := range people {
			if person.ProfilePath == "" || person.Name == "" || seen[person.Name] {
				continue
			}
			seen[person.Name] = true
			out = append(out, person)
		}
	}
	return out
}

// crewJSON builds the named crew, carrying role codes exactly as the server
// does -- the interface owns the words for them.
func crewJSON(i int) string {
	entries := []string{
		fmt.Sprintf(`{"name":%q,"role":"writing"}`, writers[i%len(writers)]),
		fmt.Sprintf(`{"name":%q,"role":"music"}`, composers[i%len(composers)]),
	}
	// Not every film gets every role: a block whose rows are always all present
	// hides the case where one is missing.
	if i%2 == 0 {
		entries = append(entries,
			fmt.Sprintf(`{"name":%q,"role":"cinematography"}`, photographers[i%len(photographers)]))
	}
	// A second writer on some, to prove the names join rather than overflow.
	if i%5 == 0 {
		entries = append(entries,
			fmt.Sprintf(`{"name":%q,"role":"writing"}`, writers[(i+1)%len(writers)]))
	}
	return "[" + strings.Join(entries, ",") + "]"
}

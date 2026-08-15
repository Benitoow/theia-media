package library

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// Results is what one search found, across both catalogues.
//
// Films and series stay in separate lists rather than being interleaved into
// one ranked stream. They are different things to click on, the interface shows
// them under their own headings, and a relevance score good enough to mix them
// honestly is not something a substring match can produce.
type Results struct {
	Movies []Movie  `json:"movies"`
	Series []Series `json:"series"`

	// Truncated says the library held more matches than were returned, so the
	// interface can say so rather than implying the list is complete.
	Truncated bool `json:"truncated"`
}

// searchKey reduces a string to what a search should compare: lowercase, no
// accents, no punctuation, single spaces.
//
// This mirrors searchKey in web/src/lib/api.js, which the library page has used
// since it was written. Typing "amelie" has always found "Amélie" there, and a
// search box in the navigation that suddenly did not would read as broken.
//
// The browser gets the accent half for free from String.normalize('NFD'). Go's
// standard library has no Unicode decomposition, and golang.org/x/text would be
// a dependency bought for one function, so the folding is spelled out below.
func searchKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := true

	for _, r := range strings.ToLower(s) {
		if folded, ok := accentFolds[r]; ok {
			b.WriteString(folded)
			space = false
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// accentFolds covers the accented Latin letters that appear in film titles:
// Latin-1 Supplement, and the parts of Latin Extended-A a European catalogue
// actually contains.
//
// The ligatures expand to two letters because that is how somebody types them.
// Nobody reaches for the oe ligature to search for "Coeur".
var accentFolds = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a", 'ă': "a", 'ą': "a",
	'ç': "c", 'ć': "c", 'ĉ': "c", 'ċ': "c", 'č': "c",
	'ď': "d", 'đ': "d",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ĕ': "e", 'ė': "e", 'ę': "e", 'ě': "e",
	'ĝ': "g", 'ğ': "g", 'ġ': "g", 'ģ': "g",
	'ĥ': "h", 'ħ': "h",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ĩ': "i", 'ī': "i", 'ĭ': "i", 'į': "i", 'ı': "i",
	'ĵ': "j",
	'ķ': "k",
	'ĺ': "l", 'ļ': "l", 'ľ': "l", 'ŀ': "l", 'ł': "l",
	'ñ': "n", 'ń': "n", 'ņ': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o", 'ŏ': "o", 'ő': "o",
	'ŕ': "r", 'ŗ': "r", 'ř': "r",
	'ś': "s", 'ŝ': "s", 'ş': "s", 'š': "s",
	'ţ': "t", 'ť': "t", 'ŧ': "t",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ũ': "u", 'ū': "u", 'ŭ': "u", 'ů': "u", 'ű': "u", 'ų': "u",
	'ŵ': "w",
	'ý': "y", 'ÿ': "y", 'ŷ': "y",
	'ź': "z", 'ż': "z", 'ž': "z",

	'æ': "ae", 'œ': "oe", 'ß': "ss", 'þ': "th", 'ð': "d",
}

// maxSearchResults caps each list. A search box either shows what you were
// looking for or it does not; nobody reads the twenty-first film.
const maxSearchResults = 20

// Search finds films and series whose title, alternate title, year or credited
// name matches.
//
// The match is a substring of the folded text, deliberately: it is what the
// library page has always done, it needs no index, and at household scale it is
// indistinguishable from something cleverer.
//
// Matching happens in Go rather than in SQL because SQLite's LIKE cannot see
// past an accent — "amelie" matches "Amélie" in no collation available without
// CGO. The candidate query stays narrow, so what crosses the boundary is a few
// columns per title rather than every synopsis and cast list; only the rows
// that matched are then read in full.
//
// If a library ever grows large enough for that to be felt, the answer is a
// folded column written at scan time, not a different matching rule.
func (s *Store) Search(ctx context.Context, profileID int64, query string) (Results, error) {
	needle := searchKey(query)
	if needle == "" {
		return Results{}, nil
	}

	movies, moreMovies, err := s.searchMovies(ctx, profileID, needle)
	if err != nil {
		return Results{}, err
	}
	series, moreSeries, err := s.searchSeries(ctx, needle)
	if err != nil {
		return Results{}, err
	}
	return Results{Movies: movies, Series: series, Truncated: moreMovies || moreSeries}, nil
}

func (s *Store) searchMovies(ctx context.Context, profileID int64, needle string) ([]Movie, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, tmdb_title, year, director
		FROM movies
		ORDER BY title COLLATE NOCASE, year`)
	if err != nil {
		return nil, false, fmt.Errorf("searching films: %w", err)
	}

	ids, truncated, err := matchingIDs(rows, needle)
	if err != nil {
		return nil, false, fmt.Errorf("searching films: %w", err)
	}
	if len(ids) == 0 {
		return nil, truncated, nil
	}

	found, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+movieSource+`
		WHERE m.id IN (`+placeholders(len(ids))+`)
		ORDER BY m.title COLLATE NOCASE, m.year`,
		append([]any{profileID}, ids...)...)
	if err != nil {
		return nil, false, fmt.Errorf("searching films: %w", err)
	}
	movies, err := collectMovies(found, len(ids))
	if err != nil {
		return nil, false, fmt.Errorf("searching films: %w", err)
	}
	return movies, truncated, nil
}

func (s *Store) searchSeries(ctx context.Context, needle string) ([]Series, bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, tmdb_name, year, original_name
		FROM series
		ORDER BY COALESCE(tmdb_name, title) COLLATE NOCASE, year`)
	if err != nil {
		return nil, false, fmt.Errorf("searching series: %w", err)
	}

	ids, truncated, err := matchingIDs(rows, needle)
	if err != nil {
		return nil, false, fmt.Errorf("searching series: %w", err)
	}
	if len(ids) == 0 {
		return nil, truncated, nil
	}

	found, err := s.db.QueryContext(ctx, `
		SELECT `+seriesColumns+` FROM series
		WHERE id IN (`+placeholders(len(ids))+`)
		ORDER BY COALESCE(tmdb_name, title) COLLATE NOCASE, year, id`, ids...)
	if err != nil {
		return nil, false, fmt.Errorf("searching series: %w", err)
	}
	series, err := collectSeries(found, len(ids))
	if err != nil {
		return nil, false, fmt.Errorf("searching series: %w", err)
	}
	return series, truncated, nil
}

// matchingIDs walks a candidate projection of (id, title, alternate title,
// year, person) and keeps the rows whose folded text contains the needle.
//
// Those four fields are what somebody actually types: the title the file gave
// it, the title TMDB gave it — which is how a film stored under its original
// language is found under the local one, and the reverse — the year, and the
// one name attached to the work.
func matchingIDs(rows *sql.Rows, needle string) ([]any, bool, error) {
	defer rows.Close()

	var (
		ids       []any
		truncated bool
	)
	for rows.Next() {
		var (
			id                      int64
			title                   string
			alternate, year, person sql.NullString
		)
		if err := rows.Scan(&id, &title, &alternate, &year, &person); err != nil {
			return nil, false, err
		}
		haystack := searchKey(strings.Join([]string{
			title, alternate.String, year.String, person.String,
		}, " "))
		if !strings.Contains(haystack, needle) {
			continue
		}
		if len(ids) == maxSearchResults {
			truncated = true
			break
		}
		ids = append(ids, id)
	}
	return ids, truncated, rows.Err()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

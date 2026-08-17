package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"strings"
)

// ErrNoSuchMovie is returned when an id matches nothing.
var ErrNoSuchMovie = errors.New("no such film")

// movieColumns is the projection every read shares, so that adding a field is
// one edit rather than four.
//
// Duration comes from the film; position, watched_at and finished come from the
// viewer's own row and are absent for a film this profile has never opened --
// hence the COALESCE rather than a NULL-scanning dance in Go.
const movieColumns = `
	m.id, m.path, m.file_name, m.size_bytes, m.modified_at, m.title, m.year,
	m.added_at, m.updated_at,` + metadataColumns + `,
	m.duration_seconds,
	COALESCE(p.position_seconds, 0), COALESCE(p.watched_at, 0), COALESCE(p.finished, 0)`

// metadataColumns is the TMDB half of that projection, in the order
// metadataRow.targets() scans it. Change one and change the other.
const metadataColumns = `
	m.tmdb_id, m.tmdb_title, m.original_title, m.original_language, m.tagline,
	m.overview, m.release_date, m.poster_path, m.backdrop_path,
	m.director, m.genres_json, m.cast_json, m.crew_json,
	m.certification, m.certification_country,
	m.collection_id, m.collection_name, m.collection_poster_path,
	m.runtime_minutes, m.vote_average, m.metadata_status, m.metadata_fetched_at`

// movieSource carries the join to one profile's history. Every query built on
// it takes the profile id as its first argument -- get that order wrong and the
// film ids shift by one, which is the kind of mistake that reads as correct.
const movieSource = `
	FROM movies m
	LEFT JOIN movie_progress p ON p.movie_id = m.id AND p.profile_id = ?`

// scanMovie reads one row projected with movieColumns.
func scanMovie(row interface{ Scan(...any) error }) (Movie, error) {
	var (
		m                              Movie
		meta                           metadataRow
		modifiedAt, addedAt, updatedAt int64
		year                           sql.NullInt64
		duration                       sql.NullFloat64
		position                       float64
		watchedAt                      int64
		finished                       int
	)
	// Built in three parts because the middle one is shared: the metadata block
	// is the same in every projection and knows its own order.
	targets := []any{&m.ID, &m.Path, &m.FileName, &m.SizeBytes, &modifiedAt,
		&m.Title, &year, &addedAt, &updatedAt}
	targets = append(targets, meta.targets()...)
	targets = append(targets, &duration, &position, &watchedAt, &finished)

	if err := row.Scan(targets...); err != nil {
		return Movie{}, err
	}
	m.ModifiedAt = unix(modifiedAt)
	m.AddedAt = unix(addedAt)
	m.UpdatedAt = unix(updatedAt)
	if year.Valid {
		m.Year = int(year.Int64)
	}
	meta.fill(&m)

	m.Progress = Progress{
		PositionSeconds: position,
		DurationSeconds: duration.Float64,
		Finished:        finished != 0,
	}
	if watchedAt > 0 {
		at := unix(watchedAt)
		m.Progress.WatchedAt = &at
	}
	return m, nil
}

func collectMovies(rows *sql.Rows, capacity int) ([]Movie, error) {
	defer rows.Close()
	out := make([]Movie, 0, capacity)
	for rows.Next() {
		m, err := scanMovie(rows)
		if err != nil {
			return nil, err
		}
		forList(&m.Metadata)
		out = append(out, m)
	}
	return out, rows.Err()
}

// forList drops what only a detail page shows.
//
// Every list read comes through collectMovies and every single-film read does
// not, which makes this the one place the distinction can be made without a
// second projection -- and a second projection is exactly what must be avoided
// here: the column list and the scan order are already paired by hand, and a
// third pairing is a shifted field waiting to happen.
//
// It is about the wire, not the query. Reading a few more columns out of a local
// SQLite file costs nothing; sending the cast, the crew, the tagline and the
// certificate of two hundred and fifty films to draw cards that show a title and
// a year is 31% of that response, measured, for data no list view reads. Decision
// 74 is the standard: text answers travel compressed, and the ones that travel
// are the ones somebody asked for.
//
// What stays is what a card, a filter or a sort actually uses: the artwork, the
// title, the year, the runtime, the rating, the genres, the director and the
// synopsis the hero prints.
func forList(m *Metadata) {
	m.Cast = nil
	m.Crew = nil
	m.Tagline = ""
	m.OriginalTitle = ""
	m.OriginalLanguage = ""
	m.Certification = ""
	m.CertificationCountry = ""
	m.Collection = nil
}

// Get returns one film by id, with the progress belonging to profileID.
func (s *Store) Get(ctx context.Context, profileID, id int64) (Movie, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+movieColumns+movieSource+` WHERE m.id = ?`, profileID, id)

	m, err := scanMovie(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNoSuchMovie
	}
	if err != nil {
		return Movie{}, fmt.Errorf("reading film %d: %w", id, err)
	}
	m.Files, err = s.MovieFiles(ctx, id)
	if err != nil {
		return Movie{}, err
	}
	if group := m.Metadata.Collection; group != nil {
		m.CollectionParts, err = s.CollectionParts(ctx, profileID, id, group.TMDBID)
		if err != nil {
			return Movie{}, err
		}
	}
	return m, nil
}

// heroMinimumRating is the quality floor for the film that headlines the home
// screen. Everything below it can still be in the library; it just does not get
// to be the first thing anybody sees.
const heroMinimumRating = 6.0

// Hero picks the film to headline the home screen.
//
// "Most recently added" alone does not work, and the reason is worth writing
// down: on a first scan every row is inserted within the same second, so
// added_at is identical across the entire library and the ordering collapses to
// insertion order. On a real library that put a mis-identified junk file on the
// front page.
//
// So the ordering is recency *then* rating, and there is a quality floor. On a
// first scan, where recency says nothing, the best-rated film wins. Afterwards a
// genuinely new film takes the slot, provided it is worth showing.
//
// A backdrop and a synopsis are required outright: a hero without artwork is a
// hole, and one without text is a title floating in the dark.
func (s *Store) Hero(ctx context.Context, profileID int64) (Movie, error) {
	const query = `
		SELECT ` + movieColumns + movieSource + `
		WHERE m.backdrop_path IS NOT NULL AND m.backdrop_path != ''
		  AND m.overview IS NOT NULL AND m.overview != ''
		  AND COALESCE(m.vote_average, 0) >= ?
		ORDER BY m.added_at DESC, m.vote_average DESC, m.id DESC
		LIMIT 1`

	m, err := scanMovie(s.db.QueryRowContext(ctx, query, profileID, heroMinimumRating))
	if errors.Is(err, sql.ErrNoRows) {
		// A small or poorly rated library still deserves a hero; drop the floor
		// rather than show nothing.
		m, err = scanMovie(s.db.QueryRowContext(ctx, query, profileID, 0.0))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNoSuchMovie
	}
	if err != nil {
		return Movie{}, fmt.Errorf("choosing a hero film: %w", err)
	}
	// The hero is one film but it is not a detail page: it shows artwork, a
	// title, the genres, the director and the synopsis. Same rule as a list.
	forList(&m.Metadata)
	return m, nil
}

// RecentlyAdded returns the newest films first.
func (s *Store) RecentlyAdded(ctx context.Context, profileID int64, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+movieSource+`
		ORDER BY m.added_at DESC, m.id DESC
		LIMIT ?`, profileID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent films: %w", err)
	}
	return collectMovies(rows, limit)
}

// TopRated returns the best-rated films first.
//
// A poster is required: this row exists to be looked at, and a placeholder card
// among nine posters reads as a fault rather than as a film.
//
// TMDB's vote count is not stored, so a film rated 8.9 by twelve people ranks
// alongside one rated 8.9 by twelve thousand. That is tolerable here and would
// not be on a public catalogue: this is somebody's own shelf of a few hundred
// films they chose to keep, not the whole of TMDB. If it ever needs fixing, the
// fix is to store vote_count, not to invent a weighting from what we have.
func (s *Store) TopRated(ctx context.Context, profileID int64, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+movieSource+`
		WHERE m.poster_path IS NOT NULL AND m.poster_path != ''
		  AND COALESCE(m.vote_average, 0) > 0
		ORDER BY m.vote_average DESC, m.added_at DESC, m.id DESC
		LIMIT ?`, profileID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing the best rated films: %w", err)
	}
	return collectMovies(rows, limit)
}

// Tonight returns a suggestion: films not yet finished, shuffled by a seed the
// caller supplies.
//
// Stability is the point. ORDER BY RANDOM() reshuffles on every page load, which
// turns a suggestion into a slot machine — you would reload until you liked the
// answer, and the row would mean nothing. Seeded with the date, the selection
// holds for the evening and turns over tomorrow.
//
// The shuffle is done here rather than in SQL, in two queries: the eligible ids,
// then the chosen rows. That is deliberate, after an ORDER BY arithmetic trick
// produced a stable order that was not remotely random. Sorting by (id * k) mod
// p and taking the first twelve selects ids at a fixed stride, so on the real
// 274-film library the row came back as 257, 234, 211, 188 … — every
// twenty-third film, every time. It reads as random only until you look at it.
//
// Two earlier attempts failed more visibly, and are worth recording because they
// look right on the page: adding the seed shifts every row equally and so almost
// never changes the order, and multiplying by a raw date seed never reaches the
// modulus for a few hundred ids, leaving plain id order. A real shuffle has no
// such edges.
func (s *Store) Tonight(ctx context.Context, profileID int64, limit int, seed int64) ([]Movie, error) {
	ids, err := s.eligibleForTonight(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	shuffler := rand.New(rand.NewSource(seed))
	shuffler.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return s.byIDs(ctx, profileID, ids)
}

// eligibleForTonight lists what may be suggested: nothing already watched to the
// end, and nothing without a poster, since this row exists to be looked at.
func (s *Store) eligibleForTonight(ctx context.Context, profileID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id
		`+movieSource+`
		WHERE COALESCE(p.finished, 0) = 0
		  AND m.poster_path IS NOT NULL AND m.poster_path != ''
		ORDER BY m.id`, profileID)
	if err != nil {
		return nil, fmt.Errorf("listing films eligible for tonight: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("listing films eligible for tonight: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// byIDs fetches films by id and returns them in the order asked for, which SQL
// will not do on its own.
func (s *Store) byIDs(ctx context.Context, profileID int64, ids []int64) ([]Movie, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// The profile id leads, because the join is written before the WHERE.
	args := make([]any, 0, len(ids)+1)
	args = append(args, profileID)
	for _, id := range ids {
		args = append(args, id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+movieColumns+movieSource+` WHERE m.id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("reading films by id: %w", err)
	}
	found, err := collectMovies(rows, len(ids))
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]Movie, len(found))
	for _, m := range found {
		byID[m.ID] = m
	}
	out := make([]Movie, 0, len(ids))
	for _, id := range ids {
		if m, ok := byID[id]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// ResumeHero returns the most recently watched unfinished film, provided it can
// carry a hero: artwork and a synopsis, the same bar Hero applies.
//
// Separate from ContinueWatching because the requirements differ. The row can
// show a film with no backdrop — it is a poster among posters. The hero cannot:
// a hero without artwork is a hole in the top of the screen.
func (s *Store) ResumeHero(ctx context.Context, profileID int64) (Movie, error) {
	const query = `
		SELECT ` + movieColumns + movieSource + `
		WHERE p.finished = 0
		  AND p.watched_at > 0
		  AND p.position_seconds >= ?
		  AND m.backdrop_path IS NOT NULL AND m.backdrop_path != ''
		  AND m.overview IS NOT NULL AND m.overview != ''
		ORDER BY p.watched_at DESC, m.id DESC
		LIMIT 1`

	m, err := scanMovie(s.db.QueryRowContext(ctx, query, profileID, minimumRememberedSeconds))
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNoSuchMovie
	}
	if err != nil {
		return Movie{}, fmt.Errorf("choosing a resume hero: %w", err)
	}
	forList(&m.Metadata)
	return m, nil
}

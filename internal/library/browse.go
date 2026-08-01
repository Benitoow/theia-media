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
const movieColumns = `
	id, path, file_name, size_bytes, modified_at, title, year,
	added_at, updated_at,
	tmdb_id, tmdb_title, overview, release_date, poster_path,
	backdrop_path, director, genres_json, cast_json,
	runtime_minutes, vote_average, metadata_status, metadata_fetched_at,
	duration_seconds, position_seconds, watched_at, finished`

// scanMovie reads one row projected with movieColumns.
func scanMovie(row interface{ Scan(...any) error }) (Movie, error) {
	var (
		m                              Movie
		modifiedAt, addedAt, updatedAt int64
		year, tmdbID, runtime          sql.NullInt64
		vote                           sql.NullFloat64
		status                         string
		fetchedAt                      int64
		tmdbTitle, overview, release   sql.NullString
		poster, backdrop, director     sql.NullString
		genresJSON, castJSON           sql.NullString
		duration                       sql.NullFloat64
		position                       float64
		watchedAt                      int64
		finished                       int
	)
	if err := row.Scan(&m.ID, &m.Path, &m.FileName, &m.SizeBytes, &modifiedAt,
		&m.Title, &year, &addedAt, &updatedAt,
		&tmdbID, &tmdbTitle, &overview, &release, &poster,
		&backdrop, &director, &genresJSON, &castJSON,
		&runtime, &vote, &status, &fetchedAt,
		&duration, &position, &watchedAt, &finished); err != nil {
		return Movie{}, err
	}
	m.ModifiedAt = unix(modifiedAt)
	m.AddedAt = unix(addedAt)
	m.UpdatedAt = unix(updatedAt)
	if year.Valid {
		m.Year = int(year.Int64)
	}
	scanMetadata(&m, tmdbID, tmdbTitle, overview, release, poster, backdrop,
		director, genresJSON, castJSON, runtime, vote, status, fetchedAt)

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
		out = append(out, m)
	}
	return out, rows.Err()
}

// Get returns one film by id.
func (s *Store) Get(ctx context.Context, id int64) (Movie, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+movieColumns+` FROM movies WHERE id = ?`, id)

	m, err := scanMovie(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNoSuchMovie
	}
	if err != nil {
		return Movie{}, fmt.Errorf("reading film %d: %w", id, err)
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
func (s *Store) Hero(ctx context.Context) (Movie, error) {
	const query = `
		SELECT ` + movieColumns + `
		FROM movies
		WHERE backdrop_path IS NOT NULL AND backdrop_path != ''
		  AND overview IS NOT NULL AND overview != ''
		  AND COALESCE(vote_average, 0) >= ?
		ORDER BY added_at DESC, vote_average DESC, id DESC
		LIMIT 1`

	m, err := scanMovie(s.db.QueryRowContext(ctx, query, heroMinimumRating))
	if errors.Is(err, sql.ErrNoRows) {
		// A small or poorly rated library still deserves a hero; drop the floor
		// rather than show nothing.
		m, err = scanMovie(s.db.QueryRowContext(ctx, query, 0.0))
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNoSuchMovie
	}
	if err != nil {
		return Movie{}, fmt.Errorf("choosing a hero film: %w", err)
	}
	return m, nil
}

// RecentlyAdded returns the newest films first.
func (s *Store) RecentlyAdded(ctx context.Context, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		FROM movies
		ORDER BY added_at DESC, id DESC
		LIMIT ?`, limit)
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
func (s *Store) TopRated(ctx context.Context, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		FROM movies
		WHERE poster_path IS NOT NULL AND poster_path != ''
		  AND COALESCE(vote_average, 0) > 0
		ORDER BY vote_average DESC, added_at DESC, id DESC
		LIMIT ?`, limit)
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
func (s *Store) Tonight(ctx context.Context, limit int, seed int64) ([]Movie, error) {
	ids, err := s.eligibleForTonight(ctx)
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
	return s.byIDs(ctx, ids)
}

// eligibleForTonight lists what may be suggested: nothing already watched to the
// end, and nothing without a poster, since this row exists to be looked at.
func (s *Store) eligibleForTonight(ctx context.Context) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM movies
		WHERE finished = 0
		  AND poster_path IS NOT NULL AND poster_path != ''
		ORDER BY id`)
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
func (s *Store) byIDs(ctx context.Context, ids []int64) ([]Movie, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+movieColumns+` FROM movies WHERE id IN (`+placeholders+`)`, args...)
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
func (s *Store) ResumeHero(ctx context.Context) (Movie, error) {
	const query = `
		SELECT ` + movieColumns + `
		FROM movies
		WHERE finished = 0
		  AND watched_at > 0
		  AND position_seconds >= ?
		  AND backdrop_path IS NOT NULL AND backdrop_path != ''
		  AND overview IS NOT NULL AND overview != ''
		ORDER BY watched_at DESC, id DESC
		LIMIT 1`

	m, err := scanMovie(s.db.QueryRowContext(ctx, query, minimumRememberedSeconds))
	if errors.Is(err, sql.ErrNoRows) {
		return Movie{}, ErrNoSuchMovie
	}
	if err != nil {
		return Movie{}, fmt.Errorf("choosing a resume hero: %w", err)
	}
	return m, nil
}

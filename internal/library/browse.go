package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// Genre is one genre and how many films carry it.
type Genre struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Genres returns the genres present in the library, most populated first.
//
// Genres are stored as a JSON array on the movie row rather than in a join
// table. json_each unrolls them, which keeps the schema simple and is entirely
// fast enough: this runs once per home-screen load over a few thousand rows.
// If the library ever outgrows that, this query is the thing to replace, not
// the storage.
func (s *Store) Genres(ctx context.Context, limit int) ([]Genre, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT je.value AS genre, COUNT(*) AS n
		FROM movies m, json_each(m.genres_json) je
		WHERE m.genres_json IS NOT NULL
		  AND m.genres_json != ''
		  AND m.genres_json != 'null'
		GROUP BY je.value
		HAVING n >= 3
		ORDER BY n DESC, genre
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing genres: %w", err)
	}
	defer rows.Close()

	var out []Genre
	for rows.Next() {
		var g Genre
		if err := rows.Scan(&g.Name, &g.Count); err != nil {
			return nil, fmt.Errorf("listing genres: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ByGenre returns films carrying a genre, best rated first so that the start of
// a row is worth looking at.
func (s *Store) ByGenre(ctx context.Context, genre string, limit int) ([]Movie, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieColumns+`
		FROM movies m
		WHERE EXISTS (
			SELECT 1 FROM json_each(m.genres_json) je WHERE je.value = ?
		)
		ORDER BY vote_average DESC, title COLLATE NOCASE
		LIMIT ?`, genre, limit)
	if err != nil {
		return nil, fmt.Errorf("listing films in %s: %w", genre, err)
	}
	return collectMovies(rows, limit)
}

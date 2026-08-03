package library

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type consolidationMovie struct {
	ID              int64
	Title           string
	Year            sql.NullInt64
	TMDBID          sql.NullInt64
	MetadataStatus  string
	MetadataFetched int64
	Duration        sql.NullFloat64
	Position        float64
	WatchedAt       int64
	Finished        int
	AddedAt         int64
	UpdatedAt       int64
	FirstSeen       int64
	LastSeen        int64
	FileName        string
}

// Consolidate merges only identities that can be proved from data Theia
// already owns: the same non-zero TMDB id, or the same parsed title and year
// without conflicting TMDB ids. The oldest movie id survives. This method is
// safe to run after every scan and on every startup.
func (s *Store) Consolidate(ctx context.Context) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("consolidating duplicate films: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	movies, err := consolidationMoviesTx(ctx, tx)
	if err != nil {
		return 0, err
	}
	if len(movies) < 2 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("consolidating duplicate films: %w", err)
		}
		return 0, nil
	}

	parent := make(map[int64]int64, len(movies))
	byID := make(map[int64]consolidationMovie, len(movies))
	for _, movie := range movies {
		parent[movie.ID] = movie.ID
		byID[movie.ID] = movie
	}
	find := func(id int64) int64 { return id }
	find = func(id int64) int64 {
		root := id
		for parent[root] != root {
			root = parent[root]
		}
		for parent[id] != id {
			next := parent[id]
			parent[id] = root
			id = next
		}
		return root
	}
	union := func(a, b int64) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		if ra < rb {
			parent[rb] = ra
		} else {
			parent[ra] = rb
		}
	}

	byTMDB := map[int64][]int64{}
	for _, movie := range movies {
		if movie.TMDBID.Valid && movie.TMDBID.Int64 > 0 {
			byTMDB[movie.TMDBID.Int64] = append(byTMDB[movie.TMDBID.Int64], movie.ID)
		}
	}
	for _, ids := range byTMDB {
		for i := 1; i < len(ids); i++ {
			union(ids[0], ids[i])
		}
	}

	type parsedKey struct {
		title string
		year  int64
	}
	byParsed := map[parsedKey][]int64{}
	for _, movie := range movies {
		if movie.Year.Valid && movie.Year.Int64 > 0 {
			key := parsedKey{title: normalizedTitle(movie.Title), year: movie.Year.Int64}
			byParsed[key] = append(byParsed[key], movie.ID)
		}
	}
	for _, ids := range byParsed {
		if len(ids) < 2 {
			continue
		}
		tmdbIDs := map[int64]bool{}
		for _, id := range ids {
			if tmdbID := byID[id].TMDBID; tmdbID.Valid && tmdbID.Int64 > 0 {
				tmdbIDs[tmdbID.Int64] = true
			}
		}
		if len(tmdbIDs) <= 1 {
			for i := 1; i < len(ids); i++ {
				union(ids[0], ids[i])
			}
		}
	}

	// The original M1 product rule also groups identical base names when no
	// year is available. Keep it exact apart from case and surrounding spaces.
	byStem := map[string][]int64{}
	for _, movie := range movies {
		if movie.Year.Valid {
			continue
		}
		stem := strings.TrimSuffix(movie.FileName, filepath.Ext(movie.FileName))
		byStem[strings.ToLower(strings.TrimSpace(stem))] = append(
			byStem[strings.ToLower(strings.TrimSpace(stem))], movie.ID)
	}
	for _, ids := range byStem {
		if len(ids) < 2 {
			continue
		}
		tmdbIDs := map[int64]bool{}
		for _, id := range ids {
			if tmdbID := byID[id].TMDBID; tmdbID.Valid && tmdbID.Int64 > 0 {
				tmdbIDs[tmdbID.Int64] = true
			}
		}
		if len(tmdbIDs) <= 1 {
			for i := 1; i < len(ids); i++ {
				union(ids[0], ids[i])
			}
		}
	}

	groups := map[int64][]int64{}
	for _, movie := range movies {
		root := find(movie.ID)
		groups[root] = append(groups[root], movie.ID)
	}
	roots := make([]int64, 0, len(groups))
	for root, ids := range groups {
		if len(ids) > 1 {
			roots = append(roots, root)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })

	merged := 0
	for _, root := range roots {
		ids := groups[root]
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		if err := mergeMovieGroupTx(ctx, tx, ids, byID); err != nil {
			return 0, err
		}
		merged += len(ids) - 1
	}
	if err := repairPrimariesTx(ctx, tx); err != nil {
		return 0, fmt.Errorf("consolidating duplicate films: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("consolidating duplicate films: %w", err)
	}
	return merged, nil
}

func consolidationMoviesTx(ctx context.Context, tx *sql.Tx) ([]consolidationMovie, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, title, year, tmdb_id, metadata_status, metadata_fetched_at,
		       duration_seconds, position_seconds, watched_at, finished,
		       added_at, updated_at, first_seen_scan, last_seen_scan, file_name
		FROM movies ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("reading films for consolidation: %w", err)
	}
	defer rows.Close()

	var movies []consolidationMovie
	for rows.Next() {
		var movie consolidationMovie
		if err := rows.Scan(
			&movie.ID, &movie.Title, &movie.Year, &movie.TMDBID,
			&movie.MetadataStatus, &movie.MetadataFetched,
			&movie.Duration, &movie.Position, &movie.WatchedAt, &movie.Finished,
			&movie.AddedAt, &movie.UpdatedAt, &movie.FirstSeen, &movie.LastSeen,
			&movie.FileName,
		); err != nil {
			return nil, fmt.Errorf("reading films for consolidation: %w", err)
		}
		movies = append(movies, movie)
	}
	return movies, rows.Err()
}

func mergeMovieGroupTx(ctx context.Context, tx *sql.Tx, ids []int64,
	byID map[int64]consolidationMovie,
) error {
	canonicalID := ids[0]
	canonical := byID[canonicalID]

	identitySource := canonical
	metadataSource := canonical
	progressSource := canonical
	addedAt, updatedAt := canonical.AddedAt, canonical.UpdatedAt
	firstSeen, lastSeen := canonical.FirstSeen, canonical.LastSeen
	bestDuration := canonical.Duration

	for _, id := range ids {
		movie := byID[id]
		if !identitySource.Year.Valid && movie.Year.Valid {
			identitySource = movie
		}
		if movie.MetadataStatus == statusOK &&
			(metadataSource.MetadataStatus != statusOK || movie.MetadataFetched > metadataSource.MetadataFetched) {
			metadataSource = movie
		}
		if movie.WatchedAt > progressSource.WatchedAt ||
			(movie.WatchedAt == progressSource.WatchedAt && movie.Position > progressSource.Position) {
			progressSource = movie
		}
		if movie.Duration.Valid && (!bestDuration.Valid || movie.Duration.Float64 > bestDuration.Float64) {
			bestDuration = movie.Duration
		}
		if movie.AddedAt < addedAt {
			addedAt = movie.AddedAt
		}
		if movie.UpdatedAt > updatedAt {
			updatedAt = movie.UpdatedAt
		}
		if movie.FirstSeen < firstSeen {
			firstSeen = movie.FirstSeen
		}
		if movie.LastSeen > lastSeen {
			lastSeen = movie.LastSeen
		}
	}

	var identityYear any
	if identitySource.Year.Valid {
		identityYear = identitySource.Year.Int64
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET title = ?, year = ?,
			added_at = ?, updated_at = ?, first_seen_scan = ?, last_seen_scan = ?
		WHERE id = ?`, identitySource.Title, identityYear,
		addedAt, updatedAt, firstSeen, lastSeen, canonicalID); err != nil {
		return fmt.Errorf("merging identity into film %d: %w", canonicalID, err)
	}

	if metadataSource.MetadataStatus == statusOK {
		if _, err := tx.ExecContext(ctx, `
			UPDATE movies SET
				tmdb_id = source.tmdb_id,
				tmdb_title = source.tmdb_title,
				overview = source.overview,
				release_date = source.release_date,
				poster_path = source.poster_path,
				backdrop_path = source.backdrop_path,
				runtime_minutes = source.runtime_minutes,
				vote_average = source.vote_average,
				director = source.director,
				genres_json = source.genres_json,
				cast_json = source.cast_json,
				metadata_status = source.metadata_status,
				metadata_fetched_at = source.metadata_fetched_at
			FROM movies AS source
			WHERE movies.id = ? AND source.id = ?`, canonicalID, metadataSource.ID); err != nil {
			return fmt.Errorf("merging metadata into film %d: %w", canonicalID, err)
		}
	}

	progressDuration := any(nil)
	if progressSource.Duration.Valid {
		progressDuration = progressSource.Duration.Float64
	} else if bestDuration.Valid {
		progressDuration = bestDuration.Float64
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET duration_seconds = ?, position_seconds = ?,
			watched_at = ?, finished = ?
		WHERE id = ?`, progressDuration, progressSource.Position,
		progressSource.WatchedAt, progressSource.Finished, canonicalID); err != nil {
		return fmt.Errorf("merging progress into film %d: %w", canonicalID, err)
	}

	// Every viewer's history moves with the film, not just the default one's.
	// The legacy columns above describe a single position, so on their own they
	// would silently drop the other profiles' rows when the duplicate is deleted
	// and its history cascades away with it.
	if err := mergeMovieProgressTx(ctx, tx, canonicalID, ids[1:]); err != nil {
		return err
	}

	for _, duplicateID := range ids[1:] {
		// A partial unique index allows one primary per film, so demote before
		// moving files under the canonical id.
		if _, err := tx.ExecContext(ctx,
			`UPDATE movie_files SET is_primary = 0 WHERE movie_id = ?`, duplicateID); err != nil {
			return fmt.Errorf("moving files into film %d: %w", canonicalID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE movie_files SET movie_id = ? WHERE movie_id = ?`, canonicalID, duplicateID); err != nil {
			return fmt.Errorf("moving files into film %d: %w", canonicalID, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM movies WHERE id = ?`, duplicateID); err != nil {
			return fmt.Errorf("removing duplicate film %d: %w", duplicateID, err)
		}
	}
	return nil
}

// mergeMovieProgressTx moves every profile's position from the duplicates onto
// the canonical film, most recently watched winning per profile.
//
// The conflict guard is what makes it per profile rather than global: two
// viewers who each watched a different copy of the same film both keep their
// own position, and neither overwrites the other.
func mergeMovieProgressTx(ctx context.Context, tx *sql.Tx, canonicalID int64, duplicates []int64) error {
	for _, duplicateID := range duplicates {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO movie_progress (profile_id, movie_id, position_seconds, watched_at, finished)
			SELECT profile_id, ?, position_seconds, watched_at, finished
			FROM movie_progress WHERE movie_id = ?
			ON CONFLICT(profile_id, movie_id) DO UPDATE SET
				position_seconds = excluded.position_seconds,
				watched_at       = excluded.watched_at,
				finished         = excluded.finished
			WHERE excluded.watched_at > movie_progress.watched_at`,
			canonicalID, duplicateID); err != nil {
			return fmt.Errorf("merging viewer history into film %d: %w", canonicalID, err)
		}
	}

	// The legacy columns follow the default profile, as everywhere else. They
	// were written above from the pre-merge legacy values; rewriting them here
	// keeps the mirror honest after the per-profile merge.
	if _, err := tx.ExecContext(ctx, `
		UPDATE movies SET
			position_seconds = COALESCE((SELECT position_seconds FROM movie_progress
				WHERE movie_id = ? AND profile_id = (SELECT id FROM profiles ORDER BY id LIMIT 1)), 0),
			watched_at = COALESCE((SELECT watched_at FROM movie_progress
				WHERE movie_id = ? AND profile_id = (SELECT id FROM profiles ORDER BY id LIMIT 1)), 0),
			finished = COALESCE((SELECT finished FROM movie_progress
				WHERE movie_id = ? AND profile_id = (SELECT id FROM profiles ORDER BY id LIMIT 1)), 0)
		WHERE id = ?`, canonicalID, canonicalID, canonicalID, canonicalID); err != nil {
		return fmt.Errorf("mirroring merged history for film %d: %w", canonicalID, err)
	}
	return nil
}

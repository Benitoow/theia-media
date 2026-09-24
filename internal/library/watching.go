package library

import (
	"context"
	"fmt"
	"time"
)

// topSeriesLimit is how many lines the ranking has when a caller does not say.
const topSeriesLimit = 5

// Watched is how much of one kind of thing a viewer has watched.
//
// Finished follows the player's own rule (finishedRule) rather than a threshold
// invented here: a film is finished when the viewer reached the last two
// minutes or five per cent of it, and a series when every one of its files is.
// Started counts what they opened at all, which is the number that moves first
// on a young library.
type Watched struct {
	Started  int     `json:"started"`
	Finished int     `json:"finished"`
	Seconds  float64 `json:"seconds"`
}

// SeriesWatch is one line of the ranking: a series and how much of it was
// watched.
//
// Episodes counts episodes, not files: a file holding S01E01E02 counts as two,
// which is why the counts are sums over episode_items rather than rows in
// episode_progress. A viewer asking "how much of this have I seen" means
// episodes, and the item table is the only place that knows a file holds more
// than one.
type SeriesWatch struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	// The TMDB poster path, so a caller can draw the series rather than only
	// naming it. Empty for a library that was never matched, which is a row
	// without a picture rather than a row that cannot be drawn.
	Poster   string  `json:"poster_path,omitempty"`
	Episodes int     `json:"episodes"`
	Finished int     `json:"finished"`
	Total    int     `json:"total"`
	Seconds  float64 `json:"seconds"`
}

// ThisMonth is the calendar month the server is in, counted the way the totals
// are: films and episodes whose *last* report falls inside it, and the time
// those positions hold.
//
// It is the last report per item and not a history, because the schema keeps
// one row per viewer and per item (see WatchStats). A film watched twice this
// month therefore counts once, and one watched in August and again in September
// counts in September only - which is what "this month" can honestly mean
// without a table that records each evening separately.
type ThisMonth struct {
	Movies   int     `json:"movies"`
	Episodes int     `json:"episodes"`
	Seconds  float64 `json:"seconds"`
}

// WatchStats is what the settings screen reports about watching, as opposed to
// what the library holds.
//
// Seconds are time actually spent: the positions the player reported, added up.
// Not the length of what was watched, which would count a film abandoned at the
// titles as two hours. The schema has one row per viewer and per item,
// overwritten on every report, and no history table, so rewatching a film moves
// its position instead of adding a second helping. These totals are therefore
// "how far into the library am I", not "how many hours has this machine been
// on", and anything else would need a table this schema does not have.
type WatchStats struct {
	Movies   Watched       `json:"movies"`
	Series   Watched       `json:"series"`
	Episodes Watched       `json:"episodes"`
	Month    ThisMonth     `json:"month"`
	Top      []SeriesWatch `json:"top_series"`
}

// WatchStats reports what one profile has watched.
//
// Separate from the store's other counts, which answer "how much is there";
// this one answers "how much of it was used", and only a profile can answer it.
// Everything is read in four counts and one ranking over the progress rows of
// that profile, so the work does not grow with the size of the library, only
// with what was watched.
func (s *Store) WatchStats(ctx context.Context, profileID int64, topLimit int) (WatchStats, error) {
	if topLimit <= 0 {
		topLimit = topSeriesLimit
	}

	stats := WatchStats{Top: []SeriesWatch{}}
	var episodeMonthSeconds float64
	// The month's window is the server's own: it is the household's machine, and
	// asking a client for its timezone to decide what "this month" means would
	// be a question no screen has ever needed to answer.
	now := time.Now()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()

	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN p.watched_at >= ? THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN p.watched_at >= ? THEN p.position_seconds ELSE 0 END), 0)
		FROM movie_progress p
		WHERE p.profile_id = ?`, since, since, profileID,
	).Scan(&stats.Month.Movies, &stats.Month.Seconds); err != nil {
		return WatchStats{}, fmt.Errorf("counting this month's films: %w", err)
	}

	// watched_at rather than a row count: the schema keeps a row for an opening
	// that was not worth remembering, with a zero position, so counting rows
	// would report a poster glimpsed as a film started. The timestamp is set
	// exactly when there is something to remember -- including when a viewer
	// marks a film watched without playing it.
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN p.watched_at > 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN p.finished = 1 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(p.position_seconds), 0)
		FROM movie_progress p
		WHERE p.profile_id = ?`, profileID,
	).Scan(&stats.Movies.Started, &stats.Movies.Finished, &stats.Movies.Seconds); err != nil {
		return WatchStats{}, fmt.Errorf("counting watched films: %w", err)
	}

	// Episodes, not files, all the way through: this is the number the ranking
	// shows as well, and the two disagreeing is how a viewer who opened a
	// two-episode file would be told they had watched one.
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN ep.watched_at > 0
		                         THEN i.last_episode - i.first_episode + 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN ep.finished = 1
		                         THEN i.last_episode - i.first_episode + 1 ELSE 0 END), 0),
		       COALESCE(SUM(ep.position_seconds), 0)
		FROM episode_progress ep
		JOIN episode_items i ON i.id = ep.episode_item_id
		WHERE ep.profile_id = ?`, profileID,
	).Scan(&stats.Episodes.Started, &stats.Episodes.Finished, &stats.Episodes.Seconds); err != nil {
		return WatchStats{}, fmt.Errorf("counting watched episodes: %w", err)
	}

	// Started series and their time in one pass. "Watched" is a property of the
	// whole series and cannot come from the same GROUP BY, so it is counted
	// below: a series is watched when every file in it is, which is a statement
	// about the items that were never opened as much as about the ones that
	// were.
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT se.series_id),
		       COALESCE(SUM(ep.position_seconds), 0)
		FROM episode_progress ep
		JOIN episode_items i ON i.id = ep.episode_item_id
		JOIN seasons se ON se.id = i.season_id
		WHERE ep.profile_id = ? AND ep.watched_at > 0`, profileID,
	).Scan(&stats.Series.Started, &stats.Series.Seconds); err != nil {
		return WatchStats{}, fmt.Errorf("counting started series: %w", err)
	}

	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT se.series_id
			FROM episode_items i
			JOIN seasons se ON se.id = i.season_id
			LEFT JOIN episode_progress ep
				ON ep.episode_item_id = i.id AND ep.profile_id = ?
			GROUP BY se.series_id
			HAVING COUNT(*) = SUM(CASE WHEN COALESCE(ep.finished, 0) = 1 THEN 1 ELSE 0 END)
		)`, profileID,
	).Scan(&stats.Series.Finished); err != nil {
		return WatchStats{}, fmt.Errorf("counting watched series: %w", err)
	}

	// Episodes watched this month are counted in episodes rather than in files,
	// as everywhere else, so the two numbers on that line mean the same thing.
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN ep.watched_at >= ?
		                         THEN i.last_episode - i.first_episode + 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN ep.watched_at >= ? THEN ep.position_seconds ELSE 0 END), 0)
		FROM episode_progress ep
		JOIN episode_items i ON i.id = ep.episode_item_id
		WHERE ep.profile_id = ?`, since, since, profileID,
	).Scan(&stats.Month.Episodes, &episodeMonthSeconds); err != nil {
		return WatchStats{}, fmt.Errorf("counting this month's episodes: %w", err)
	}
	stats.Month.Seconds += episodeMonthSeconds

	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id,
		       COALESCE(s.tmdb_name, s.title),
		       COALESCE(s.poster_path, ''),
		       COALESCE(SUM(i.last_episode - i.first_episode + 1), 0),
		       COALESCE(SUM(CASE WHEN ep.finished = 1
		                         THEN i.last_episode - i.first_episode + 1 ELSE 0 END), 0),
		       -- Every episode the series has, watched or not: the ranking is
		       -- read as "one of ten", and a denominator of what was already
		       -- watched would read as progress nobody has made.
		       (SELECT COALESCE(SUM(i2.last_episode - i2.first_episode + 1), 0)
		          FROM episode_items i2
		          JOIN seasons se2 ON se2.id = i2.season_id
		         WHERE se2.series_id = s.id),
		       COALESCE(SUM(ep.position_seconds), 0)
		FROM episode_progress ep
		JOIN episode_items i ON i.id = ep.episode_item_id
		JOIN seasons se ON se.id = i.season_id
		JOIN series s ON s.id = se.series_id
		WHERE ep.profile_id = ? AND ep.watched_at > 0
		GROUP BY s.id
		ORDER BY SUM(ep.position_seconds) DESC,
		         COALESCE(s.tmdb_name, s.title) COLLATE NOCASE,
		         s.id
		LIMIT ?`, profileID, topLimit)
	if err != nil {
		return WatchStats{}, fmt.Errorf("ranking watched series: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var one SeriesWatch
		if err := rows.Scan(&one.ID, &one.Title, &one.Poster, &one.Episodes, &one.Finished,
			&one.Total, &one.Seconds); err != nil {
			return WatchStats{}, fmt.Errorf("reading a watched series: %w", err)
		}
		stats.Top = append(stats.Top, one)
	}
	if err := rows.Err(); err != nil {
		return WatchStats{}, fmt.Errorf("reading watched series: %w", err)
	}

	return stats, nil
}

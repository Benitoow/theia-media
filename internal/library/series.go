package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNoSuchSeries      = errors.New("no such series")
	ErrNoSuchSeason      = errors.New("no such season")
	ErrNoSuchEpisodeItem = errors.New("no such episode")
	ErrNoSuchEpisodeFile = errors.New("no such episode file")
)

type SeriesMetadata struct {
	TMDBID       int       `json:"tmdb_id,omitempty"`
	Name         string    `json:"tmdb_name,omitempty"`
	OriginalName string    `json:"original_name,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	FirstAirDate string    `json:"first_air_date,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	BackdropPath string    `json:"backdrop_path,omitempty"`
	VoteAverage  float64   `json:"vote_average,omitempty"`
	Genres       []string  `json:"genres,omitempty"`
	Cast         []Credit  `json:"cast,omitempty"`
	Creators     []string  `json:"creators,omitempty"`
	Status       string    `json:"status"`
	FetchedAt    time.Time `json:"fetched_at,omitempty"`
}

type Series struct {
	Kind      string         `json:"kind"`
	ID        int64          `json:"id"`
	Title     string         `json:"title"`
	Year      int            `json:"year,omitempty"`
	Metadata  SeriesMetadata `json:"metadata"`
	Seasons   []Season       `json:"seasons,omitempty"`
	AddedAt   time.Time      `json:"added_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type SeasonMetadata struct {
	TMDBID       int       `json:"tmdb_id,omitempty"`
	Name         string    `json:"name,omitempty"`
	Overview     string    `json:"overview,omitempty"`
	AirDate      string    `json:"air_date,omitempty"`
	PosterPath   string    `json:"poster_path,omitempty"`
	EpisodeCount int       `json:"episode_count,omitempty"`
	Status       string    `json:"status"`
	FetchedAt    time.Time `json:"fetched_at,omitempty"`
}

type Season struct {
	ID        int64          `json:"id"`
	SeriesID  int64          `json:"series_id"`
	Number    int            `json:"season_number"`
	Metadata  SeasonMetadata `json:"metadata"`
	Items     []EpisodeItem  `json:"episodes,omitempty"`
	AddedAt   time.Time      `json:"added_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type EpisodeMetadata struct {
	TMDBID         int       `json:"tmdb_id,omitempty"`
	Name           string    `json:"name,omitempty"`
	Overview       string    `json:"overview,omitempty"`
	AirDate        string    `json:"air_date,omitempty"`
	StillPath      string    `json:"still_path,omitempty"`
	RuntimeMinutes int       `json:"runtime_minutes,omitempty"`
	VoteAverage    float64   `json:"vote_average,omitempty"`
	Status         string    `json:"status"`
	FetchedAt      time.Time `json:"fetched_at,omitempty"`
}

type Episode struct {
	ID         int64           `json:"id"`
	Number     int             `json:"episode_number"`
	LocalTitle string          `json:"local_title,omitempty"`
	Metadata   EpisodeMetadata `json:"metadata"`
}

// EpisodeItem is the local playable unit. It usually has one Episode, but a
// file named S01E01E02 owns one item with two ordered members and one progress
// position. Multiple encodes of the same member set appear under Files.
type EpisodeItem struct {
	Kind           string        `json:"kind"`
	ID             int64         `json:"id"`
	SeriesID       int64         `json:"series_id"`
	SeriesTitle    string        `json:"series_title"`
	SeasonID       int64         `json:"season_id"`
	SeasonNumber   int           `json:"season_number"`
	EpisodeNumbers []int         `json:"episode_numbers"`
	Episodes       []Episode     `json:"episode_metadata"`
	Files          []EpisodeFile `json:"files,omitempty"`
	Progress       Progress      `json:"progress"`
	NextEpisodeID  *int64        `json:"next_episode_id,omitempty"`
	NextHasGap     bool          `json:"next_has_gap,omitempty"`
	AddedAt        time.Time     `json:"added_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type EpisodeFile struct {
	ID            int64     `json:"id"`
	EpisodeItemID int64     `json:"-"`
	Path          string    `json:"-"`
	FileName      string    `json:"file_name"`
	SizeBytes     int64     `json:"size_bytes"`
	ModifiedAt    time.Time `json:"modified_at"`
	Extension     string    `json:"extension"`
	IsPrimary     bool      `json:"is_primary"`
	Media         FileMedia `json:"media"`
}

type SeriesHome struct {
	Continue []EpisodeItem `json:"continue_watching"`
	Recent   []Series      `json:"recent_series"`
}

const seriesColumns = `
	id, title, year,
	tmdb_id, tmdb_name, original_name, overview, first_air_date,
	poster_path, backdrop_path, vote_average, genres_json, cast_json, creators_json,
	metadata_status, metadata_fetched_at, added_at, updated_at`

const seriesColumnsAliased = `
	se.id, se.title, se.year,
	se.tmdb_id, se.tmdb_name, se.original_name, se.overview, se.first_air_date,
	se.poster_path, se.backdrop_path, se.vote_average, se.genres_json, se.cast_json, se.creators_json,
	se.metadata_status, se.metadata_fetched_at, se.added_at, se.updated_at`

func scanSeries(row interface{ Scan(...any) error }) (Series, error) {
	var (
		series                                 Series
		year, tmdbID                           sql.NullInt64
		name, original, overview, firstAir     sql.NullString
		poster, backdrop, genresJSON, castJSON sql.NullString
		creatorsJSON                           sql.NullString
		vote                                   sql.NullFloat64
		fetchedAt, addedAt, updatedAt          int64
		status                                 string
	)
	if err := row.Scan(&series.ID, &series.Title, &year,
		&tmdbID, &name, &original, &overview, &firstAir,
		&poster, &backdrop, &vote, &genresJSON, &castJSON, &creatorsJSON,
		&status, &fetchedAt, &addedAt, &updatedAt); err != nil {
		return Series{}, err
	}
	series.Kind = "series"
	series.Year = int(year.Int64)
	series.Metadata = SeriesMetadata{
		TMDBID:       int(tmdbID.Int64),
		Name:         name.String,
		OriginalName: original.String,
		Overview:     overview.String,
		FirstAirDate: firstAir.String,
		PosterPath:   poster.String,
		BackdropPath: backdrop.String,
		VoteAverage:  vote.Float64,
		Status:       status,
	}
	_ = json.Unmarshal([]byte(genresJSON.String), &series.Metadata.Genres)
	_ = json.Unmarshal([]byte(castJSON.String), &series.Metadata.Cast)
	_ = json.Unmarshal([]byte(creatorsJSON.String), &series.Metadata.Creators)
	if fetchedAt > 0 {
		series.Metadata.FetchedAt = unix(fetchedAt)
	}
	series.AddedAt = unix(addedAt)
	series.UpdatedAt = unix(updatedAt)
	return series, nil
}

func collectSeries(rows *sql.Rows, capacity int) ([]Series, error) {
	defer rows.Close()
	out := make([]Series, 0, capacity)
	for rows.Next() {
		series, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, series)
	}
	return out, rows.Err()
}

func (s *Store) SeriesCount(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM series`).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting series: %w", err)
	}
	return count, nil
}

func (s *Store) EpisodeItemCount(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episode_items`).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting episodes: %w", err)
	}
	return count, nil
}

func (s *Store) ListSeries(ctx context.Context, limit, offset int) ([]Series, error) {
	limit, offset = pageBounds(limit, offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+seriesColumns+` FROM series
		ORDER BY COALESCE(tmdb_name, title) COLLATE NOCASE, year, id
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing series: %w", err)
	}
	return collectSeries(rows, limit)
}

func pageBounds(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 100
	} else if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Store) GetSeries(ctx context.Context, profileID, id int64) (Series, error) {
	series, err := scanSeries(s.db.QueryRowContext(ctx,
		`SELECT `+seriesColumns+` FROM series WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Series{}, ErrNoSuchSeries
	}
	if err != nil {
		return Series{}, fmt.Errorf("reading series %d: %w", id, err)
	}
	series.Seasons, err = s.Seasons(ctx, profileID, id)
	if err != nil {
		return Series{}, err
	}
	return series, nil
}

const seasonColumns = `
	id, series_id, season_number, tmdb_id, name, overview, air_date, poster_path,
	episode_count, metadata_status, metadata_fetched_at, added_at, updated_at`

func scanSeason(row interface{ Scan(...any) error }) (Season, error) {
	var (
		season                          Season
		tmdbID, episodeCount            sql.NullInt64
		name, overview, airDate, poster sql.NullString
		status                          string
		fetchedAt, addedAt, updatedAt   int64
	)
	if err := row.Scan(&season.ID, &season.SeriesID, &season.Number,
		&tmdbID, &name, &overview, &airDate, &poster, &episodeCount,
		&status, &fetchedAt, &addedAt, &updatedAt); err != nil {
		return Season{}, err
	}
	season.Metadata = SeasonMetadata{
		TMDBID:       int(tmdbID.Int64),
		Name:         name.String,
		Overview:     overview.String,
		AirDate:      airDate.String,
		PosterPath:   poster.String,
		EpisodeCount: int(episodeCount.Int64),
		Status:       status,
	}
	if fetchedAt > 0 {
		season.Metadata.FetchedAt = unix(fetchedAt)
	}
	season.AddedAt = unix(addedAt)
	season.UpdatedAt = unix(updatedAt)
	return season, nil
}

func (s *Store) Seasons(ctx context.Context, profileID, seriesID int64) ([]Season, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+seasonColumns+` FROM seasons
		WHERE series_id = ? ORDER BY season_number`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("listing seasons for series %d: %w", seriesID, err)
	}
	defer rows.Close()
	var out []Season
	for rows.Next() {
		season, err := scanSeason(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, season)
	}
	return out, rows.Err()
}

func (s *Store) GetSeason(ctx context.Context, profileID, seriesID int64, number int) (Season, error) {
	season, err := scanSeason(s.db.QueryRowContext(ctx, `
		SELECT `+seasonColumns+` FROM seasons
		WHERE series_id = ? AND season_number = ?`, seriesID, number))
	if errors.Is(err, sql.ErrNoRows) {
		return Season{}, ErrNoSuchSeason
	}
	if err != nil {
		return Season{}, fmt.Errorf("reading season %d of series %d: %w", number, seriesID, err)
	}
	season.Items, err = s.EpisodeItemsForSeason(ctx, profileID, season.ID)
	if err != nil {
		return Season{}, err
	}
	return season, nil
}

// Progress joins in from the viewer's own table; the duration stays on the item
// because it describes the file. Same split as movieColumns, same reason.
const episodeItemColumns = `
	i.id, i.season_id, se.id, se.title, s.season_number,
	i.episode_key, i.duration_seconds,
	COALESCE(ep.position_seconds, 0), COALESCE(ep.watched_at, 0), COALESCE(ep.finished, 0),
	i.added_at, i.updated_at`

// episodeItemSource is the shared FROM. The profile id is always the first
// argument of a query built on it.
const episodeItemSource = `
	FROM episode_items i
	JOIN seasons s ON s.id = i.season_id
	JOIN series se ON se.id = s.series_id
	LEFT JOIN episode_progress ep ON ep.episode_item_id = i.id AND ep.profile_id = ?`

func scanEpisodeItem(row interface{ Scan(...any) error }) (EpisodeItem, error) {
	var (
		item               EpisodeItem
		key                string
		duration           sql.NullFloat64
		watchedAt, addedAt int64
		updatedAt          int64
		finished           int
	)
	if err := row.Scan(&item.ID, &item.SeasonID, &item.SeriesID, &item.SeriesTitle,
		&item.SeasonNumber, &key, &duration, &item.Progress.PositionSeconds,
		&watchedAt, &finished, &addedAt, &updatedAt); err != nil {
		return EpisodeItem{}, err
	}
	item.Kind = "episode"
	item.EpisodeNumbers = parseEpisodeKey(key)
	item.Progress.DurationSeconds = duration.Float64
	item.Progress.Finished = finished != 0
	if watchedAt > 0 {
		at := unix(watchedAt)
		item.Progress.WatchedAt = &at
	}
	item.AddedAt = unix(addedAt)
	item.UpdatedAt = unix(updatedAt)
	return item, nil
}

func parseEpisodeKey(key string) []int {
	parts := strings.Split(key, ",")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err == nil {
			out = append(out, value)
		}
	}
	return out
}

func (s *Store) EpisodeItemsForSeason(ctx context.Context, profileID, seasonID int64) ([]EpisodeItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+episodeItemColumns+episodeItemSource+`
		WHERE i.season_id = ?
		ORDER BY i.first_episode, i.last_episode, i.id`, profileID, seasonID)
	if err != nil {
		return nil, fmt.Errorf("listing episodes for season %d: %w", seasonID, err)
	}
	defer rows.Close()
	var out []EpisodeItem
	for rows.Next() {
		item, err := scanEpisodeItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Episodes, err = s.episodeMembers(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) GetEpisodeItem(ctx context.Context, profileID, id int64) (EpisodeItem, error) {
	item, err := scanEpisodeItem(s.db.QueryRowContext(ctx, `
		SELECT `+episodeItemColumns+episodeItemSource+`
		WHERE i.id = ?`, profileID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return EpisodeItem{}, ErrNoSuchEpisodeItem
	}
	if err != nil {
		return EpisodeItem{}, fmt.Errorf("reading episode %d: %w", id, err)
	}
	item.Episodes, err = s.episodeMembers(ctx, id)
	if err != nil {
		return EpisodeItem{}, err
	}
	item.Files, err = s.EpisodeFiles(ctx, id)
	if err != nil {
		return EpisodeItem{}, err
	}
	item.NextEpisodeID, item.NextHasGap, err = s.nextEpisode(ctx, item)
	if err != nil {
		return EpisodeItem{}, err
	}
	return item, nil
}

func (s *Store) episodeMembers(ctx context.Context, itemID int64) ([]Episode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.episode_number, e.local_title,
		       e.tmdb_id, e.name, e.overview, e.air_date, e.still_path,
		       e.runtime_minutes, e.vote_average, e.metadata_status, e.metadata_fetched_at
		FROM episode_item_members m
		JOIN episodes e ON e.id = m.episode_id
		WHERE m.episode_item_id = ? ORDER BY m.ordinal`, itemID)
	if err != nil {
		return nil, fmt.Errorf("reading episode metadata for item %d: %w", itemID, err)
	}
	defer rows.Close()
	var out []Episode
	for rows.Next() {
		var (
			episode                    Episode
			localTitle, name, overview sql.NullString
			airDate, still             sql.NullString
			tmdbID, runtime            sql.NullInt64
			vote                       sql.NullFloat64
			status                     string
			fetchedAt                  int64
		)
		if err := rows.Scan(&episode.ID, &episode.Number, &localTitle,
			&tmdbID, &name, &overview, &airDate, &still,
			&runtime, &vote, &status, &fetchedAt); err != nil {
			return nil, err
		}
		episode.LocalTitle = localTitle.String
		episode.Metadata = EpisodeMetadata{
			TMDBID:         int(tmdbID.Int64),
			Name:           name.String,
			Overview:       overview.String,
			AirDate:        airDate.String,
			StillPath:      still.String,
			RuntimeMinutes: int(runtime.Int64),
			VoteAverage:    vote.Float64,
			Status:         status,
		}
		if fetchedAt > 0 {
			episode.Metadata.FetchedAt = unix(fetchedAt)
		}
		out = append(out, episode)
	}
	return out, rows.Err()
}

func (s *Store) nextEpisode(ctx context.Context, current EpisodeItem) (*int64, bool, error) {
	if current.SeasonNumber == 0 || len(current.EpisodeNumbers) == 0 {
		return nil, false, nil
	}
	last := current.EpisodeNumbers[len(current.EpisodeNumbers)-1]
	var id int64
	var season, first int
	err := s.db.QueryRowContext(ctx, `
		SELECT i.id, s.season_number, i.first_episode
		FROM episode_items i
		JOIN seasons s ON s.id = i.season_id
		WHERE s.series_id = ? AND s.season_number > 0
		  AND (s.season_number > ? OR (s.season_number = ? AND i.first_episode > ?))
		ORDER BY s.season_number, i.first_episode, i.last_episode, i.id
		LIMIT 1`, current.SeriesID, current.SeasonNumber, current.SeasonNumber, last,
	).Scan(&id, &season, &first)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("finding the episode after %d: %w", current.ID, err)
	}
	hasGap := season != current.SeasonNumber || first != last+1
	if season == current.SeasonNumber+1 && first == 1 {
		hasGap = false
	}
	return &id, hasGap, nil
}

func (s *Store) ContinueEpisodes(ctx context.Context, profileID int64, limit int) ([]EpisodeItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ep.episode_item_id
		FROM episode_progress ep
		WHERE ep.profile_id = ? AND ep.finished = 0 AND ep.watched_at > 0
		  AND ep.position_seconds >= ?
		ORDER BY ep.watched_at DESC, ep.episode_item_id DESC LIMIT ?`,
		profileID, minimumRememberedSeconds, limit)
	if err != nil {
		return nil, fmt.Errorf("listing episodes in progress: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	out := make([]EpisodeItem, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetEpisodeItem(ctx, profileID, id)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) RecentSeries(ctx context.Context, limit int) ([]Series, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+seriesColumnsAliased+`
		FROM series se
		JOIN seasons s ON s.series_id = se.id
		JOIN episode_items i ON i.season_id = s.id
		GROUP BY se.id
		ORDER BY MAX(i.added_at) DESC, se.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing recent series: %w", err)
	}
	return collectSeries(rows, limit)
}

func (s *Store) SeriesHome(ctx context.Context, profileID int64, limit int) (*SeriesHome, error) {
	continued, err := s.ContinueEpisodes(ctx, profileID, limit)
	if err != nil {
		return nil, err
	}
	recent, err := s.RecentSeries(ctx, limit)
	if err != nil {
		return nil, err
	}
	return &SeriesHome{Continue: continued, Recent: recent}, nil
}

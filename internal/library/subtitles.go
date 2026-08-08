package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Benitoow/theia-media/internal/subtitles"
)

// ErrNoSuchSubtitleTrack covers an unknown id and an id belonging to another
// file, for the same reason ErrNoSuchMovieFile does: a caller must not be able
// to reach one film's tracks through another film's identifiers.
var ErrNoSuchSubtitleTrack = errors.New("no such subtitle track")

// SubtitleTrack is one way of reading a film.
//
// Two provenances share the type. An embedded track has a StreamIndex and is
// pulled out of the container by ffmpeg; an external one is a `.srt` beside the
// film and needs no ffmpeg at all. The interface shows both in one list because
// the difference is not the viewer's problem -- but Kind is, since a bitmap
// track cannot be shown at all (decision 3) and saying so is better than an
// empty menu.
type SubtitleTrack struct {
	ID          int64  `json:"id"`
	StreamIndex *int   `json:"stream_index,omitempty"`
	Codec       string `json:"codec"`
	Language    string `json:"language,omitempty"`
	Title       string `json:"title,omitempty"`
	IsDefault   bool   `json:"is_default"`
	IsForced    bool   `json:"is_forced"`
	IsExternal  bool   `json:"is_external"`

	// Kind is "text" or "image". Only text can be served.
	Kind string `json:"kind"`

	// Path is the external file on disk. It never crosses the HTTP boundary:
	// clients name a track by id and the server resolves the rest.
	Path string `json:"-"`
}

// Renderable reports whether this track can become WebVTT.
func (t SubtitleTrack) Renderable() bool { return t.Kind == string(subtitles.KindText) }

// subtitleTable names the two nearly identical tables. Films and episodes have
// separate tables for the same reason their files do -- a foreign key that
// actually constrains something -- and the queries below are shared rather than
// written twice and drifting apart.
type subtitleTable struct {
	name  string
	owner string
}

var (
	movieSubtitles   = subtitleTable{"movie_file_subtitle_tracks", "movie_file_id"}
	episodeSubtitles = subtitleTable{"episode_file_subtitle_tracks", "episode_file_id"}
)

func scanSubtitleRows(rows *sql.Rows) (map[int64][]SubtitleTrack, error) {
	byFile := make(map[int64][]SubtitleTrack)
	for rows.Next() {
		var (
			track                 SubtitleTrack
			fileID                int64
			streamIndex           sql.NullInt64
			path, language, title sql.NullString
			isDefault, isForced   int
		)
		if err := rows.Scan(&track.ID, &fileID, &streamIndex, &path, &track.Codec,
			&language, &title, &isDefault, &isForced); err != nil {
			return nil, err
		}
		if streamIndex.Valid {
			index := int(streamIndex.Int64)
			track.StreamIndex = &index
		}
		track.Path = path.String
		track.IsExternal = path.Valid
		track.Language = language.String
		track.Title = title.String
		track.IsDefault = isDefault != 0
		track.IsForced = isForced != 0
		track.Kind = string(subtitles.ClassifyCodec(track.Codec))
		byFile[fileID] = append(byFile[fileID], track)
	}
	return byFile, rows.Err()
}

const subtitleColumns = `id, %s, stream_index, source_path, codec, language, title, is_default, is_forced`

// loadSubtitles fills one batch of files in a single query.
func (s *Store) loadSubtitles(ctx context.Context, t subtitleTable, ids []int64) (map[int64][]SubtitleTrack, error) {
	if len(ids) == 0 {
		return map[int64][]SubtitleTrack{}, nil
	}
	args := make([]any, len(ids))
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		placeholders[i] = "?"
	}

	// Embedded tracks first, in container order, then the external files: a
	// person scanning the menu meets what the film itself carries before what
	// somebody dropped next to it.
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT `+subtitleColumns+`
		FROM %s
		WHERE %s IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY %s, source_path IS NOT NULL, stream_index, id`,
		t.owner, t.name, t.owner, t.owner), args...)
	if err != nil {
		return nil, fmt.Errorf("reading subtitle tracks: %w", err)
	}
	defer rows.Close()

	byFile, err := scanSubtitleRows(rows)
	if err != nil {
		return nil, fmt.Errorf("reading subtitle tracks: %w", err)
	}
	return byFile, nil
}

// saveEmbeddedSubtitlesTx replaces the embedded tracks of one file. External
// rows are untouched: they are discovered from the filesystem, not from ffmpeg,
// and an inspection must not delete them.
func saveEmbeddedSubtitlesTx(ctx context.Context, tx *sql.Tx, t subtitleTable, fileID int64, tracks []SubtitleTrack) error {
	seen := make(map[int]bool, len(tracks))
	for _, track := range tracks {
		if track.StreamIndex == nil {
			continue
		}
		seen[*track.StreamIndex] = true
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (%s, stream_index, source_path, codec, language, title, is_default, is_forced)
			VALUES (?, ?, NULL, ?, ?, ?, ?, ?)
			ON CONFLICT(%s, stream_index) WHERE stream_index IS NOT NULL DO UPDATE SET
				codec = excluded.codec,
				language = excluded.language,
				title = excluded.title,
				is_default = excluded.is_default,
				is_forced = excluded.is_forced`, t.name, t.owner, t.owner),
			fileID, *track.StreamIndex, track.Codec,
			nullString(track.Language), nullString(track.Title),
			boolInt(track.IsDefault), boolInt(track.IsForced)); err != nil {
			return fmt.Errorf("saving subtitle tracks for file %d: %w", fileID, err)
		}
	}
	return pruneSubtitlesTx(ctx, tx, t, fileID,
		`stream_index IS NOT NULL`, func(index sql.NullInt64, _ sql.NullString) bool {
			return !seen[int(index.Int64)]
		})
}

// SyncExternalSubtitles records the `.srt` files sitting beside a media file.
//
// This runs without ffmpeg on purpose: a browser-friendly MP4 with a subtitle
// file next to it must gain its subtitles without downloading 80 MB to find out
// what was already readable from the directory listing.
func (s *Store) SyncExternalSubtitles(ctx context.Context, t subtitleTable, fileID int64, found []subtitles.Sidecar) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("saving external subtitles: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	seen := make(map[string]bool, len(found))
	for _, sidecar := range found {
		seen[sidecar.Path] = true
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (%s, stream_index, source_path, codec, language, title, is_default, is_forced)
			VALUES (?, NULL, ?, ?, ?, ?, 0, ?)
			ON CONFLICT(%s, source_path) WHERE source_path IS NOT NULL DO UPDATE SET
				codec = excluded.codec,
				language = excluded.language,
				title = excluded.title,
				is_forced = excluded.is_forced`, t.name, t.owner, t.owner),
			fileID, sidecar.Path, sidecar.Codec,
			nullString(sidecar.Language), nullString(sidecar.Title),
			boolInt(sidecar.Forced)); err != nil {
			return fmt.Errorf("saving external subtitles for file %d: %w", fileID, err)
		}
	}
	if err := pruneSubtitlesTx(ctx, tx, t, fileID,
		`source_path IS NOT NULL`, func(_ sql.NullInt64, path sql.NullString) bool {
			return !seen[path.String]
		}); err != nil {
		return err
	}
	return tx.Commit()
}

// pruneSubtitlesTx deletes the rows of one provenance that the latest look no
// longer found. A deleted `.srt` disappears from the menu on the next playback
// rather than becoming an entry that fails when chosen.
func pruneSubtitlesTx(ctx context.Context, tx *sql.Tx, t subtitleTable, fileID int64,
	where string, stale func(sql.NullInt64, sql.NullString) bool) error {

	rows, err := tx.QueryContext(ctx, fmt.Sprintf(
		`SELECT id, stream_index, source_path FROM %s WHERE %s = ? AND %s`,
		t.name, t.owner, where), fileID)
	if err != nil {
		return fmt.Errorf("pruning subtitle tracks for file %d: %w", fileID, err)
	}
	var doomed []int64
	for rows.Next() {
		var (
			id          int64
			streamIndex sql.NullInt64
			path        sql.NullString
		)
		if err := rows.Scan(&id, &streamIndex, &path); err != nil {
			rows.Close()
			return fmt.Errorf("pruning subtitle tracks for file %d: %w", fileID, err)
		}
		if stale(streamIndex, path) {
			doomed = append(doomed, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("pruning subtitle tracks for file %d: %w", fileID, err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("pruning subtitle tracks for file %d: %w", fileID, err)
	}
	for _, id := range doomed {
		if _, err := tx.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, t.name), id); err != nil {
			return fmt.Errorf("pruning subtitle tracks for file %d: %w", fileID, err)
		}
	}
	return nil
}

// subtitleTrack resolves one track id inside one file, with the ownership join
// spelled out by the caller so a film id cannot be swapped for an episode id.
func (s *Store) subtitleTrack(ctx context.Context, t subtitleTable, ownerJoin, ownerTable string,
	parentID, fileID, trackID int64) (SubtitleTrack, error) {

	var (
		track               SubtitleTrack
		streamIndex         sql.NullInt64
		path, language      sql.NullString
		title               sql.NullString
		isDefault, isForced int
	)
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT t.id, t.stream_index, t.source_path, t.codec,
		       t.language, t.title, t.is_default, t.is_forced
		FROM %s t
		JOIN %s f ON f.id = t.%s
		WHERE t.id = ? AND f.id = ? AND f.%s = ?`,
		t.name, ownerTable, t.owner, ownerJoin), trackID, fileID, parentID,
	).Scan(&track.ID, &streamIndex, &path, &track.Codec,
		&language, &title, &isDefault, &isForced)
	if errors.Is(err, sql.ErrNoRows) {
		return SubtitleTrack{}, ErrNoSuchSubtitleTrack
	}
	if err != nil {
		return SubtitleTrack{}, fmt.Errorf("reading subtitle track %d: %w", trackID, err)
	}

	if streamIndex.Valid {
		index := int(streamIndex.Int64)
		track.StreamIndex = &index
	}
	track.Path = path.String
	track.IsExternal = path.Valid
	track.Language = language.String
	track.Title = title.String
	track.IsDefault = isDefault != 0
	track.IsForced = isForced != 0
	track.Kind = string(subtitles.ClassifyCodec(track.Codec))
	return track, nil
}

// MovieSubtitleTrack resolves a track inside a film's file.
func (s *Service) MovieSubtitleTrack(ctx context.Context, movieID, fileID, trackID int64) (SubtitleTrack, error) {
	return s.store.MovieSubtitleTrack(ctx, movieID, fileID, trackID)
}

// EpisodeSubtitleTrack resolves a track inside an episode's file.
func (s *Service) EpisodeSubtitleTrack(ctx context.Context, itemID, fileID, trackID int64) (SubtitleTrack, error) {
	return s.store.EpisodeSubtitleTrack(ctx, itemID, fileID, trackID)
}

// SyncMovieFileSubtitles records the `.srt` files beside one film file.
func (s *Service) SyncMovieFileSubtitles(ctx context.Context, fileID int64, found []subtitles.Sidecar) error {
	return s.store.SyncMovieFileSubtitles(ctx, fileID, found)
}

// SyncEpisodeFileSubtitles records the `.srt` files beside one episode file.
func (s *Service) SyncEpisodeFileSubtitles(ctx context.Context, fileID int64, found []subtitles.Sidecar) error {
	return s.store.SyncEpisodeFileSubtitles(ctx, fileID, found)
}

func (s *Store) MovieSubtitleTrack(ctx context.Context, movieID, fileID, trackID int64) (SubtitleTrack, error) {
	return s.subtitleTrack(ctx, movieSubtitles, "movie_id", "movie_files", movieID, fileID, trackID)
}

// EpisodeSubtitleTrack resolves a track inside an episode's file.
func (s *Store) EpisodeSubtitleTrack(ctx context.Context, itemID, fileID, trackID int64) (SubtitleTrack, error) {
	return s.subtitleTrack(ctx, episodeSubtitles, "episode_item_id", "episode_files", itemID, fileID, trackID)
}

// SyncMovieFileSubtitles records the `.srt` files beside one film file.
func (s *Store) SyncMovieFileSubtitles(ctx context.Context, fileID int64, found []subtitles.Sidecar) error {
	return s.SyncExternalSubtitles(ctx, movieSubtitles, fileID, found)
}

// SyncEpisodeFileSubtitles records the `.srt` files beside one episode file.
func (s *Store) SyncEpisodeFileSubtitles(ctx context.Context, fileID int64, found []subtitles.Sidecar) error {
	return s.SyncExternalSubtitles(ctx, episodeSubtitles, fileID, found)
}

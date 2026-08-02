package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const episodeFileColumns = `
	id, episode_item_id, path, file_name, size_bytes, modified_at, is_primary,
	media_status, media_container, media_duration_seconds,
	video_stream_index, video_codec, video_width, video_height,
	media_inspected_at`

func scanEpisodeFile(row interface{ Scan(...any) error }) (EpisodeFile, error) {
	var (
		file                                EpisodeFile
		modifiedAt, inspectedAt             int64
		primary                             int
		container, videoCodec               sql.NullString
		duration                            sql.NullFloat64
		videoIndex, videoWidth, videoHeight sql.NullInt64
	)
	if err := row.Scan(
		&file.ID, &file.EpisodeItemID, &file.Path, &file.FileName,
		&file.SizeBytes, &modifiedAt, &primary,
		&file.Media.Status, &container, &duration,
		&videoIndex, &videoCodec, &videoWidth, &videoHeight,
		&inspectedAt,
	); err != nil {
		return EpisodeFile{}, err
	}

	file.ModifiedAt = unix(modifiedAt)
	file.Extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(file.FileName)), ".")
	file.IsPrimary = primary != 0
	file.Media.Container = container.String
	file.Media.DurationSeconds = duration.Float64
	file.Media.AudioTracks = []AudioTrack{}
	if videoIndex.Valid && videoCodec.Valid {
		file.Media.Video = &VideoStream{
			StreamIndex: int(videoIndex.Int64),
			Codec:       videoCodec.String,
			Width:       int(videoWidth.Int64),
			Height:      int(videoHeight.Int64),
		}
	}
	if inspectedAt > 0 {
		at := unix(inspectedAt)
		file.Media.InspectedAt = &at
	}
	return file, nil
}

// EpisodeFiles returns every physical encode for one local playable item.
func (s *Store) EpisodeFiles(ctx context.Context, itemID int64) ([]EpisodeFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+episodeFileColumns+`
		FROM episode_files
		WHERE episode_item_id = ?
		ORDER BY is_primary DESC, id`, itemID)
	if err != nil {
		return nil, fmt.Errorf("reading files for episode %d: %w", itemID, err)
	}
	defer rows.Close()

	var files []EpisodeFile
	for rows.Next() {
		file, err := scanEpisodeFile(rows)
		if err != nil {
			return nil, fmt.Errorf("reading files for episode %d: %w", itemID, err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading files for episode %d: %w", itemID, err)
	}
	if err := s.loadEpisodeAudioTracks(ctx, files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetEpisodeFile enforces item -> file ownership for every API operation.
func (s *Store) GetEpisodeFile(ctx context.Context, itemID, fileID int64) (EpisodeFile, error) {
	file, err := scanEpisodeFile(s.db.QueryRowContext(ctx, `
		SELECT `+episodeFileColumns+`
		FROM episode_files
		WHERE id = ? AND episode_item_id = ?`, fileID, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return EpisodeFile{}, ErrNoSuchEpisodeFile
	}
	if err != nil {
		return EpisodeFile{}, fmt.Errorf("reading file %d for episode %d: %w", fileID, itemID, err)
	}
	files := []EpisodeFile{file}
	if err := s.loadEpisodeAudioTracks(ctx, files); err != nil {
		return EpisodeFile{}, err
	}
	return files[0], nil
}

func (s *Store) loadEpisodeAudioTracks(ctx context.Context, files []EpisodeFile) error {
	if len(files) == 0 {
		return nil
	}
	byID := make(map[int64]*EpisodeFile, len(files))
	args := make([]any, 0, len(files))
	placeholders := make([]string, 0, len(files))
	for i := range files {
		files[i].Media.AudioTracks = []AudioTrack{}
		byID[files[i].ID] = &files[i]
		args = append(args, files[i].ID)
		placeholders = append(placeholders, "?")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, episode_file_id, stream_index, codec,
		       language, title, channels, is_default
		FROM episode_file_audio_tracks
		WHERE episode_file_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY episode_file_id, stream_index`, args...)
	if err != nil {
		return fmt.Errorf("reading episode audio tracks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			track                     AudioTrack
			fileID                    int64
			language, title, channels sql.NullString
			isDefault                 int
		)
		if err := rows.Scan(&track.ID, &fileID, &track.StreamIndex, &track.Codec,
			&language, &title, &channels, &isDefault); err != nil {
			return fmt.Errorf("reading episode audio tracks: %w", err)
		}
		track.Language = language.String
		track.Title = title.String
		track.Channels = channels.String
		track.IsDefault = isDefault != 0
		if file := byID[fileID]; file != nil {
			file.Media.AudioTracks = append(file.Media.AudioTracks, track)
		}
	}
	return rows.Err()
}

// SaveEpisodeFileMedia atomically replaces measured media characteristics.
// Track IDs survive repeat inspections when their ffmpeg stream index survives.
func (s *Store) SaveEpisodeFileMedia(ctx context.Context, itemID, fileID int64, media FileMedia) (EpisodeFile, error) {
	if media.Status != MediaOK || media.Video == nil {
		return EpisodeFile{}, fmt.Errorf("saving episode file media: incomplete successful inspection")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return EpisodeFile{}, fmt.Errorf("saving episode file media: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, `
		UPDATE episode_files SET
			media_status = ?, media_container = ?, media_duration_seconds = ?,
			video_stream_index = ?, video_codec = ?, video_width = ?, video_height = ?,
			media_inspected_at = ?
		WHERE id = ? AND episode_item_id = ?`,
		MediaOK, nullString(media.Container), nullPositiveFloat(media.DurationSeconds),
		media.Video.StreamIndex, media.Video.Codec,
		nullPositiveInt(media.Video.Width), nullPositiveInt(media.Video.Height),
		now, fileID, itemID)
	if err != nil {
		return EpisodeFile{}, fmt.Errorf("saving media for episode file %d: %w", fileID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return EpisodeFile{}, ErrNoSuchEpisodeFile
	}

	seen := make(map[int]bool, len(media.AudioTracks))
	for _, track := range media.AudioTracks {
		seen[track.StreamIndex] = true
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO episode_file_audio_tracks
				(episode_file_id, stream_index, codec, language, title, channels, is_default)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(episode_file_id, stream_index) DO UPDATE SET
				codec = excluded.codec,
				language = excluded.language,
				title = excluded.title,
				channels = excluded.channels,
				is_default = excluded.is_default`,
			fileID, track.StreamIndex, track.Codec,
			nullString(track.Language), nullString(track.Title), nullString(track.Channels),
			boolInt(track.IsDefault)); err != nil {
			return EpisodeFile{}, fmt.Errorf("saving audio tracks for episode file %d: %w", fileID, err)
		}
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, stream_index FROM episode_file_audio_tracks WHERE episode_file_id = ?`, fileID)
	if err != nil {
		return EpisodeFile{}, fmt.Errorf("pruning audio tracks for episode file %d: %w", fileID, err)
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var streamIndex int
		if err := rows.Scan(&id, &streamIndex); err != nil {
			rows.Close()
			return EpisodeFile{}, fmt.Errorf("pruning audio tracks for episode file %d: %w", fileID, err)
		}
		if !seen[streamIndex] {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return EpisodeFile{}, fmt.Errorf("pruning audio tracks for episode file %d: %w", fileID, err)
	}
	if err := rows.Close(); err != nil {
		return EpisodeFile{}, fmt.Errorf("pruning audio tracks for episode file %d: %w", fileID, err)
	}
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM episode_file_audio_tracks WHERE id = ?`, id); err != nil {
			return EpisodeFile{}, fmt.Errorf("pruning audio tracks for episode file %d: %w", fileID, err)
		}
	}

	if media.DurationSeconds > 0 {
		if _, err := tx.ExecContext(ctx, `
			UPDATE episode_items SET duration_seconds = COALESCE(duration_seconds, ?)
			WHERE id = ?`, media.DurationSeconds, itemID); err != nil {
			return EpisodeFile{}, fmt.Errorf("saving media duration for episode %d: %w", itemID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return EpisodeFile{}, fmt.Errorf("saving media for episode file %d: %w", fileID, err)
	}
	return s.GetEpisodeFile(ctx, itemID, fileID)
}

func (s *Store) MarkEpisodeFileMediaError(ctx context.Context, itemID, fileID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recording failed episode media inspection: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE episode_files SET
			media_status = ?, media_container = NULL,
			media_duration_seconds = NULL,
			video_stream_index = NULL, video_codec = NULL,
			video_width = NULL, video_height = NULL,
			media_inspected_at = ?
		WHERE id = ? AND episode_item_id = ?`, MediaError, time.Now().Unix(), fileID, itemID)
	if err != nil {
		return fmt.Errorf("recording failed media inspection for episode file %d: %w", fileID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchEpisodeFile
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM episode_file_audio_tracks WHERE episode_file_id = ?`, fileID); err != nil {
		return fmt.Errorf("clearing stale audio tracks for episode file %d: %w", fileID, err)
	}
	return tx.Commit()
}

// EpisodeAudioTrack resolves a stable track id inside one episode file.
func (s *Store) EpisodeAudioTrack(ctx context.Context, itemID, fileID, trackID int64) (AudioTrack, error) {
	var (
		track                     AudioTrack
		language, title, channels sql.NullString
		isDefault                 int
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.stream_index, a.codec, a.language, a.title, a.channels, a.is_default
		FROM episode_file_audio_tracks a
		JOIN episode_files f ON f.id = a.episode_file_id
		WHERE a.id = ? AND f.id = ? AND f.episode_item_id = ?`, trackID, fileID, itemID,
	).Scan(&track.ID, &track.StreamIndex, &track.Codec,
		&language, &title, &channels, &isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return AudioTrack{}, ErrNoSuchAudioTrack
	}
	if err != nil {
		return AudioTrack{}, fmt.Errorf("reading episode audio track %d: %w", trackID, err)
	}
	track.Language = language.String
	track.Title = title.String
	track.Channels = channels.String
	track.IsDefault = isDefault != 0
	return track, nil
}

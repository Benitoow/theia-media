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

const (
	MediaPending = "pending"
	MediaOK      = "ok"
	MediaError   = "error"
)

var (
	// ErrNoSuchMovieFile deliberately covers both an unknown file id and a file
	// that belongs to another movie. API callers must not be able to swap a
	// movie id and a file id to bypass ownership checks.
	ErrNoSuchMovieFile  = errors.New("no such film file")
	ErrNoSuchAudioTrack = errors.New("no such audio track")
)

// MovieFile is one playable file attached to a film. Path never crosses the
// HTTP boundary: clients select this stable id and the server resolves the path
// from its own database.
type MovieFile struct {
	ID         int64     `json:"id"`
	MovieID    int64     `json:"-"`
	Path       string    `json:"-"`
	FileName   string    `json:"file_name"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	Extension  string    `json:"extension"`
	IsPrimary  bool      `json:"is_primary"`
	Media      FileMedia `json:"media"`
}

// FileMedia contains only characteristics measured by ffmpeg. A pending file
// may still carry a duration learned by the player, but it never invents a
// codec, resolution, container or audio track from its filename.
type FileMedia struct {
	Status          string          `json:"status"`
	Container       string          `json:"container,omitempty"`
	DurationSeconds float64         `json:"duration_seconds,omitempty"`
	Video           *VideoStream    `json:"video,omitempty"`
	AudioTracks     []AudioTrack    `json:"audio_tracks"`
	SubtitleTracks  []SubtitleTrack `json:"subtitle_tracks"`
	InspectedAt     *time.Time      `json:"inspected_at,omitempty"`

	// SubtitlesScanned distinguishes a file with no embedded subtitles from one
	// inspected before Theia knew to look. Bookkeeping, not a measurement, so it
	// stays on this side of the HTTP boundary.
	SubtitlesScanned bool `json:"-"`
}

type VideoStream struct {
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`

	// FrameRate is the file's own cadence, as ffmpeg printed it. Zero means the
	// stream line did not say, which is a real answer and not a slow film: the
	// player falls back to a fixed floor when it is absent.
	FrameRate float64 `json:"frame_rate,omitempty"`

	// ColorTransfer is the raw transfer function -- "smpte2084" for PQ,
	// "arib-std-b67" for HLG, empty for everything else. It decides whether a
	// re-encode has to tone map, and it lets the interface say HDR without
	// guessing from a filename.
	ColorTransfer string `json:"color_transfer,omitempty"`

	// DolbyVision is a name, not a decision: the base layer is what gets decoded
	// either way.
	DolbyVision bool `json:"dolby_vision,omitempty"`
}

type AudioTrack struct {
	ID          int64  `json:"id"`
	StreamIndex int    `json:"stream_index"`
	Codec       string `json:"codec"`
	Language    string `json:"language,omitempty"`
	Title       string `json:"title,omitempty"`
	Channels    string `json:"channels,omitempty"`
	Profile     string `json:"profile,omitempty"`
	IsDefault   bool   `json:"is_default"`
}

const movieFileColumns = `
	id, movie_id, path, file_name, size_bytes, modified_at, is_primary,
	media_status, media_container, media_duration_seconds,
	video_stream_index, video_codec, video_width, video_height, video_frame_rate,
	video_color_transfer, video_dolby_vision,
	media_inspected_at, subtitles_scanned`

func scanMovieFile(row interface{ Scan(...any) error }) (MovieFile, error) {
	var (
		file                                MovieFile
		modifiedAt, inspectedAt             int64
		primary                             int
		container, videoCodec               sql.NullString
		duration                            sql.NullFloat64
		videoIndex, videoWidth, videoHeight sql.NullInt64
		videoFrameRate                      sql.NullFloat64
		videoTransfer                       sql.NullString
		dolbyVision                         int
		subtitlesScanned                    int
	)
	if err := row.Scan(
		&file.ID, &file.MovieID, &file.Path, &file.FileName,
		&file.SizeBytes, &modifiedAt, &primary,
		&file.Media.Status, &container, &duration,
		&videoIndex, &videoCodec, &videoWidth, &videoHeight, &videoFrameRate,
		&videoTransfer, &dolbyVision,
		&inspectedAt, &subtitlesScanned,
	); err != nil {
		return MovieFile{}, err
	}

	file.ModifiedAt = unix(modifiedAt)
	file.Extension = strings.TrimPrefix(strings.ToLower(filepath.Ext(file.FileName)), ".")
	file.IsPrimary = primary != 0
	file.Media.Container = container.String
	file.Media.DurationSeconds = duration.Float64
	file.Media.AudioTracks = []AudioTrack{}
	file.Media.SubtitleTracks = []SubtitleTrack{}
	file.Media.SubtitlesScanned = subtitlesScanned != 0
	if videoIndex.Valid && videoCodec.Valid {
		file.Media.Video = &VideoStream{
			StreamIndex:   int(videoIndex.Int64),
			Codec:         videoCodec.String,
			Width:         int(videoWidth.Int64),
			Height:        int(videoHeight.Int64),
			FrameRate:     videoFrameRate.Float64,
			ColorTransfer: videoTransfer.String,
			DolbyVision:   dolbyVision != 0,
		}
	}
	if inspectedAt > 0 {
		at := unix(inspectedAt)
		file.Media.InspectedAt = &at
	}
	return file, nil
}

// MovieFiles returns every file for a movie, primary first and then by stable
// id. It is used only by the detail endpoint; catalogue rows stay compact.
func (s *Store) MovieFiles(ctx context.Context, movieID int64) ([]MovieFile, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+movieFileColumns+`
		FROM movie_files
		WHERE movie_id = ?
		ORDER BY is_primary DESC, id`, movieID)
	if err != nil {
		return nil, fmt.Errorf("reading files for film %d: %w", movieID, err)
	}
	defer rows.Close()

	var files []MovieFile
	for rows.Next() {
		file, err := scanMovieFile(rows)
		if err != nil {
			return nil, fmt.Errorf("reading files for film %d: %w", movieID, err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading files for film %d: %w", movieID, err)
	}
	if err := s.loadAudioTracks(ctx, files); err != nil {
		return nil, err
	}
	return files, nil
}

// GetMovieFile resolves a client-supplied file id inside a specific film.
func (s *Store) GetMovieFile(ctx context.Context, movieID, fileID int64) (MovieFile, error) {
	file, err := scanMovieFile(s.db.QueryRowContext(ctx, `
		SELECT `+movieFileColumns+`
		FROM movie_files
		WHERE id = ? AND movie_id = ?`, fileID, movieID))
	if errors.Is(err, sql.ErrNoRows) {
		return MovieFile{}, ErrNoSuchMovieFile
	}
	if err != nil {
		return MovieFile{}, fmt.Errorf("reading file %d for film %d: %w", fileID, movieID, err)
	}
	files := []MovieFile{file}
	if err := s.loadAudioTracks(ctx, files); err != nil {
		return MovieFile{}, err
	}
	return files[0], nil
}

func (s *Store) loadAudioTracks(ctx context.Context, files []MovieFile) error {
	if len(files) == 0 {
		return nil
	}
	byID := make(map[int64]*MovieFile, len(files))
	args := make([]any, 0, len(files))
	placeholders := make([]string, 0, len(files))
	for i := range files {
		files[i].Media.AudioTracks = []AudioTrack{}
		byID[files[i].ID] = &files[i]
		args = append(args, files[i].ID)
		placeholders = append(placeholders, "?")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, movie_file_id, stream_index, codec,
		       language, title, channels, profile, is_default
		FROM movie_file_audio_tracks
		WHERE movie_file_id IN (`+strings.Join(placeholders, ",")+`)
		ORDER BY movie_file_id, stream_index`, args...)
	if err != nil {
		return fmt.Errorf("reading audio tracks: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			track                              AudioTrack
			fileID                             int64
			language, title, channels, profile sql.NullString
			isDefault                          int
		)
		if err := rows.Scan(&track.ID, &fileID, &track.StreamIndex, &track.Codec,
			&language, &title, &channels, &profile, &isDefault); err != nil {
			return fmt.Errorf("reading audio tracks: %w", err)
		}
		track.Language = language.String
		track.Title = title.String
		track.Channels = channels.String
		track.Profile = profile.String
		track.IsDefault = isDefault != 0
		if file := byID[fileID]; file != nil {
			file.Media.AudioTracks = append(file.Media.AudioTracks, track)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	ids := make([]int64, 0, len(files))
	for i := range files {
		ids = append(ids, files[i].ID)
	}
	byFile, err := s.loadSubtitles(ctx, movieSubtitles, ids)
	if err != nil {
		return err
	}
	for i := range files {
		if tracks := byFile[files[i].ID]; tracks != nil {
			files[i].Media.SubtitleTracks = tracks
		}
	}
	return nil
}

// SaveFileMedia atomically replaces the measured characteristics of one file.
// Existing track ids are retained when ffmpeg reports the same stream index on
// a later inspection.
func (s *Store) SaveFileMedia(ctx context.Context, movieID, fileID int64, media FileMedia) (MovieFile, error) {
	if media.Status != MediaOK || media.Video == nil {
		return MovieFile{}, fmt.Errorf("saving file media: incomplete successful inspection")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MovieFile{}, fmt.Errorf("saving file media: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().Unix()
	res, err := tx.ExecContext(ctx, `
		UPDATE movie_files SET
			media_status = ?, media_container = ?, media_duration_seconds = ?,
			video_stream_index = ?, video_codec = ?, video_width = ?, video_height = ?,
			video_frame_rate = ?, video_color_transfer = ?, video_dolby_vision = ?,
			media_inspected_at = ?, subtitles_scanned = 1
		WHERE id = ? AND movie_id = ?`,
		MediaOK, nullString(media.Container), nullPositiveFloat(media.DurationSeconds),
		media.Video.StreamIndex, media.Video.Codec,
		nullPositiveInt(media.Video.Width), nullPositiveInt(media.Video.Height),
		nullPositiveFloat(media.Video.FrameRate),
		nullString(media.Video.ColorTransfer), boolInt(media.Video.DolbyVision),
		now, fileID, movieID)
	if err != nil {
		return MovieFile{}, fmt.Errorf("saving media for file %d: %w", fileID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return MovieFile{}, ErrNoSuchMovieFile
	}

	seen := make(map[int]bool, len(media.AudioTracks))
	for _, track := range media.AudioTracks {
		seen[track.StreamIndex] = true
		_, err := tx.ExecContext(ctx, `
			INSERT INTO movie_file_audio_tracks
				(movie_file_id, stream_index, codec, language, title, channels, profile, is_default)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(movie_file_id, stream_index) DO UPDATE SET
				codec = excluded.codec,
				language = excluded.language,
				title = excluded.title,
				channels = excluded.channels,
				profile = excluded.profile,
				is_default = excluded.is_default`,
			fileID, track.StreamIndex, track.Codec,
			nullString(track.Language), nullString(track.Title), nullString(track.Channels),
			nullString(track.Profile),
			boolInt(track.IsDefault))
		if err != nil {
			return MovieFile{}, fmt.Errorf("saving audio tracks for file %d: %w", fileID, err)
		}
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, stream_index FROM movie_file_audio_tracks WHERE movie_file_id = ?`, fileID)
	if err != nil {
		return MovieFile{}, fmt.Errorf("pruning audio tracks for file %d: %w", fileID, err)
	}
	var stale []int64
	for rows.Next() {
		var id int64
		var streamIndex int
		if err := rows.Scan(&id, &streamIndex); err != nil {
			rows.Close()
			return MovieFile{}, fmt.Errorf("pruning audio tracks for file %d: %w", fileID, err)
		}
		if !seen[streamIndex] {
			stale = append(stale, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return MovieFile{}, fmt.Errorf("pruning audio tracks for file %d: %w", fileID, err)
	}
	if err := rows.Close(); err != nil {
		return MovieFile{}, fmt.Errorf("pruning audio tracks for file %d: %w", fileID, err)
	}
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM movie_file_audio_tracks WHERE id = ?`, id); err != nil {
			return MovieFile{}, fmt.Errorf("pruning audio tracks for file %d: %w", fileID, err)
		}
	}

	if err := saveEmbeddedSubtitlesTx(ctx, tx, movieSubtitles, fileID, media.SubtitleTracks); err != nil {
		return MovieFile{}, err
	}

	if media.DurationSeconds > 0 {
		// The movie duration drives resume progress for both the old and new
		// frontend. Keep a known value, but do not overwrite a duration recorded
		// by an actual playback of another variant.
		if _, err := tx.ExecContext(ctx, `
			UPDATE movies SET duration_seconds = COALESCE(duration_seconds, ?)
			WHERE id = ?`, media.DurationSeconds, movieID); err != nil {
			return MovieFile{}, fmt.Errorf("saving media duration for film %d: %w", movieID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return MovieFile{}, fmt.Errorf("saving media for file %d: %w", fileID, err)
	}
	return s.GetMovieFile(ctx, movieID, fileID)
}

// MarkFileMediaError records a failed explicit inspection without persisting a
// machine-specific ffmpeg error in the API-facing database.
func (s *Store) MarkFileMediaError(ctx context.Context, movieID, fileID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("recording failed media inspection: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		UPDATE movie_files SET
			media_status = ?, media_container = NULL,
			media_duration_seconds = NULL,
			video_stream_index = NULL, video_codec = NULL,
			video_width = NULL, video_height = NULL, video_frame_rate = NULL,
			video_color_transfer = NULL, video_dolby_vision = 0,
			media_inspected_at = ?, subtitles_scanned = 1
		WHERE id = ? AND movie_id = ?`, MediaError, time.Now().Unix(), fileID, movieID)
	if err != nil {
		return fmt.Errorf("recording failed media inspection for file %d: %w", fileID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSuchMovieFile
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM movie_file_audio_tracks WHERE movie_file_id = ?`, fileID); err != nil {
		return fmt.Errorf("clearing stale audio tracks for file %d: %w", fileID, err)
	}
	// Only the embedded tracks go. A `.srt` next to an unreadable film is still
	// a file on disk, and its existence was never ffmpeg's finding to retract.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM movie_file_subtitle_tracks
		 WHERE movie_file_id = ? AND stream_index IS NOT NULL`, fileID); err != nil {
		return fmt.Errorf("clearing stale subtitle tracks for file %d: %w", fileID, err)
	}
	return tx.Commit()
}

// AudioTrack resolves a stable track id inside a specific file.
func (s *Store) AudioTrack(ctx context.Context, movieID, fileID, trackID int64) (AudioTrack, error) {
	var (
		track                              AudioTrack
		language, title, channels, profile sql.NullString
		isDefault                          int
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.stream_index, a.codec, a.language, a.title, a.channels, a.profile, a.is_default
		FROM movie_file_audio_tracks a
		JOIN movie_files f ON f.id = a.movie_file_id
		WHERE a.id = ? AND f.id = ? AND f.movie_id = ?`, trackID, fileID, movieID,
	).Scan(&track.ID, &track.StreamIndex, &track.Codec,
		&language, &title, &channels, &profile, &isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return AudioTrack{}, ErrNoSuchAudioTrack
	}
	if err != nil {
		return AudioTrack{}, fmt.Errorf("reading audio track %d: %w", trackID, err)
	}
	track.Language = language.String
	track.Title = title.String
	track.Channels = channels.String
	track.Profile = profile.String
	track.IsDefault = isDefault != 0
	return track, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullPositiveInt(v int) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullPositiveFloat(v float64) any {
	if v <= 0 {
		return nil
	}
	return v
}

package library

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Benitoow/theia-media/internal/scanner"
	"github.com/Benitoow/theia-media/internal/tmdb"
)

// ErrScanInProgress is returned when a scan is requested while one is already
// running. Two concurrent scans would fight over the same rows to no purpose.
var ErrScanInProgress = errors.New("a scan is already running")

// ScanReport is what one reconciliation between disk and database did.
type ScanReport struct {
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"-"`
	Seconds   float64       `json:"duration_seconds"`

	Found   int `json:"found"`
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
	Merged  int `json:"merged"`

	MovieFiles   int `json:"movie_files"`
	EpisodeFiles int `json:"episode_files"`
	Series       int `json:"series"`
	Episodes     int `json:"episodes"`

	// Metadata lookups performed during this pass.
	Enriched       int `json:"enriched"`
	NotFound       int `json:"not_found"`
	MetadataErrors int `json:"metadata_errors"`

	// What went wrong, as codes the interface turns into sentences. A scan that
	// hit problems still reports everything it did manage to index.
	Problems []scanner.Problem `json:"problems,omitempty"`
}

// defaultMetadataBatch caps how many TMDB lookups one scan performs.
//
// A first scan of a large library would otherwise sit there making thousands of
// requests before the interface shows anything. Capping the batch means posters
// fill in over the first few scans instead, which is visible progress rather
// than a long silence.
const defaultMetadataBatch = 200

// Service reconciles the files on disk with the library in the database, and
// fills in metadata for what it finds.
type Service struct {
	store *Store
	log   *slog.Logger

	// tmdb is nil when no API key is configured. Everything else still works;
	// the library is simply built from filenames alone.
	tmdb          *tmdb.Client
	metadataBatch int

	mu       sync.Mutex
	scanning bool
	last     *ScanReport
}

// NewService builds the scanning service. client may be nil, in which case no
// metadata is fetched and scanning carries on regardless.
func NewService(store *Store, client *tmdb.Client, log *slog.Logger) *Service {
	return &Service{
		store:         store,
		tmdb:          client,
		metadataBatch: defaultMetadataBatch,
		log:           log,
	}
}

// HasMetadataSource reports whether a TMDB key was configured.
func (s *Service) HasMetadataSource() bool { return s.tmdb != nil }

// LastScan returns the most recent report, or nil if nothing has run yet.
func (s *Service) LastScan() *ScanReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

// Scanning reports whether a scan is running right now.
func (s *Service) Scanning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanning
}

// Count returns the number of films currently in the library.
func (s *Service) Count(ctx context.Context) (int, error) {
	return s.store.Count(ctx)
}

// List returns a page of the library, ordered by title.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Movie, error) {
	return s.store.List(ctx, limit, offset)
}

// Get returns one film.
func (s *Service) Get(ctx context.Context, id int64) (Movie, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) SeriesCount(ctx context.Context) (int, error) {
	return s.store.SeriesCount(ctx)
}

func (s *Service) EpisodeCount(ctx context.Context) (int, error) {
	return s.store.EpisodeItemCount(ctx)
}

func (s *Service) ListSeries(ctx context.Context, limit, offset int) ([]Series, error) {
	return s.store.ListSeries(ctx, limit, offset)
}

func (s *Service) GetSeries(ctx context.Context, id int64) (Series, error) {
	return s.store.GetSeries(ctx, id)
}

func (s *Service) GetSeason(ctx context.Context, seriesID int64, number int) (Season, error) {
	return s.store.GetSeason(ctx, seriesID, number)
}

func (s *Service) GetEpisodeItem(ctx context.Context, id int64) (EpisodeItem, error) {
	return s.store.GetEpisodeItem(ctx, id)
}

func (s *Service) SeriesHome(ctx context.Context, limit int) (*SeriesHome, error) {
	return s.store.SeriesHome(ctx, limit)
}

func (s *Service) GetEpisodeFile(ctx context.Context, itemID, fileID int64) (EpisodeFile, error) {
	return s.store.GetEpisodeFile(ctx, itemID, fileID)
}

func (s *Service) SaveEpisodeFileMedia(ctx context.Context, itemID, fileID int64, media FileMedia) (EpisodeFile, error) {
	return s.store.SaveEpisodeFileMedia(ctx, itemID, fileID, media)
}

func (s *Service) MarkEpisodeFileMediaError(ctx context.Context, itemID, fileID int64) error {
	return s.store.MarkEpisodeFileMediaError(ctx, itemID, fileID)
}

func (s *Service) EpisodeAudioTrack(ctx context.Context, itemID, fileID, trackID int64) (AudioTrack, error) {
	return s.store.EpisodeAudioTrack(ctx, itemID, fileID, trackID)
}

// GetMovieFile resolves a playable file inside a film.
func (s *Service) GetMovieFile(ctx context.Context, movieID, fileID int64) (MovieFile, error) {
	return s.store.GetMovieFile(ctx, movieID, fileID)
}

// SaveFileMedia records characteristics measured by ffmpeg.
func (s *Service) SaveFileMedia(ctx context.Context, movieID, fileID int64, media FileMedia) (MovieFile, error) {
	return s.store.SaveFileMedia(ctx, movieID, fileID, media)
}

// MarkFileMediaError records an explicit inspection failure.
func (s *Service) MarkFileMediaError(ctx context.Context, movieID, fileID int64) error {
	return s.store.MarkFileMediaError(ctx, movieID, fileID)
}

// AudioTrack resolves a stable track id under the requested movie and file.
func (s *Service) AudioTrack(ctx context.Context, movieID, fileID, trackID int64) (AudioTrack, error) {
	return s.store.AudioTrack(ctx, movieID, fileID, trackID)
}

// Consolidate merges duplicates that have a proven identity. It is exposed so
// startup can finish a data migration before the HTTP server becomes visible.
func (s *Service) Consolidate(ctx context.Context) (int, error) {
	movies, err := s.store.Consolidate(ctx)
	if err != nil {
		return 0, err
	}
	series, err := s.store.ConsolidateSeries(ctx)
	return movies + series, err
}

// SaveProgress records where a viewer got to.
func (s *Service) SaveProgress(ctx context.Context, id int64, position, duration float64) (Progress, error) {
	return s.store.SaveProgress(ctx, id, position, duration, time.Now())
}

// ResetProgress forgets a viewing position.
func (s *Service) ResetProgress(ctx context.Context, id int64) error {
	return s.store.ResetProgress(ctx, id)
}

// SaveDuration records a duration learned from probing a file.
func (s *Service) SaveDuration(ctx context.Context, id int64, seconds float64) error {
	return s.store.SaveDuration(ctx, id, seconds)
}

func (s *Service) SaveEpisodeProgress(ctx context.Context, id int64, position, duration float64) (Progress, error) {
	return s.store.SaveEpisodeProgress(ctx, id, position, duration, time.Now())
}

func (s *Service) ResetEpisodeProgress(ctx context.Context, id int64) error {
	return s.store.ResetEpisodeProgress(ctx, id)
}

func (s *Service) SaveEpisodeDuration(ctx context.Context, id int64, seconds float64) error {
	return s.store.SaveEpisodeDuration(ctx, id, seconds)
}

// Row kinds. The server names what a row *is*; the interface writes what it is
// called, per decision 25. Before this the server sent "Continuer à regarder"
// as a title, which put French in the API and meant a second language would
// have had to be added in Go.
const (
	RowContinue = "continue"
	RowRecent   = "recent"
	RowTopRated = "top_rated"
	RowTonight  = "tonight"
)

// Hero kinds, same rule.
const (
	// HeroResume is a film already under way. It outranks everything else:
	// somebody who stopped forty minutes into a film last night is far more
	// likely to want that than anything the library could suggest.
	HeroResume = "resume"
	// HeroFeatured is the recent-and-well-rated pick, used when nothing is in
	// progress.
	HeroFeatured = "featured"
)

// Row is one horizontal strip on the home screen. Kind says what it is; the
// interface decides what it is called and where its "see all" link goes.
type Row struct {
	Kind   string  `json:"kind"`
	Movies []Movie `json:"movies"`
}

// Home is everything the home screen needs, in one request.
type Home struct {
	Hero     *Movie `json:"hero"`
	HeroKind string `json:"hero_kind,omitempty"`
	Rows     []Row  `json:"rows"`
	Total    int    `json:"total"`
}

// HomeScreen assembles the home screen.
//
// The home screen is a personal surface, not a second catalogue. /films already
// searches, sorts and filters the whole library, so this answers a narrower and
// more useful question: what were you watching, what is new, and what should you
// put on tonight. It deliberately no longer lists a row per genre — that was the
// library pretending to be a shop front, and browsing by genre now belongs to
// the page built for it.
//
// Rows are short for the same reason. Each one carries a way through to /films
// pre-filtered, which is where an inventory belongs.
func (s *Service) HomeScreen(ctx context.Context, perRow int) (*Home, error) {
	total, err := s.store.Count(ctx)
	if err != nil {
		return nil, err
	}
	home := &Home{Total: total}
	if total == 0 {
		return home, nil
	}

	// The hero is whichever of the two is true right now, never a fixed choice:
	// a film in progress if there is one, otherwise something worth starting.
	switch hero, err := s.store.ResumeHero(ctx); {
	case err == nil:
		home.Hero, home.HeroKind = &hero, HeroResume
	case errors.Is(err, ErrNoSuchMovie):
		if featured, err := s.store.Hero(ctx); err == nil {
			home.Hero, home.HeroKind = &featured, HeroFeatured
		} else if !errors.Is(err, ErrNoSuchMovie) {
			return nil, err
		}
	default:
		return nil, err
	}

	// Seeded with the local date so tonight's suggestion holds for the evening.
	// See Store.Tonight for why it is not simply random.
	year, month, day := time.Now().Date()
	seed := int64(year)*10000 + int64(month)*100 + int64(day)

	// The whole shape of the home screen, in the order it is read. Continue
	// watching is first and stays first: somebody who stopped mid-film wants one
	// click, not a search.
	sources := []struct {
		kind  string
		fetch func() ([]Movie, error)
	}{
		{RowContinue, func() ([]Movie, error) { return s.store.ContinueWatching(ctx, perRow) }},
		{RowRecent, func() ([]Movie, error) { return s.store.RecentlyAdded(ctx, perRow) }},
		{RowTopRated, func() ([]Movie, error) { return s.store.TopRated(ctx, perRow) }},
		{RowTonight, func() ([]Movie, error) { return s.store.Tonight(ctx, perRow, seed) }},
	}

	for _, src := range sources {
		movies, err := src.fetch()
		if err != nil {
			return nil, err
		}
		if len(movies) == 0 {
			continue
		}
		home.Rows = append(home.Rows, Row{Kind: src.kind, Movies: movies})
	}
	return home, nil
}

// Scan walks the given roots and brings the database in line with what is on
// disk: new files are added, known ones refreshed, and rows whose file has
// disappeared are removed.
//
// Only one scan runs at a time; a second caller gets ErrScanInProgress rather
// than queueing behind the first.
func (s *Service) Scan(ctx context.Context, roots []string) (*ScanReport, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, ErrScanInProgress
	}
	s.scanning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	startedAt := time.Now()
	report := &ScanReport{StartedAt: startedAt.UTC()}

	if len(roots) == 0 {
		s.log.Info("scan skipped, no library directories are configured")
		report.Duration = time.Since(startedAt)
		report.Seconds = report.Duration.Seconds()
		s.remember(report)
		return report, nil
	}

	// Every row this pass touches is stamped with the same generation, so that
	// "not seen by the latest scan" is one exact integer comparison.
	generation, err := s.store.NextScanGeneration(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Info("scan started", "directories", len(roots), "generation", generation)

	found, scanErr := scanner.Scan(ctx, roots, s.log)
	if scanErr != nil {
		return nil, scanErr
	}
	report.Found = len(found.Files)
	report.Problems = found.Problems

	for _, file := range found.Files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		parsedEpisode := ParseEpisodePath(file.Relative)
		if parsedEpisode.Matched {
			if parsedEpisode.Ambiguous {
				report.Problems = append(report.Problems, scanner.Problem{
					Kind: scanner.KindEpisodeSeriesUnknown,
					Path: file.Path,
				})
				continue
			}
			report.EpisodeFiles++
			res, err := s.store.UpsertEpisode(ctx, file, parsedEpisode, generation)
			if err != nil {
				s.log.Warn("an episode could not be saved", "path", file.Path, "error", err)
				report.Problems = append(report.Problems,
					scanner.Problem{Kind: scanner.KindSaveFailed, Path: file.Path})
				continue
			}
			if res.inserted {
				report.Added++
			} else {
				report.Updated++
			}
			continue
		}

		report.MovieFiles++
		parsed := ParseFileName(file.Name)
		res, err := s.store.Upsert(ctx, Movie{
			Path:       file.Path,
			FileName:   file.Name,
			SizeBytes:  file.SizeBytes,
			ModifiedAt: file.ModifiedAt,
			Title:      parsed.Title,
			Year:       parsed.Year,
		}, generation)
		if err != nil {
			// One unwritable row must not cost us the other four thousand.
			s.log.Warn("a film could not be saved", "path", file.Path, "error", err)
			report.Problems = append(report.Problems,
				scanner.Problem{Kind: scanner.KindSaveFailed, Path: file.Path})
			continue
		}
		if res.inserted {
			report.Added++
		} else {
			report.Updated++
		}
	}

	// Only prune when both the walk and every database write completed without
	// trouble. A disconnected drive or a rejected row looks like a missing file
	// from here; turning either into deletions would compound a recoverable error.
	if !hasBlockingScanProblem(report.Problems) {
		removed, err := s.store.DeleteNotSeenIn(ctx, generation)
		if err != nil {
			return nil, err
		}
		episodeRemoved, err := s.store.DeleteEpisodeFilesNotSeenIn(ctx, generation)
		if err != nil {
			return nil, err
		}
		report.Removed = removed + episodeRemoved
	} else {
		s.log.Warn("skipping removal of missing files, the scan reported problems",
			"problems", len(report.Problems))
	}

	// Parsed title + year can merge variants without a network call. Running
	// this before metadata also collapses existing rows migrated from v1.
	merged, err := s.store.Consolidate(ctx)
	if err != nil {
		return nil, err
	}
	report.Merged += merged
	merged, err = s.store.ConsolidateSeries(ctx)
	if err != nil {
		return nil, err
	}
	report.Merged += merged

	// Metadata comes last, so that the library is complete and browsable before
	// a single network request is made. A TMDB outage delays posters; it never
	// delays knowing what you own.
	s.enrich(ctx, report)
	s.enrichSeries(ctx, report)

	// TMDB identity resolves the hard but real cases: a yearless filename or a
	// localized title that parsed differently. It is authoritative only after a
	// successful lookup, so this second pass remains conservative.
	merged, err = s.store.Consolidate(ctx)
	if err != nil {
		return nil, err
	}
	report.Merged += merged
	merged, err = s.store.ConsolidateSeries(ctx)
	if err != nil {
		return nil, err
	}
	report.Merged += merged
	report.Series, err = s.store.SeriesCount(ctx)
	if err != nil {
		return nil, err
	}
	report.Episodes, err = s.store.EpisodeItemCount(ctx)
	if err != nil {
		return nil, err
	}

	report.Duration = time.Since(startedAt)
	report.Seconds = report.Duration.Seconds()

	s.log.Info("scan finished",
		"found", report.Found,
		"added", report.Added,
		"updated", report.Updated,
		"removed", report.Removed,
		"merged", report.Merged,
		"enriched", report.Enriched,
		"not_found", report.NotFound,
		"problems", len(report.Problems),
		"duration", report.Duration,
	)
	s.remember(report)
	return report, nil
}

func hasBlockingScanProblem(problems []scanner.Problem) bool {
	for _, problem := range problems {
		// An ambiguous episode was read successfully and intentionally omitted.
		// It is safe to reconcile the rest of the disk; unreadable paths and failed
		// writes are not.
		if problem.Kind != scanner.KindEpisodeSeriesUnknown {
			return true
		}
	}
	return false
}

func (s *Service) remember(r *ScanReport) {
	s.mu.Lock()
	s.last = r
	s.mu.Unlock()
}

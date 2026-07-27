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

	// Metadata lookups performed during this pass.
	Enriched       int `json:"enriched"`
	NotFound       int `json:"not_found"`
	MetadataErrors int `json:"metadata_errors"`

	// Directories that could not be read, already rendered for display. A scan
	// that hit problems still reports everything it did manage to index.
	Problems []string `json:"problems,omitempty"`
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

// Row is one horizontal strip on the home screen.
type Row struct {
	Title  string  `json:"title"`
	Genre  string  `json:"genre,omitempty"`
	Movies []Movie `json:"movies"`
}

// Home is everything the home screen needs, in one request: the hero and the
// rows beneath it. One round trip rather than one per row, because the client
// cannot know which genres exist until the server tells it.
type Home struct {
	Hero  *Movie `json:"hero"`
	Rows  []Row  `json:"rows"`
	Total int    `json:"total"`
}

// HomeScreen assembles the hero and the genre rows.
//
// A film appears in every row its genres put it in; deduplicating across rows
// would leave the later ones thin for no benefit, and seeing a favourite twice
// under two genres is how every streaming service behaves.
func (s *Service) HomeScreen(ctx context.Context, rowCount, perRow int) (*Home, error) {
	total, err := s.store.Count(ctx)
	if err != nil {
		return nil, err
	}
	home := &Home{Total: total}
	if total == 0 {
		return home, nil
	}

	if hero, err := s.store.Hero(ctx); err == nil {
		home.Hero = &hero
	} else if !errors.Is(err, ErrNoSuchMovie) {
		return nil, err
	}

	// Continue watching comes first when there is anything in it. Somebody
	// opening a media server mid-film wants one click, not a search.
	inProgress, err := s.store.ContinueWatching(ctx, perRow)
	if err != nil {
		return nil, err
	}
	if len(inProgress) > 0 {
		home.Rows = append(home.Rows, Row{Title: "Continuer à regarder", Movies: inProgress})
	}

	// Recently added next, because it answers the other question people open a
	// media server with: what is new since last time.
	recent, err := s.store.RecentlyAdded(ctx, perRow)
	if err != nil {
		return nil, err
	}
	if len(recent) > 0 {
		home.Rows = append(home.Rows, Row{Title: "Récemment ajoutés", Movies: recent})
	}

	genres, err := s.store.Genres(ctx, rowCount)
	if err != nil {
		return nil, err
	}
	for _, genre := range genres {
		movies, err := s.store.ByGenre(ctx, genre.Name, perRow)
		if err != nil {
			return nil, err
		}
		if len(movies) == 0 {
			continue
		}
		home.Rows = append(home.Rows, Row{Title: genre.Name, Genre: genre.Name, Movies: movies})
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
			report.Problems = append(report.Problems, err.Error())
			continue
		}
		if res.inserted {
			report.Added++
		} else {
			report.Updated++
		}
	}

	// Only prune when the walk completed without trouble. A disconnected drive
	// looks exactly like an emptied one from here, and wiping a library because
	// a USB cable came loose is not a recoverable mistake.
	if len(found.Problems) == 0 {
		removed, err := s.store.DeleteNotSeenIn(ctx, generation)
		if err != nil {
			return nil, err
		}
		report.Removed = removed
	} else {
		s.log.Warn("skipping removal of missing files, some directories could not be read",
			"problems", len(found.Problems))
	}

	// Metadata comes last, so that the library is complete and browsable before
	// a single network request is made. A TMDB outage delays posters; it never
	// delays knowing what you own.
	s.enrich(ctx, report)

	report.Duration = time.Since(startedAt)
	report.Seconds = report.Duration.Seconds()

	s.log.Info("scan finished",
		"found", report.Found,
		"added", report.Added,
		"updated", report.Updated,
		"removed", report.Removed,
		"enriched", report.Enriched,
		"not_found", report.NotFound,
		"problems", len(report.Problems),
		"duration", report.Duration,
	)
	s.remember(report)
	return report, nil
}

func (s *Service) remember(r *ScanReport) {
	s.mu.Lock()
	s.last = r
	s.mu.Unlock()
}

// Package tmdb is a small client for the parts of The Movie Database Theia
// actually uses: finding a film from a title and a year, and reading its
// details and cast.
//
// It follows the same rule as the filename parser: a film that cannot be
// identified is not an error. ErrNotFound is an ordinary outcome, and the
// caller keeps whatever the filename gave it.
//
// This product uses the TMDB API but is not endorsed or certified by TMDB.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// ErrNotFound means TMDB has no film matching the search. It is expected often
// enough that callers must handle it as a normal result rather than a failure.
var ErrNotFound = errors.New("tmdb: no matching film")

// ErrUnauthorized means the API key was rejected. Unlike everything else here
// this is worth surfacing loudly: every lookup will fail the same way until
// somebody fixes the key.
var ErrUnauthorized = errors.New("tmdb: the API key was rejected")

const (
	defaultBaseURL  = "https://api.themoviedb.org/3"
	defaultImageURL = "https://image.tmdb.org/t/p"

	// The interface is French, so ask TMDB for French titles and synopses. It
	// falls back to English on its own when a translation is missing.
	language = "fr-FR"

	// TMDB tolerates around 50 requests a second. Staying well under that
	// costs nothing on a library scan and keeps us a long way from a 429.
	requestInterval = 50 * time.Millisecond

	// How many cast members are worth keeping. Past the first handful nobody
	// reads them, and the rows get large.
	maxCast = 10
)

// Client talks to TMDB.
type Client struct {
	http     *http.Client
	token    string
	baseURL  string
	imageURL string
	limiter  *limiter
}

// Option customises a Client. Only tests are expected to use these.
type Option func(*Client)

// WithBaseURL points the client at a different API host.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithImageBaseURL points the client at a different image host.
func WithImageBaseURL(u string) Option { return func(c *Client) { c.imageURL = u } }

// WithHTTPClient replaces the underlying HTTP client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithRequestInterval changes the minimum spacing between requests.
func WithRequestInterval(d time.Duration) Option {
	return func(c *Client) { c.limiter = &limiter{interval: d} }
}

// New builds a client. The token is a TMDB read access token, sent as a bearer
// credential rather than as a query parameter -- a key in a URL ends up in every
// proxy log between here and TMDB.
func New(token string, opts ...Option) *Client {
	c := &Client{
		http:     &http.Client{Timeout: 20 * time.Second},
		token:    token,
		baseURL:  defaultBaseURL,
		imageURL: defaultImageURL,
		limiter:  &limiter{interval: requestInterval},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Film is everything Theia keeps from TMDB about one film.
type Film struct {
	TMDBID           int      `json:"tmdb_id"`
	Title            string   `json:"title"`
	OriginalTitle    string   `json:"original_title"`
	OriginalLanguage string   `json:"original_language"`
	Tagline          string   `json:"tagline"`
	Overview         string   `json:"overview"`
	ReleaseDate      string   `json:"release_date"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	Runtime          int      `json:"runtime"`
	VoteAverage      float64  `json:"vote_average"`
	Genres           []string `json:"genres"`
	Cast             []Person `json:"cast"`
	Director         string   `json:"director"`

	// Crew beyond the director, carrying a role code rather than TMDB's job
	// title: the interface writes the sentence, so it cannot be handed
	// "Original Music Composer" to print.
	Crew []Person `json:"crew"`

	// Certification is the age rating as an authority wrote it -- "12", "R",
	// "TP" -- with the country it was issued in, because "16" means different
	// things in different places and the interface says which.
	Certification        string `json:"certification"`
	CertificationCountry string `json:"certification_country"`

	// Collection is the TMDB grouping a film belongs to, when it belongs to
	// one. Theia uses it to find the other parts the user already owns; it
	// never lists parts they do not have.
	Collection *Collection `json:"collection,omitempty"`
}

// Collection is a TMDB film grouping: a trilogy, a saga, a pair of sequels.
type Collection struct {
	TMDBID     int    `json:"tmdb_id"`
	Name       string `json:"name"`
	PosterPath string `json:"poster_path,omitempty"`
}

// Person is one credited name. Role is empty for the cast, whose part is in
// Character, and one of the role codes below for the crew.
type Person struct {
	Name        string `json:"name"`
	Character   string `json:"character,omitempty"`
	Role        string `json:"role,omitempty"`
	ProfilePath string `json:"profile_path,omitempty"`
}

// Crew role codes. These cross the API to the interface, which owns the words
// (decision 25), so they are stable identifiers and not job titles.
const (
	RoleWriting        = "writing"
	RoleMusic          = "music"
	RoleCinematography = "cinematography"
)

// creditedJobs maps the TMDB job titles worth keeping onto role codes. It is a
// whitelist: a film has upwards of a hundred crew entries and this page is not
// a call sheet.
var creditedJobs = map[string]string{
	"Screenplay":              RoleWriting,
	"Writer":                  RoleWriting,
	"Story":                   RoleWriting,
	"Novel":                   RoleWriting,
	"Original Music Composer": RoleMusic,
	"Music":                   RoleMusic,
	"Director of Photography": RoleCinematography,
}

// maxCrewPerRole keeps a role from filling the line on a film with four
// credited writers.
const maxCrewPerRole = 2

// Search finds the film best matching a title and, when known, a year.
//
// Year is the strongest signal available: two films share a title far more
// often than they share a title and a release year. When the search with a year
// comes back empty the search is retried without it, because a filename can
// carry the wrong year and a slightly wrong match beats no match at all.
func (c *Client) Search(ctx context.Context, title string, year int) (int, error) {
	id, err := c.search(ctx, title, year)
	if err == nil {
		return id, nil
	}
	if year != 0 && errors.Is(err, ErrNotFound) {
		return c.search(ctx, title, 0)
	}
	return 0, err
}

type searchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	Popularity    float64 `json:"popularity"`

	// Read only when the results are being shown to somebody rather than
	// resolved to a single id. See Candidates.
	Overview   string `json:"overview"`
	PosterPath string `json:"poster_path"`
}

type searchResponse struct {
	Results []searchResult `json:"results"`
}

func (c *Client) search(ctx context.Context, title string, year int) (int, error) {
	if strings.TrimSpace(title) == "" {
		return 0, ErrNotFound
	}

	params := url.Values{}
	params.Set("query", title)
	params.Set("language", language)
	params.Set("include_adult", "false")
	if year != 0 {
		params.Set("primary_release_year", strconv.Itoa(year))
	}

	var body searchResponse
	if err := c.get(ctx, "/search/movie?"+params.Encode(), &body); err != nil {
		return 0, err
	}
	if len(body.Results) == 0 {
		return 0, ErrNotFound
	}
	return pick(body.Results, title), nil
}

// pick chooses the best result for a search.
//
// TMDB does not order results by popularity, whatever the field name suggests
// -- it orders them by its own text relevance. Searching "The Handmaiden" with
// the right year puts "Making of The Handmaiden" (popularity 0.6) ahead of the
// film itself (popularity 19.1), because the making-of's title contains the
// query literally. Taking the first result therefore catalogues a library full
// of documentaries about films instead of the films.
//
// Two rules, in order:
//
//  1. An exact title match wins. Both title and original_title are checked,
//     because the request asks for French and a Korean film comes back as
//     "Mademoiselle" -- a filename saying "The Handmaiden" matches neither the
//     French title nor the Korean original, but plenty of films do match one
//     or the other.
//  2. Failing that, the most popular result wins. A film is reliably more
//     popular than the making-of about it.
func pick(results []searchResult, title string) int {
	wanted := normalise(title)

	best := results[0]
	for _, r := range results {
		if normalise(r.Title) == wanted || normalise(r.OriginalTitle) == wanted {
			return r.ID
		}
		if r.Popularity > best.Popularity {
			best = r
		}
	}
	return best.ID
}

type detailsResponse struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	OriginalLanguage string  `json:"original_language"`
	Tagline          string  `json:"tagline"`
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	Runtime          int     `json:"runtime"`
	VoteAverage      float64 `json:"vote_average"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	BelongsToCollection *struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		PosterPath string `json:"poster_path"`
	} `json:"belongs_to_collection"`
	Credits struct {
		Cast []struct {
			Name        string `json:"name"`
			Character   string `json:"character"`
			ProfilePath string `json:"profile_path"`
		} `json:"cast"`
		Crew []struct {
			Name        string `json:"name"`
			Job         string `json:"job"`
			ProfilePath string `json:"profile_path"`
		} `json:"crew"`
	} `json:"credits"`
	ReleaseDates struct {
		Results []releaseDateCountry `json:"results"`
	} `json:"release_dates"`
}

// releaseDateCountry is one country block of TMDB release_dates: several dated
// releases, each of which may carry the certificate in force for it.
type releaseDateCountry struct {
	Country string             `json:"iso_3166_1"`
	Dates   []releaseDateEntry `json:"release_dates"`
}

type releaseDateEntry struct {
	Certification string `json:"certification"`
	Type          int    `json:"type"`
}

// Details fetches one film, with its credits and its certificates, in the same
// round trip.
//
// append_to_response is the whole point: credits and release_dates cost a
// larger body, where two further requests would each pay the latency and the
// rate limit again. Everything the film page shows comes from this one call.
func (c *Client) Details(ctx context.Context, id int) (*Film, error) {
	var body detailsResponse
	path := fmt.Sprintf("/movie/%d?language=%s&append_to_response=credits,release_dates", id, language)
	if err := c.get(ctx, path, &body); err != nil {
		return nil, err
	}

	film := &Film{
		TMDBID:           body.ID,
		Title:            body.Title,
		OriginalTitle:    strings.TrimSpace(body.OriginalTitle),
		OriginalLanguage: body.OriginalLanguage,
		Tagline:          strings.TrimSpace(body.Tagline),
		Overview:         body.Overview,
		ReleaseDate:      body.ReleaseDate,
		PosterPath:       body.PosterPath,
		BackdropPath:     body.BackdropPath,
		Runtime:          body.Runtime,
		VoteAverage:      body.VoteAverage,
	}
	for _, g := range body.Genres {
		film.Genres = append(film.Genres, g.Name)
	}
	if group := body.BelongsToCollection; group != nil && group.ID > 0 {
		film.Collection = &Collection{
			TMDBID:     group.ID,
			Name:       strings.TrimSpace(group.Name),
			PosterPath: group.PosterPath,
		}
	}
	for i, person := range body.Credits.Cast {
		if i >= maxCast {
			break
		}
		film.Cast = append(film.Cast, Person{
			Name:        person.Name,
			Character:   person.Character,
			ProfilePath: person.ProfilePath,
		})
	}

	perRole := map[string]int{}
	for _, person := range body.Credits.Crew {
		if person.Job == "Director" {
			if film.Director == "" {
				film.Director = person.Name
			}
			continue
		}
		role, wanted := creditedJobs[person.Job]
		if !wanted || person.Name == "" {
			continue
		}
		// TMDB credits somebody once per job, so a writer-director appears in
		// both lists and an adapted screenplay lists the novelist twice. A name
		// is kept once per role, and never as the writer of the film it already
		// says they directed.
		if person.Name == film.Director || hasCredit(film.Crew, role, person.Name) {
			continue
		}
		if perRole[role] >= maxCrewPerRole {
			continue
		}
		perRole[role]++
		film.Crew = append(film.Crew, Person{
			Name:        person.Name,
			Role:        role,
			ProfilePath: person.ProfilePath,
		})
	}

	film.Certification, film.CertificationCountry = pickCertification(body.ReleaseDates.Results)
	return film, nil
}

// hasCredit reports whether a name already holds a role.
func hasCredit(crew []Person, role, name string) bool {
	for _, person := range crew {
		if person.Role == role && person.Name == name {
			return true
		}
	}
	return false
}

// certificationCountries is the order of preference. The interface defaults to
// French, so a French certificate is the one this household recognises; the
// others are fallbacks, in the order of how often TMDB actually holds them.
var certificationCountries = []string{"FR", "US", "GB", "CA"}

// releaseTypeTheatrical is TMDB's code for a cinema release. Where a country
// lists several, the cinema certificate is the one that was argued over.
const releaseTypeTheatrical = 3

// pickCertification chooses one age rating out of TMDB's per-country lists and
// returns it with the country that issued it. Empty is an ordinary answer:
// plenty of films carry no certificate in any country we ask about.
func pickCertification(results []releaseDateCountry) (string, string) {
	for _, wanted := range certificationCountries {
		for _, result := range results {
			if result.Country != wanted {
				continue
			}
			var fallback string
			for _, date := range result.Dates {
				value := strings.TrimSpace(date.Certification)
				if value == "" {
					continue
				}
				if date.Type == releaseTypeTheatrical {
					return value, result.Country
				}
				if fallback == "" {
					fallback = value
				}
			}
			if fallback != "" {
				return fallback, result.Country
			}
		}
	}
	return "", ""
}

// Lookup is Search followed by Details, which is what callers actually want.
func (c *Client) Lookup(ctx context.Context, title string, year int) (*Film, error) {
	id, err := c.Search(ctx, title, year)
	if err != nil {
		return nil, err
	}
	return c.Details(ctx, id)
}

// ImageURL builds the URL for a poster or backdrop path as TMDB returns it.
func (c *Client) ImageURL(size, path string) string {
	if path == "" {
		return ""
	}
	return c.imageURL + "/" + size + "/" + strings.TrimPrefix(path, "/")
}

// FetchImage downloads an image and returns its bytes and content type.
func (c *Client) FetchImage(ctx context.Context, size, path string) ([]byte, string, error) {
	target := c.ImageURL(size, path)
	if target == "" {
		return nil, "", ErrNotFound
	}

	if err := c.limiter.wait(ctx); err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", fmt.Errorf("tmdb: building image request: %w", err)
	}
	// The image CDN is public and does not want the bearer token.
	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("tmdb: fetching image: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, "", ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("tmdb: fetching image: unexpected status %d", res.StatusCode)
	}

	// Posters top out around 200 KB; the ceiling is here so that a redirect to
	// something unexpected cannot fill the cache directory.
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, "", fmt.Errorf("tmdb: reading image: %w", err)
	}

	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return data, contentType, nil
}

// get performs a rate-limited, retrying GET against the API.
func (c *Client) get(ctx context.Context, path string, into any) error {
	const attempts = 3

	var lastErr error
	for attempt := range attempts {
		if attempt > 0 {
			// A flat, short backoff. TMDB rate limits are measured in seconds,
			// not minutes, and a scan should not stall on one stubborn film.
			delay := time.Duration(attempt) * time.Second
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		retryable, err := c.attempt(ctx, path, into)
		if err == nil {
			return nil
		}
		lastErr = err
		if !retryable {
			return err
		}
	}
	return lastErr
}

// attempt makes one request. The boolean says whether trying again could help.
func (c *Client) attempt(ctx context.Context, path string, into any) (retryable bool, err error) {
	if err := c.limiter.wait(ctx); err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return false, fmt.Errorf("tmdb: building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return true, fmt.Errorf("tmdb: %w", err)
	}
	defer res.Body.Close()

	switch {
	case res.StatusCode == http.StatusOK:
		if err := json.NewDecoder(res.Body).Decode(into); err != nil {
			return false, fmt.Errorf("tmdb: decoding response: %w", err)
		}
		return false, nil

	case res.StatusCode == http.StatusUnauthorized, res.StatusCode == http.StatusForbidden:
		return false, ErrUnauthorized

	case res.StatusCode == http.StatusNotFound:
		return false, ErrNotFound

	case res.StatusCode == http.StatusTooManyRequests:
		return true, fmt.Errorf("tmdb: rate limited")

	case res.StatusCode >= 500:
		return true, fmt.Errorf("tmdb: server error %d", res.StatusCode)

	default:
		io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
		return false, fmt.Errorf("tmdb: unexpected status %d", res.StatusCode)
	}
}

// normalise reduces a title for comparison: lowercase, no punctuation, single
// spaces. "The Matrix" matches "The Matrix.".
//
// Classification is by Unicode category rather than by codepoint range.
// Anything above 127 is not automatically a letter -- that would keep the
// interpunct in "WALL·E" and the guillemets around a French title -- while
// IsLetter keeps é and à, which do carry meaning and must survive.
func normalise(s string) string {
	var b strings.Builder
	lastWasSpace := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastWasSpace = false
			continue
		}
		if !lastWasSpace {
			b.WriteByte(' ')
			lastWasSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// limiter spaces requests apart without any dependency on a rate-limiting
// library. Callers are serialised: each one claims the next slot and waits for
// it, so concurrent lookups queue rather than burst.
type limiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func (l *limiter) wait(ctx context.Context) error {
	if l == nil || l.interval <= 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	wait := l.next.Sub(now)
	l.next = l.next.Add(l.interval)
	l.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

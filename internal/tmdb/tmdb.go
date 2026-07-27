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
	TMDBID       int      `json:"tmdb_id"`
	Title        string   `json:"title"`
	Overview     string   `json:"overview"`
	ReleaseDate  string   `json:"release_date"`
	PosterPath   string   `json:"poster_path"`
	BackdropPath string   `json:"backdrop_path"`
	Runtime      int      `json:"runtime"`
	VoteAverage  float64  `json:"vote_average"`
	Genres       []string `json:"genres"`
	Cast         []Person `json:"cast"`
	Director     string   `json:"director"`
}

// Person is one credited name.
type Person struct {
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
}

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
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	Runtime      int     `json:"runtime"`
	VoteAverage  float64 `json:"vote_average"`
	Genres       []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Credits struct {
		Cast []struct {
			Name      string `json:"name"`
			Character string `json:"character"`
		} `json:"cast"`
		Crew []struct {
			Name string `json:"name"`
			Job  string `json:"job"`
		} `json:"crew"`
	} `json:"credits"`
}

// Details fetches one film, with its credits in the same round trip.
func (c *Client) Details(ctx context.Context, id int) (*Film, error) {
	var body detailsResponse
	path := fmt.Sprintf("/movie/%d?language=%s&append_to_response=credits", id, language)
	if err := c.get(ctx, path, &body); err != nil {
		return nil, err
	}

	film := &Film{
		TMDBID:       body.ID,
		Title:        body.Title,
		Overview:     body.Overview,
		ReleaseDate:  body.ReleaseDate,
		PosterPath:   body.PosterPath,
		BackdropPath: body.BackdropPath,
		Runtime:      body.Runtime,
		VoteAverage:  body.VoteAverage,
	}
	for _, g := range body.Genres {
		film.Genres = append(film.Genres, g.Name)
	}
	for i, person := range body.Credits.Cast {
		if i >= maxCast {
			break
		}
		film.Cast = append(film.Cast, Person{Name: person.Name, Character: person.Character})
	}
	for _, person := range body.Credits.Crew {
		if person.Job == "Director" {
			film.Director = person.Name
			break
		}
	}
	return film, nil
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

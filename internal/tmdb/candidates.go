package tmdb

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Candidate is one possible match, offered to somebody correcting a wrong one.
//
// It carries what a person needs to recognise the right film across a row of
// near-identical titles: the poster, the year, and the first line of the
// synopsis. Not what the library stores — nothing here is written anywhere
// until a choice is made and the full record is fetched by id.
type Candidate struct {
	TMDBID        int    `json:"tmdb_id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	PosterPath    string `json:"poster_path,omitempty"`
}

// maxCandidates is how many possibilities are worth showing. TMDB returns
// twenty per page; past the first handful they stop resembling the query.
const maxCandidates = 8

// Candidates lists the films that could be meant by a query.
//
// Search answers "which one is it", by the rules in pick. This answers the
// different question a human asks when pick got it wrong: "what else was
// there". Same request, no verdict.
func (c *Client) Candidates(ctx context.Context, query string, year int) ([]Candidate, error) {
	results, err := c.searchCandidates(ctx, query, year)
	if err != nil {
		return nil, err
	}
	// A wrong year is one of the two reasons somebody is on this screen at all
	// — the other being a wrong title — so a search that comes back empty is
	// retried without it, exactly as Search does.
	if len(results) == 0 && year != 0 {
		if results, err = c.searchCandidates(ctx, query, 0); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (c *Client) searchCandidates(ctx context.Context, query string, year int) ([]Candidate, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("language", language)
	params.Set("include_adult", "false")
	if year != 0 {
		params.Set("primary_release_year", strconv.Itoa(year))
	}

	var body searchResponse
	if err := c.get(ctx, "/search/movie?"+params.Encode(), &body); err != nil {
		return nil, err
	}

	ranked := rankByLikelihood(body.Results, query, func(r searchResult) (string, string, float64) {
		return r.Title, r.OriginalTitle, r.Popularity
	})

	out := make([]Candidate, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, Candidate{
			TMDBID:        r.ID,
			Title:         r.Title,
			OriginalTitle: r.OriginalTitle,
			Year:          yearOf(r.ReleaseDate),
			Overview:      r.Overview,
			PosterPath:    r.PosterPath,
		})
	}
	return out, nil
}

// TVCandidates is Candidates for series.
func (c *Client) TVCandidates(ctx context.Context, query string, year int) ([]Candidate, error) {
	results, err := c.searchTVCandidates(ctx, query, year)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 && year != 0 {
		if results, err = c.searchTVCandidates(ctx, query, 0); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (c *Client) searchTVCandidates(ctx context.Context, query string, year int) ([]Candidate, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("language", language)
	params.Set("include_adult", "false")
	if year != 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}

	var body tvSearchResponse
	if err := c.get(ctx, "/search/tv?"+params.Encode(), &body); err != nil {
		return nil, err
	}

	ranked := rankByLikelihood(body.Results, query, func(r tvSearchResult) (string, string, float64) {
		return r.Name, r.OriginalName, r.Popularity
	})

	out := make([]Candidate, 0, len(ranked))
	for _, r := range ranked {
		out = append(out, Candidate{
			TMDBID:        r.ID,
			Title:         r.Name,
			OriginalTitle: r.OriginalName,
			Year:          yearOf(r.FirstAirDate),
			Overview:      r.Overview,
			PosterPath:    r.PosterPath,
		})
	}
	return out, nil
}

// rankByLikelihood orders results the way pick chooses between them: an exact
// title match first, then the most popular.
//
// Not TMDB's own order, which is text relevance and is what put "Making of The
// Handmaiden" above the film in the first place. Somebody who came here to fix
// a wrong match should not have to scroll past the same wrong match.
func rankByLikelihood[T any](results []T, query string, fields func(T) (string, string, float64)) []T {
	wanted := normalise(query)

	ranked := make([]T, len(results))
	copy(ranked, results)

	exact := func(v T) bool {
		title, original, _ := fields(v)
		return normalise(title) == wanted || normalise(original) == wanted
	}
	popularity := func(v T) float64 {
		_, _, p := fields(v)
		return p
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if a, b := exact(ranked[i]), exact(ranked[j]); a != b {
			return a
		}
		return popularity(ranked[i]) > popularity(ranked[j])
	})

	if len(ranked) > maxCandidates {
		ranked = ranked[:maxCandidates]
	}
	return ranked
}

// yearOf reads the year off a TMDB date, which is "1999-03-31" or empty.
func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

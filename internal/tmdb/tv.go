package tmdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// TVSeries is the subset of TMDB's series details persisted by Theia.
type TVSeries struct {
	TMDBID       int
	Name         string
	OriginalName string
	Tagline      string
	Overview     string
	FirstAirDate string
	LastAirDate  string
	PosterPath   string
	BackdropPath string
	VoteAverage  float64
	Genres       []string
	Cast         []Person
	Creators     []string
	Networks     []string

	// Status is a code, not TMDB's English label: the interface owns the
	// sentence that says a series has ended (decision 25).
	Status string

	// Certification is the content rating and the country that set it, chosen
	// the same way as a film's certificate.
	Certification        string
	CertificationCountry string
}

// Series status codes.
const (
	SeriesEnded        = "ended"
	SeriesReturning    = "returning"
	SeriesCanceled     = "canceled"
	SeriesInProduction = "in_production"
	SeriesPlanned      = "planned"
	SeriesPilot        = "pilot"
)

// seriesStatuses maps TMDB's status strings onto those codes. Anything absent
// from this map is dropped rather than passed through: an unknown English
// string reaching the interface is the fault decision 25 exists to prevent.
var seriesStatuses = map[string]string{
	"Ended":            SeriesEnded,
	"Returning Series": SeriesReturning,
	"Canceled":         SeriesCanceled,
	"Cancelled":        SeriesCanceled,
	"In Production":    SeriesInProduction,
	"Planned":          SeriesPlanned,
	"Pilot":            SeriesPilot,
}

// maxNetworks keeps the line short on a series that changed broadcaster twice.
const maxNetworks = 2

type TVSeason struct {
	TMDBID       int
	Number       int
	Name         string
	Overview     string
	AirDate      string
	PosterPath   string
	EpisodeCount int
	Episodes     []TVEpisode
}

type TVEpisode struct {
	TMDBID         int
	Number         int
	Name           string
	Overview       string
	AirDate        string
	StillPath      string
	RuntimeMinutes int
	VoteAverage    float64
}

type tvSearchResult struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	FirstAirDate string  `json:"first_air_date"`
	Popularity   float64 `json:"popularity"`

	// Shown, not matched on. See Candidates.
	Overview   string `json:"overview"`
	PosterPath string `json:"poster_path"`
}

type tvSearchResponse struct {
	Results []tvSearchResult `json:"results"`
}

// SearchTV finds the strongest series match and retries without a year only
// after an ordinary miss. The year filter is first_air_date_year, which is the
// TMDB TV equivalent of primary_release_year for films.
func (c *Client) SearchTV(ctx context.Context, title string, year int) (int, error) {
	id, err := c.searchTV(ctx, title, year)
	if err == nil {
		return id, nil
	}
	if year != 0 && errors.Is(err, ErrNotFound) {
		return c.searchTV(ctx, title, 0)
	}
	return 0, err
}

func (c *Client) searchTV(ctx context.Context, title string, year int) (int, error) {
	if strings.TrimSpace(title) == "" {
		return 0, ErrNotFound
	}
	params := url.Values{}
	params.Set("query", title)
	params.Set("language", language)
	params.Set("include_adult", "false")
	if year != 0 {
		params.Set("first_air_date_year", strconv.Itoa(year))
	}

	var body tvSearchResponse
	if err := c.get(ctx, "/search/tv?"+params.Encode(), &body); err != nil {
		return 0, err
	}
	if len(body.Results) == 0 {
		return 0, ErrNotFound
	}
	return pickTV(body.Results, title), nil
}

func pickTV(results []tvSearchResult, title string) int {
	wanted := normalise(title)
	best := results[0]
	for _, result := range results {
		if normalise(result.Name) == wanted || normalise(result.OriginalName) == wanted {
			return result.ID
		}
		if result.Popularity > best.Popularity {
			best = result
		}
	}
	return best.ID
}

type tvDetailsResponse struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	Tagline      string  `json:"tagline"`
	Overview     string  `json:"overview"`
	FirstAirDate string  `json:"first_air_date"`
	LastAirDate  string  `json:"last_air_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	Status       string  `json:"status"`
	Genres       []struct {
		Name string `json:"name"`
	} `json:"genres"`
	CreatedBy []struct {
		Name string `json:"name"`
	} `json:"created_by"`
	Networks []struct {
		Name string `json:"name"`
	} `json:"networks"`
	Credits struct {
		Cast []struct {
			Name        string `json:"name"`
			Character   string `json:"character"`
			ProfilePath string `json:"profile_path"`
		} `json:"cast"`
	} `json:"credits"`
	ContentRatings struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Rating  string `json:"rating"`
		} `json:"results"`
	} `json:"content_ratings"`
}

// TVDetails fetches a series with its credits and its content ratings in one
// round trip, for the same reason Details does.
func (c *Client) TVDetails(ctx context.Context, id int) (*TVSeries, error) {
	var body tvDetailsResponse
	path := fmt.Sprintf("/tv/%d?language=%s&append_to_response=credits,content_ratings", id, language)
	if err := c.get(ctx, path, &body); err != nil {
		return nil, err
	}
	series := &TVSeries{
		TMDBID:       body.ID,
		Name:         body.Name,
		OriginalName: strings.TrimSpace(body.OriginalName),
		Tagline:      strings.TrimSpace(body.Tagline),
		Overview:     body.Overview,
		FirstAirDate: body.FirstAirDate,
		LastAirDate:  body.LastAirDate,
		PosterPath:   body.PosterPath,
		BackdropPath: body.BackdropPath,
		VoteAverage:  body.VoteAverage,
		Status:       seriesStatuses[strings.TrimSpace(body.Status)],
	}
	for _, genre := range body.Genres {
		series.Genres = append(series.Genres, genre.Name)
	}
	for _, creator := range body.CreatedBy {
		if creator.Name != "" {
			series.Creators = append(series.Creators, creator.Name)
		}
	}
	for _, network := range body.Networks {
		if network.Name == "" || len(series.Networks) >= maxNetworks {
			continue
		}
		series.Networks = append(series.Networks, network.Name)
	}
	for index, person := range body.Credits.Cast {
		if index >= maxCast {
			break
		}
		series.Cast = append(series.Cast, Person{
			Name:        person.Name,
			Character:   person.Character,
			ProfilePath: person.ProfilePath,
		})
	}
	series.Certification, series.CertificationCountry = pickTVRating(body.ContentRatings.Results)
	return series, nil
}

// pickTVRating is pickCertification for series, where TMDB gives one rating per
// country and no release types to choose between.
func pickTVRating(results []struct {
	Country string `json:"iso_3166_1"`
	Rating  string `json:"rating"`
}) (string, string) {
	for _, wanted := range certificationCountries {
		for _, result := range results {
			if result.Country != wanted {
				continue
			}
			if rating := strings.TrimSpace(result.Rating); rating != "" {
				return rating, result.Country
			}
		}
	}
	return "", ""
}

type tvSeasonDetailsResponse struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	AirDate      string `json:"air_date"`
	PosterPath   string `json:"poster_path"`
	Episodes     []struct {
		ID            int     `json:"id"`
		EpisodeNumber int     `json:"episode_number"`
		Name          string  `json:"name"`
		Overview      string  `json:"overview"`
		AirDate       string  `json:"air_date"`
		StillPath     string  `json:"still_path"`
		Runtime       int     `json:"runtime"`
		VoteAverage   float64 `json:"vote_average"`
	} `json:"episodes"`
}

func (c *Client) TVSeasonDetails(ctx context.Context, seriesID, seasonNumber int) (*TVSeason, error) {
	var body tvSeasonDetailsResponse
	path := fmt.Sprintf("/tv/%d/season/%d?language=%s", seriesID, seasonNumber, language)
	if err := c.get(ctx, path, &body); err != nil {
		return nil, err
	}
	season := &TVSeason{
		TMDBID:       body.ID,
		Number:       body.SeasonNumber,
		Name:         body.Name,
		Overview:     body.Overview,
		AirDate:      body.AirDate,
		PosterPath:   body.PosterPath,
		EpisodeCount: len(body.Episodes),
	}
	for _, episode := range body.Episodes {
		season.Episodes = append(season.Episodes, TVEpisode{
			TMDBID:         episode.ID,
			Number:         episode.EpisodeNumber,
			Name:           episode.Name,
			Overview:       episode.Overview,
			AirDate:        episode.AirDate,
			StillPath:      episode.StillPath,
			RuntimeMinutes: episode.Runtime,
			VoteAverage:    episode.VoteAverage,
		})
	}
	return season, nil
}

func (c *Client) LookupTV(ctx context.Context, title string, year int) (*TVSeries, error) {
	id, err := c.SearchTV(ctx, title, year)
	if err != nil {
		return nil, err
	}
	return c.TVDetails(ctx, id)
}

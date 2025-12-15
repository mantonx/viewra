package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL       = "https://api.themoviedb.org/3"
	imageBaseURL  = "https://image.tmdb.org/t/p"
	requestTimout = 10 * time.Second
)

// Client handles TMDb API requests.
type Client struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new TMDb API client.
func NewClient(apiKey string, logger *slog.Logger) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: requestTimout,
		},
		logger: logger,
	}
}

// MovieSearchResult represents a movie from TMDb search results.
type MovieSearchResult struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	ReleaseDate      string  `json:"release_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	OriginalLanguage string  `json:"original_language"`
	Popularity       float64 `json:"popularity"`
}

// MovieSearchResponse is the response from TMDb movie search.
type MovieSearchResponse struct {
	Page         int                 `json:"page"`
	TotalPages   int                 `json:"total_pages"`
	TotalResults int                 `json:"total_results"`
	Results      []MovieSearchResult `json:"results"`
}

// MovieDetails contains full movie information from TMDb.
type MovieDetails struct {
	ID               int      `json:"id"`
	IMDbID           string   `json:"imdb_id"`
	Title            string   `json:"title"`
	OriginalTitle    string   `json:"original_title"`
	Overview         string   `json:"overview"`
	Tagline          string   `json:"tagline"`
	ReleaseDate      string   `json:"release_date"`
	Runtime          int      `json:"runtime"`
	Budget           int64    `json:"budget"`
	Revenue          int64    `json:"revenue"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	OriginalLanguage string   `json:"original_language"`
	Genres           []Genre  `json:"genres"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	ProductionCountries []ProductionCountry `json:"production_countries"`

	// Appended responses (when using append_to_response)
	Credits *Credits `json:"credits,omitempty"`
	Images  *Images  `json:"images,omitempty"`
}

// TVSearchResult represents a TV show from TMDb search results.
type TVSearchResult struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	Overview         string  `json:"overview"`
	FirstAirDate     string  `json:"first_air_date"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	OriginalLanguage string  `json:"original_language"`
	Popularity       float64 `json:"popularity"`
}

// TVSearchResponse is the response from TMDb TV search.
type TVSearchResponse struct {
	Page         int              `json:"page"`
	TotalPages   int              `json:"total_pages"`
	TotalResults int              `json:"total_results"`
	Results      []TVSearchResult `json:"results"`
}

// TVDetails contains full TV show information from TMDb.
type TVDetails struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	OriginalName     string    `json:"original_name"`
	Overview         string    `json:"overview"`
	Tagline          string    `json:"tagline"`
	FirstAirDate     string    `json:"first_air_date"`
	LastAirDate      string    `json:"last_air_date"`
	Status           string    `json:"status"`
	Type             string    `json:"type"`
	NumberOfSeasons  int       `json:"number_of_seasons"`
	NumberOfEpisodes int       `json:"number_of_episodes"`
	EpisodeRunTime   []int     `json:"episode_run_time"`
	VoteAverage      float64   `json:"vote_average"`
	VoteCount        int       `json:"vote_count"`
	PosterPath       string    `json:"poster_path"`
	BackdropPath     string    `json:"backdrop_path"`
	OriginalLanguage string    `json:"original_language"`
	Genres           []Genre   `json:"genres"`
	Networks         []Network `json:"networks"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	CreatedBy        []Creator `json:"created_by"`

	// External IDs (when using append_to_response)
	ExternalIDs *ExternalIDs `json:"external_ids,omitempty"`
	Credits     *Credits     `json:"credits,omitempty"`
	Images      *Images      `json:"images,omitempty"`
}

// Genre represents a genre.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ProductionCompany represents a production company/studio.
type ProductionCompany struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// ProductionCountry represents a production country.
type ProductionCountry struct {
	ISO3166_1 string `json:"iso_3166_1"`
	Name      string `json:"name"`
}

// Network represents a TV network.
type Network struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// Creator represents a TV show creator.
type Creator struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}

// Credits contains cast and crew information.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// CastMember represents an actor in the cast.
type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
}

// CrewMember represents a crew member.
type CrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

// Images contains image information.
type Images struct {
	Backdrops []Image `json:"backdrops"`
	Posters   []Image `json:"posters"`
	Logos     []Image `json:"logos"`
}

// Image represents an image from TMDb.
type Image struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	ISO639_1    string  `json:"iso_639_1"`
}

// ExternalIDs contains external IDs for a TV show.
type ExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	TVDbID      int    `json:"tvdb_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

// FindByExternalIDResponse is the response from TMDb find endpoint.
type FindByExternalIDResponse struct {
	MovieResults []MovieSearchResult `json:"movie_results"`
	TVResults    []TVSearchResult    `json:"tv_results"`
}

// SearchMovies searches for movies by title and optional year.
func (c *Client) SearchMovies(ctx context.Context, title string, year int) (*MovieSearchResponse, error) {
	params := url.Values{}
	params.Set("query", title)
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	var result MovieSearchResponse
	if err := c.get(ctx, "/search/movie", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMovieDetails fetches detailed movie information.
func (c *Client) GetMovieDetails(ctx context.Context, tmdbID int) (*MovieDetails, error) {
	params := url.Values{}
	params.Set("append_to_response", "credits,images")

	var result MovieDetails
	if err := c.get(ctx, fmt.Sprintf("/movie/%d", tmdbID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchTV searches for TV shows by title and optional year.
func (c *Client) SearchTV(ctx context.Context, title string, year int) (*TVSearchResponse, error) {
	params := url.Values{}
	params.Set("query", title)
	if year > 0 {
		params.Set("first_air_date_year", fmt.Sprintf("%d", year))
	}

	var result TVSearchResponse
	if err := c.get(ctx, "/search/tv", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTVDetails fetches detailed TV show information.
func (c *Client) GetTVDetails(ctx context.Context, tmdbID int) (*TVDetails, error) {
	params := url.Values{}
	params.Set("append_to_response", "credits,images,external_ids")

	var result TVDetails
	if err := c.get(ctx, fmt.Sprintf("/tv/%d", tmdbID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FindByIMDbID looks up a movie or TV show by IMDb ID.
func (c *Client) FindByIMDbID(ctx context.Context, imdbID string) (*FindByExternalIDResponse, error) {
	params := url.Values{}
	params.Set("external_source", "imdb_id")

	var result FindByExternalIDResponse
	if err := c.get(ctx, fmt.Sprintf("/find/%s", imdbID), params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ImageURL returns the full URL for an image path.
func ImageURL(path string, size string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s%s", imageBaseURL, size, path)
}

// get performs a GET request to the TMDb API.
func (c *Client) get(ctx context.Context, path string, params url.Values, result interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.apiKey)

	reqURL := fmt.Sprintf("%s%s?%s", baseURL, path, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("TMDb API error (status %d): %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

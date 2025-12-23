package internal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// lookupStrategy defines media-type-specific lookup and response building.
type lookupStrategy interface {
	// extractTitle returns the title to search for.
	extractTitle(req *sdk.EnrichRequest) string
	// findByIMDb extracts the TMDb ID from an IMDb lookup response.
	findByIMDb(resp *FindByExternalIDResponse) int
	// search performs a title/year search and returns the best match ID.
	search(ctx context.Context, client *Client, title string, year int) (int, error)
	// getDetails fetches full details and builds the response.
	getDetails(ctx context.Context, client *Client, tmdbID int) (*sdk.EnrichResponse, error)
}

// enrich performs the common lookup flow using a media-type-specific strategy.
func (p *TMDbPlugin) enrich(ctx context.Context, client *Client, req *sdk.EnrichRequest, strategy lookupStrategy) (*sdk.EnrichResponse, error) {
	var tmdbID int
	var err error

	title := strategy.extractTitle(req)

	// Strategy 1: Use existing TMDb ID if available
	if idStr, ok := req.ExistingIDs["tmdb"]; ok {
		tmdbID, err = strconv.Atoi(idStr)
		if err != nil {
			p.logger.Warn("invalid tmdb ID", "id", idStr, "error", err)
			tmdbID = 0
		}
	}

	// Strategy 2: Look up by IMDb ID if available
	if tmdbID == 0 {
		if imdbID, ok := req.ExistingIDs["imdb"]; ok && imdbID != "" {
			p.logger.Debug("looking up by IMDb ID", "imdb_id", imdbID)
			findResp, err := client.FindByIMDbID(ctx, imdbID)
			if err != nil {
				p.recordError()
				return nil, fmt.Errorf("IMDb lookup failed: %w", err)
			}
			tmdbID = strategy.findByIMDb(findResp)
			if tmdbID > 0 {
				p.logger.Debug("found via IMDb ID", "tmdb_id", tmdbID)
			}
		}
	}

	// Strategy 3: Search by title/year
	if tmdbID == 0 && title != "" {
		p.logger.Debug("searching by title", "title", title, "year", req.Year)
		tmdbID, err = strategy.search(ctx, client, title, req.Year)
		if err != nil {
			p.recordError()
			return nil, err
		}
		if tmdbID > 0 {
			p.logger.Debug("found via search", "tmdb_id", tmdbID)
		}
	}

	if tmdbID == 0 {
		return &sdk.EnrichResponse{
			Matched:    false,
			Skipped:    true,
			SkipReason: "no match found",
		}, nil
	}

	// Fetch full details
	resp, err := strategy.getDetails(ctx, client, tmdbID)
	if err != nil {
		p.recordError()
		return nil, err
	}
	return resp, nil
}

// movieStrategy implements lookupStrategy for movies.
type movieStrategy struct {
	plugin *TMDbPlugin
}

func (s *movieStrategy) extractTitle(req *sdk.EnrichRequest) string {
	return req.Title
}

func (s *movieStrategy) findByIMDb(resp *FindByExternalIDResponse) int {
	if len(resp.MovieResults) > 0 {
		return resp.MovieResults[0].ID
	}
	return 0
}

func (s *movieStrategy) search(ctx context.Context, client *Client, title string, year int) (int, error) {
	searchResp, err := client.SearchMovies(ctx, title, year)
	if err != nil {
		return 0, fmt.Errorf("movie search failed: %w", err)
	}
	if len(searchResp.Results) > 0 {
		return searchResp.Results[0].ID, nil
	}
	return 0, nil
}

func (s *movieStrategy) getDetails(ctx context.Context, client *Client, tmdbID int) (*sdk.EnrichResponse, error) {
	details, err := client.GetMovieDetails(ctx, tmdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie details: %w", err)
	}
	return s.plugin.buildMovieResponse(details), nil
}

// tvStrategy implements lookupStrategy for TV shows.
type tvStrategy struct {
	plugin *TMDbPlugin
}

func (s *tvStrategy) extractTitle(req *sdk.EnrichRequest) string {
	if req.ShowTitle != "" {
		return req.ShowTitle
	}
	return req.Title
}

func (s *tvStrategy) findByIMDb(resp *FindByExternalIDResponse) int {
	if len(resp.TVResults) > 0 {
		return resp.TVResults[0].ID
	}
	return 0
}

func (s *tvStrategy) search(ctx context.Context, client *Client, title string, year int) (int, error) {
	searchResp, err := client.SearchTV(ctx, title, year)
	if err != nil {
		return 0, fmt.Errorf("TV search failed: %w", err)
	}
	if len(searchResp.Results) > 0 {
		return searchResp.Results[0].ID, nil
	}
	return 0, nil
}

func (s *tvStrategy) getDetails(ctx context.Context, client *Client, tmdbID int) (*sdk.EnrichResponse, error) {
	details, err := client.GetTVDetails(ctx, tmdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TV details: %w", err)
	}
	return s.plugin.buildTVResponse(details), nil
}

// enrichMovie fetches movie metadata from TMDb.
func (p *TMDbPlugin) enrichMovie(ctx context.Context, client *Client, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
	return p.enrich(ctx, client, req, &movieStrategy{plugin: p})
}

// enrichTV fetches TV show metadata from TMDb.
func (p *TMDbPlugin) enrichTV(ctx context.Context, client *Client, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
	return p.enrich(ctx, client, req, &tvStrategy{plugin: p})
}

// buildMovieResponse converts TMDb movie details to EnrichResponse.
func (p *TMDbPlugin) buildMovieResponse(movie *MovieDetails) *sdk.EnrichResponse {
	resp := &sdk.EnrichResponse{
		Matched:         true,
		DiscoveredIDs:   make(map[string]string),
		ConfidenceScore: 0.9,
	}

	// External IDs
	resp.DiscoveredIDs["tmdb"] = strconv.Itoa(movie.ID)
	if movie.IMDbID != "" {
		resp.DiscoveredIDs["imdb"] = movie.IMDbID
	}

	// Extract year from release date
	var year int
	if len(movie.ReleaseDate) >= 4 {
		if y, err := strconv.Atoi(movie.ReleaseDate[:4]); err == nil {
			year = y
		}
	}

	// Build metadata
	metadata := &sdk.EnrichedMetadata{
		Title:          strPtr(movie.Title),
		OriginalTitle:  strPtr(movie.OriginalTitle),
		Year:           intPtr(year),
		Plot:           strPtr(movie.Overview),
		Tagline:        strPtr(movie.Tagline),
		RuntimeMinutes: intPtr(movie.Runtime),
		Rating:         float32Ptr(float32(movie.VoteAverage)),
		RatingVotes:    intPtr(movie.VoteCount),
	}

	// Genres
	for _, g := range movie.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	// Studios/Production Companies
	for _, pc := range movie.ProductionCompanies {
		metadata.Studios = append(metadata.Studios, pc.Name)
	}

	// Credits
	if movie.Credits != nil {
		// Directors, writers, and producers from crew
		for _, crew := range movie.Credits.Crew {
			switch crew.Job {
			case "Director":
				metadata.Directors = append(metadata.Directors, crew.Name)
			case "Writer", "Screenplay":
				metadata.Writers = append(metadata.Writers, crew.Name)
			}
		}

		// Cast (all members for accurate search)
		for _, cast := range movie.Credits.Cast {
			metadata.Cast = append(metadata.Cast, sdk.CastMember{
				Name:  cast.Name,
				Role:  cast.Character,
				Thumb: ImageURL(cast.ProfilePath, "w185"),
				Order: cast.Order,
			})
		}
	}

	// Keywords (for location-based and thematic search)
	if movie.Keywords != nil {
		for _, kw := range movie.Keywords.Keywords {
			metadata.Keywords = append(metadata.Keywords, sdk.Keyword{
				ID:         kw.ID,
				Name:       kw.Name,
				IsLocation: isLocationKeyword(kw.Name),
			})
		}
	}

	resp.Metadata = metadata

	// Images
	resp.Images = append(resp.Images, sdk.EnrichedImage{
		Type:     "poster",
		Path:     ImageURL(movie.PosterPath, "original"),
		IsRemote: true,
	})
	resp.Images = append(resp.Images, sdk.EnrichedImage{
		Type:     "fanart",
		Path:     ImageURL(movie.BackdropPath, "original"),
		IsRemote: true,
	})

	// Additional images from credits
	if movie.Images != nil {
		for i, img := range movie.Images.Backdrops {
			if i >= 5 {
				break // Limit additional backdrops
			}
			resp.Images = append(resp.Images, sdk.EnrichedImage{
				Type:     "fanart",
				Path:     ImageURL(img.FilePath, "original"),
				IsRemote: true,
				Width:    img.Width,
				Height:   img.Height,
				Language: img.ISO639_1,
				Rating:   float32(img.VoteAverage),
			})
		}
	}

	return resp
}

// buildTVResponse converts TMDb TV details to EnrichResponse.
func (p *TMDbPlugin) buildTVResponse(tv *TVDetails) *sdk.EnrichResponse {
	resp := &sdk.EnrichResponse{
		Matched:         true,
		DiscoveredIDs:   make(map[string]string),
		ConfidenceScore: 0.9,
	}

	// External IDs
	resp.DiscoveredIDs["tmdb"] = strconv.Itoa(tv.ID)
	if tv.ExternalIDs != nil {
		if tv.ExternalIDs.IMDbID != "" {
			resp.DiscoveredIDs["imdb"] = tv.ExternalIDs.IMDbID
		}
		if tv.ExternalIDs.TVDbID > 0 {
			resp.DiscoveredIDs["tvdb"] = strconv.Itoa(tv.ExternalIDs.TVDbID)
		}
	}

	// Extract year from first air date
	var year int
	if len(tv.FirstAirDate) >= 4 {
		if y, err := strconv.Atoi(tv.FirstAirDate[:4]); err == nil {
			year = y
		}
	}

	// Average runtime
	var avgRuntime int
	if len(tv.EpisodeRunTime) > 0 {
		var total int
		for _, r := range tv.EpisodeRunTime {
			total += r
		}
		avgRuntime = total / len(tv.EpisodeRunTime)
	}

	// Build metadata
	metadata := &sdk.EnrichedMetadata{
		Title:          strPtr(tv.Name),
		OriginalTitle:  strPtr(tv.OriginalName),
		Year:           intPtr(year),
		Plot:           strPtr(tv.Overview),
		Tagline:        strPtr(tv.Tagline),
		RuntimeMinutes: intPtr(avgRuntime),
		Rating:         float32Ptr(float32(tv.VoteAverage)),
		RatingVotes:    intPtr(tv.VoteCount),
	}

	// Genres
	for _, g := range tv.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	// Studios/Production Companies
	for _, pc := range tv.ProductionCompanies {
		metadata.Studios = append(metadata.Studios, pc.Name)
	}

	// Credits
	if tv.Credits != nil {
		// Cast (all members for accurate search)
		for _, cast := range tv.Credits.Cast {
			metadata.Cast = append(metadata.Cast, sdk.CastMember{
				Name:  cast.Name,
				Role:  cast.Character,
				Thumb: ImageURL(cast.ProfilePath, "w185"),
				Order: cast.Order,
			})
		}
	}

	// Keywords (for location-based and thematic search)
	// Note: TV shows use "results" instead of "keywords" in the TMDB response
	if tv.Keywords != nil {
		for _, kw := range tv.Keywords.Results {
			metadata.Keywords = append(metadata.Keywords, sdk.Keyword{
				ID:         kw.ID,
				Name:       kw.Name,
				IsLocation: isLocationKeyword(kw.Name),
			})
		}
	}

	resp.Metadata = metadata

	// Images
	resp.Images = append(resp.Images, sdk.EnrichedImage{
		Type:     "poster",
		Path:     ImageURL(tv.PosterPath, "original"),
		IsRemote: true,
	})
	resp.Images = append(resp.Images, sdk.EnrichedImage{
		Type:     "fanart",
		Path:     ImageURL(tv.BackdropPath, "original"),
		IsRemote: true,
	})

	// Additional images
	if tv.Images != nil {
		for i, img := range tv.Images.Backdrops {
			if i >= 5 {
				break
			}
			resp.Images = append(resp.Images, sdk.EnrichedImage{
				Type:     "fanart",
				Path:     ImageURL(img.FilePath, "original"),
				IsRemote: true,
				Width:    img.Width,
				Height:   img.Height,
				Language: img.ISO639_1,
				Rating:   float32(img.VoteAverage),
			})
		}
	}

	return resp
}

// Helper functions for optional fields

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	// Clean up the string
	s = strings.TrimSpace(s)
	return &s
}

func intPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func float32Ptr(f float32) *float32 {
	if f == 0 {
		return nil
	}
	return &f
}

// isLocationKeyword checks if a keyword represents a location/setting.
// Uses a curated list of location patterns and known location keywords.
func isLocationKeyword(keyword string) bool {
	kw := strings.ToLower(keyword)

	// Major world cities
	cities := []string{
		"new york", "los angeles", "chicago", "san francisco", "boston", "miami", "las vegas",
		"seattle", "washington", "philadelphia", "detroit", "atlanta", "houston", "dallas",
		"london", "paris", "berlin", "rome", "madrid", "barcelona", "amsterdam", "vienna",
		"tokyo", "hong kong", "shanghai", "beijing", "seoul", "singapore", "bangkok", "mumbai",
		"sydney", "melbourne", "toronto", "montreal", "vancouver", "mexico city", "rio de janeiro",
		"buenos aires", "moscow", "st. petersburg", "cairo", "dubai", "istanbul",
		"manhattan", "brooklyn", "queens", "bronx", "hollywood", "beverly hills",
	}
	for _, city := range cities {
		if strings.Contains(kw, city) {
			return true
		}
	}

	// Countries
	countries := []string{
		"united states", "america", "usa", "uk", "england", "britain", "france", "germany",
		"italy", "spain", "japan", "china", "korea", "india", "australia", "canada", "mexico",
		"brazil", "russia", "ireland", "scotland", "wales", "greece", "egypt", "israel",
		"thailand", "vietnam", "indonesia", "philippines", "south africa", "nigeria",
	}
	for _, country := range countries {
		if strings.Contains(kw, country) {
			return true
		}
	}

	// US States and regions
	states := []string{
		"california", "texas", "florida", "new york state", "illinois", "pennsylvania",
		"ohio", "georgia", "north carolina", "michigan", "new jersey", "virginia",
		"washington state", "arizona", "massachusetts", "tennessee", "indiana",
		"missouri", "maryland", "wisconsin", "colorado", "minnesota", "oregon",
		"louisiana", "alabama", "kentucky", "connecticut", "oklahoma", "iowa",
		"mississippi", "arkansas", "kansas", "utah", "nevada", "new mexico",
		"west virginia", "nebraska", "maine", "new hampshire", "hawaii", "alaska",
		"montana", "idaho", "wyoming", "vermont", "south dakota", "north dakota",
	}
	for _, state := range states {
		if strings.Contains(kw, state) {
			return true
		}
	}

	// Geographic region patterns (must be specific to avoid false positives)
	regionPatterns := []string{
		"suburb", "countryside", "rural area", "urban setting",
		"small town", "big city", "outback", "bayou", "appalachia", "midwest",
		"deep south", "new england", "pacific northwest", "southwest", "heartland",
		"caribbean", "mediterranean", "latin america", "south america", "central america", "oceania",
	}
	for _, pattern := range regionPatterns {
		if strings.Contains(kw, pattern) {
			return true
		}
	}

	// Exact matches for ambiguous terms that could be themes or locations
	// Only match if the keyword IS the location, not contains it
	exactLocationMatches := []string{
		"beach", "island", "mountain", "desert", "forest", "jungle",
		"arctic", "antarctic", "europe", "asia", "africa", "middle east",
	}
	for _, exact := range exactLocationMatches {
		if kw == exact {
			return true
		}
	}

	return false
}

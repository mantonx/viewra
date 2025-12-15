package internal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// enrichMovie fetches movie metadata from TMDb.
func (p *TMDbPlugin) enrichMovie(ctx context.Context, client *Client, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	var tmdbID int
	var err error

	// Strategy 1: Use existing TMDb ID if available
	if idStr, ok := req.ExistingIds["tmdb"]; ok {
		tmdbID, err = strconv.Atoi(idStr)
		if err != nil {
			p.logger.Warn("invalid tmdb ID", "id", idStr, "error", err)
		}
	}

	// Strategy 2: Look up by IMDb ID if available
	if tmdbID == 0 {
		if imdbID, ok := req.ExistingIds["imdb"]; ok && imdbID != "" {
			p.logger.Debug("looking up movie by IMDb ID", "imdb_id", imdbID)
			findResp, err := client.FindByIMDbID(ctx, imdbID)
			if err != nil {
				p.mu.Lock()
				p.errorsTotal++
				p.mu.Unlock()
				return nil, fmt.Errorf("IMDb lookup failed: %w", err)
			}
			if len(findResp.MovieResults) > 0 {
				tmdbID = findResp.MovieResults[0].ID
				p.logger.Debug("found movie via IMDb ID", "tmdb_id", tmdbID)
			}
		}
	}

	// Strategy 3: Search by title/year
	if tmdbID == 0 && req.Title != "" {
		p.logger.Debug("searching movie by title", "title", req.Title, "year", req.Year)
		searchResp, err := client.SearchMovies(ctx, req.Title, int(req.Year))
		if err != nil {
			p.mu.Lock()
			p.errorsTotal++
			p.mu.Unlock()
			return nil, fmt.Errorf("movie search failed: %w", err)
		}
		if len(searchResp.Results) > 0 {
			// Take the first result (most popular/relevant)
			tmdbID = searchResp.Results[0].ID
			p.logger.Debug("found movie via search", "tmdb_id", tmdbID, "match_title", searchResp.Results[0].Title)
		}
	}

	if tmdbID == 0 {
		return &pluginv1.EnrichResponse{
			Matched:    false,
			Skipped:    true,
			SkipReason: "no match found",
		}, nil
	}

	// Fetch full details
	details, err := client.GetMovieDetails(ctx, tmdbID)
	if err != nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return nil, fmt.Errorf("failed to get movie details: %w", err)
	}

	return p.buildMovieResponse(details), nil
}

// enrichTV fetches TV show metadata from TMDb.
func (p *TMDbPlugin) enrichTV(ctx context.Context, client *Client, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	var tmdbID int
	var err error

	// Use show title for TV shows/episodes
	title := req.Title
	if req.Tv != nil && req.Tv.ShowTitle != "" {
		title = req.Tv.ShowTitle
	}

	// Strategy 1: Use existing TMDb ID if available
	if idStr, ok := req.ExistingIds["tmdb"]; ok {
		tmdbID, err = strconv.Atoi(idStr)
		if err != nil {
			p.logger.Warn("invalid tmdb ID", "id", idStr, "error", err)
		}
	}

	// Strategy 2: Look up by IMDb ID if available
	if tmdbID == 0 {
		if imdbID, ok := req.ExistingIds["imdb"]; ok && imdbID != "" {
			p.logger.Debug("looking up TV show by IMDb ID", "imdb_id", imdbID)
			findResp, err := client.FindByIMDbID(ctx, imdbID)
			if err != nil {
				p.mu.Lock()
				p.errorsTotal++
				p.mu.Unlock()
				return nil, fmt.Errorf("IMDb lookup failed: %w", err)
			}
			if len(findResp.TVResults) > 0 {
				tmdbID = findResp.TVResults[0].ID
				p.logger.Debug("found TV show via IMDb ID", "tmdb_id", tmdbID)
			}
		}
	}

	// Strategy 3: Search by title/year
	if tmdbID == 0 && title != "" {
		p.logger.Debug("searching TV show by title", "title", title, "year", req.Year)
		searchResp, err := client.SearchTV(ctx, title, int(req.Year))
		if err != nil {
			p.mu.Lock()
			p.errorsTotal++
			p.mu.Unlock()
			return nil, fmt.Errorf("TV search failed: %w", err)
		}
		if len(searchResp.Results) > 0 {
			tmdbID = searchResp.Results[0].ID
			p.logger.Debug("found TV show via search", "tmdb_id", tmdbID, "match_title", searchResp.Results[0].Name)
		}
	}

	if tmdbID == 0 {
		return &pluginv1.EnrichResponse{
			Matched:    false,
			Skipped:    true,
			SkipReason: "no match found",
		}, nil
	}

	// Fetch full details
	details, err := client.GetTVDetails(ctx, tmdbID)
	if err != nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return nil, fmt.Errorf("failed to get TV details: %w", err)
	}

	return p.buildTVResponse(details), nil
}

// buildMovieResponse converts TMDb movie details to EnrichResponse.
func (p *TMDbPlugin) buildMovieResponse(movie *MovieDetails) *pluginv1.EnrichResponse {
	resp := &pluginv1.EnrichResponse{
		Matched:       true,
		DiscoveredIds: make(map[string]string),
		Confidence:    0.9,
	}

	// External IDs
	resp.DiscoveredIds["tmdb"] = strconv.Itoa(movie.ID)
	if movie.IMDbID != "" {
		resp.DiscoveredIds["imdb"] = movie.IMDbID
	}

	// Extract year from release date
	var year int32
	if len(movie.ReleaseDate) >= 4 {
		if y, err := strconv.Atoi(movie.ReleaseDate[:4]); err == nil {
			year = int32(y)
		}
	}

	// Build metadata
	metadata := &pluginv1.EnrichedMetadata{
		Title:            strPtr(movie.Title),
		OriginalTitle:    strPtr(movie.OriginalTitle),
		Year:             &year,
		Plot:             strPtr(movie.Overview),
		Tagline:          strPtr(movie.Tagline),
		RuntimeMinutes:   int32Ptr(int32(movie.Runtime)),
		Rating:           float32Ptr(float32(movie.VoteAverage)),
		RatingVotes:      int32Ptr(int32(movie.VoteCount)),
		OriginalLanguage: strPtr(movie.OriginalLanguage),
	}

	if movie.Budget > 0 {
		metadata.Budget = &movie.Budget
	}
	if movie.Revenue > 0 {
		metadata.Revenue = &movie.Revenue
	}

	// Genres
	for _, g := range movie.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	// Studios/Production Companies
	for _, pc := range movie.ProductionCompanies {
		metadata.Studios = append(metadata.Studios, pc.Name)
	}

	// Country of origin
	if len(movie.ProductionCountries) > 0 {
		metadata.CountryOfOrigin = strPtr(movie.ProductionCountries[0].Name)
	}

	// Credits
	if movie.Credits != nil {
		// Directors and writers from crew
		for _, crew := range movie.Credits.Crew {
			switch crew.Job {
			case "Director":
				metadata.Directors = append(metadata.Directors, crew.Name)
			case "Writer", "Screenplay":
				metadata.Writers = append(metadata.Writers, crew.Name)
			}
		}

		// Cast (top 20)
		for i, cast := range movie.Credits.Cast {
			if i >= 20 {
				break
			}
			metadata.Cast = append(metadata.Cast, &pluginv1.CastMember{
				Name:  cast.Name,
				Role:  cast.Character,
				Thumb: ImageURL(cast.ProfilePath, "w185"),
				Order: int32(cast.Order),
			})
		}
	}

	resp.Metadata = metadata

	// Images
	resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
		Type:     "poster",
		Path:     ImageURL(movie.PosterPath, "original"),
		IsRemote: true,
	})
	resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
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
			resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
				Type:     "fanart",
				Path:     ImageURL(img.FilePath, "original"),
				IsRemote: true,
				Width:    int32(img.Width),
				Height:   int32(img.Height),
				Language: img.ISO639_1,
				Rating:   float32(img.VoteAverage),
				Votes:    int32(img.VoteCount),
			})
		}
	}

	return resp
}

// buildTVResponse converts TMDb TV details to EnrichResponse.
func (p *TMDbPlugin) buildTVResponse(tv *TVDetails) *pluginv1.EnrichResponse {
	resp := &pluginv1.EnrichResponse{
		Matched:       true,
		DiscoveredIds: make(map[string]string),
		Confidence:    0.9,
	}

	// External IDs
	resp.DiscoveredIds["tmdb"] = strconv.Itoa(tv.ID)
	if tv.ExternalIDs != nil {
		if tv.ExternalIDs.IMDbID != "" {
			resp.DiscoveredIds["imdb"] = tv.ExternalIDs.IMDbID
		}
		if tv.ExternalIDs.TVDbID > 0 {
			resp.DiscoveredIds["tvdb"] = strconv.Itoa(tv.ExternalIDs.TVDbID)
		}
	}

	// Extract year from first air date
	var year int32
	if len(tv.FirstAirDate) >= 4 {
		if y, err := strconv.Atoi(tv.FirstAirDate[:4]); err == nil {
			year = int32(y)
		}
	}

	// Average runtime
	var avgRuntime int32
	if len(tv.EpisodeRunTime) > 0 {
		var total int
		for _, r := range tv.EpisodeRunTime {
			total += r
		}
		avgRuntime = int32(total / len(tv.EpisodeRunTime))
	}

	// Build metadata
	metadata := &pluginv1.EnrichedMetadata{
		Title:            strPtr(tv.Name),
		OriginalTitle:    strPtr(tv.OriginalName),
		Year:             &year,
		Plot:             strPtr(tv.Overview),
		Tagline:          strPtr(tv.Tagline),
		RuntimeMinutes:   &avgRuntime,
		Rating:           float32Ptr(float32(tv.VoteAverage)),
		RatingVotes:      int32Ptr(int32(tv.VoteCount)),
		OriginalLanguage: strPtr(tv.OriginalLanguage),
		Status:           strPtr(tv.Status),
		Premiered:        strPtr(tv.FirstAirDate),
		LastAirDate:      strPtr(tv.LastAirDate),
	}

	// Network (first one)
	if len(tv.Networks) > 0 {
		metadata.Network = strPtr(tv.Networks[0].Name)
	}

	// Genres
	for _, g := range tv.Genres {
		metadata.Genres = append(metadata.Genres, g.Name)
	}

	// Studios/Production Companies
	for _, pc := range tv.ProductionCompanies {
		metadata.Studios = append(metadata.Studios, pc.Name)
	}

	// Creators
	for _, creator := range tv.CreatedBy {
		metadata.Creators = append(metadata.Creators, creator.Name)
	}

	// Credits
	if tv.Credits != nil {
		// Cast (top 20)
		for i, cast := range tv.Credits.Cast {
			if i >= 20 {
				break
			}
			metadata.Cast = append(metadata.Cast, &pluginv1.CastMember{
				Name:  cast.Name,
				Role:  cast.Character,
				Thumb: ImageURL(cast.ProfilePath, "w185"),
				Order: int32(cast.Order),
			})
		}
	}

	resp.Metadata = metadata

	// Images
	resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
		Type:     "poster",
		Path:     ImageURL(tv.PosterPath, "original"),
		IsRemote: true,
	})
	resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
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
			resp.Images = append(resp.Images, &pluginv1.EnrichedImage{
				Type:     "fanart",
				Path:     ImageURL(img.FilePath, "original"),
				IsRemote: true,
				Width:    int32(img.Width),
				Height:   int32(img.Height),
				Language: img.ISO639_1,
				Rating:   float32(img.VoteAverage),
				Votes:    int32(img.VoteCount),
			})
		}
	}

	return resp
}

// Helper functions for optional proto fields

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	// Clean up the string
	s = strings.TrimSpace(s)
	return &s
}

func int32Ptr(i int32) *int32 {
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

package internal

import (
	"context"
	"os"
	"testing"
	"time"
)

// Integration tests that hit the real TMDb API.
// Run with: go test -tags=integration -v ./...
// Requires TMDB_API_KEY environment variable.

func skipIfNoAPIKey(t *testing.T) string {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		t.Skip("TMDB_API_KEY not set, skipping integration test")
	}
	return apiKey
}

func createIntegrationClient(t *testing.T) *Client {
	t.Helper()
	apiKey := skipIfNoAPIKey(t)

	client, err := NewClient(ClientConfig{
		APIKey:        apiKey,
		CacheTTLHours: 1,
		Logger:        newTestLogger(),
		// No storage - caching disabled for integration tests
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func TestIntegration_SearchMovies_TheMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	result, err := client.SearchMovies(ctx, "The Matrix", 1999)
	if err != nil {
		t.Fatalf("SearchMovies failed: %v", err)
	}

	if result.TotalResults == 0 {
		t.Fatal("expected at least one result")
	}

	// The Matrix (1999) should be TMDb ID 603
	found := false
	for _, movie := range result.Results {
		if movie.ID == 603 {
			found = true
			if movie.Title != "The Matrix" {
				t.Errorf("expected title 'The Matrix', got %q", movie.Title)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find The Matrix (ID 603) in results")
	}
}

func TestIntegration_GetMovieDetails_TheMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	// The Matrix = TMDb ID 603
	result, err := client.GetMovieDetails(ctx, 603)
	if err != nil {
		t.Fatalf("GetMovieDetails failed: %v", err)
	}

	if result.ID != 603 {
		t.Errorf("expected ID 603, got %d", result.ID)
	}
	if result.IMDbID != "tt0133093" {
		t.Errorf("expected IMDb ID tt0133093, got %s", result.IMDbID)
	}
	if result.Title != "The Matrix" {
		t.Errorf("expected title 'The Matrix', got %q", result.Title)
	}
	if result.Runtime < 130 || result.Runtime > 140 {
		t.Errorf("unexpected runtime: %d (expected ~136)", result.Runtime)
	}

	// Should have genres
	if len(result.Genres) == 0 {
		t.Error("expected genres")
	}

	// Should have credits (we request append_to_response=credits)
	if result.Credits == nil {
		t.Error("expected credits")
	} else {
		if len(result.Credits.Cast) == 0 {
			t.Error("expected cast members")
		}
		// Keanu Reeves should be in the cast
		foundKeanu := false
		for _, cast := range result.Credits.Cast {
			if cast.Name == "Keanu Reeves" {
				foundKeanu = true
				break
			}
		}
		if !foundKeanu {
			t.Error("expected Keanu Reeves in cast")
		}
	}
}

func TestIntegration_FindByIMDbID(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	// tt0133093 = The Matrix
	result, err := client.FindByIMDbID(ctx, "tt0133093")
	if err != nil {
		t.Fatalf("FindByIMDbID failed: %v", err)
	}

	if len(result.MovieResults) == 0 {
		t.Fatal("expected movie results")
	}
	if result.MovieResults[0].ID != 603 {
		t.Errorf("expected ID 603, got %d", result.MovieResults[0].ID)
	}
}

func TestIntegration_SearchTV_GameOfThrones(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	result, err := client.SearchTV(ctx, "Game of Thrones", 0)
	if err != nil {
		t.Fatalf("SearchTV failed: %v", err)
	}

	if result.TotalResults == 0 {
		t.Fatal("expected at least one result")
	}

	// Game of Thrones = TMDb ID 1399
	found := false
	for _, show := range result.Results {
		if show.ID == 1399 {
			found = true
			if show.Name != "Game of Thrones" {
				t.Errorf("expected name 'Game of Thrones', got %q", show.Name)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find Game of Thrones (ID 1399) in results")
	}
}

func TestIntegration_GetTVDetails_BreakingBad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	// Breaking Bad = TMDb ID 1396
	result, err := client.GetTVDetails(ctx, 1396)
	if err != nil {
		t.Fatalf("GetTVDetails failed: %v", err)
	}

	if result.ID != 1396 {
		t.Errorf("expected ID 1396, got %d", result.ID)
	}
	if result.Name != "Breaking Bad" {
		t.Errorf("expected name 'Breaking Bad', got %q", result.Name)
	}
	if result.NumberOfSeasons < 5 {
		t.Errorf("expected at least 5 seasons, got %d", result.NumberOfSeasons)
	}

	// Should have external IDs
	if result.ExternalIDs == nil {
		t.Error("expected external IDs")
	} else {
		if result.ExternalIDs.IMDbID == "" {
			t.Error("expected IMDb ID")
		}
	}

	// Should have credits
	if result.Credits == nil {
		t.Error("expected credits")
	} else {
		// Bryan Cranston should be in the cast
		foundBryan := false
		for _, cast := range result.Credits.Cast {
			if cast.Name == "Bryan Cranston" {
				foundBryan = true
				break
			}
		}
		if !foundBryan {
			t.Error("expected Bryan Cranston in cast")
		}
	}
}

func TestIntegration_RateLimiting_MultipleRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	// Make 10 rapid requests to test rate limiting
	// These should complete without hitting TMDb's rate limit (429)
	movies := []struct {
		title string
		year  int
	}{
		{"The Matrix", 1999},
		{"Inception", 2010},
		{"The Dark Knight", 2008},
		{"Pulp Fiction", 1994},
		{"Fight Club", 1999},
		{"Forrest Gump", 1994},
		{"The Shawshank Redemption", 1994},
		{"The Godfather", 1972},
		{"Interstellar", 2014},
		{"Gladiator", 2000},
	}

	start := time.Now()
	for _, m := range movies {
		result, err := client.SearchMovies(ctx, m.title, m.year)
		if err != nil {
			t.Fatalf("SearchMovies(%q, %d) failed: %v", m.title, m.year, err)
		}
		if result.TotalResults == 0 {
			t.Errorf("no results for %q (%d)", m.title, m.year)
		}
	}
	elapsed := time.Since(start)

	// 10 requests with 250ms rate limiting = at least 2.25 seconds
	// (first request is immediate, 9 more with 250ms delays)
	expectedMin := 2 * time.Second
	if elapsed < expectedMin {
		t.Errorf("requests completed too fast: %v (expected >= %v)", elapsed, expectedMin)
	}

	t.Logf("10 requests completed in %v (rate limiting working)", elapsed)
}

func TestIntegration_MultipleMediaTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	// Test movie search
	movieResult, err := client.SearchMovies(ctx, "Blade Runner", 1982)
	if err != nil {
		t.Fatalf("movie search failed: %v", err)
	}
	if movieResult.TotalResults == 0 {
		t.Error("expected movie results for Blade Runner")
	}

	// Test TV search
	tvResult, err := client.SearchTV(ctx, "The Expanse", 0)
	if err != nil {
		t.Fatalf("TV search failed: %v", err)
	}
	if tvResult.TotalResults == 0 {
		t.Error("expected TV results for The Expanse")
	}

	// Test IMDb lookup (movie)
	imdbMovieResult, err := client.FindByIMDbID(ctx, "tt0083658") // Blade Runner
	if err != nil {
		t.Fatalf("IMDb movie lookup failed: %v", err)
	}
	if len(imdbMovieResult.MovieResults) == 0 {
		t.Error("expected movie results for Blade Runner IMDb ID")
	}

	// Test IMDb lookup (TV)
	imdbTVResult, err := client.FindByIMDbID(ctx, "tt3230854") // The Expanse
	if err != nil {
		t.Fatalf("IMDb TV lookup failed: %v", err)
	}
	if len(imdbTVResult.TVResults) == 0 {
		t.Error("expected TV results for The Expanse IMDb ID")
	}

	t.Log("all media type searches completed successfully")
}

func TestIntegration_MovieNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client := createIntegrationClient(t)
	ctx := context.Background()

	// Search for something that shouldn't exist
	result, err := client.SearchMovies(ctx, "xyzzy12345notarealmovie67890", 2099)
	if err != nil {
		t.Fatalf("SearchMovies failed: %v", err)
	}

	if result.TotalResults != 0 {
		t.Errorf("expected 0 results for nonsense query, got %d", result.TotalResults)
	}
}

func TestIntegration_JWTAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	apiKey := skipIfNoAPIKey(t)

	// Check if it's a JWT token (v4 auth)
	client, err := NewClient(ClientConfig{
		APIKey:        apiKey,
		CacheTTLHours: 1,
		Logger:        newTestLogger(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	isJWT := client.isJWTToken()
	t.Logf("API key is JWT: %v", isJWT)

	// Either way, the request should work
	ctx := context.Background()
	result, err := client.SearchMovies(ctx, "The Matrix", 1999)
	if err != nil {
		t.Fatalf("SearchMovies failed with %s auth: %v",
			map[bool]string{true: "JWT", false: "legacy"}[isJWT], err)
	}

	if result.TotalResults == 0 {
		t.Error("expected results")
	}
}

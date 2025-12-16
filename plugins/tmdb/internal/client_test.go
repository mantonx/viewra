package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// newTestLogger creates a logger for tests.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// newTestClient creates a client pointing at a test server.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(ClientConfig{
		APIKey:        "test-api-key",
		CacheTTLHours: 1,
		Logger:        newTestLogger(),
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Override the base URL to point to test server
	// We need to patch the get method's URL construction
	// Since baseURL is a const, we'll use a wrapper handler
	client.httpClient = server.Client()

	// Create a new client that uses the test server
	return &Client{
		apiKey:       "test-api-key",
		httpClient:   server.Client(),
		logger:       newTestLogger(),
		cacheTTLSecs: 3600,
	}
}

func TestSearchMovies(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request (path includes /3 from baseURL)
		if r.URL.Path != "/3/search/movie" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("query"); q != "The Matrix" {
			t.Errorf("unexpected query: %s", q)
		}
		if year := r.URL.Query().Get("year"); year != "1999" {
			t.Errorf("unexpected year: %s", year)
		}

		resp := MovieSearchResponse{
			Page:         1,
			TotalPages:   1,
			TotalResults: 1,
			Results: []MovieSearchResult{
				{
					ID:          603,
					Title:       "The Matrix",
					ReleaseDate: "1999-03-30",
					Overview:    "A computer hacker learns about the true nature of reality.",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Create client with custom HTTP client that rewrites URLs
	client := createTestClientWithServer(t, server)

	ctx := context.Background()
	result, err := client.SearchMovies(ctx, "The Matrix", 1999)
	if err != nil {
		t.Fatalf("SearchMovies failed: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].ID != 603 {
		t.Errorf("expected ID 603, got %d", result.Results[0].ID)
	}
	if result.Results[0].Title != "The Matrix" {
		t.Errorf("expected title 'The Matrix', got %q", result.Results[0].Title)
	}
}

func TestGetMovieDetails(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/movie/603" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := MovieDetails{
			ID:           603,
			IMDbID:       "tt0133093",
			Title:        "The Matrix",
			Overview:     "A computer hacker learns about the true nature of reality.",
			ReleaseDate:  "1999-03-30",
			Runtime:      136,
			VoteAverage:  8.2,
			PosterPath:   "/f89U3ADr1oiB1s9GkdPOEpXUk5H.jpg",
			BackdropPath: "/fNG7i7RqMErkcqhohV2a6cV1Ehy.jpg",
			Genres: []Genre{
				{ID: 28, Name: "Action"},
				{ID: 878, Name: "Science Fiction"},
			},
			Credits: &Credits{
				Cast: []CastMember{
					{ID: 6384, Name: "Keanu Reeves", Character: "Thomas A. Anderson / Neo", Order: 0},
					{ID: 2975, Name: "Laurence Fishburne", Character: "Morpheus", Order: 1},
				},
				Crew: []CrewMember{
					{ID: 9340, Name: "Lana Wachowski", Job: "Director"},
					{ID: 9339, Name: "Lilly Wachowski", Job: "Director"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := createTestClientWithServer(t, server)

	ctx := context.Background()
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
	if result.Runtime != 136 {
		t.Errorf("expected runtime 136, got %d", result.Runtime)
	}
	if len(result.Genres) != 2 {
		t.Errorf("expected 2 genres, got %d", len(result.Genres))
	}
	if result.Credits == nil {
		t.Fatal("expected credits, got nil")
	}
	if len(result.Credits.Cast) != 2 {
		t.Errorf("expected 2 cast members, got %d", len(result.Credits.Cast))
	}
}

func TestRetryOnServerError(t *testing.T) {
	var attempts int32

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attempts, 1)
		if attempt < 3 {
			// First two attempts return 503
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service Unavailable"))
			return
		}
		// Third attempt succeeds
		resp := MovieSearchResponse{
			Page:         1,
			TotalResults: 1,
			Results: []MovieSearchResult{
				{ID: 603, Title: "The Matrix"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := createTestClientWithServer(t, server)

	ctx := context.Background()
	result, err := client.SearchMovies(ctx, "The Matrix", 0)
	if err != nil {
		t.Fatalf("SearchMovies failed after retries: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Results))
	}
}

func TestRateLimiting(t *testing.T) {
	var requestTimes []time.Time

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		resp := MovieSearchResponse{Page: 1, Results: []MovieSearchResult{{ID: 1}}}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := createTestClientWithServer(t, server)

	ctx := context.Background()

	// Make 5 rapid requests
	for i := 0; i < 5; i++ {
		_, err := client.SearchMovies(ctx, "Test", 0)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}

	// Verify rate limiting was applied
	if len(requestTimes) != 5 {
		t.Fatalf("expected 5 requests, got %d", len(requestTimes))
	}

	// Check that requests are spaced by at least minRequestInterval (250ms)
	// Allow some tolerance for test timing
	minInterval := 200 * time.Millisecond
	for i := 1; i < len(requestTimes); i++ {
		interval := requestTimes[i].Sub(requestTimes[i-1])
		if interval < minInterval {
			t.Errorf("request %d came too fast: %v (expected >= %v)", i, interval, minInterval)
		}
	}
}

func TestFindByIMDbID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/find/tt0133093" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if src := r.URL.Query().Get("external_source"); src != "imdb_id" {
			t.Errorf("unexpected external_source: %s", src)
		}

		resp := FindByExternalIDResponse{
			MovieResults: []MovieSearchResult{
				{ID: 603, Title: "The Matrix"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := createTestClientWithServer(t, server)

	ctx := context.Background()
	result, err := client.FindByIMDbID(ctx, "tt0133093")
	if err != nil {
		t.Fatalf("FindByIMDbID failed: %v", err)
	}

	if len(result.MovieResults) != 1 {
		t.Fatalf("expected 1 movie result, got %d", len(result.MovieResults))
	}
	if result.MovieResults[0].ID != 603 {
		t.Errorf("expected ID 603, got %d", result.MovieResults[0].ID)
	}
}

func TestSearchTV(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/3/search/tv" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := TVSearchResponse{
			Page:         1,
			TotalResults: 1,
			Results: []TVSearchResult{
				{ID: 1399, Name: "Game of Thrones", FirstAirDate: "2011-04-17"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client := createTestClientWithServer(t, server)

	ctx := context.Background()
	result, err := client.SearchTV(ctx, "Game of Thrones", 2011)
	if err != nil {
		t.Fatalf("SearchTV failed: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	if result.Results[0].ID != 1399 {
		t.Errorf("expected ID 1399, got %d", result.Results[0].ID)
	}
}

func TestImageURL(t *testing.T) {
	tests := []struct {
		path     string
		size     string
		expected string
	}{
		{"/abc123.jpg", "original", "https://image.tmdb.org/t/p/original/abc123.jpg"},
		{"/abc123.jpg", "w500", "https://image.tmdb.org/t/p/w500/abc123.jpg"},
		{"", "original", ""},
	}

	for _, tt := range tests {
		result := ImageURL(tt.path, tt.size)
		if result != tt.expected {
			t.Errorf("ImageURL(%q, %q) = %q, want %q", tt.path, tt.size, result, tt.expected)
		}
	}
}

func TestIsJWTToken(t *testing.T) {
	// Create a long enough JWT-like token (must be > 100 chars and start with "eyJ")
	longJWT := "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiI1YTU2ODc0YjRmMzU4YjIzZDhkM2YzZmI5ZDc4NDNiOSIsIm5iZiI6MTc0ODYzOTc1Ny40MDEsInN1YiI6IjY4M2EyMDBkNzA5OGI4MzMzNThmZThmOSIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.signature"

	tests := []struct {
		apiKey   string
		expected bool
	}{
		{"abc123", false},         // Short key (legacy API key)
		{longJWT, true},           // JWT v4 token (> 100 chars, starts with eyJ)
		{"notaJWT" + string(make([]byte, 100)), false}, // Long but doesn't start with eyJ
	}

	for _, tt := range tests {
		client := &Client{apiKey: tt.apiKey}
		result := client.isJWTToken()
		if result != tt.expected {
			t.Errorf("isJWTToken() for key starting with %q = %v, want %v",
				tt.apiKey[:min(10, len(tt.apiKey))], result, tt.expected)
		}
	}
}

// createTestClientWithServer creates a client that redirects requests to the test server.
// This works around the const baseURL by using a custom transport.
func createTestClientWithServer(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	// Create a transport that rewrites URLs to the test server
	transport := &rewriteTransport{
		base:      http.DefaultTransport,
		serverURL: server.URL,
	}

	return &Client{
		apiKey: "test-api-key",
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		logger:       newTestLogger(),
		cacheTTLSecs: 3600,
	}
}

// rewriteTransport rewrites request URLs to point to the test server.
type rewriteTransport struct {
	base      http.RoundTripper
	serverURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to test server
	req.URL.Scheme = "http"
	req.URL.Host = t.serverURL[7:] // Strip "http://"
	return t.base.RoundTrip(req)
}

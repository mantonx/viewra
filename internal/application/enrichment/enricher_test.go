package enrichment

import (
	"testing"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

func TestEnricherCapabilities_SupportsMediaType(t *testing.T) {
	caps := EnricherCapabilities{
		MediaTypes: []enrichment.MediaType{
			enrichment.MediaTypeMovie,
			enrichment.MediaTypeTV,
		},
	}

	tests := []struct {
		name      string
		mediaType enrichment.MediaType
		expected  bool
	}{
		{"supports movie", enrichment.MediaTypeMovie, true},
		{"supports TV", enrichment.MediaTypeTV, true},
		{"does not support music", enrichment.MediaTypeMusic, false},
		{"does not support empty", enrichment.MediaType(""), false},
		{"does not support unknown", enrichment.MediaType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := caps.SupportsMediaType(tt.mediaType)
			if got != tt.expected {
				t.Errorf("SupportsMediaType(%s) = %v, want %v", tt.mediaType, got, tt.expected)
			}
		})
	}
}

func TestEnricherCapabilities_SupportsMediaType_Empty(t *testing.T) {
	caps := EnricherCapabilities{
		MediaTypes: nil,
	}

	if caps.SupportsMediaType(enrichment.MediaTypeMovie) {
		t.Error("empty MediaTypes should not support any type")
	}
}

func TestEnrichRequest_HasExternalID(t *testing.T) {
	req := &EnrichRequest{
		MediaID: 42,
		ExistingIDs: map[string]string{
			"imdb":  "tt0133093",
			"tmdb":  "603",
			"tvdb":  "12345",
		},
	}

	tests := []struct {
		provider string
		expected bool
	}{
		{"imdb", true},
		{"tmdb", true},
		{"tvdb", true},
		{"musicbrainz", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := req.HasExternalID(tt.provider)
			if got != tt.expected {
				t.Errorf("HasExternalID(%s) = %v, want %v", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestEnrichRequest_HasExternalID_NilMap(t *testing.T) {
	req := &EnrichRequest{
		MediaID:     42,
		ExistingIDs: nil,
	}

	if req.HasExternalID("imdb") {
		t.Error("nil ExistingIDs should return false")
	}
}

func TestEnrichRequest_GetExternalID(t *testing.T) {
	req := &EnrichRequest{
		MediaID: 42,
		ExistingIDs: map[string]string{
			"imdb": "tt0133093",
			"tmdb": "603",
		},
	}

	tests := []struct {
		provider string
		expected string
	}{
		{"imdb", "tt0133093"},
		{"tmdb", "603"},
		{"tvdb", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := req.GetExternalID(tt.provider)
			if got != tt.expected {
				t.Errorf("GetExternalID(%s) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestEnrichRequest_GetExternalID_NilMap(t *testing.T) {
	req := &EnrichRequest{
		MediaID:     42,
		ExistingIDs: nil,
	}

	got := req.GetExternalID("imdb")
	if got != "" {
		t.Errorf("GetExternalID on nil map should return empty string, got %q", got)
	}
}

func TestEnrichResponse_AddDiscoveredID(t *testing.T) {
	resp := &EnrichResponse{}

	// First add should initialize the map
	resp.AddDiscoveredID("imdb", "tt0133093")

	if resp.DiscoveredIDs == nil {
		t.Fatal("DiscoveredIDs should be initialized")
	}
	if resp.DiscoveredIDs["imdb"] != "tt0133093" {
		t.Errorf("DiscoveredIDs[imdb] = %q, want tt0133093", resp.DiscoveredIDs["imdb"])
	}

	// Second add should work without re-initializing
	resp.AddDiscoveredID("tmdb", "603")
	if resp.DiscoveredIDs["tmdb"] != "603" {
		t.Errorf("DiscoveredIDs[tmdb] = %q, want 603", resp.DiscoveredIDs["tmdb"])
	}

	// Overwrite existing
	resp.AddDiscoveredID("imdb", "tt9999999")
	if resp.DiscoveredIDs["imdb"] != "tt9999999" {
		t.Errorf("DiscoveredIDs[imdb] = %q, want tt9999999", resp.DiscoveredIDs["imdb"])
	}
}

func TestEnrichResponse_AddImage(t *testing.T) {
	resp := &EnrichResponse{}

	img1 := EnrichedImage{
		Type:     "poster",
		Path:     "/path/to/poster.jpg",
		IsRemote: false,
	}
	resp.AddImage(img1)

	if len(resp.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(resp.Images))
	}
	if resp.Images[0].Type != "poster" {
		t.Errorf("Images[0].Type = %s, want poster", resp.Images[0].Type)
	}

	img2 := EnrichedImage{
		Type:     "fanart",
		Path:     "https://example.com/fanart.jpg",
		IsRemote: true,
	}
	resp.AddImage(img2)

	if len(resp.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(resp.Images))
	}
	if resp.Images[1].Type != "fanart" {
		t.Errorf("Images[1].Type = %s, want fanart", resp.Images[1].Type)
	}
}

func TestSkip(t *testing.T) {
	resp := Skip("no NFO file found")

	if !resp.Skipped {
		t.Error("Skipped should be true")
	}
	if resp.SkipReason != "no NFO file found" {
		t.Errorf("SkipReason = %q, want 'no NFO file found'", resp.SkipReason)
	}
	if resp.Matched {
		t.Error("Matched should be false for skipped response")
	}
}

func TestNoMatch(t *testing.T) {
	resp := NoMatch()

	if resp.Matched {
		t.Error("Matched should be false")
	}
	if resp.Skipped {
		t.Error("Skipped should be false")
	}
}

func TestMatch(t *testing.T) {
	resp := Match()

	if !resp.Matched {
		t.Error("Matched should be true")
	}
	if resp.Skipped {
		t.Error("Skipped should be false")
	}
	if resp.DiscoveredIDs == nil {
		t.Error("DiscoveredIDs should be initialized")
	}
}

func TestEnrichedMetadata_Fields(t *testing.T) {
	title := "The Matrix"
	year := 1999
	rating := 8.7

	meta := &EnrichedMetadata{
		Title:  &title,
		Year:   &year,
		Rating: &rating,
		Genre:  []string{"Action", "Sci-Fi"},
	}

	if *meta.Title != "The Matrix" {
		t.Errorf("Title = %s, want 'The Matrix'", *meta.Title)
	}
	if *meta.Year != 1999 {
		t.Errorf("Year = %d, want 1999", *meta.Year)
	}
	if *meta.Rating != 8.7 {
		t.Errorf("Rating = %f, want 8.7", *meta.Rating)
	}
	if len(meta.Genre) != 2 {
		t.Errorf("Genre length = %d, want 2", len(meta.Genre))
	}
}

func TestEnrichedImage_Fields(t *testing.T) {
	img := EnrichedImage{
		Type:     "poster",
		Path:     "/path/to/poster.jpg",
		IsRemote: false,
		Language: "en",
		Width:    1000,
		Height:   1500,
		Rating:   8.5,
	}

	if img.Type != "poster" {
		t.Errorf("Type = %s, want poster", img.Type)
	}
	if img.Path != "/path/to/poster.jpg" {
		t.Errorf("Path = %s, want /path/to/poster.jpg", img.Path)
	}
	if img.IsRemote {
		t.Error("IsRemote should be false")
	}
	if img.Language != "en" {
		t.Errorf("Language = %s, want en", img.Language)
	}
	if img.Width != 1000 {
		t.Errorf("Width = %d, want 1000", img.Width)
	}
	if img.Height != 1500 {
		t.Errorf("Height = %d, want 1500", img.Height)
	}
	if img.Rating != 8.5 {
		t.Errorf("Rating = %f, want 8.5", img.Rating)
	}
}

func TestEnrichRequest_Fields(t *testing.T) {
	req := &EnrichRequest{
		MediaID:       42,
		MediaType:     enrichment.MediaTypeMovie,
		FilePath:      "/movies/The Matrix (1999)/The Matrix.mkv",
		LibraryID:     1,
		Title:         "The Matrix",
		Year:          1999,
		OriginalTitle: "The Matrix",
		ShowTitle:     "",
		SeasonNumber:  0,
		EpisodeNumber: 0,
		Artist:        "",
		Album:         "",
		TrackNumber:   0,
		ExistingIDs: map[string]string{
			"imdb": "tt0133093",
		},
	}

	if req.MediaID != 42 {
		t.Errorf("MediaID = %d, want 42", req.MediaID)
	}
	if req.MediaType != enrichment.MediaTypeMovie {
		t.Errorf("MediaType = %s, want movie", req.MediaType)
	}
	if req.Title != "The Matrix" {
		t.Errorf("Title = %s, want 'The Matrix'", req.Title)
	}
	if req.Year != 1999 {
		t.Errorf("Year = %d, want 1999", req.Year)
	}
}

func TestEnrichResponse_Fields(t *testing.T) {
	resp := &EnrichResponse{
		Matched:         true,
		Metadata:        &EnrichedMetadata{},
		DiscoveredIDs:   map[string]string{"tmdb": "603"},
		Images:          []EnrichedImage{{Type: "poster"}},
		Skipped:         false,
		SkipReason:      "",
		ConfidenceScore: 95,
	}

	if !resp.Matched {
		t.Error("Matched should be true")
	}
	if resp.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
	if resp.DiscoveredIDs["tmdb"] != "603" {
		t.Errorf("DiscoveredIDs[tmdb] = %s, want 603", resp.DiscoveredIDs["tmdb"])
	}
	if len(resp.Images) != 1 {
		t.Errorf("Images length = %d, want 1", len(resp.Images))
	}
	if resp.ConfidenceScore != 95 {
		t.Errorf("ConfidenceScore = %d, want 95", resp.ConfidenceScore)
	}
}

func TestEnricherCapabilities_Fields(t *testing.T) {
	caps := EnricherCapabilities{
		MediaTypes: []enrichment.MediaType{enrichment.MediaTypeMovie, enrichment.MediaTypeTV},
		Provides:   []string{"metadata", "artwork", "external_ids"},
		IsLocal:    true,
		RateLimit:  0,
		Requires:   []string{"imdb"},
	}

	if len(caps.MediaTypes) != 2 {
		t.Errorf("MediaTypes length = %d, want 2", len(caps.MediaTypes))
	}
	if len(caps.Provides) != 3 {
		t.Errorf("Provides length = %d, want 3", len(caps.Provides))
	}
	if !caps.IsLocal {
		t.Error("IsLocal should be true")
	}
	if caps.RateLimit != 0 {
		t.Errorf("RateLimit = %d, want 0", caps.RateLimit)
	}
	if len(caps.Requires) != 1 {
		t.Errorf("Requires length = %d, want 1", len(caps.Requires))
	}
}

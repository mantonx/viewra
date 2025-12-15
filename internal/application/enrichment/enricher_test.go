package enrichment

import (
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

func TestEnricherCapabilities_SupportsMediaType(t *testing.T) {
	caps := NewCapabilities(&pluginv1.EnricherCapabilities{
		MediaTypes: []string{
			string(enrichment.MediaTypeMovie),
			string(enrichment.MediaTypeTV),
		},
	})

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
	caps := NewCapabilities(&pluginv1.EnricherCapabilities{
		MediaTypes: nil,
	})

	if caps.SupportsMediaType(enrichment.MediaTypeMovie) {
		t.Error("empty MediaTypes should not support any type")
	}
}

func TestHasExternalID(t *testing.T) {
	req := &pluginv1.EnrichRequest{
		MediaId: 42,
		ExistingIds: map[string]string{
			"imdb": "tt0133093",
			"tmdb": "603",
			"tvdb": "12345",
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
			got := HasExternalID(req, tt.provider)
			if got != tt.expected {
				t.Errorf("HasExternalID(%s) = %v, want %v", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestHasExternalID_NilMap(t *testing.T) {
	req := &pluginv1.EnrichRequest{
		MediaId:     42,
		ExistingIds: nil,
	}

	if HasExternalID(req, "imdb") {
		t.Error("nil ExistingIds should return false")
	}
}

func TestGetExternalID(t *testing.T) {
	req := &pluginv1.EnrichRequest{
		MediaId: 42,
		ExistingIds: map[string]string{
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
			got := GetExternalID(req, tt.provider)
			if got != tt.expected {
				t.Errorf("GetExternalID(%s) = %q, want %q", tt.provider, got, tt.expected)
			}
		})
	}
}

func TestGetExternalID_NilMap(t *testing.T) {
	req := &pluginv1.EnrichRequest{
		MediaId:     42,
		ExistingIds: nil,
	}

	got := GetExternalID(req, "imdb")
	if got != "" {
		t.Errorf("GetExternalID on nil map should return empty string, got %q", got)
	}
}

func TestAddDiscoveredID(t *testing.T) {
	resp := &pluginv1.EnrichResponse{}

	// First add should initialize the map
	AddDiscoveredID(resp, "imdb", "tt0133093")

	if resp.DiscoveredIds == nil {
		t.Fatal("DiscoveredIds should be initialized")
	}
	if resp.DiscoveredIds["imdb"] != "tt0133093" {
		t.Errorf("DiscoveredIds[imdb] = %q, want tt0133093", resp.DiscoveredIds["imdb"])
	}

	// Second add should work without re-initializing
	AddDiscoveredID(resp, "tmdb", "603")
	if resp.DiscoveredIds["tmdb"] != "603" {
		t.Errorf("DiscoveredIds[tmdb] = %q, want 603", resp.DiscoveredIds["tmdb"])
	}

	// Overwrite existing
	AddDiscoveredID(resp, "imdb", "tt9999999")
	if resp.DiscoveredIds["imdb"] != "tt9999999" {
		t.Errorf("DiscoveredIds[imdb] = %q, want tt9999999", resp.DiscoveredIds["imdb"])
	}
}

func TestAddImage(t *testing.T) {
	resp := &pluginv1.EnrichResponse{}

	img1 := &pluginv1.EnrichedImage{
		Type:     "poster",
		Path:     "/path/to/poster.jpg",
		IsRemote: false,
	}
	AddImage(resp, img1)

	if len(resp.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(resp.Images))
	}
	if resp.Images[0].Type != "poster" {
		t.Errorf("Images[0].Type = %s, want poster", resp.Images[0].Type)
	}

	img2 := &pluginv1.EnrichedImage{
		Type:     "fanart",
		Path:     "https://example.com/fanart.jpg",
		IsRemote: true,
	}
	AddImage(resp, img2)

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
	if resp.DiscoveredIds == nil {
		t.Error("DiscoveredIds should be initialized")
	}
}

func TestPtr(t *testing.T) {
	strVal := Ptr("test")
	if *strVal != "test" {
		t.Errorf("Ptr(string) = %q, want 'test'", *strVal)
	}

	intVal := Ptr(42)
	if *intVal != 42 {
		t.Errorf("Ptr(int) = %d, want 42", *intVal)
	}
}

func TestNewMetadata(t *testing.T) {
	meta := NewMetadata()
	if meta == nil {
		t.Error("NewMetadata should return non-nil")
	}
}

func TestCapabilitiesBuilder(t *testing.T) {
	caps := NewCapabilitiesBuilder().
		WithMediaTypes(enrichment.MediaTypeMovie, enrichment.MediaTypeTV).
		WithProvides("metadata", "artwork").
		AsLocal().
		WithRequires("imdb").
		Build()

	if len(caps.MediaTypes) != 2 {
		t.Errorf("MediaTypes length = %d, want 2", len(caps.MediaTypes))
	}
	if !caps.SupportsMediaType(enrichment.MediaTypeMovie) {
		t.Error("Should support movie")
	}
	if !caps.IsLocalEnricher() {
		t.Error("Should be local")
	}
	if len(caps.Provides) != 2 {
		t.Errorf("Provides length = %d, want 2", len(caps.Provides))
	}
	if len(caps.Requires) != 1 {
		t.Errorf("Requires length = %d, want 1", len(caps.Requires))
	}
}

func TestCapabilitiesBuilder_AsRemote(t *testing.T) {
	caps := NewCapabilitiesBuilder().
		WithMediaTypes(enrichment.MediaTypeMovie).
		AsRemote(60).
		Build()

	if caps.IsLocalEnricher() {
		t.Error("Should not be local")
	}
	if caps.GetRateLimitPerMinute() != 60 {
		t.Errorf("RateLimit = %d, want 60", caps.GetRateLimitPerMinute())
	}
}

package enrichment

import (
	"bytes"
	"testing"
)

func TestRegistry_PrintTable(t *testing.T) {
	registry := NewRegistry()

	// Simulate built-in enrichers
	registry.Register(&EnricherInfo{
		Stage:      "nfo",
		Name:       "NFO Parser",
		Version:    "1.0.0",
		Source:     SourceBuiltin,
		Provides:   []string{"metadata", "external_ids"},
		MediaTypes: []string{"movie", "tv", "tv_show"},
		IsLocal:    true,
	})

	registry.Register(&EnricherInfo{
		Stage:      "local-images",
		Name:       "Local Images",
		Version:    "1.0.0",
		Source:     SourceBuiltin,
		Provides:   []string{"artwork"},
		MediaTypes: []string{"movie", "tv", "tv_show", "music", "music_album"},
		IsLocal:    true,
	})

	// Simulate external plugins
	registry.Register(&EnricherInfo{
		Stage:      "tmdb",
		Name:       "TMDb Enricher",
		Version:    "1.0.0",
		Source:     SourceExternal,
		Provides:   []string{"metadata", "artwork", "external_ids"},
		MediaTypes: []string{"movie", "tv", "tv_show"},
		RateLimit:  40,
	})

	registry.Register(&EnricherInfo{
		Stage:      "musicbrainz",
		Name:       "MusicBrainz Enricher",
		Version:    "1.0.0",
		Source:     SourceExternal,
		Provides:   []string{"metadata", "artwork", "external_ids"},
		MediaTypes: []string{"music", "music_album", "music_artist"},
		RateLimit:  60,
	})

	var buf bytes.Buffer
	registry.PrintTable(&buf, "Plugins (4 loaded)")

	output := buf.String()
	t.Logf("Table output:\n%s", output)

	// Verify key content is present
	if !bytes.Contains(buf.Bytes(), []byte("NFO Parser")) {
		t.Error("Expected 'NFO Parser' in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("MusicBrainz")) {
		t.Error("Expected 'MusicBrainz' in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("builtin")) {
		t.Error("Expected 'builtin' in output")
	}
	if !bytes.Contains(buf.Bytes(), []byte("external")) {
		t.Error("Expected 'external' in output")
	}
}

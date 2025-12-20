package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

func TestNFOEnricher_Stage(t *testing.T) {
	e := NewNFOEnricher()
	if e.Stage() != "nfo" {
		t.Errorf("Stage() = %v, want 'nfo'", e.Stage())
	}
}

func TestNFOEnricher_Metadata(t *testing.T) {
	e := NewNFOEnricher()
	name, version := e.Metadata()
	if name != "NFO Parser" {
		t.Errorf("name = %v, want 'NFO Parser'", name)
	}
	if version != "1.0.0" {
		t.Errorf("version = %v, want '1.0.0'", version)
	}
}

func TestNFOEnricher_Capabilities(t *testing.T) {
	e := NewNFOEnricher()
	caps := e.Capabilities()

	// Should support movie, tv, and tvshow media types
	if !caps.SupportsMediaType(enrichment.MediaTypeMovie) {
		t.Error("expected to support movie media type")
	}
	if !caps.SupportsMediaType(enrichment.MediaTypeTV) {
		t.Error("expected to support tv media type")
	}
	if !caps.SupportsMediaType(enrichment.MediaTypeTVShow) {
		t.Error("expected to support tvshow media type")
	}

	// Should be a local enricher
	if !caps.IsLocalEnricher() {
		t.Error("expected to be a local enricher")
	}
}

func TestNFOEnricher_Movie(t *testing.T) {
	tmpDir := t.TempDir()
	movieDir := filepath.Join(tmpDir, "Test Movie (2024)")
	if err := os.MkdirAll(movieDir, 0755); err != nil {
		t.Fatalf("failed to create movie dir: %v", err)
	}

	moviePath := filepath.Join(movieDir, "Test Movie (2024).mkv")
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create movie file: %v", err)
	}

	nfoPath := filepath.Join(movieDir, "Test Movie (2024).nfo")
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>NFO Title Override</title>
  <originaltitle>Original NFO Title</originaltitle>
  <sorttitle>NFO, Test</sorttitle>
  <year>2024</year>
  <releasedate>2024-06-15</releasedate>
  <runtime>120</runtime>
  <imdb>tt1234567</imdb>
  <tmdbid>98765</tmdbid>
  <director>Test Director</director>
  <actor><name>Actor One</name><role>Role One</role></actor>
  <actor><name>Actor Two</name><role>Role Two</role></actor>
  <genre>Action</genre>
  <genre>Sci-Fi</genre>
  <plot>This is the test plot from NFO.</plot>
  <tagline>Test tagline</tagline>
  <mpaa>PG-13</mpaa>
</movie>`
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("failed to create NFO file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeMovie),
		FilePath:  moviePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if resp.Skipped {
		t.Fatalf("expected match, got skipped: %s", resp.SkipReason)
	}
	if !resp.Matched {
		t.Fatal("expected matched to be true")
	}
	if resp.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", resp.Confidence)
	}

	// Verify metadata
	meta := resp.Metadata
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if meta.Title == nil || *meta.Title != "NFO Title Override" {
		t.Errorf("Title = %v, want 'NFO Title Override'", meta.Title)
	}
	if meta.OriginalTitle == nil || *meta.OriginalTitle != "Original NFO Title" {
		t.Errorf("OriginalTitle = %v, want 'Original NFO Title'", meta.OriginalTitle)
	}
	if meta.Year == nil || *meta.Year != 2024 {
		t.Errorf("Year = %v, want 2024", meta.Year)
	}
	if meta.Plot == nil || *meta.Plot != "This is the test plot from NFO." {
		t.Errorf("Plot = %v, want 'This is the test plot from NFO.'", meta.Plot)
	}
	if meta.Tagline == nil || *meta.Tagline != "Test tagline" {
		t.Errorf("Tagline = %v, want 'Test tagline'", meta.Tagline)
	}
	if meta.ContentRating == nil || *meta.ContentRating != "PG-13" {
		t.Errorf("ContentRating = %v, want 'PG-13'", meta.ContentRating)
	}
	if meta.RuntimeMinutes == nil || *meta.RuntimeMinutes != 120 {
		t.Errorf("RuntimeMinutes = %v, want 120", meta.RuntimeMinutes)
	}
	if len(meta.Directors) != 1 || meta.Directors[0] != "Test Director" {
		t.Errorf("Directors = %v, want ['Test Director']", meta.Directors)
	}
	if len(meta.Cast) != 2 {
		t.Errorf("Cast count = %d, want 2", len(meta.Cast))
	}
	if len(meta.Genres) != 2 || meta.Genres[0] != "Action" || meta.Genres[1] != "Sci-Fi" {
		t.Errorf("Genres = %v, want ['Action', 'Sci-Fi']", meta.Genres)
	}

	// Verify discovered IDs
	if resp.DiscoveredIds == nil {
		t.Fatal("expected non-nil DiscoveredIds")
	}
	if resp.DiscoveredIds["imdb"] != "tt1234567" {
		t.Errorf("imdb ID = %v, want 'tt1234567'", resp.DiscoveredIds["imdb"])
	}
	if resp.DiscoveredIds["tmdb"] != "98765" {
		t.Errorf("tmdb ID = %v, want '98765'", resp.DiscoveredIds["tmdb"])
	}
}

func TestNFOEnricher_Movie_NoNFO(t *testing.T) {
	tmpDir := t.TempDir()
	moviePath := filepath.Join(tmpDir, "Movie Without NFO.mkv")
	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create movie file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeMovie),
		FilePath:  moviePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if !resp.Skipped {
		t.Error("expected skipped response when no NFO file exists")
	}
	if resp.SkipReason != "no NFO file found" {
		t.Errorf("SkipReason = %v, want 'no NFO file found'", resp.SkipReason)
	}
}

func TestNFOEnricher_Movie_EmptyNFO(t *testing.T) {
	tmpDir := t.TempDir()
	moviePath := filepath.Join(tmpDir, "Empty NFO Movie.mkv")
	nfoPath := filepath.Join(tmpDir, "Empty NFO Movie.nfo")

	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create movie file: %v", err)
	}
	if err := os.WriteFile(nfoPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create NFO file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeMovie),
		FilePath:  moviePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if !resp.Skipped {
		t.Error("expected skipped response for empty NFO file")
	}
	if resp.SkipReason != "NFO file is empty" {
		t.Errorf("SkipReason = %v, want 'NFO file is empty'", resp.SkipReason)
	}
}

func TestNFOEnricher_Movie_WrongNFOType(t *testing.T) {
	tmpDir := t.TempDir()
	moviePath := filepath.Join(tmpDir, "Wrong Type Movie.mkv")
	nfoPath := filepath.Join(tmpDir, "Wrong Type Movie.nfo")

	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create movie file: %v", err)
	}

	// NFO contains tvshow instead of movie
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
  <title>Some TV Show</title>
</tvshow>`
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("failed to create NFO file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeMovie),
		FilePath:  moviePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if !resp.Skipped {
		t.Error("expected skipped response for wrong NFO type")
	}
	if resp.SkipReason != "NFO contains wrong type: tvshow" {
		t.Errorf("SkipReason = %v, want 'NFO contains wrong type: tvshow'", resp.SkipReason)
	}
}

func TestNFOEnricher_TVEpisode(t *testing.T) {
	tmpDir := t.TempDir()
	showDir := filepath.Join(tmpDir, "Test Show", "Season 01")
	if err := os.MkdirAll(showDir, 0755); err != nil {
		t.Fatalf("failed to create show dir: %v", err)
	}

	episodePath := filepath.Join(showDir, "Test Show - S01E05 - Episode Title.mkv")
	if err := os.WriteFile(episodePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create episode file: %v", err)
	}

	nfoPath := filepath.Join(showDir, "Test Show - S01E05 - Episode Title.nfo")
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<episodedetails>
  <title>NFO Episode Title</title>
  <showtitle>NFO Show Title</showtitle>
  <season>1</season>
  <episode>5</episode>
  <plot>This is the episode plot from NFO.</plot>
  <aired>2024-05-15</aired>
  <runtime>45</runtime>
  <uniqueid type="imdb">tt9876543</uniqueid>
  <uniqueid type="tvdb">123456</uniqueid>
</episodedetails>`
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("failed to create NFO file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeTV),
		FilePath:  episodePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if resp.Skipped {
		t.Fatalf("expected match, got skipped: %s", resp.SkipReason)
	}
	if !resp.Matched {
		t.Fatal("expected matched to be true")
	}
	if resp.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", resp.Confidence)
	}

	// Verify metadata
	meta := resp.Metadata
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if meta.Title == nil || *meta.Title != "NFO Episode Title" {
		t.Errorf("Title = %v, want 'NFO Episode Title'", meta.Title)
	}
	if meta.Plot == nil || *meta.Plot != "This is the episode plot from NFO." {
		t.Errorf("Plot = %v, want 'This is the episode plot from NFO.'", meta.Plot)
	}
	if meta.Premiered == nil || *meta.Premiered != "2024-05-15" {
		t.Errorf("Premiered = %v, want '2024-05-15'", meta.Premiered)
	}
	if meta.RuntimeMinutes == nil || *meta.RuntimeMinutes != 45 {
		t.Errorf("RuntimeMinutes = %v, want 45", meta.RuntimeMinutes)
	}

	// Verify discovered IDs
	if resp.DiscoveredIds == nil {
		t.Fatal("expected non-nil DiscoveredIds")
	}
	if resp.DiscoveredIds["imdb"] != "tt9876543" {
		t.Errorf("imdb ID = %v, want 'tt9876543'", resp.DiscoveredIds["imdb"])
	}
	if resp.DiscoveredIds["tvdb"] != "123456" {
		t.Errorf("tvdb ID = %v, want '123456'", resp.DiscoveredIds["tvdb"])
	}
}

func TestNFOEnricher_TVEpisode_NoNFO(t *testing.T) {
	tmpDir := t.TempDir()
	episodePath := filepath.Join(tmpDir, "Episode Without NFO.mkv")
	if err := os.WriteFile(episodePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create episode file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeTV),
		FilePath:  episodePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if !resp.Skipped {
		t.Error("expected skipped response when no NFO file exists")
	}
}

func TestNFOEnricher_TVShow(t *testing.T) {
	tmpDir := t.TempDir()
	showDir := filepath.Join(tmpDir, "Test TV Show (2020)")
	if err := os.MkdirAll(showDir, 0755); err != nil {
		t.Fatalf("failed to create show dir: %v", err)
	}

	nfoPath := filepath.Join(showDir, "tvshow.nfo")
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
  <title>NFO Show Title</title>
  <originaltitle>Original Show Title</originaltitle>
  <sorttitle>Show, NFO</sorttitle>
  <year>2020</year>
  <premiered>2020-09-01</premiered>
  <plot>This is the show plot from NFO.</plot>
  <tagline>Show tagline</tagline>
  <mpaa>TV-MA</mpaa>
  <imdb>tt7654321</imdb>
  <tvdbid>654321</tvdbid>
  <tmdbid>54321</tmdbid>
  <actor><name>Star Actor</name><role>Main Role</role></actor>
  <genre>Drama</genre>
  <genre>Mystery</genre>
  <network>HBO</network>
</tvshow>`
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("failed to create NFO file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeTVShow),
		FilePath:  showDir, // For TV shows, FilePath is the show directory
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if resp.Skipped {
		t.Fatalf("expected match, got skipped: %s", resp.SkipReason)
	}
	if !resp.Matched {
		t.Fatal("expected matched to be true")
	}
	if resp.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", resp.Confidence)
	}

	// Verify metadata
	meta := resp.Metadata
	if meta == nil {
		t.Fatal("expected non-nil metadata")
	}
	if meta.Title == nil || *meta.Title != "NFO Show Title" {
		t.Errorf("Title = %v, want 'NFO Show Title'", meta.Title)
	}
	if meta.OriginalTitle == nil || *meta.OriginalTitle != "Original Show Title" {
		t.Errorf("OriginalTitle = %v, want 'Original Show Title'", meta.OriginalTitle)
	}
	if meta.Year == nil || *meta.Year != 2020 {
		t.Errorf("Year = %v, want 2020", meta.Year)
	}
	if meta.Plot == nil || *meta.Plot != "This is the show plot from NFO." {
		t.Errorf("Plot = %v, want 'This is the show plot from NFO.'", meta.Plot)
	}
	if meta.Tagline == nil || *meta.Tagline != "Show tagline" {
		t.Errorf("Tagline = %v, want 'Show tagline'", meta.Tagline)
	}
	if meta.ContentRating == nil || *meta.ContentRating != "TV-MA" {
		t.Errorf("ContentRating = %v, want 'TV-MA'", meta.ContentRating)
	}
	if meta.Premiered == nil || *meta.Premiered != "2020-09-01" {
		t.Errorf("Premiered = %v, want '2020-09-01'", meta.Premiered)
	}
	if len(meta.Cast) != 1 || meta.Cast[0].Name != "Star Actor" {
		t.Errorf("Cast = %v, want [Star Actor]", meta.Cast)
	}
	if len(meta.Genres) != 2 || meta.Genres[0] != "Drama" || meta.Genres[1] != "Mystery" {
		t.Errorf("Genres = %v, want ['Drama', 'Mystery']", meta.Genres)
	}
	if len(meta.Studios) != 1 || meta.Studios[0] != "HBO" {
		t.Errorf("Studios = %v, want ['HBO']", meta.Studios)
	}

	// Verify discovered IDs
	if resp.DiscoveredIds == nil {
		t.Fatal("expected non-nil DiscoveredIds")
	}
	if resp.DiscoveredIds["imdb"] != "tt7654321" {
		t.Errorf("imdb ID = %v, want 'tt7654321'", resp.DiscoveredIds["imdb"])
	}
	if resp.DiscoveredIds["tvdb"] != "654321" {
		t.Errorf("tvdb ID = %v, want '654321'", resp.DiscoveredIds["tvdb"])
	}
	if resp.DiscoveredIds["tmdb"] != "54321" {
		t.Errorf("tmdb ID = %v, want '54321'", resp.DiscoveredIds["tmdb"])
	}
}

func TestNFOEnricher_TVShow_NoNFO(t *testing.T) {
	tmpDir := t.TempDir()
	showDir := filepath.Join(tmpDir, "Show Without NFO")
	if err := os.MkdirAll(showDir, 0755); err != nil {
		t.Fatalf("failed to create show dir: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeTVShow),
		FilePath:  showDir,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if !resp.Skipped {
		t.Error("expected skipped response when no NFO file exists")
	}
}

func TestNFOEnricher_UnsupportedMediaType(t *testing.T) {
	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: "unsupported",
		FilePath:  "/some/path",
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if !resp.Skipped {
		t.Error("expected skipped response for unsupported media type")
	}
	if resp.SkipReason != "unsupported media type: unsupported" {
		t.Errorf("SkipReason = %v, want 'unsupported media type: unsupported'", resp.SkipReason)
	}
}

func TestNFOEnricher_Movie_PartialMetadata(t *testing.T) {
	// Test that NFO with partial data only sets what's available
	tmpDir := t.TempDir()
	moviePath := filepath.Join(tmpDir, "Partial Movie.mkv")
	nfoPath := filepath.Join(tmpDir, "Partial Movie.nfo")

	if err := os.WriteFile(moviePath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create movie file: %v", err)
	}

	// NFO with only title and year
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>Minimal Movie</title>
  <year>2023</year>
</movie>`
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("failed to create NFO file: %v", err)
	}

	e := NewNFOEnricher()
	req := &pluginv1.EnrichRequest{
		MediaType: string(enrichment.MediaTypeMovie),
		FilePath:  moviePath,
	}

	resp, err := e.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if resp.Skipped {
		t.Fatalf("expected match, got skipped: %s", resp.SkipReason)
	}

	meta := resp.Metadata
	if meta.Title == nil || *meta.Title != "Minimal Movie" {
		t.Errorf("Title = %v, want 'Minimal Movie'", meta.Title)
	}
	if meta.Year == nil || *meta.Year != 2023 {
		t.Errorf("Year = %v, want 2023", meta.Year)
	}

	// Optional fields should be nil
	if meta.OriginalTitle != nil {
		t.Errorf("OriginalTitle = %v, want nil", meta.OriginalTitle)
	}
	if meta.Plot != nil {
		t.Errorf("Plot = %v, want nil", meta.Plot)
	}
	if meta.Tagline != nil {
		t.Errorf("Tagline = %v, want nil", meta.Tagline)
	}
	if meta.RuntimeMinutes != nil {
		t.Errorf("RuntimeMinutes = %v, want nil", meta.RuntimeMinutes)
	}
}

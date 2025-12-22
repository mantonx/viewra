package nfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseTVShowNFO(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a sample tvshow.nfo file
	nfoContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<tvshow>
	<title>Breaking Bad</title>
	<originaltitle>Breaking Bad</originaltitle>
	<sorttitle>Breaking Bad</sorttitle>
	<year>2008</year>
	<premiered>2008-01-20</premiered>
	<plot>A high school chemistry teacher turned methamphetamine producer.</plot>
	<tagline>Change the equation</tagline>
	<status>Ended</status>
	<rating>9.5</rating>
	<mpaa>TV-MA</mpaa>
	<imdb>tt0903747</imdb>
	<tvdbid>81189</tvdbid>
	<tmdbid>1396</tmdbid>
	<genre>Crime</genre>
	<genre>Drama</genre>
	<genre>Thriller</genre>
	<actor>
		<name>Bryan Cranston</name>
		<role>Walter White</role>
	</actor>
	<actor>
		<name>Aaron Paul</name>
		<role>Jesse Pinkman</role>
	</actor>
	<actor>
		<name>Anna Gunn</name>
		<role>Skyler White</role>
	</actor>
	<studio>AMC</studio>
	<network>AMC</network>
	<country>USA</country>
	<originallanguage>en</originallanguage>
	<airday>Sunday</airday>
	<airtime>9:00 PM</airtime>
</tvshow>`

	nfoPath := filepath.Join(tmpDir, "tvshow.nfo")
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create test NFO file: %v", err)
	}

	// Parse the NFO
	metadata, err := ParseTVShowNFO(nfoPath)
	if err != nil {
		t.Fatalf("ParseTVShowNFO failed: %v", err)
	}

	// Verify basic fields
	if metadata.Title != "Breaking Bad" {
		t.Errorf("Expected title 'Breaking Bad', got '%s'", metadata.Title)
	}

	if metadata.Year != 2008 {
		t.Errorf("Expected year 2008, got %d", metadata.Year)
	}

	if metadata.Status != "Ended" {
		t.Errorf("Expected status 'Ended', got '%s'", metadata.Status)
	}

	if metadata.Network != "AMC" {
		t.Errorf("Expected network 'AMC', got '%s'", metadata.Network)
	}

	// Verify IDs
	if metadata.IMDbID != "tt0903747" {
		t.Errorf("Expected IMDb ID 'tt0903747', got '%s'", metadata.IMDbID)
	}

	if metadata.TVDbID != 81189 {
		t.Errorf("Expected TVDb ID 81189, got %d", metadata.TVDbID)
	}

	if metadata.TMDbID != 1396 {
		t.Errorf("Expected TMDb ID 1396, got %d", metadata.TMDbID)
	}

	// Verify cast
	if len(metadata.Cast) != 3 {
		t.Errorf("Expected 3 cast members, got %d", len(metadata.Cast))
	}

	expectedCast := []string{"Bryan Cranston", "Aaron Paul", "Anna Gunn"}
	for i, name := range expectedCast {
		if i >= len(metadata.Cast) || metadata.Cast[i].Name != name {
			castName := ""
			if i < len(metadata.Cast) {
				castName = metadata.Cast[i].Name
			}
			t.Errorf("Expected cast[%d] to be '%s', got '%s'", i, name, castName)
		}
	}

	// Verify genres
	if len(metadata.Genre) != 3 {
		t.Errorf("Expected 3 genres, got %d", len(metadata.Genre))
	}

	// Verify premiere date
	expectedDate := time.Date(2008, 1, 20, 0, 0, 0, 0, time.UTC)
	if !metadata.PremiereDate.Equal(expectedDate) {
		t.Errorf("Expected premiere date %v, got %v", expectedDate, metadata.PremiereDate)
	}

	// Verify air time info
	if metadata.AirDay != "Sunday" {
		t.Errorf("Expected air day 'Sunday', got '%s'", metadata.AirDay)
	}

	if metadata.AirTime != "9:00 PM" {
		t.Errorf("Expected air time '9:00 PM', got '%s'", metadata.AirTime)
	}

	// Verify rating
	if metadata.MaturityRating != 9 {
		t.Errorf("Expected maturity rating 9, got %d", metadata.MaturityRating)
	}
}

func TestParseEpisodeNFO(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a sample episode.nfo file
	nfoContent := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<episodedetails>
	<title>Pilot</title>
	<showtitle>Breaking Bad</showtitle>
	<season>1</season>
	<episode>1</episode>
	<aired>2008-01-20</aired>
	<plot>Walter White is diagnosed with cancer and turns to making meth.</plot>
	<runtime>58</runtime>
	<director>Vince Gilligan</director>
	<credits>Vince Gilligan</credits>
	<rating>8.2</rating>
	<imdb>tt0959621</imdb>
	<tvdbid>349232</tvdbid>
	<actor>
		<name>Max Arciniega</name>
		<role>Krazy-8</role>
	</actor>
</episodedetails>`

	nfoPath := filepath.Join(tmpDir, "episode.nfo")
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create test NFO file: %v", err)
	}

	// Parse the NFO
	metadata, err := ParseEpisodeNFO(nfoPath)
	if err != nil {
		t.Fatalf("ParseEpisodeNFO failed: %v", err)
	}

	// Verify basic fields
	if metadata.Title != "Pilot" {
		t.Errorf("Expected title 'Pilot', got '%s'", metadata.Title)
	}

	if metadata.ShowTitle != "Breaking Bad" {
		t.Errorf("Expected show title 'Breaking Bad', got '%s'", metadata.ShowTitle)
	}

	if metadata.Season != 1 {
		t.Errorf("Expected season 1, got %d", metadata.Season)
	}

	if metadata.Episode != 1 {
		t.Errorf("Expected episode 1, got %d", metadata.Episode)
	}

	if metadata.RuntimeMinutes != 58 {
		t.Errorf("Expected runtime 58, got %d", metadata.RuntimeMinutes)
	}

	if metadata.Director != "Vince Gilligan" {
		t.Errorf("Expected director 'Vince Gilligan', got '%s'", metadata.Director)
	}

	// Verify writers
	if len(metadata.Writers) != 1 || metadata.Writers[0] != "Vince Gilligan" {
		t.Errorf("Expected writer 'Vince Gilligan', got %v", metadata.Writers)
	}

	// Verify IDs
	if metadata.IMDbID != "tt0959621" {
		t.Errorf("Expected IMDb ID 'tt0959621', got '%s'", metadata.IMDbID)
	}

	if metadata.TVDbID != 349232 {
		t.Errorf("Expected TVDb ID 349232, got %d", metadata.TVDbID)
	}

	// Verify air date
	expectedDate := time.Date(2008, 1, 20, 0, 0, 0, 0, time.UTC)
	if !metadata.AirDate.Equal(expectedDate) {
		t.Errorf("Expected air date %v, got %v", expectedDate, metadata.AirDate)
	}

	// Verify rating
	if metadata.Rating != 8.2 {
		t.Errorf("Expected rating 8.2, got %f", metadata.Rating)
	}

	// Verify guest stars
	if len(metadata.GuestStars) != 1 || metadata.GuestStars[0] != "Max Arciniega" {
		t.Errorf("Expected guest star 'Max Arciniega', got %v", metadata.GuestStars)
	}
}

func TestParseTVShowNFO_WithUniqueIDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create NFO with new UniqueID format
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<tvshow>
	<title>The Mandalorian</title>
	<year>2019</year>
	<uniqueid type="imdb" default="true">tt8111088</uniqueid>
	<uniqueid type="tvdb">361753</uniqueid>
	<uniqueid type="tmdb">82856</uniqueid>
	<status>Continuing</status>
</tvshow>`

	nfoPath := filepath.Join(tmpDir, "tvshow.nfo")
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create test NFO file: %v", err)
	}

	metadata, err := ParseTVShowNFO(nfoPath)
	if err != nil {
		t.Fatalf("ParseTVShowNFO failed: %v", err)
	}

	if metadata.IMDbID != "tt8111088" {
		t.Errorf("Expected IMDb ID 'tt8111088', got '%s'", metadata.IMDbID)
	}

	if metadata.TVDbID != 361753 {
		t.Errorf("Expected TVDb ID 361753, got %d", metadata.TVDbID)
	}

	if metadata.TMDbID != 82856 {
		t.Errorf("Expected TMDb ID 82856, got %d", metadata.TMDbID)
	}
}

func TestParseEpisodeNFO_WithDisplayNumbers(t *testing.T) {
	tmpDir := t.TempDir()

	// Create episode NFO with display season/episode (for specials)
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<episodedetails>
	<title>Christmas Special</title>
	<showtitle>Doctor Who</showtitle>
	<season>0</season>
	<episode>1</episode>
	<displayseason>2023</displayseason>
	<displayepisode>1</displayepisode>
	<aired>2023-12-25</aired>
	<runtime>60 min</runtime>
</episodedetails>`

	nfoPath := filepath.Join(tmpDir, "episode.nfo")
	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create test NFO file: %v", err)
	}

	metadata, err := ParseEpisodeNFO(nfoPath)
	if err != nil {
		t.Fatalf("ParseEpisodeNFO failed: %v", err)
	}

	if metadata.Season != 0 {
		t.Errorf("Expected season 0, got %d", metadata.Season)
	}

	if metadata.Episode != 1 {
		t.Errorf("Expected episode 1, got %d", metadata.Episode)
	}

	if metadata.DisplaySeason != 2023 {
		t.Errorf("Expected display season 2023, got %d", metadata.DisplaySeason)
	}

	if metadata.DisplayEpisode != 1 {
		t.Errorf("Expected display episode 1, got %d", metadata.DisplayEpisode)
	}

	// Runtime with "min" suffix should be parsed correctly
	if metadata.RuntimeMinutes != 60 {
		t.Errorf("Expected runtime 60, got %d", metadata.RuntimeMinutes)
	}
}

func TestFindTVShowNFO(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: No NFO file exists
	_, err := FindTVShowNFO(tmpDir)
	if err == nil {
		t.Error("Expected error when no NFO exists, got nil")
	}

	// Test 2: tvshow.nfo exists
	nfoPath := filepath.Join(tmpDir, "tvshow.nfo")
	if err := os.WriteFile(nfoPath, []byte("<tvshow></tvshow>"), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	foundPath, err := FindTVShowNFO(tmpDir)
	if err != nil {
		t.Fatalf("FindTVShowNFO failed: %v", err)
	}

	if foundPath != nfoPath {
		t.Errorf("Expected to find '%s', got '%s'", nfoPath, foundPath)
	}
}

func TestFindEpisodeNFO(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an episode file
	episodePath := filepath.Join(tmpDir, "Breaking Bad - S01E01 - Pilot.mkv")
	if err := os.WriteFile(episodePath, []byte("fake episode"), 0644); err != nil {
		t.Fatalf("Failed to create test episode file: %v", err)
	}

	// Test 1: No NFO file exists
	_, err := FindEpisodeNFO(episodePath)
	if err == nil {
		t.Error("Expected error when no NFO exists, got nil")
	}

	// Test 2: Exact match NFO
	nfoPath := filepath.Join(tmpDir, "Breaking Bad - S01E01 - Pilot.nfo")
	if err := os.WriteFile(nfoPath, []byte("<episodedetails></episodedetails>"), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	foundPath, err := FindEpisodeNFO(episodePath)
	if err != nil {
		t.Fatalf("FindEpisodeNFO failed: %v", err)
	}

	if foundPath != nfoPath {
		t.Errorf("Expected to find '%s', got '%s'", nfoPath, foundPath)
	}
}

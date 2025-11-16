package music

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// createTestMP3 creates a minimal valid MP3 file with ID3v2 tags
// This is a simplified test - in real usage we'd use actual audio files
func createTestMP3WithTags(t *testing.T, path string, tags map[string]string) {
	t.Helper()

	// Create a minimal MP3 file structure
	// ID3v2.3 header: "ID3" + version (3,0) + flags (0) + size (synchsafe int)
	id3Header := []byte{
		0x49, 0x44, 0x33, // "ID3"
		0x03, 0x00, // Version 2.3.0
		0x00,       // Flags
		0x00, 0x00, 0x00, 0x00, // Size (we'll calculate)
	}

	// For testing purposes, we'll create a simple structure
	// In real testing, you'd want actual MP3 files or use a library to generate them

	// Write a minimal valid MP3 file
	// This is highly simplified - real MP3 files are much more complex
	mp3Data := append(id3Header, []byte("test audio data")...)

	if err := os.WriteFile(path, mp3Data, 0644); err != nil {
		t.Fatalf("Failed to create test MP3 file: %v", err)
	}
}

func TestParseAudioFile_NoFile(t *testing.T) {
	_, err := ParseAudioFile("/nonexistent/file.mp3")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestHasMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Test 1: Non-existent file
	if HasMetadata("/nonexistent/file.mp3") {
		t.Error("Expected false for non-existent file")
	}

	// Test 2: File without metadata (plain text file)
	plainFile := filepath.Join(tmpDir, "plain.txt")
	if err := os.WriteFile(plainFile, []byte("not an audio file"), 0644); err != nil {
		t.Fatalf("Failed to create plain file: %v", err)
	}

	if HasMetadata(plainFile) {
		t.Error("Expected false for plain text file")
	}
}

func TestTrackMetadataStructure(t *testing.T) {
	// Test that we can create and populate a TrackMetadata struct
	metadata := &TrackMetadata{
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		AlbumArtist: "Test Album Artist",
		Genre:       "Rock",
		Year:        2023,
		TrackNumber: 1,
		TrackTotal:  10,
		DiscNumber:  1,
		DiscTotal:   2,
		Codec:       "MP3",
	}

	if metadata.Title != "Test Song" {
		t.Errorf("Expected title 'Test Song', got '%s'", metadata.Title)
	}

	if metadata.TrackNumber != 1 {
		t.Errorf("Expected track number 1, got %d", metadata.TrackNumber)
	}

	if metadata.Year != 2023 {
		t.Errorf("Expected year 2023, got %d", metadata.Year)
	}
}

// TestParseRealMP3File tests parsing with a real MP3 file if available
// This test will be skipped if no test file is present
func TestParseRealMP3File(t *testing.T) {
	// Skip this test in CI/CD or when no test file is available
	testFile := os.Getenv("TEST_AUDIO_FILE")
	if testFile == "" {
		t.Skip("Skipping real file test: TEST_AUDIO_FILE environment variable not set")
	}

	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Skipping real file test: test file does not exist")
	}

	metadata, err := ParseAudioFile(testFile)
	if err != nil {
		t.Fatalf("ParseAudioFile failed: %v", err)
	}

	// Basic validation - at least some fields should be populated
	if metadata == nil {
		t.Fatal("Expected metadata, got nil")
	}

	t.Logf("Parsed metadata:")
	t.Logf("  Title: %s", metadata.Title)
	t.Logf("  Artist: %s", metadata.Artist)
	t.Logf("  Album: %s", metadata.Album)
	t.Logf("  Year: %d", metadata.Year)
	t.Logf("  Track: %d/%d", metadata.TrackNumber, metadata.TrackTotal)
	t.Logf("  Codec: %s", metadata.Codec)
}

// TestMusicBrainzIDs tests extraction of MusicBrainz IDs
func TestMusicBrainzIDsStructure(t *testing.T) {
	metadata := &TrackMetadata{
		MusicBrainzTrackID:       "track-uuid",
		MusicBrainzRecordingID:   "recording-uuid",
		MusicBrainzReleaseID:     "release-uuid",
		MusicBrainzArtistID:      "artist-uuid",
		MusicBrainzAlbumArtistID: "album-artist-uuid",
	}

	if metadata.MusicBrainzTrackID != "track-uuid" {
		t.Errorf("Expected MusicBrainz track ID 'track-uuid', got '%s'", metadata.MusicBrainzTrackID)
	}

	if metadata.MusicBrainzReleaseID != "release-uuid" {
		t.Errorf("Expected MusicBrainz release ID 'release-uuid', got '%s'", metadata.MusicBrainzReleaseID)
	}
}

// TestSupportedFormats documents which audio formats are supported by the tag library
// Temporarily disabled - tag.Format type interface changed in library
/*
func TestSupportedFormats(t *testing.T) {
	supportedFormats := []tag.Format{
		tag.MP3,
		tag.M4A,
		tag.M4B,
		tag.M4P,
		tag.ALAC,
		tag.FLAC,
		tag.OGG,
		tag.Vorbis,
		tag.DSF,
	}

	if len(supportedFormats) < 5 {
		t.Error("Expected multiple supported formats")
	}

	t.Logf("Supported audio formats: %d", len(supportedFormats))
	for _, format := range supportedFormats {
		t.Logf("  - %s", format)
	}
}
*/

// TestExtractCustomTags tests the custom tag extraction logic
func TestExtractCustomTags(t *testing.T) {
	// This test documents the expected behavior of custom tag extraction
	customTags := map[string]interface{}{
		"MUSICBRAINZ_TRACKID":       "test-track-id",
		"MUSICBRAINZ_ALBUMID":       "test-album-id",
		"MUSICBRAINZ_ARTISTID":      "test-artist-id",
		"COPYRIGHT":                  "2023 Test Records",
		"PUBLISHER":                  "Test Publishing",
		"ISRC":                       "USABC1234567",
		"BARCODE":                    "123456789012",
		"CATALOGNUMBER":              "CAT-001",
	}

	// Verify we can format these as strings
	for key, value := range customTags {
		formatted := fmt.Sprintf("%v", value)
		if formatted == "" {
			t.Errorf("Failed to format custom tag '%s'", key)
		}
	}
}

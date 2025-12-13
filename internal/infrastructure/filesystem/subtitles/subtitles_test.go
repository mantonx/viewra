package subtitles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSubtitleSuffix(t *testing.T) {
	tests := []struct {
		name         string
		suffix       string
		codec        string
		wantLanguage string
		wantIsForced bool
		wantIsSDH    bool
		wantTitle    string
	}{
		// Language detection - ISO codes
		{
			name:         "english - en",
			suffix:       ".en",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "english - eng",
			suffix:       ".eng",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "spanish - es",
			suffix:       ".es",
			codec:        "subrip",
			wantLanguage: "spa",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "french - fr",
			suffix:       ".fr",
			codec:        "subrip",
			wantLanguage: "fra",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "german - de",
			suffix:       ".de",
			codec:        "subrip",
			wantLanguage: "deu",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "japanese - ja",
			suffix:       ".ja",
			codec:        "ass",
			wantLanguage: "jpn",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "chinese - zh",
			suffix:       ".zh",
			codec:        "subrip",
			wantLanguage: "zho",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "portuguese - pt",
			suffix:       ".pt",
			codec:        "subrip",
			wantLanguage: "por",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "russian - ru",
			suffix:       ".ru",
			codec:        "subrip",
			wantLanguage: "rus",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "arabic - ar",
			suffix:       ".ar",
			codec:        "subrip",
			wantLanguage: "ara",
			wantIsForced: false,
			wantIsSDH:    false,
		},

		// Language detection - full names
		{
			name:         "full name - english",
			suffix:       ".english",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "full name - spanish",
			suffix:       ".spanish",
			codec:        "subrip",
			wantLanguage: "spa",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "full name - french",
			suffix:       ".french",
			codec:        "subrip",
			wantLanguage: "fra",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "full name - german",
			suffix:       ".german",
			codec:        "subrip",
			wantLanguage: "deu",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "full name - italian",
			suffix:       ".italian",
			codec:        "subrip",
			wantLanguage: "ita",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "full name - portuguese",
			suffix:       ".portuguese",
			codec:        "subrip",
			wantLanguage: "por",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "full name - japanese",
			suffix:       ".japanese",
			codec:        "ass",
			wantLanguage: "jpn",
			wantIsForced: false,
			wantIsSDH:    false,
		},

		// Flag detection - forced
		{
			name:         "forced flag",
			suffix:       ".en.forced",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    false,
		},
		{
			name:         "force flag",
			suffix:       ".en.force",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    false,
		},
		{
			name:         "forced before language",
			suffix:       ".forced.en",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    false,
		},

		// Flag detection - SDH/CC/HI
		{
			name:         "sdh flag",
			suffix:       ".en.sdh",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    true,
		},
		{
			name:         "cc flag",
			suffix:       ".en.cc",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    true,
		},
		{
			name:         "hi flag",
			suffix:       ".en.hi",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    true,
		},
		{
			name:         "sdh before language",
			suffix:       ".sdh.en",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    true,
		},

		// Flag detection - commentary
		{
			name:         "commentary flag",
			suffix:       ".en.commentary",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: false,
			wantIsSDH:    false,
			wantTitle:    "Commentary",
		},
		{
			name:         "commentary only",
			suffix:       ".commentary",
			codec:        "subrip",
			wantLanguage: "",
			wantIsForced: false,
			wantIsSDH:    false,
			wantTitle:    "Commentary",
		},

		// Multiple flags
		{
			name:         "forced and sdh",
			suffix:       ".en.forced.sdh",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    true,
		},
		{
			name:         "sdh and forced reversed",
			suffix:       ".en.sdh.forced",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    true,
		},
		{
			name:         "all flags",
			suffix:       ".spanish.forced.cc",
			codec:        "subrip",
			wantLanguage: "spa",
			wantIsForced: true,
			wantIsSDH:    true,
		},

		// Edge cases
		{
			name:         "empty suffix",
			suffix:       "",
			codec:        "subrip",
			wantLanguage: "",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "no leading dot",
			suffix:       "en.forced",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    false,
		},
		{
			name:         "unknown language",
			suffix:       ".xyz",
			codec:        "subrip",
			wantLanguage: "",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "case insensitive",
			suffix:       ".EN.FORCED",
			codec:        "subrip",
			wantLanguage: "eng",
			wantIsForced: true,
			wantIsSDH:    false,
		},

		// Additional languages coverage
		{
			name:         "dutch - nl",
			suffix:       ".nl",
			codec:        "subrip",
			wantLanguage: "nld",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "polish - pl",
			suffix:       ".pl",
			codec:        "subrip",
			wantLanguage: "pol",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "swedish - sv",
			suffix:       ".sv",
			codec:        "subrip",
			wantLanguage: "swe",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "norwegian - no",
			suffix:       ".no",
			codec:        "subrip",
			wantLanguage: "nor",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "danish - da",
			suffix:       ".da",
			codec:        "subrip",
			wantLanguage: "dan",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "finnish - fi",
			suffix:       ".fi",
			codec:        "subrip",
			wantLanguage: "fin",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "turkish - tr",
			suffix:       ".tr",
			codec:        "subrip",
			wantLanguage: "tur",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "greek - el",
			suffix:       ".el",
			codec:        "subrip",
			wantLanguage: "ell",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "hebrew - he",
			suffix:       ".he",
			codec:        "subrip",
			wantLanguage: "heb",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "hindi - hi (should be language not flag)",
			suffix:       ".hindi",
			codec:        "subrip",
			wantLanguage: "hin",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "thai - th",
			suffix:       ".th",
			codec:        "subrip",
			wantLanguage: "tha",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "vietnamese - vi",
			suffix:       ".vi",
			codec:        "subrip",
			wantLanguage: "vie",
			wantIsForced: false,
			wantIsSDH:    false,
		},
		{
			name:         "korean - ko",
			suffix:       ".ko",
			codec:        "subrip",
			wantLanguage: "kor",
			wantIsForced: false,
			wantIsSDH:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSuffix(tt.suffix, tt.codec)

			if got.Language != tt.wantLanguage {
				t.Errorf("Language: got %q, want %q", got.Language, tt.wantLanguage)
			}
			if got.IsForced != tt.wantIsForced {
				t.Errorf("IsForced: got %v, want %v", got.IsForced, tt.wantIsForced)
			}
			if got.IsSDH != tt.wantIsSDH {
				t.Errorf("IsSDH: got %v, want %v", got.IsSDH, tt.wantIsSDH)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("Title: got %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Codec != tt.codec {
				t.Errorf("Codec: got %q, want %q", got.Codec, tt.codec)
			}
		})
	}
}

func TestDiscoverExternal(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	// Create video file
	if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subtitle files
	subtitleFiles := []struct {
		name         string
		wantLanguage string
		wantIsForced bool
		wantIsSDH    bool
		wantCodec    string
	}{
		{"movie.en.srt", "eng", false, false, "subrip"},
		{"movie.en.forced.srt", "eng", true, false, "subrip"},
		{"movie.es.srt", "spa", false, false, "subrip"},
		{"movie.fr.sdh.srt", "fra", false, true, "subrip"},
		{"movie.de.ass", "deu", false, false, "ass"},
		{"movie.it.vtt", "ita", false, false, "webvtt"},
		{"movie.pt.sub", "por", false, false, "subviewer"},
		{"movie.japanese.ass", "jpn", false, false, "ass"},
		{"movie.english.forced.cc.srt", "eng", true, true, "subrip"},
		{"movie.srt", "", false, false, "subrip"}, // No language
	}

	for _, sf := range subtitleFiles {
		path := filepath.Join(tmpDir, sf.name)
		if err := os.WriteFile(path, []byte("fake subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create files that WILL match (because they start with "movie")
	matchingButUnwantedFiles := []string{
		"movie_trailer.en.srt", // Starts with "movie" so will match
		"movie2.en.srt",        // Starts with "movie" so will match
	}

	for _, name := range matchingButUnwantedFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("fake file"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create files that should NOT match
	nonMatchingFiles := []string{
		"other_movie.en.srt", // Different base name
		"movie.txt",          // Not a subtitle
		"movie.nfo",          // Metadata file
		"mov.en.srt",         // Shorter base name
		"amovie.en.srt",      // Different prefix
	}

	for _, name := range nonMatchingFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("fake file"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Run discovery
	subtitles := DiscoverExternal(videoPath)

	// Verify we found the expected subtitles + the ones that match by prefix
	expectedCount := len(subtitleFiles) + len(matchingButUnwantedFiles)
	if len(subtitles) != expectedCount {
		t.Errorf("Expected %d subtitles, got %d", expectedCount, len(subtitles))
	}

	// Create a map for easier verification
	subMap := make(map[string]External)
	for _, sub := range subtitles {
		name := filepath.Base(sub.FilePath)
		subMap[name] = sub
	}

	// Verify each expected subtitle
	for _, expected := range subtitleFiles {
		sub, found := subMap[expected.name]
		if !found {
			t.Errorf("Subtitle %q not found", expected.name)
			continue
		}

		if sub.Language != expected.wantLanguage {
			t.Errorf("%s: Language: got %q, want %q", expected.name, sub.Language, expected.wantLanguage)
		}
		if sub.IsForced != expected.wantIsForced {
			t.Errorf("%s: IsForced: got %v, want %v", expected.name, sub.IsForced, expected.wantIsForced)
		}
		if sub.IsSDH != expected.wantIsSDH {
			t.Errorf("%s: IsSDH: got %v, want %v", expected.name, sub.IsSDH, expected.wantIsSDH)
		}
		if sub.Codec != expected.wantCodec {
			t.Errorf("%s: Codec: got %q, want %q", expected.name, sub.Codec, expected.wantCodec)
		}
		if sub.FilePath != filepath.Join(tmpDir, expected.name) {
			t.Errorf("%s: FilePath: got %q, want %q", expected.name, sub.FilePath, filepath.Join(tmpDir, expected.name))
		}
	}

	// Verify non-matching files were not included
	for _, name := range nonMatchingFiles {
		if _, found := subMap[name]; found {
			t.Errorf("Non-matching file %q should not be included", name)
		}
	}
}

func TestDiscoverExternal_CaseInsensitivity(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "MyMovie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subtitle with different case
	subtitles := []string{
		"MyMovie.en.srt",
		"mymovie.fr.srt",
		"MYMOVIE.es.srt",
		"myMOVIE.de.ass",
	}

	for _, name := range subtitles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := DiscoverExternal(videoPath)

	if len(result) != len(subtitles) {
		t.Errorf("Expected %d subtitles, got %d", len(subtitles), len(result))
	}
}

func TestDiscoverExternal_NoSubtitles(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Only create non-subtitle files
	files := []string{"movie.nfo", "movie.jpg", "poster.png"}
	for _, name := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("file"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := DiscoverExternal(videoPath)

	if len(result) != 0 {
		t.Errorf("Expected 0 subtitles, got %d", len(result))
	}
}

func TestDiscoverExternal_NonExistentDirectory(t *testing.T) {
	videoPath := "/nonexistent/directory/movie.mkv"

	result := DiscoverExternal(videoPath)

	if len(result) != 0 {
		t.Errorf("Expected 0 subtitles for non-existent directory, got %d", len(result))
	}
}

func TestDiscoverInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create "Subs" subdirectory
	subsDir := filepath.Join(tmpDir, "Subs")
	if err := os.Mkdir(subsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create subtitles in Subs directory
	subtitlesInSubs := []struct {
		name         string
		wantLanguage string
		wantCodec    string
	}{
		{"movie.en.srt", "eng", "subrip"},
		{"movie.es.forced.srt", "spa", "subrip"},
		{"movie.fr.ass", "fra", "ass"},
	}

	for _, sf := range subtitlesInSubs {
		path := filepath.Join(subsDir, sf.name)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create "Subtitles" subdirectory
	subtitlesDir := filepath.Join(tmpDir, "Subtitles")
	if err := os.Mkdir(subtitlesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create subtitle in Subtitles directory
	path := filepath.Join(subtitlesDir, "movie.de.srt")
	if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DiscoverInSubdirectory(videoPath)

	expectedCount := 4 // 3 from Subs + 1 from Subtitles
	if len(result) != expectedCount {
		t.Errorf("Expected %d subtitles, got %d", expectedCount, len(result))
	}

	// Verify languages and codecs
	langCount := make(map[string]int)
	for _, sub := range result {
		if sub.Language != "" {
			langCount[sub.Language]++
		}
	}

	expectedLangs := map[string]int{
		"eng": 1,
		"spa": 1,
		"fra": 1,
		"deu": 1,
	}

	for lang, expectedCount := range expectedLangs {
		if langCount[lang] != expectedCount {
			t.Errorf("Expected %d subtitle(s) for language %q, got %d", expectedCount, lang, langCount[lang])
		}
	}
}

func TestDiscoverInSubdirectory_LanguageSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Subs directory with language subdirectories
	subsDir := filepath.Join(tmpDir, "Subs")
	if err := os.Mkdir(subsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create language subdirectories
	langDirs := []struct {
		dirName      string
		fileName     string
		wantLanguage string
	}{
		{"English", "movie.srt", "eng"},
		{"Spanish", "movie.srt", "spa"},
		{"french", "movie.srt", "fra"},
		{"ger", "movie.forced.srt", "deu"},
	}

	for _, ld := range langDirs {
		langDir := filepath.Join(subsDir, ld.dirName)
		if err := os.Mkdir(langDir, 0755); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(langDir, ld.fileName)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := DiscoverInSubdirectory(videoPath)

	if len(result) != len(langDirs) {
		t.Errorf("Expected %d subtitles, got %d", len(langDirs), len(result))
	}

	// Verify language detection from directory names
	langCount := make(map[string]int)
	for _, sub := range result {
		if sub.Language != "" {
			langCount[sub.Language]++
		}
	}

	expectedLangs := map[string]int{
		"eng": 1,
		"spa": 1,
		"fra": 1,
		"deu": 1,
	}

	for lang, expectedCount := range expectedLangs {
		if langCount[lang] != expectedCount {
			t.Errorf("Expected %d subtitle(s) for language %q, got %d", expectedCount, lang, langCount[lang])
		}
	}

	// Verify the forced flag is still parsed from filename
	forcedCount := 0
	for _, sub := range result {
		if sub.IsForced {
			forcedCount++
		}
	}

	if forcedCount != 1 {
		t.Errorf("Expected 1 forced subtitle, got %d", forcedCount)
	}
}

func TestDiscoverInSubdirectory_GenericFilenames(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Subs directory with language subdirectories
	subsDir := filepath.Join(tmpDir, "Subs")
	if err := os.Mkdir(subsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create English subdirectory with generic filenames
	englishDir := filepath.Join(subsDir, "English")
	if err := os.Mkdir(englishDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Generic filenames that don't match video name
	genericFiles := []string{"subtitle.srt", "forced.srt", "sdh.srt"}
	for _, name := range genericFiles {
		path := filepath.Join(englishDir, name)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := DiscoverInSubdirectory(videoPath)

	// Generic filenames should be accepted when in language subdirectories
	if len(result) != len(genericFiles) {
		t.Errorf("Expected %d subtitles, got %d", len(result), len(result))
	}

	// All should have English language from directory name
	for _, sub := range result {
		if sub.Language != "eng" {
			t.Errorf("Expected language 'eng', got %q", sub.Language)
		}
	}
}

func TestDiscoverInSubdirectory_NoSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DiscoverInSubdirectory(videoPath)

	if len(result) != 0 {
		t.Errorf("Expected 0 subtitles when no subdirectories exist, got %d", len(result))
	}
}

func TestMatchSubtitleFile(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		videoBaseLower string
		langHint       string
		wantMatch      bool
		wantLanguage   string
		wantCodec      string
	}{
		{
			name:           "exact match with language",
			filename:       "movie.en.srt",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      true,
			wantLanguage:   "eng",
			wantCodec:      "subrip",
		},
		{
			name:           "match with language hint",
			filename:       "subtitle.srt",
			videoBaseLower: "movie",
			langHint:       "English",
			wantMatch:      true,
			wantLanguage:   "eng",
			wantCodec:      "subrip",
		},
		{
			name:           "generic name no hint",
			filename:       "subtitle.srt",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      false,
		},
		{
			name:           "non-subtitle file",
			filename:       "movie.txt",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      false,
		},
		{
			name:           "different base name no hint",
			filename:       "othervideo.en.srt",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      false,
		},
		{
			name:           "case insensitive match",
			filename:       "MOVIE.en.srt",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      true,
			wantLanguage:   "eng",
			wantCodec:      "subrip",
		},
		{
			name:           "ass subtitle",
			filename:       "movie.ja.ass",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      true,
			wantLanguage:   "jpn",
			wantCodec:      "ass",
		},
		{
			name:           "vtt subtitle",
			filename:       "movie.es.vtt",
			videoBaseLower: "movie",
			langHint:       "",
			wantMatch:      true,
			wantLanguage:   "spa",
			wantCodec:      "webvtt",
		},
		{
			name:           "language from hint overrides empty",
			filename:       "movie.srt",
			videoBaseLower: "movie",
			langHint:       "Spanish",
			wantMatch:      true,
			wantLanguage:   "spa",
			wantCodec:      "subrip",
		},
		{
			name:           "language from filename takes precedence",
			filename:       "movie.en.srt",
			videoBaseLower: "movie",
			langHint:       "Spanish",
			wantMatch:      true,
			wantLanguage:   "eng", // Filename language, not hint
			wantCodec:      "subrip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchFile("/tmp", tt.filename, tt.videoBaseLower, tt.langHint)

			if tt.wantMatch {
				if result == nil {
					t.Fatal("Expected match, got nil")
				}
				if result.Language != tt.wantLanguage {
					t.Errorf("Language: got %q, want %q", result.Language, tt.wantLanguage)
				}
				if result.Codec != tt.wantCodec {
					t.Errorf("Codec: got %q, want %q", result.Codec, tt.wantCodec)
				}
				expectedPath := filepath.Join("/tmp", tt.filename)
				if result.FilePath != expectedPath {
					t.Errorf("FilePath: got %q, want %q", result.FilePath, expectedPath)
				}
			} else {
				if result != nil {
					t.Errorf("Expected no match, got %+v", result)
				}
			}
		})
	}
}

func TestDiscoverAll(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subtitles in same directory
	sameDirSubs := []string{
		"movie.en.srt",
		"movie.es.forced.srt",
	}
	for _, name := range sameDirSubs {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create Subs subdirectory with subtitles
	subsDir := filepath.Join(tmpDir, "Subs")
	if err := os.Mkdir(subsDir, 0755); err != nil {
		t.Fatal(err)
	}

	subDirSubs := []string{
		"movie.fr.srt",
		"movie.de.ass",
	}
	for _, name := range subDirSubs {
		path := filepath.Join(subsDir, name)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result := DiscoverAll(videoPath)

	expectedTotal := len(sameDirSubs) + len(subDirSubs)
	if len(result) != expectedTotal {
		t.Errorf("Expected %d total subtitles, got %d", expectedTotal, len(result))
	}

	// Verify we have subtitles from both locations
	sameDirCount := 0
	subDirCount := 0
	for _, sub := range result {
		if filepath.Dir(sub.FilePath) == tmpDir {
			sameDirCount++
		} else if filepath.Dir(sub.FilePath) == subsDir {
			subDirCount++
		}
	}

	if sameDirCount != len(sameDirSubs) {
		t.Errorf("Expected %d subtitles from same directory, got %d", len(sameDirSubs), sameDirCount)
	}
	if subDirCount != len(subDirSubs) {
		t.Errorf("Expected %d subtitles from subdirectory, got %d", len(subDirSubs), subDirCount)
	}
}

func TestSubtitleCodecExtensions(t *testing.T) {
	tests := []struct {
		ext       string
		wantCodec string
		isValid   bool
	}{
		{".srt", "subrip", true},
		{".ass", "ass", true},
		{".ssa", "ssa", true},
		{".vtt", "webvtt", true},
		{".sub", "subviewer", true},
		{".idx", "vobsub", true},
		{".sup", "hdmv_pgs_subtitle", true},
		{".txt", "", false},
		{".mkv", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			codec, found := extensions[tt.ext]

			if tt.isValid {
				if !found {
					t.Errorf("Extension %q should be valid", tt.ext)
				}
				if codec != tt.wantCodec {
					t.Errorf("Codec: got %q, want %q", codec, tt.wantCodec)
				}
			} else {
				if found {
					t.Errorf("Extension %q should not be valid", tt.ext)
				}
			}
		})
	}
}

func TestLanguagePatternsComprehensive(t *testing.T) {
	// Test that all defined language patterns map to valid ISO 639-2 codes
	tests := []struct {
		pattern  string
		wantCode string
	}{
		// English variants
		{"english", "eng"},
		{"eng", "eng"},
		{"en", "eng"},

		// Spanish variants
		{"spanish", "spa"},
		{"espanol", "spa"},
		{"español", "spa"},
		{"spa", "spa"},
		{"es", "spa"},

		// French variants
		{"french", "fra"},
		{"francais", "fra"},
		{"français", "fra"},
		{"fra", "fra"},
		{"fre", "fra"},
		{"fr", "fra"},

		// German variants
		{"german", "deu"},
		{"deutsch", "deu"},
		{"deu", "deu"},
		{"ger", "deu"},
		{"de", "deu"},

		// Italian variants
		{"italian", "ita"},
		{"italiano", "ita"},
		{"ita", "ita"},
		{"it", "ita"},

		// Portuguese variants
		{"portuguese", "por"},
		{"portugues", "por"},
		{"português", "por"},
		{"por", "por"},
		{"pt", "por"},

		// Additional languages
		{"russian", "rus"},
		{"japanese", "jpn"},
		{"chinese", "zho"},
		{"korean", "kor"},
		{"arabic", "ara"},
		{"dutch", "nld"},
		{"polish", "pol"},
		{"swedish", "swe"},
		{"norwegian", "nor"},
		{"danish", "dan"},
		{"finnish", "fin"},
		{"turkish", "tur"},
		{"greek", "ell"},
		{"hebrew", "heb"},
		{"hindi", "hin"},
		{"thai", "tha"},
		{"vietnamese", "vie"},
		{"indonesian", "ind"},
		{"czech", "ces"},
		{"hungarian", "hun"},
		{"romanian", "ron"},
		{"ukrainian", "ukr"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			code, found := languagePatterns[tt.pattern]
			if !found {
				t.Errorf("Pattern %q not found in languagePatterns", tt.pattern)
			}
			if code != tt.wantCode {
				t.Errorf("Pattern %q: got code %q, want %q", tt.pattern, code, tt.wantCode)
			}
		})
	}
}

func TestDiscoverExternal_ComplexFilenames(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with complex video filename
	videoPath := filepath.Join(tmpDir, "Movie.Name.2023.1080p.BluRay.x264-GROUP.mkv")
	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create matching subtitles
	subtitles := []string{
		"Movie.Name.2023.1080p.BluRay.x264-GROUP.en.srt",
		"Movie.Name.2023.1080p.BluRay.x264-GROUP.en.forced.srt",
		"Movie.Name.2023.1080p.BluRay.x264-GROUP.es.srt",
	}

	for _, name := range subtitles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("subtitle"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create non-matching subtitle
	nonMatching := filepath.Join(tmpDir, "Movie.Name.2023.720p.WEB.x264-OTHER.en.srt")
	if err := os.WriteFile(nonMatching, []byte("subtitle"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DiscoverExternal(videoPath)

	if len(result) != len(subtitles) {
		t.Errorf("Expected %d subtitles, got %d", len(subtitles), len(result))
	}

	// Verify the non-matching subtitle was excluded
	for _, sub := range result {
		if filepath.Base(sub.FilePath) == "Movie.Name.2023.720p.WEB.x264-OTHER.en.srt" {
			t.Error("Non-matching subtitle should not be included")
		}
	}
}

func TestDiscoverExternal_MultipleFlags(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subtitle with multiple flags
	subPath := filepath.Join(tmpDir, "movie.en.forced.sdh.srt")
	if err := os.WriteFile(subPath, []byte("subtitle"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DiscoverExternal(videoPath)

	if len(result) != 1 {
		t.Fatalf("Expected 1 subtitle, got %d", len(result))
	}

	sub := result[0]
	if sub.Language != "eng" {
		t.Errorf("Language: got %q, want 'eng'", sub.Language)
	}
	if !sub.IsForced {
		t.Error("IsForced should be true")
	}
	if !sub.IsSDH {
		t.Error("IsSDH should be true")
	}
}

func TestDiscoverInSubdirectory_NestedDirInLanguageDir(t *testing.T) {
	tmpDir := t.TempDir()
	videoPath := filepath.Join(tmpDir, "movie.mkv")

	if err := os.WriteFile(videoPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Subs/English with a nested directory (should be skipped)
	engDir := filepath.Join(tmpDir, "Subs", "English")
	if err := os.MkdirAll(engDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a nested directory inside language directory (should be skipped)
	nestedDir := filepath.Join(engDir, "nested_folder")
	if err := os.Mkdir(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a valid subtitle file
	subPath := filepath.Join(engDir, "movie.srt")
	if err := os.WriteFile(subPath, []byte("subtitle"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a subtitle inside nested dir (should NOT be found)
	nestedSub := filepath.Join(nestedDir, "movie.srt")
	if err := os.WriteFile(nestedSub, []byte("nested subtitle"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DiscoverInSubdirectory(videoPath)

	// Should only find the one in engDir, not the one in nestedDir
	if len(result) != 1 {
		t.Errorf("Expected 1 subtitle (ignoring nested), got %d", len(result))
	}

	if len(result) > 0 && result[0].FilePath != subPath {
		t.Errorf("Expected %s, got %s", subPath, result[0].FilePath)
	}
}

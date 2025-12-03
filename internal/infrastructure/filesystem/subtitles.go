package filesystem

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExternalSubtitle represents a discovered external subtitle file.
type ExternalSubtitle struct {
	FilePath string // Full path to subtitle file
	Language string // ISO 639-2 language code (eng, spa, fra, etc.)
	Title    string // Optional title/description
	IsForced bool   // Forced subtitles (translations of foreign dialogue)
	IsSDH    bool   // Subtitles for deaf/hard of hearing
	Codec    string // Subtitle format (subrip, ass, webvtt, etc.)
}

// subtitleExtensions maps file extensions to codec names
var subtitleExtensions = map[string]string{
	".srt": "subrip",
	".ass": "ass",
	".ssa": "ssa",
	".vtt": "webvtt",
	".sub": "subviewer", // Could also be MicroDVD, but subviewer is more common
	".idx": "vobsub",    // VobSub index file (paired with .sub)
	".sup": "hdmv_pgs_subtitle",
}

// languagePatterns maps common language indicators to ISO 639-2 codes
var languagePatterns = map[string]string{
	"english":    "eng",
	"eng":        "eng",
	"en":         "eng",
	"spanish":    "spa",
	"espanol":    "spa",
	"español":    "spa",
	"spa":        "spa",
	"es":         "spa",
	"french":     "fra",
	"francais":   "fra",
	"français":   "fra",
	"fra":        "fra",
	"fre":        "fra",
	"fr":         "fra",
	"german":     "deu",
	"deutsch":    "deu",
	"deu":        "deu",
	"ger":        "deu",
	"de":         "deu",
	"italian":    "ita",
	"italiano":   "ita",
	"ita":        "ita",
	"it":         "ita",
	"portuguese": "por",
	"portugues":  "por",
	"português":  "por",
	"por":        "por",
	"pt":         "por",
	"russian":    "rus",
	"rus":        "rus",
	"ru":         "rus",
	"japanese":   "jpn",
	"jpn":        "jpn",
	"ja":         "jpn",
	"jp":         "jpn",
	"chinese":    "zho",
	"chi":        "zho",
	"zho":        "zho",
	"zh":         "zho",
	"korean":     "kor",
	"kor":        "kor",
	"ko":         "kor",
	"arabic":     "ara",
	"ara":        "ara",
	"ar":         "ara",
	"dutch":      "nld",
	"nld":        "nld",
	"dut":        "nld",
	"nl":         "nld",
	"polish":     "pol",
	"pol":        "pol",
	"pl":         "pol",
	"swedish":    "swe",
	"swe":        "swe",
	"sv":         "swe",
	"norwegian":  "nor",
	"nor":        "nor",
	"no":         "nor",
	"danish":     "dan",
	"dan":        "dan",
	"da":         "dan",
	"finnish":    "fin",
	"fin":        "fin",
	"fi":         "fin",
	"turkish":    "tur",
	"tur":        "tur",
	"tr":         "tur",
	"greek":      "ell",
	"ell":        "ell",
	"gre":        "ell",
	"el":         "ell",
	"hebrew":     "heb",
	"heb":        "heb",
	"he":         "heb",
	"hindi":      "hin",
	"hin":        "hin",
	"hi":         "hin",
	"thai":       "tha",
	"tha":        "tha",
	"th":         "tha",
	"vietnamese": "vie",
	"vie":        "vie",
	"vi":         "vie",
	"indonesian": "ind",
	"ind":        "ind",
	"id":         "ind",
	"czech":      "ces",
	"ces":        "ces",
	"cze":        "ces",
	"cs":         "ces",
	"hungarian":  "hun",
	"hun":        "hun",
	"hu":         "hun",
	"romanian":   "ron",
	"ron":        "ron",
	"rum":        "ron",
	"ro":         "ron",
	"ukrainian":  "ukr",
	"ukr":        "ukr",
	"uk":         "ukr",
}

// subtitleTagPattern matches common subtitle file naming patterns
// Examples: movie.en.srt, movie.english.srt, movie.eng.forced.srt, movie.en.sdh.srt
var subtitleTagPattern = regexp.MustCompile(`(?i)\.([a-z]{2,3}|[a-z]+)(?:\.(forced|sdh|cc|hi|commentary))?(?:\.(forced|sdh|cc|hi|commentary))?$`)

// DiscoverExternalSubtitles finds external subtitle files for a given video file.
// It looks for files with the same base name but subtitle extensions in the same directory.
func DiscoverExternalSubtitles(videoPath string) []ExternalSubtitle {
	dir := filepath.Dir(videoPath)
	videoBase := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoBaseLower := strings.ToLower(videoBase)

	var subtitles []ExternalSubtitle

	entries, err := os.ReadDir(dir)
	if err != nil {
		return subtitles
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		codec, isSubtitle := subtitleExtensions[ext]
		if !isSubtitle {
			continue
		}

		// Check if this subtitle matches our video file
		nameLower := strings.ToLower(name)
		nameWithoutExt := strings.TrimSuffix(nameLower, ext)

		// Must start with video base name
		if !strings.HasPrefix(nameWithoutExt, videoBaseLower) {
			continue
		}

		// Parse the suffix for language and flags
		suffix := nameWithoutExt[len(videoBaseLower):]
		sub := parseSubtitleSuffix(suffix, codec)
		sub.FilePath = filepath.Join(dir, name)

		subtitles = append(subtitles, sub)
	}

	return subtitles
}

// parseSubtitleSuffix extracts language and flags from a subtitle filename suffix.
// Examples: ".en", ".english", ".eng.forced", ".en.sdh", ".french.hi"
func parseSubtitleSuffix(suffix string, codec string) ExternalSubtitle {
	sub := ExternalSubtitle{
		Codec: codec,
	}

	// Remove leading dot if present
	suffix = strings.TrimPrefix(suffix, ".")
	if suffix == "" {
		return sub
	}

	// Split by dots
	parts := strings.Split(strings.ToLower(suffix), ".")

	for _, part := range parts {
		// Check for flags
		switch part {
		case "forced", "force":
			sub.IsForced = true
			continue
		case "sdh", "cc", "hi":
			sub.IsSDH = true
			continue
		case "commentary":
			sub.Title = "Commentary"
			continue
		}

		// Check for language
		if lang, ok := languagePatterns[part]; ok {
			sub.Language = lang
		}
	}

	return sub
}

// DiscoverSubtitlesSubdirectory looks for subtitles in common subdirectories.
// Media servers often have subtitles in "Subs", "Subtitles", or "Sub" folders.
func DiscoverSubtitlesSubdirectory(videoPath string) []ExternalSubtitle {
	dir := filepath.Dir(videoPath)
	videoBase := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoBaseLower := strings.ToLower(videoBase)

	var subtitles []ExternalSubtitle

	// Common subtitle subdirectory names
	subDirs := []string{"Subs", "subs", "Subtitles", "subtitles", "Sub", "sub"}

	for _, subDir := range subDirs {
		subPath := filepath.Join(dir, subDir)
		entries, err := os.ReadDir(subPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				// Could be language-specific subdirectory (e.g., Subs/English/)
				langDir := filepath.Join(subPath, entry.Name())
				langEntries, err := os.ReadDir(langDir)
				if err != nil {
					continue
				}
				for _, langEntry := range langEntries {
					if langEntry.IsDir() {
						continue
					}
					if sub := matchSubtitleFile(langDir, langEntry.Name(), videoBaseLower, entry.Name()); sub != nil {
						subtitles = append(subtitles, *sub)
					}
				}
				continue
			}

			if sub := matchSubtitleFile(subPath, entry.Name(), videoBaseLower, ""); sub != nil {
				subtitles = append(subtitles, *sub)
			}
		}
	}

	return subtitles
}

// matchSubtitleFile checks if a file is a subtitle for the given video and returns it.
func matchSubtitleFile(dir, filename, videoBaseLower, langHint string) *ExternalSubtitle {
	ext := strings.ToLower(filepath.Ext(filename))
	codec, isSubtitle := subtitleExtensions[ext]
	if !isSubtitle {
		return nil
	}

	nameLower := strings.ToLower(filename)
	nameWithoutExt := strings.TrimSuffix(nameLower, ext)

	// Check if matches video base name
	if !strings.HasPrefix(nameWithoutExt, videoBaseLower) {
		// Also accept generic names in language subdirectories
		if langHint == "" {
			return nil
		}
	}

	suffix := ""
	if strings.HasPrefix(nameWithoutExt, videoBaseLower) {
		suffix = nameWithoutExt[len(videoBaseLower):]
	}

	sub := parseSubtitleSuffix(suffix, codec)
	sub.FilePath = filepath.Join(dir, filename)

	// Use directory name as language hint if not detected from filename
	if sub.Language == "" && langHint != "" {
		if lang, ok := languagePatterns[strings.ToLower(langHint)]; ok {
			sub.Language = lang
		}
	}

	return &sub
}

// DiscoverAllExternalSubtitles finds all external subtitles for a video file,
// checking both the same directory and common subtitle subdirectories.
func DiscoverAllExternalSubtitles(videoPath string) []ExternalSubtitle {
	// Start with same-directory subtitles
	subtitles := DiscoverExternalSubtitles(videoPath)

	// Add subdirectory subtitles
	subDirSubs := DiscoverSubtitlesSubdirectory(videoPath)
	subtitles = append(subtitles, subDirSubs...)

	return subtitles
}

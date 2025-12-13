package subtitles

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoverExternal finds external subtitle files for a given video file.
// It looks for files with the same base name but subtitle extensions in the same directory.
func DiscoverExternal(videoPath string) []External {
	dir := filepath.Dir(videoPath)
	videoBase := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoBaseLower := strings.ToLower(videoBase)

	var subs []External

	entries, err := os.ReadDir(dir)
	if err != nil {
		return subs
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		codec, isSubtitle := extensions[ext]
		if !isSubtitle {
			continue
		}

		nameLower := strings.ToLower(name)
		nameWithoutExt := strings.TrimSuffix(nameLower, ext)

		if !strings.HasPrefix(nameWithoutExt, videoBaseLower) {
			continue
		}

		suffix := nameWithoutExt[len(videoBaseLower):]
		sub := parseSuffix(suffix, codec)
		sub.FilePath = filepath.Join(dir, name)

		subs = append(subs, sub)
	}

	return subs
}

// DiscoverInSubdirectory looks for subtitles in common subdirectories.
// Media servers often have subtitles in "Subs", "Subtitles", or "Sub" folders.
func DiscoverInSubdirectory(videoPath string) []External {
	dir := filepath.Dir(videoPath)
	videoBase := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	videoBaseLower := strings.ToLower(videoBase)

	var subs []External

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
					if sub := matchFile(langDir, langEntry.Name(), videoBaseLower, entry.Name()); sub != nil {
						subs = append(subs, *sub)
					}
				}
				continue
			}

			if sub := matchFile(subPath, entry.Name(), videoBaseLower, ""); sub != nil {
				subs = append(subs, *sub)
			}
		}
	}

	return subs
}

// DiscoverAll finds all external subtitles for a video file,
// checking both the same directory and common subtitle subdirectories.
func DiscoverAll(videoPath string) []External {
	subs := DiscoverExternal(videoPath)
	subDirSubs := DiscoverInSubdirectory(videoPath)
	return append(subs, subDirSubs...)
}

// matchFile checks if a file is a subtitle for the given video and returns it.
func matchFile(dir, filename, videoBaseLower, langHint string) *External {
	ext := strings.ToLower(filepath.Ext(filename))
	codec, isSubtitle := extensions[ext]
	if !isSubtitle {
		return nil
	}

	nameLower := strings.ToLower(filename)
	nameWithoutExt := strings.TrimSuffix(nameLower, ext)

	if !strings.HasPrefix(nameWithoutExt, videoBaseLower) {
		if langHint == "" {
			return nil
		}
	}

	suffix := ""
	if strings.HasPrefix(nameWithoutExt, videoBaseLower) {
		suffix = nameWithoutExt[len(videoBaseLower):]
	}

	sub := parseSuffix(suffix, codec)
	sub.FilePath = filepath.Join(dir, filename)

	if sub.Language == "" && langHint != "" {
		if lang, ok := languagePatterns[strings.ToLower(langHint)]; ok {
			sub.Language = lang
		}
	}

	return &sub
}

// parseSuffix extracts language and flags from a subtitle filename suffix.
// Examples: ".en", ".english", ".eng.forced", ".en.sdh", ".french.hi"
func parseSuffix(suffix string, codec string) External {
	sub := External{
		Codec: codec,
	}

	suffix = strings.TrimPrefix(suffix, ".")
	if suffix == "" {
		return sub
	}

	parts := strings.Split(strings.ToLower(suffix), ".")

	for _, part := range parts {
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

		if lang, ok := languagePatterns[part]; ok {
			sub.Language = lang
		}
	}

	return sub
}

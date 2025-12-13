package subtitles

import "regexp"

// External represents a discovered external subtitle file.
type External struct {
	FilePath string // Full path to subtitle file
	Language string // ISO 639-2 language code (eng, spa, fra, etc.)
	Title    string // Optional title/description
	IsForced bool   // Forced subtitles (translations of foreign dialogue)
	IsSDH    bool   // Subtitles for deaf/hard of hearing
	Codec    string // Subtitle format (subrip, ass, webvtt, etc.)
}

// extensions maps file extensions to codec names
var extensions = map[string]string{
	".srt": "subrip",
	".ass": "ass",
	".ssa": "ssa",
	".vtt": "webvtt",
	".sub": "subviewer", // Could also be MicroDVD, but subviewer is more common
	".idx": "vobsub",    // VobSub index file (paired with .sub)
	".sup": "hdmv_pgs_subtitle",
}

// tagPattern matches common subtitle file naming patterns
// Examples: movie.en.srt, movie.english.srt, movie.eng.forced.srt, movie.en.sdh.srt
var tagPattern = regexp.MustCompile(`(?i)\.([a-z]{2,3}|[a-z]+)(?:\.(forced|sdh|cc|hi|commentary))?(?:\.(forced|sdh|cc|hi|commentary))?$`)

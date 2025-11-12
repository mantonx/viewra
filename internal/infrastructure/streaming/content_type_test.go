package streaming

import (
	"testing"
)

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     string
	}{
		// Video formats
		{"MP4 video", "movie.mp4", "video/mp4"},
		{"MKV video", "movie.mkv", "video/x-matroska"},
		{"WebM video", "movie.webm", "video/webm"},
		{"AVI video", "movie.avi", "video/x-msvideo"},
		{"MOV video", "movie.mov", "video/quicktime"},
		{"WMV video", "movie.wmv", "video/x-ms-wmv"},
		{"FLV video", "movie.flv", "video/x-flv"},
		{"M4V video", "movie.m4v", "video/x-m4v"},
		{"MPG video", "movie.mpg", "video/mpeg"},
		{"MPEG video", "movie.mpeg", "video/mpeg"},
		{"TS video", "movie.ts", "video/mp2t"},
		{"3GP video", "movie.3gp", "video/3gpp"},
		{"OGV video", "movie.ogv", "video/ogg"},

		// Audio formats
		{"MP3 audio", "song.mp3", "audio/mpeg"},
		{"FLAC audio", "song.flac", "audio/flac"},
		{"WAV audio", "song.wav", "audio/wav"},
		{"M4A audio", "song.m4a", "audio/mp4"},
		{"AAC audio", "song.aac", "audio/aac"},
		{"OGG audio", "song.ogg", "audio/ogg"},
		{"OPUS audio", "song.opus", "audio/opus"},
		{"WMA audio", "song.wma", "audio/x-ms-wma"},
		{"OGA audio", "song.oga", "audio/ogg"},
		// Note: WebM can be both video/audio, video takes precedence
		{"WebM file", "song.webm", "video/webm"},

		// Case insensitive
		{"Uppercase MP4", "MOVIE.MP4", "video/mp4"},
		{"Mixed case MKV", "Movie.MkV", "video/x-matroska"},

		// Full paths
		{"Path with MP4", "/path/to/movie.mp4", "video/mp4"},
		{"Complex path", "/videos/2023/vacation.mkv", "video/x-matroska"},

		// Unknown formats
		{"Unknown extension", "file.xyz", "application/octet-stream"},
		{"No extension", "file", "application/octet-stream"},
		{"Text file", "readme.txt", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectContentType(tt.fileName)
			if got != tt.want {
				t.Errorf("DetectContentType(%q) = %q, want %q", tt.fileName, got, tt.want)
			}
		})
	}
}

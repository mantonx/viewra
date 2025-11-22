package filesystem

import (
	"os"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() interface{}   { return nil }

func TestFilter_GetMediaType(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		want      scanner.MediaType
	}{
		// Video extensions
		{name: "mkv video", extension: ".mkv", want: scanner.MediaTypeMovie},
		{name: "mp4 video", extension: ".mp4", want: scanner.MediaTypeMovie},
		{name: "avi video", extension: ".avi", want: scanner.MediaTypeMovie},
		{name: "mov video", extension: ".mov", want: scanner.MediaTypeMovie},
		{name: "wmv video", extension: ".wmv", want: scanner.MediaTypeMovie},
		{name: "flv video", extension: ".flv", want: scanner.MediaTypeMovie},
		{name: "webm video", extension: ".webm", want: scanner.MediaTypeMovie},
		{name: "m4v video", extension: ".m4v", want: scanner.MediaTypeMovie},
		{name: "ts video", extension: ".ts", want: scanner.MediaTypeMovie},
		{name: "mpg video", extension: ".mpg", want: scanner.MediaTypeMovie},
		{name: "mpeg video", extension: ".mpeg", want: scanner.MediaTypeMovie},
		{name: "vob video", extension: ".vob", want: scanner.MediaTypeMovie},
		{name: "m2ts video", extension: ".m2ts", want: scanner.MediaTypeMovie},

		// Audio extensions
		{name: "mp3 audio", extension: ".mp3", want: scanner.MediaTypeTrack},
		{name: "flac audio", extension: ".flac", want: scanner.MediaTypeTrack},
		{name: "wav audio", extension: ".wav", want: scanner.MediaTypeTrack},
		{name: "m4a audio", extension: ".m4a", want: scanner.MediaTypeTrack},
		{name: "aac audio", extension: ".aac", want: scanner.MediaTypeTrack},
		{name: "ogg audio", extension: ".ogg", want: scanner.MediaTypeTrack},
		{name: "wma audio", extension: ".wma", want: scanner.MediaTypeTrack},
		{name: "opus audio", extension: ".opus", want: scanner.MediaTypeTrack},
		{name: "ape audio", extension: ".ape", want: scanner.MediaTypeTrack},
		{name: "alac audio", extension: ".alac", want: scanner.MediaTypeTrack},

		// Case insensitivity
		{name: "uppercase MKV", extension: ".MKV", want: scanner.MediaTypeMovie},
		{name: "mixed case Mp4", extension: ".Mp4", want: scanner.MediaTypeMovie},
		{name: "uppercase MP3", extension: ".MP3", want: scanner.MediaTypeTrack},
		{name: "mixed case FlAc", extension: ".FlAc", want: scanner.MediaTypeTrack},

		// Unknown extensions
		{name: "txt file", extension: ".txt", want: scanner.MediaTypeUnknown},
		{name: "nfo file", extension: ".nfo", want: scanner.MediaTypeUnknown},
		{name: "jpg image", extension: ".jpg", want: scanner.MediaTypeUnknown},
		{name: "pdf document", extension: ".pdf", want: scanner.MediaTypeUnknown},
		{name: "empty extension", extension: "", want: scanner.MediaTypeUnknown},
		{name: "no extension", extension: "noext", want: scanner.MediaTypeUnknown},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.GetMediaType(tt.extension)
			if got != tt.want {
				t.Errorf("GetMediaType(%q) = %v, want %v", tt.extension, got, tt.want)
			}
		})
	}
}

func TestFilter_IsMediaFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		// Video files
		{name: "mkv file", path: "/movies/movie.mkv", want: true},
		{name: "mp4 file", path: "/movies/video.mp4", want: true},
		{name: "avi file", path: "/videos/old.avi", want: true},

		// Audio files
		{name: "mp3 file", path: "/music/song.mp3", want: true},
		{name: "flac file", path: "/music/track.flac", want: true},
		{name: "wav file", path: "/audio/sound.wav", want: true},

		// Non-media files
		{name: "txt file", path: "/docs/readme.txt", want: false},
		{name: "nfo file", path: "/movies/movie.nfo", want: false},
		{name: "jpg file", path: "/images/poster.jpg", want: false},
		{name: "no extension", path: "/files/noext", want: false},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.IsMediaFile(tt.path)
			if got != tt.want {
				t.Errorf("IsMediaFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilter_ShouldProcess(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		fileInfo os.FileInfo
		want     bool
	}{
		// Valid media files
		{
			name: "valid mkv file",
			path: "/movies/Inception.mkv",
			fileInfo: mockFileInfo{
				name:  "Inception.mkv",
				size:  1024 * 1024 * 1024,
				isDir: false,
			},
			want: true,
		},
		{
			name: "valid mp3 file",
			path: "/music/song.mp3",
			fileInfo: mockFileInfo{
				name:  "song.mp3",
				size:  5 * 1024 * 1024,
				isDir: false,
			},
			want: true,
		},

		// Directories
		{
			name: "directory",
			path: "/movies/Season 01",
			fileInfo: mockFileInfo{
				name:  "Season 01",
				isDir: true,
			},
			want: false,
		},

		// Hidden files
		{
			name: "hidden file",
			path: "/movies/.hidden.mkv",
			fileInfo: mockFileInfo{
				name:  ".hidden.mkv",
				size:  1024,
				isDir: false,
			},
			want: false,
		},

		// Artwork files
		{
			name: "poster image",
			path: "/movies/poster.jpg",
			fileInfo: mockFileInfo{
				name:  "poster.jpg",
				size:  512 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "banner image",
			path: "/movies/banner.png",
			fileInfo: mockFileInfo{
				name:  "banner.png",
				size:  256 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "fanart image",
			path: "/movies/fanart.jpg",
			fileInfo: mockFileInfo{
				name:  "fanart.jpg",
				size:  1024 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "movie poster with dash",
			path: "/movies/Movie-poster.jpg",
			fileInfo: mockFileInfo{
				name:  "Movie-poster.jpg",
				size:  512 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "thumbnail",
			path: "/videos/thumb.jpg",
			fileInfo: mockFileInfo{
				name:  "thumb.jpg",
				size:  128 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "cover art",
			path: "/music/cover.jpg",
			fileInfo: mockFileInfo{
				name:  "cover.jpg",
				size:  256 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "folder image",
			path: "/movies/folder.jpg",
			fileInfo: mockFileInfo{
				name:  "folder.jpg",
				size:  512 * 1024,
				isDir: false,
			},
			want: false,
		},

		// Metadata files
		{
			name: "nfo file",
			path: "/movies/movie.nfo",
			fileInfo: mockFileInfo{
				name:  "movie.nfo",
				size:  2048,
				isDir: false,
			},
			want: false,
		},
		{
			name: "xml metadata",
			path: "/movies/metadata.xml",
			fileInfo: mockFileInfo{
				name:  "metadata.xml",
				size:  4096,
				isDir: false,
			},
			want: false,
		},
		{
			name: "subtitle srt",
			path: "/movies/movie.srt",
			fileInfo: mockFileInfo{
				name:  "movie.srt",
				size:  32 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "subtitle vtt",
			path: "/movies/movie.vtt",
			fileInfo: mockFileInfo{
				name:  "movie.vtt",
				size:  24 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "subtitle ass",
			path: "/anime/episode.ass",
			fileInfo: mockFileInfo{
				name:  "episode.ass",
				size:  16 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "cue file",
			path: "/music/album.cue",
			fileInfo: mockFileInfo{
				name:  "album.cue",
				size:  1024,
				isDir: false,
			},
			want: false,
		},

		// System files
		{
			name: "DS_Store",
			path: "/movies/.DS_Store",
			fileInfo: mockFileInfo{
				name:  ".DS_Store",
				size:  6148,
				isDir: false,
			},
			want: false,
		},
		{
			name: "Thumbs.db",
			path: "/movies/Thumbs.db",
			fileInfo: mockFileInfo{
				name:  "Thumbs.db",
				size:  10240,
				isDir: false,
			},
			want: false,
		},
		{
			name: "temp file",
			path: "/movies/video.tmp",
			fileInfo: mockFileInfo{
				name:  "video.tmp",
				size:  1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "backup file",
			path: "/movies/movie.mkv.bak",
			fileInfo: mockFileInfo{
				name:  "movie.mkv.bak",
				size:  1024 * 1024,
				isDir: false,
			},
			want: false,
		},
		{
			name: "desktop.ini",
			path: "/movies/desktop.ini",
			fileInfo: mockFileInfo{
				name:  "desktop.ini",
				size:  256,
				isDir: false,
			},
			want: false,
		},

		// Edge cases
		{
			name: "video with poster in filename (should be valid)",
			path: "/movies/The Poster Movie.mkv",
			fileInfo: mockFileInfo{
				name:  "The Poster Movie.mkv",
				size:  2 * 1024 * 1024 * 1024,
				isDir: false,
			},
			want: true, // "poster" in title is different from "poster." artwork pattern
		},
		{
			name: "case insensitive mkv",
			path: "/movies/Movie.MKV",
			fileInfo: mockFileInfo{
				name:  "Movie.MKV",
				size:  1024 * 1024 * 1024,
				isDir: false,
			},
			want: true,
		},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.ShouldProcess(tt.path, tt.fileInfo)
			if got != tt.want {
				t.Errorf("ShouldProcess(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFilter_isArtworkFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		// Artwork patterns
		{name: "poster", fileName: "poster.jpg", want: true},
		{name: "banner", fileName: "banner.png", want: true},
		{name: "thumb", fileName: "thumb.jpg", want: true},
		{name: "thumbnail", fileName: "thumbnail.jpg", want: true},
		{name: "cover", fileName: "cover.jpg", want: true},
		{name: "artwork", fileName: "artwork.png", want: true},
		{name: "fanart", fileName: "fanart.jpg", want: true},
		{name: "background", fileName: "background.jpg", want: true},
		{name: "backdrop", fileName: "backdrop.png", want: true},
		{name: "clearlogo", fileName: "clearlogo.png", want: true},
		{name: "clearart", fileName: "clearart.png", want: true},
		{name: "landscape", fileName: "landscape.jpg", want: true},
		{name: "disc", fileName: "disc.png", want: true},
		{name: "folder", fileName: "folder.jpg", want: true},
		{name: "albumart", fileName: "albumart.jpg", want: true},
		{name: "front", fileName: "front.jpg", want: true},
		{name: "back", fileName: "back.jpg", want: true},

		// Artwork with dash
		{name: "movie-poster", fileName: "movie-poster.jpg", want: true},
		{name: "show-banner", fileName: "show-banner.png", want: true},
		{name: "episode-thumb", fileName: "episode-thumb.jpg", want: true},

		// Season posters
		{name: "season01-poster", fileName: "season01-poster.jpg", want: true},
		{name: "season-poster", fileName: "season-poster.jpg", want: true},
		{name: "show-poster", fileName: "show-poster.jpg", want: true},

		// Image extensions (always artwork)
		{name: "random jpg", fileName: "movie.jpg", want: true},
		{name: "random jpeg", fileName: "file.jpeg", want: true},
		{name: "random png", fileName: "image.png", want: true},
		{name: "random gif", fileName: "animated.gif", want: true},
		{name: "random bmp", fileName: "bitmap.bmp", want: true},
		{name: "random webp", fileName: "modern.webp", want: true},

		// Non-artwork files
		{name: "video file", fileName: "movie.mkv", want: false},
		{name: "audio file", fileName: "song.mp3", want: false},
		{name: "text file", fileName: "readme.txt", want: false},
		{name: "nfo file", fileName: "movie.nfo", want: false},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.isArtworkFile(tt.fileName)
			if got != tt.want {
				t.Errorf("isArtworkFile(%q) = %v, want %v", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestFilter_isMetadataFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		// Metadata extensions
		{name: "nfo", fileName: "movie.nfo", want: true},
		{name: "xml", fileName: "metadata.xml", want: true},
		{name: "txt", fileName: "info.txt", want: true},
		{name: "info", fileName: "movie.info", want: true},
		{name: "cue", fileName: "album.cue", want: true},
		{name: "m3u", fileName: "playlist.m3u", want: true},
		{name: "m3u8", fileName: "playlist.m3u8", want: true},
		{name: "pls", fileName: "playlist.pls", want: true},

		// Subtitle extensions
		{name: "srt", fileName: "movie.srt", want: true},
		{name: "vtt", fileName: "movie.vtt", want: true},
		{name: "ass", fileName: "episode.ass", want: true},
		{name: "ssa", fileName: "episode.ssa", want: true},
		{name: "sub", fileName: "movie.sub", want: true},
		{name: "idx", fileName: "movie.idx", want: true},

		// Case insensitivity
		{name: "uppercase NFO", fileName: "MOVIE.NFO", want: true},
		{name: "mixed case Srt", fileName: "Movie.Srt", want: true},

		// Non-metadata files
		{name: "video", fileName: "movie.mkv", want: false},
		{name: "audio", fileName: "song.mp3", want: false},
		{name: "image", fileName: "poster.jpg", want: false},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.isMetadataFile(tt.fileName)
			if got != tt.want {
				t.Errorf("isMetadataFile(%q) = %v, want %v", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestFilter_isSystemFile(t *testing.T) {
	tests := []struct {
		name     string
		fileName string
		want     bool
	}{
		// System files
		{name: "DS_Store", fileName: ".ds_store", want: true},
		{name: "Thumbs.db", fileName: "thumbs.db", want: true},
		{name: "tmp file", fileName: "video.tmp", want: true},
		{name: "temp file", fileName: "data.temp", want: true},
		{name: "bak file", fileName: "movie.mkv.bak", want: true},
		{name: "backup file", fileName: "file.backup", want: true},
		{name: "old file", fileName: "version.old", want: true},
		{name: "orig file", fileName: "original.orig", want: true},
		{name: "desktop.ini", fileName: "desktop.ini", want: true},
		{name: ".directory", fileName: ".directory", want: true},

		// Case insensitivity
		{name: "uppercase THUMBS.DB", fileName: "THUMBS.DB", want: true},
		{name: "mixed case Desktop.Ini", fileName: "Desktop.Ini", want: true},

		// Non-system files
		{name: "video", fileName: "movie.mkv", want: false},
		{name: "audio", fileName: "song.mp3", want: false},
		{name: "document", fileName: "readme.txt", want: false},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.isSystemFile(tt.fileName)
			if got != tt.want {
				t.Errorf("isSystemFile(%q) = %v, want %v", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestFilter_isHidden(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "hidden file", path: "/path/.hidden", want: true},
		{name: "hidden dir", path: "/path/.config", want: true},
		{name: "DS_Store", path: "/movies/.DS_Store", want: true},
		{name: "hidden in path", path: "/home/.config/file.txt", want: false}, // Only checks basename
		{name: "normal file", path: "/path/file.txt", want: false},
		{name: "normal dir", path: "/path/Movies", want: false},
		{name: "dot in name", path: "/path/file.with.dots.txt", want: false},
	}

	filter := NewFilter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter.isHidden(tt.path)
			if got != tt.want {
				t.Errorf("isHidden(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

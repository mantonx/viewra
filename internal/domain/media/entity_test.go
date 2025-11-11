package media

import (
	"testing"
	"time"
)

func TestMedia_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		media   Media
		wantErr error
	}{
		{
			name: "valid media",
			media: Media{
				ID:        1,
				LibraryID: 1,
				Title:     "Test Movie",
				FilePath:  "movies/test.mp4",
				FileSize:  1024000,
				Duration:  7200,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "invalid library ID",
			media: Media{
				LibraryID: 0,
				Title:     "Test",
				FilePath:  "test.mp4",
			},
			wantErr: ErrInvalidLibraryID,
		},
		{
			name: "empty title",
			media: Media{
				LibraryID: 1,
				Title:     "",
				FilePath:  "test.mp4",
			},
			wantErr: ErrEmptyTitle,
		},
		{
			name: "title with only whitespace",
			media: Media{
				LibraryID: 1,
				Title:     "   ",
				FilePath:  "test.mp4",
			},
			wantErr: ErrEmptyTitle,
		},
		{
			name: "title too long",
			media: Media{
				LibraryID: 1,
				Title:     string(make([]byte, 501)),
				FilePath:  "test.mp4",
			},
			wantErr: ErrTitleTooLong,
		},
		{
			name: "empty file path",
			media: Media{
				LibraryID: 1,
				Title:     "Test",
				FilePath:  "",
			},
			wantErr: ErrEmptyFilePath,
		},
		{
			name: "absolute file path",
			media: Media{
				LibraryID: 1,
				Title:     "Test",
				FilePath:  "/absolute/path/test.mp4",
			},
			wantErr: ErrAbsoluteFilePath,
		},
		{
			name: "path traversal",
			media: Media{
				LibraryID: 1,
				Title:     "Test",
				FilePath:  "../../../etc/passwd",
			},
			wantErr: ErrPathTraversal,
		},
		{
			name: "missing file extension",
			media: Media{
				LibraryID: 1,
				Title:     "Test",
				FilePath:  "movies/test",
			},
			wantErr: ErrMissingFileExtension,
		},
		{
			name: "negative file size",
			media: Media{
				LibraryID: 1,
				Title:     "Test",
				FilePath:  "test.mp4",
				FileSize:  -100,
			},
			wantErr: ErrInvalidFileSize,
		},
		{
			name: "negative duration",
			media: Media{
				LibraryID: 1,
				Title:     "Test",
				FilePath:  "test.mp4",
				Duration:  -100,
			},
			wantErr: ErrInvalidDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.media.IsValid()
			if err != tt.wantErr {
				t.Errorf("Media.IsValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMedia_GetFileExtension(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "mp4 file",
			filePath: "movies/test.mp4",
			want:     "mp4",
		},
		{
			name:     "mkv file",
			filePath: "movies/test.MKV",
			want:     "mkv",
		},
		{
			name:     "file with multiple dots",
			filePath: "movies/test.2023.mp4",
			want:     "mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Media{FilePath: tt.filePath}
			if got := m.GetFileExtension(); got != tt.want {
				t.Errorf("Media.GetFileExtension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMedia_GetFileName(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "simple filename",
			filePath: "movies/test.mp4",
			want:     "test",
		},
		{
			name:     "filename with multiple dots",
			filePath: "movies/test.2023.mp4",
			want:     "test.2023",
		},
		{
			name:     "nested path",
			filePath: "movies/action/test.mp4",
			want:     "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Media{FilePath: tt.filePath}
			if got := m.GetFileName(); got != tt.want {
				t.Errorf("Media.GetFileName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		mediaType MediaType
		want      bool
	}{
		{
			name:      "valid movie type",
			mediaType: MediaTypeMovie,
			want:      true,
		},
		{
			name:      "valid TV type",
			mediaType: MediaTypeTV,
			want:      true,
		},
		{
			name:      "valid music type",
			mediaType: MediaTypeMusic,
			want:      true,
		},
		{
			name:      "invalid type",
			mediaType: MediaType("invalid"),
			want:      false,
		},
		{
			name:      "empty type",
			mediaType: MediaType(""),
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mediaType.IsValid(); got != tt.want {
				t.Errorf("MediaType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMovie_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		movie   Movie
		wantErr error
	}{
		{
			name: "valid movie",
			movie: Movie{
				Media: Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
				},
				Year:   2023,
				Rating: 7.5,
			},
			wantErr: nil,
		},
		{
			name: "invalid year (too low)",
			movie: Movie{
				Media: Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
				},
				Year: 1799,
			},
			wantErr: ErrInvalidYear,
		},
		{
			name: "invalid year (too high)",
			movie: Movie{
				Media: Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
				},
				Year: 2101,
			},
			wantErr: ErrInvalidYear,
		},
		{
			name: "year zero (acceptable)",
			movie: Movie{
				Media: Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
				},
				Year: 0,
			},
			wantErr: nil,
		},
		{
			name: "invalid rating (too high)",
			movie: Movie{
				Media: Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
				},
				Rating: 10.5,
			},
			wantErr: ErrInvalidRating,
		},
		{
			name: "invalid rating (negative)",
			movie: Movie{
				Media: Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
				},
				Rating: -1,
			},
			wantErr: ErrInvalidRating,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.movie.IsValid()
			if err != tt.wantErr {
				t.Errorf("Movie.IsValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTVEpisode_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		episode TVEpisode
		wantErr error
	}{
		{
			name: "valid episode",
			episode: TVEpisode{
				Media: Media{
					LibraryID: 1,
					Title:     "S01E01",
					FilePath:  "tv/show/s01e01.mp4",
				},
				ShowTitle: "Test Show",
				Season:    1,
				Episode:   1,
			},
			wantErr: nil,
		},
		{
			name: "season 0 (acceptable)",
			episode: TVEpisode{
				Media: Media{
					LibraryID: 1,
					Title:     "Special",
					FilePath:  "tv/show/special.mp4",
				},
				ShowTitle: "Test Show",
				Season:    0,
				Episode:   1,
			},
			wantErr: nil,
		},
		{
			name: "invalid season (negative)",
			episode: TVEpisode{
				Media: Media{
					LibraryID: 1,
					Title:     "Episode",
					FilePath:  "tv/show/episode.mp4",
				},
				Season:  -1,
				Episode: 1,
			},
			wantErr: ErrInvalidSeason,
		},
		{
			name: "invalid episode (zero)",
			episode: TVEpisode{
				Media: Media{
					LibraryID: 1,
					Title:     "Episode",
					FilePath:  "tv/show/episode.mp4",
				},
				Season:  1,
				Episode: 0,
			},
			wantErr: ErrInvalidEpisode,
		},
		{
			name: "invalid episode (negative)",
			episode: TVEpisode{
				Media: Media{
					LibraryID: 1,
					Title:     "Episode",
					FilePath:  "tv/show/episode.mp4",
				},
				Season:  1,
				Episode: -1,
			},
			wantErr: ErrInvalidEpisode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.episode.IsValid()
			if err != tt.wantErr {
				t.Errorf("TVEpisode.IsValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMusicTrack_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		track   MusicTrack
		wantErr error
	}{
		{
			name: "valid track",
			track: MusicTrack{
				Media: Media{
					LibraryID: 1,
					Title:     "Song Title",
					FilePath:  "music/artist/album/song.mp3",
				},
				Artist:      "Test Artist",
				Album:       "Test Album",
				TrackNumber: 1,
				Year:        2023,
			},
			wantErr: nil,
		},
		{
			name: "track number zero (acceptable)",
			track: MusicTrack{
				Media: Media{
					LibraryID: 1,
					Title:     "Song",
					FilePath:  "music/song.mp3",
				},
				TrackNumber: 0,
			},
			wantErr: nil,
		},
		{
			name: "invalid track number (negative)",
			track: MusicTrack{
				Media: Media{
					LibraryID: 1,
					Title:     "Song",
					FilePath:  "music/song.mp3",
				},
				TrackNumber: -1,
			},
			wantErr: ErrInvalidTrackNumber,
		},
		{
			name: "invalid year (negative)",
			track: MusicTrack{
				Media: Media{
					LibraryID: 1,
					Title:     "Song",
					FilePath:  "music/song.mp3",
				},
				Year: -1,
			},
			wantErr: ErrInvalidYear,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.track.IsValid()
			if err != tt.wantErr {
				t.Errorf("MusicTrack.IsValid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

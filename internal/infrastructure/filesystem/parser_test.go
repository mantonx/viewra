package filesystem

import (
	"testing"

	"github.com/viewra/viewra/internal/domain/scanner"
)

func TestParser_ParseMovie(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		filename string
		want     *scanner.MovieInfo
		wantNil  bool
	}{
		// Real filenames from /cifs/fictionalserver/movies
		{
			name:     "Inception with full metadata",
			filename: "Inception (2010) [imdbid-tt1375666] - [Remux-2160p][DTS-HD MA 5.1][HEVC]-4K4U.mkv",
			want: &scanner.MovieInfo{
				Title:      "Inception",
				Year:       2010,
				Resolution: "2160p",
				Quality:    "Remux",
			},
		},
		{
			name:     "12 Angry Men with HDR metadata",
			filename: "12 Angry Men (1957) [imdbid-tt0050083] - [Remux-2160p][DV HDR10][DTS-HD MA 2.0][HEVC]-HDH.mkv",
			want: &scanner.MovieInfo{
				Title:      "12 Angry Men",
				Year:       1957,
				Resolution: "2160p",
				Quality:    "Remux",
			},
		},
		{
			name:     "127 Hours 1080p",
			filename: "127 Hours (2010) [imdbid-tt1542344] - [Remux-1080p][DTS-HD MA 5.1][AVC]-playBD.mkv",
			want: &scanner.MovieInfo{
				Title:      "127 Hours",
				Year:       2010,
				Resolution: "1080p",
				Quality:    "Remux",
			},
		},
		{
			name: "2001 A Space Odyssey with dash",
			filename: "2001 A Space Odyssey (1968) [imdbid-tt0062622] - " +
				"[Remux-2160p Proper][DV HDR10][DTS-HD MA 5.1][HEVC]-FraMeSToR.mkv",
			want: &scanner.MovieInfo{
				Title:      "2001 A Space Odyssey",
				Year:       1968,
				Resolution: "2160p",
				Quality:    "Remux",
			},
		},
		{
			name:     "500 Days of Summer with parentheses",
			filename: "500 Days of Summer (2009) [imdbid-tt1022603] - [Remux-1080p][DTS-HD MA 5.1][AVC]-FraMeSToR.mkv",
			want: &scanner.MovieInfo{
				Title:      "500 Days of Summer",
				Year:       2009,
				Resolution: "1080p",
				Quality:    "Remux",
			},
		},
		{
			name:     "10 - number at start",
			filename: "10 (1979) [imdbid-tt0078721] - [Bluray-1080p][AC3 1.0][x264]-FHD.mkv",
			want: &scanner.MovieInfo{
				Title:      "10",
				Year:       1979,
				Resolution: "1080p",
				Quality:    "BluRay",
			},
		},
		{
			name: "10 Cloverfield Lane with 4K",
			filename: "10 Cloverfield Lane (2016) [imdbid-tt1179933] - " +
				"[Remux-2160p][HDR10][TrueHD Atmos 7.1][HEVC]-4K4U.mkv",
			want: &scanner.MovieInfo{
				Title:      "10 Cloverfield Lane",
				Year:       2016,
				Resolution: "2160p",
				Quality:    "Remux",
			},
		},
		{
			name:     "1917 with x265 codec",
			filename: "1917 (2019) [imdbid-tt8579674] - [Remux-2160p][HDR10][TrueHD Atmos 7.1][x265]-4K4U.mkv",
			want: &scanner.MovieInfo{
				Title:      "1917",
				Year:       2019,
				Resolution: "2160p",
				Quality:    "Remux",
			},
		},

		// Simple patterns (directory names)
		{
			name:     "Simple title and year",
			filename: "Inception (2010)",
			want: &scanner.MovieInfo{
				Title:      "Inception",
				Year:       2010,
				Resolution: "",
				Quality:    "",
			},
		},
		{
			name:     "Old movie",
			filename: "20,000 Leagues Under the Sea (1954)",
			want: &scanner.MovieInfo{
				Title:      "20,000 Leagues Under the Sea",
				Year:       1954,
				Resolution: "",
				Quality:    "",
			},
		},
		{
			name:     "Future movie",
			filename: "28 Years Later (2025)",
			want: &scanner.MovieInfo{
				Title: "28 Years Later",
				Year:  2025,
			},
		},

		// Edge cases
		{
			name:     "Multiple parentheses",
			filename: "(500) Days of Summer (2009).mkv",
			want: &scanner.MovieInfo{
				Title: "(500) Days of Summer",
				Year:  2009,
			},
		},
		{
			name:     "With path",
			filename: "/movies/Inception (2010)/Inception (2010).mkv",
			want: &scanner.MovieInfo{
				Title: "Inception",
				Year:  2010,
			},
		},

		// Invalid patterns (should return nil)
		{
			name:     "No year",
			filename: "Movie Without Year.mkv",
			wantNil:  true,
		},
		{
			name:     "Invalid year - too old",
			filename: "Ancient Movie (1899).mkv",
			wantNil:  true,
		},
		{
			name:     "Invalid year - too new",
			filename: "Future Movie (2100).mkv",
			wantNil:  true,
		},
		{
			name:     "Not a movie file",
			filename: "poster.jpg",
			wantNil:  true,
		},
		{
			name:     "Empty filename",
			filename: "",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseMovie(tt.filename)

			if err != nil {
				t.Errorf("ParseMovie() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseMovie() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("ParseMovie() = nil, want %+v", tt.want)
			}

			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Year != tt.want.Year {
				t.Errorf("Year = %d, want %d", got.Year, tt.want.Year)
			}
			if got.Resolution != tt.want.Resolution {
				t.Errorf("Resolution = %q, want %q", got.Resolution, tt.want.Resolution)
			}
			if got.Quality != tt.want.Quality {
				t.Errorf("Quality = %q, want %q", got.Quality, tt.want.Quality)
			}
		})
	}
}

func TestParser_ParseTVEpisode(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		filename string
		want     *scanner.TVEpisodeInfo
		wantNil  bool
	}{
		// Real filenames from /cifs/fictionalserver/tv
		{
			name:     "Breaking Bad S01E01",
			filename: "Breaking Bad (2008) - S01E01 - Pilot [Bluray-1080p][EAC3 5.1][x265]-iVy.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Breaking Bad",
				Year:         2008,
				Season:       1,
				Episode:      1,
				EpisodeTitle: "Pilot",
			},
		},
		{
			name:     "Breaking Bad S01E02",
			filename: "Breaking Bad (2008) - S01E02 - Cats in the Bag [Bluray-1080p][EAC3 5.1][x265]-iVy.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Breaking Bad",
				Year:         2008,
				Season:       1,
				Episode:      2,
				EpisodeTitle: "Cats in the Bag",
			},
		},
		{
			name:     "Breaking Bad S01E03 long title",
			filename: "Breaking Bad (2008) - S01E03 - And the Bags in the River [Bluray-1080p][EAC3 5.1][x265]-iVy.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Breaking Bad",
				Year:         2008,
				Season:       1,
				Episode:      3,
				EpisodeTitle: "And the Bags in the River",
			},
		},
		{
			name:     "1883 S01E01 - show name is number",
			filename: "1883 (2021) - S01E01 - 1883 [Bluray-1080p][AAC 5.1][x265]-Vyndros.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "1883",
				Year:         2021,
				Season:       1,
				Episode:      1,
				EpisodeTitle: "1883",
			},
		},
		{
			name:     "1883 S01E02 - episode title with special chars",
			filename: "1883 (2021) - S01E02 - Behind Us A Cliff [Bluray-1080p][AAC 5.1][x265]-Vyndros.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "1883",
				Year:         2021,
				Season:       1,
				Episode:      2,
				EpisodeTitle: "Behind Us A Cliff",
			},
		},
		{
			name:     "1899 S01E01 - 4K WEBRip",
			filename: "1899 (2022) - S01E01 - The Ship [WEBRip-2160p][DV HDR10][EAC3 Atmos 5.1][x265]-TrollUHD.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "1899",
				Year:         2022,
				Season:       1,
				Episode:      1,
				EpisodeTitle: "The Ship",
			},
		},
		{
			name:     "1923 S01E01 - WEBDL",
			filename: "1923 (2022) - S01E01 - 1923 [WEBDL-2160p][EAC3 5.1][x265]-GLHF.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "1923",
				Year:         2022,
				Season:       1,
				Episode:      1,
				EpisodeTitle: "1923",
			},
		},

		// Season 0 (specials)
		{
			name:     "Special episode S00E01",
			filename: "Show Name (2020) - S00E01 - Special Episode.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Show Name",
				Year:         2020,
				Season:       0,
				Episode:      1,
				EpisodeTitle: "Special Episode",
			},
		},

		// High season/episode numbers
		{
			name:     "Season 15 Episode 20",
			filename: "Long Running Show (2005) - S15E20 - Late Season Episode.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Long Running Show",
				Year:         2005,
				Season:       15,
				Episode:      20,
				EpisodeTitle: "Late Season Episode",
			},
		},

		// With path
		{
			name:     "With full path",
			filename: "/tv/Breaking Bad/Season 01/Breaking Bad (2008) - S01E01 - Pilot.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Breaking Bad",
				Year:         2008,
				Season:       1,
				Episode:      1,
				EpisodeTitle: "Pilot",
			},
		},

		// Invalid patterns (should return nil)
		{
			name:     "No season/episode",
			filename: "Show Name (2020) - Episode.mkv",
			wantNil:  true,
		},
		{
			name:     "Movie pattern (not TV)",
			filename: "Movie Name (2020).mkv",
			wantNil:  true,
		},
		{
			name:     "Invalid year",
			filename: "Show (1899) - S01E01 - Episode.mkv",
			wantNil:  true,
		},
		{
			name:     "Invalid season number - too high",
			filename: "Show (2020) - S100E01 - Episode.mkv",
			wantNil:  true,
		},
		{
			name:     "Empty filename",
			filename: "",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseTVEpisode(tt.filename)

			if err != nil {
				t.Errorf("ParseTVEpisode() unexpected error: %v", err)
				return
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseTVEpisode() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("ParseTVEpisode() = nil, want %+v", tt.want)
			}

			if got.ShowName != tt.want.ShowName {
				t.Errorf("ShowName = %q, want %q", got.ShowName, tt.want.ShowName)
			}
			if got.Year != tt.want.Year {
				t.Errorf("Year = %d, want %d", got.Year, tt.want.Year)
			}
			if got.Season != tt.want.Season {
				t.Errorf("Season = %d, want %d", got.Season, tt.want.Season)
			}
			if got.Episode != tt.want.Episode {
				t.Errorf("Episode = %d, want %d", got.Episode, tt.want.Episode)
			}
			if got.EpisodeTitle != tt.want.EpisodeTitle {
				t.Errorf("EpisodeTitle = %q, want %q", got.EpisodeTitle, tt.want.EpisodeTitle)
			}
		})
	}
}

func TestParser_ParseMusic(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		filename string
		want     *scanner.MusicInfo
	}{
		// Real filenames from /cifs/fictionalserver/music
		{
			name:     "Arcade Fire - full pattern",
			filename: "Arcade Fire - Funeral - 01 - Neighborhood #1 (Tunnels).flac",
			want: &scanner.MusicInfo{
				Artist:      "Arcade Fire",
				Album:       "Funeral",
				TrackNumber: 1,
				Title:       "Neighborhood #1 (Tunnels)",
			},
		},
		{
			name:     "Arcade Fire - with accents",
			filename: "Arcade Fire - Funeral - 02 - Neighborhood #2 (Laïka).flac",
			want: &scanner.MusicInfo{
				Artist:      "Arcade Fire",
				Album:       "Funeral",
				TrackNumber: 2,
				Title:       "Neighborhood #2 (Laïka)",
			},
		},
		{
			name:     "French title",
			filename: "Arcade Fire - Funeral - 03 - Une année sans lumière.flac",
			want: &scanner.MusicInfo{
				Artist:      "Arcade Fire",
				Album:       "Funeral",
				TrackNumber: 3,
				Title:       "Une année sans lumière",
			},
		},
		{
			name:     "Track with special chars in title",
			filename: "Arcade Fire - Funeral - 04 - Neighborhood #3 (Power Out).flac",
			want: &scanner.MusicInfo{
				Artist:      "Arcade Fire",
				Album:       "Funeral",
				TrackNumber: 4,
				Title:       "Neighborhood #3 (Power Out)",
			},
		},
		{
			name:     "Simple pattern - track and title only",
			filename: "01 - Track Name.mp3",
			want: &scanner.MusicInfo{
				TrackNumber: 1,
				Title:       "Track Name",
			},
		},
		{
			name:     "Two digit track",
			filename: "Artist - Album - 10 - Track Title.mp3",
			want: &scanner.MusicInfo{
				Artist:      "Artist",
				Album:       "Album",
				TrackNumber: 10,
				Title:       "Track Title",
			},
		},
		{
			name:     "Multiple dashes in title",
			filename: "Artist - Album - 05 - Title - With - Dashes.flac",
			want: &scanner.MusicInfo{
				Artist:      "Artist",
				Album:       "Album",
				TrackNumber: 5,
				Title:       "Title - With - Dashes",
			},
		},

		// Fallback to filename
		{
			name:     "No pattern - use filename",
			filename: "Random Song Name.mp3",
			want: &scanner.MusicInfo{
				Title: "Random Song Name",
			},
		},
		{
			name:     "With path",
			filename: "/music/artist/album/01 - Track.mp3",
			want: &scanner.MusicInfo{
				TrackNumber: 1,
				Title:       "Track",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseMusic(tt.filename)

			if err != nil {
				t.Errorf("ParseMusic() unexpected error: %v", err)
				return
			}

			if got == nil {
				t.Fatalf("ParseMusic() = nil, want %+v", tt.want)
			}

			if got.Artist != tt.want.Artist {
				t.Errorf("Artist = %q, want %q", got.Artist, tt.want.Artist)
			}
			if got.Album != tt.want.Album {
				t.Errorf("Album = %q, want %q", got.Album, tt.want.Album)
			}
			if got.TrackNumber != tt.want.TrackNumber {
				t.Errorf("TrackNumber = %d, want %d", got.TrackNumber, tt.want.TrackNumber)
			}
			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
		})
	}
}

func TestParser_ParseMusic_WithID3Tags(t *testing.T) {
	// This test would require creating actual audio files with ID3 tags
	// For now, we'll skip it and rely on filename parsing tests
	t.Skip("ID3 tag testing requires real audio files with tags")
}

// TestParser_ParseMovie_EdgeCases tests additional edge cases for movie parsing
func TestParser_ParseMovie_EdgeCases(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		filename string
		want     *scanner.MovieInfo
	}{
		{
			name:     "4K resolution normalization",
			filename: "Movie Title (2020) [4K].mkv",
			want: &scanner.MovieInfo{
				Title:      "Movie Title",
				Year:       2020,
				Resolution: "2160p",
			},
		},
		{
			name:     "8K resolution normalization",
			filename: "Future Film (2025) [8K].mkv",
			want: &scanner.MovieInfo{
				Title:      "Future Film",
				Year:       2025,
				Resolution: "4320p",
			},
		},
		{
			name:     "WEBRip quality",
			filename: "Web Movie (2021) [WEBRip-1080p].mkv",
			want: &scanner.MovieInfo{
				Title:      "Web Movie",
				Year:       2021,
				Resolution: "1080p",
				Quality:    "WEBRip",
			},
		},
		{
			name:     "WEBDL quality normalization",
			filename: "Stream Film (2022) [WEBDL-720p].mkv",
			want: &scanner.MovieInfo{
				Title:      "Stream Film",
				Year:       2022,
				Resolution: "720p",
				Quality:    "WEB-DL",
			},
		},
		{
			name:     "WEB-DL with hyphen",
			filename: "Another Stream (2023) [WEB-DL-1080p].mkv",
			want: &scanner.MovieInfo{
				Title:      "Another Stream",
				Year:       2023,
				Resolution: "1080p",
				Quality:    "WEB-DL",
			},
		},
		{
			name:     "480p resolution",
			filename: "Old Movie (2000) [480p].avi",
			want: &scanner.MovieInfo{
				Title:      "Old Movie",
				Year:       2000,
				Resolution: "480p",
			},
		},
		{
			name:     "720p resolution",
			filename: "HD Movie (2015) [720p].mp4",
			want: &scanner.MovieInfo{
				Title:      "HD Movie",
				Year:       2015,
				Resolution: "720p",
			},
		},
		{
			name:     "HDTV quality",
			filename: "TV Recording (2019) [HDTV-1080p].ts",
			want: &scanner.MovieInfo{
				Title:      "TV Recording",
				Year:       2019,
				Resolution: "1080p",
				Quality:    "HDTV",
			},
		},
		{
			name:     "DVDRip quality",
			filename: "DVD Movie (2005) [DVDRip].avi",
			want: &scanner.MovieInfo{
				Title:   "DVD Movie",
				Year:    2005,
				Quality: "DVDRip",
			},
		},
		{
			name:     "BDRip quality",
			filename: "Blu-ray Rip (2018) [BDRip-1080p].mkv",
			want: &scanner.MovieInfo{
				Title:      "Blu-ray Rip",
				Year:       2018,
				Resolution: "1080p",
				Quality:    "BDRip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseMovie(tt.filename)

			if err != nil {
				t.Errorf("ParseMovie() unexpected error: %v", err)
				return
			}

			if got == nil {
				t.Fatalf("ParseMovie() = nil, want %+v", tt.want)
			}

			if got.Title != tt.want.Title {
				t.Errorf("Title = %q, want %q", got.Title, tt.want.Title)
			}
			if got.Year != tt.want.Year {
				t.Errorf("Year = %d, want %d", got.Year, tt.want.Year)
			}
			if got.Resolution != tt.want.Resolution {
				t.Errorf("Resolution = %q, want %q", got.Resolution, tt.want.Resolution)
			}
			if got.Quality != tt.want.Quality {
				t.Errorf("Quality = %q, want %q", got.Quality, tt.want.Quality)
			}
		})
	}
}

// TestParser_ParseTVEpisode_EdgeCases tests additional edge cases for TV episode parsing
func TestParser_ParseTVEpisode_EdgeCases(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		filename string
		want     *scanner.TVEpisodeInfo
	}{
		{
			name:     "Multi-episode file S01E01E02",
			filename: "Show Name (2020) - S01E01E02 - Double Episode.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Show Name",
				Year:         2020,
				Season:       1,
				Episode:      1,
				EpisodeEnd:   2,
				EpisodeTitle: "Double Episode",
			},
		},
		{
			name:     "Multi-episode file S02E10E11",
			filename: "Another Show (2021) - S02E10E11 - Two Parter.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Another Show",
				Year:         2021,
				Season:       2,
				Episode:      10,
				EpisodeEnd:   11,
				EpisodeTitle: "Two Parter",
			},
		},
		{
			name:     "Multi-episode with invalid end (same as start)",
			filename: "Bad Multi (2022) - S01E05E05 - Invalid.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Bad Multi",
				Year:         2022,
				Season:       1,
				Episode:      5,
				EpisodeEnd:   0, // Should not set if end <= start
				EpisodeTitle: "Invalid",
			},
		},
		{
			name:     "Multi-episode with invalid end (lower than start)",
			filename: "Bad Show (2023) - S01E10E05 - Backwards.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Bad Show",
				Year:         2023,
				Season:       1,
				Episode:      10,
				EpisodeEnd:   0, // Should not set if end < start
				EpisodeTitle: "Backwards",
			},
		},
		{
			name:     "Multi-episode with max valid 2-digit range",
			filename: "Range Show (2024) - S01E10E99 - Max Range.mkv",
			want: &scanner.TVEpisodeInfo{
				ShowName:     "Range Show",
				Year:         2024,
				Season:       1,
				Episode:      10,
				EpisodeEnd:   99, // Should set if valid 2-digit
				EpisodeTitle: "Max Range",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseTVEpisode(tt.filename)

			if err != nil {
				t.Errorf("ParseTVEpisode() unexpected error: %v", err)
				return
			}

			if got == nil {
				t.Fatalf("ParseTVEpisode() = nil, want %+v", tt.want)
			}

			if got.ShowName != tt.want.ShowName {
				t.Errorf("ShowName = %q, want %q", got.ShowName, tt.want.ShowName)
			}
			if got.Year != tt.want.Year {
				t.Errorf("Year = %d, want %d", got.Year, tt.want.Year)
			}
			if got.Season != tt.want.Season {
				t.Errorf("Season = %d, want %d", got.Season, tt.want.Season)
			}
			if got.Episode != tt.want.Episode {
				t.Errorf("Episode = %d, want %d", got.Episode, tt.want.Episode)
			}
			if got.EpisodeEnd != tt.want.EpisodeEnd {
				t.Errorf("EpisodeEnd = %d, want %d", got.EpisodeEnd, tt.want.EpisodeEnd)
			}
			if got.EpisodeTitle != tt.want.EpisodeTitle {
				t.Errorf("EpisodeTitle = %q, want %q", got.EpisodeTitle, tt.want.EpisodeTitle)
			}
		})
	}
}

// TestDefaultFileSystem tests the FileSystem wrapper functions
func TestDefaultFileSystem(t *testing.T) {
	fs := &DefaultFileSystem{}

	// Create a temp directory for testing
	tmpDir := t.TempDir()

	// Test Stat
	_, err := fs.Stat(tmpDir)
	if err != nil {
		t.Errorf("Stat() unexpected error: %v", err)
	}

	// Test ReadDir
	_, err = fs.ReadDir(tmpDir)
	if err != nil {
		t.Errorf("ReadDir() unexpected error: %v", err)
	}
}

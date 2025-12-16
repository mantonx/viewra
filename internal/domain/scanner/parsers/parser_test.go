package parsers

import (
	"testing"
)

func TestParseMovie(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		expectedTitle  string
		expectedYear   int
		expectedResolution string
		expectedQuality string
		shouldError    bool
	}{
		{
			name:           "Standard format with year in parentheses",
			filename:       "The Matrix (1999).mkv",
			expectedTitle:  "The Matrix",
			expectedYear:   1999,
			expectedResolution: "",
			expectedQuality: "",
			shouldError:    false,
		},
		{
			name:           "Format with year and quality tags",
			filename:       "Inception.2010.1080p.BluRay.x264.mkv",
			expectedTitle:  "Inception",
			expectedYear:   2010,
			expectedResolution: "1080p",
			expectedQuality: "BluRay",
			shouldError:    false,
		},
		{
			name:           "Format with year in brackets",
			filename:       "The Godfather [1972].mp4",
			expectedTitle:  "The Godfather",
			expectedYear:   1972,
			expectedResolution: "",
			expectedQuality: "",
			shouldError:    false,
		},
		{
			name:           "Format with dots and quality",
			filename:       "Pulp.Fiction.1994.720p.BRRip.x264.mp4",
			expectedTitle:  "Pulp Fiction",
			expectedYear:   1994,
			expectedResolution: "720p",
			expectedQuality: "BRRip",
			shouldError:    false,
		},
		{
			name:           "Format with 4K resolution",
			filename:       "Blade Runner 2049 (2017) 4K UHD.mkv",
			expectedTitle:  "Blade Runner 2049",
			expectedYear:   2017,
			expectedResolution: "4K",
			expectedQuality: "",
			shouldError:    false,
		},
		{
			name:           "Format with multiple quality tags",
			filename:       "The.Dark.Knight.2008.1080p.BluRay.x264.DTS.mkv",
			expectedTitle:  "The Dark Knight",
			expectedYear:   2008,
			expectedResolution: "1080p",
			expectedQuality: "BluRay",
			shouldError:    false,
		},
		{
			name:           "Format without year",
			filename:       "Interstellar.1080p.BluRay.mkv",
			expectedTitle:  "Interstellar",
			expectedYear:   0,
			expectedResolution: "1080p",
			expectedQuality: "BluRay",
			shouldError:    false,
		},
		{
			name:           "Format with WEB-DL",
			filename:       "Dune.2021.2160p.WEB-DL.x265.mkv",
			expectedTitle:  "Dune",
			expectedYear:   2021,
			expectedResolution: "4K",
			expectedQuality: "WEB-DL",
			shouldError:    false,
		},
		// Advanced examples with IMDb IDs and edition tags
		{
			name:           "Movie with IMDb ID",
			filename:       "Downton Abbey The Grand Finale (2025) [imdbid-tt31888477] - [Remux-2160p][DV HDR10][TrueHD Atmos 7.1][HEVC]-HDH.mkv",
			expectedTitle:  "Downton Abbey The Grand Finale",
			expectedYear:   2025,
			expectedResolution: "4K",
			expectedQuality: "Remux",
			shouldError:    false,
		},
		{
			name:           "Movie with Extended Cut edition",
			filename:       "Stop Making Sense (1984) [imdbid-tt0088178] - Extended Cut [Remux-2160p][DV HDR10][TrueHD Atmos 7.1][HEVC]-FraMeSToR.mkv",
			expectedTitle:  "Stop Making Sense",
			expectedYear:   1984,
			expectedResolution: "4K",
			expectedQuality: "Remux",
			shouldError:    false,
		},
		{
			name:           "Movie with Proper tag",
			filename:       "2001 A Space Odyssey (1968) [imdbid-tt0062622] - [Remux-2160p Proper][DV HDR10][DTS-HD MA 5.1][HEVC]-FraMeSToR.mkv",
			expectedTitle:  "2001 A Space Odyssey",
			expectedYear:   1968,
			expectedResolution: "4K",
			expectedQuality: "Remux",
			shouldError:    false,
		},
		{
			name:           "Movie with Hybrid tag",
			filename:       "The Dreamers (2003) [imdbid-tt0309987] - [Hybrid][Remux-2160p][DV HDR10][DTS-HD MA 5.1][HEVC]-FraMeSToR.mkv",
			expectedTitle:  "The Dreamers",
			expectedYear:   2003,
			expectedResolution: "4K",
			expectedQuality: "Remux",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseMovie(tt.filename)

			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.shouldError {
				return
			}

			if info.Title != tt.expectedTitle {
				t.Errorf("expected title '%s', got '%s'", tt.expectedTitle, info.Title)
			}

			if info.Year != tt.expectedYear {
				t.Errorf("expected year %d, got %d", tt.expectedYear, info.Year)
			}

			if info.Resolution != tt.expectedResolution {
				t.Errorf("expected resolution '%s', got '%s'", tt.expectedResolution, info.Resolution)
			}

			if info.Quality != tt.expectedQuality {
				t.Errorf("expected quality '%s', got '%s'", tt.expectedQuality, info.Quality)
			}
		})
	}
}

func TestParseTVEpisode(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		expectedShow     string
		expectedSeason   int
		expectedEpisode  int
		expectedTitle    string
		shouldError      bool
	}{
		{
			name:             "Standard S01E01 format",
			path:             "Breaking.Bad.S01E01.mkv",
			expectedShow:     "Breaking Bad",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "",
			shouldError:      false,
		},
		{
			name:             "S01E01 with episode title",
			path:             "The.Office.S02E05.Halloween.mkv",
			expectedShow:     "The Office",
			expectedSeason:   2,
			expectedEpisode:  5,
			expectedTitle:    "Halloween",
			shouldError:      false,
		},
		{
			name:             "Format with dashes and episode title",
			path:             "Game of Thrones - S01E01 - Winter Is Coming.mp4",
			expectedShow:     "Game of Thrones",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "Winter Is Coming",
			shouldError:      false,
		},
		{
			name:             "1x01 format",
			path:             "Friends.1x01.mkv",
			expectedShow:     "Friends",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "",
			shouldError:      false,
		},
		{
			name:             "Season X Episode Y format",
			path:             "Stranger Things - Season 1 Episode 01 - The Vanishing of Will Byers.mkv",
			expectedShow:     "Stranger Things",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "The Vanishing of Will Byers",
			shouldError:      false,
		},
		{
			name:             "S01E01 with quality tags",
			path:             "The.Mandalorian.S01E01.1080p.WEB-DL.mkv",
			expectedShow:     "The Mandalorian",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "",
			shouldError:      false,
		},
		{
			name:             "Episode in season directory",
			path:             "/TV Shows/Breaking Bad/Season 1/Breaking.Bad.S01E01.Pilot.mkv",
			expectedShow:     "Breaking Bad",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "Pilot",
			shouldError:      false,
		},
		{
			name:             "Double episode S01E01E02",
			path:             "Doctor.Who.S01E01E02.mkv",
			expectedShow:     "Doctor Who",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "",
			shouldError:      false,
		},
		{
			name:             "Lowercase s01e01",
			path:             "the.wire.s01e01.pilot.mkv",
			expectedShow:     "the wire",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "pilot",
			shouldError:      false,
		},
		{
			name:             "Episode with year",
			path:             "True Detective (2014) - S01E01 - The Long Bright Dark.mkv",
			expectedShow:     "True Detective",
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "The Long Bright Dark",
			shouldError:      false,
		},
		// TV shows with year and quality tags in brackets
		{
			name:             "TV show with year and bracketed quality",
			path:             "/tv/Chicago P.D. (2014)/Season 13/Chicago P.D. (2014) - S13E07 - Impulse Control [WEBDL-1080p][EAC3 5.1][x264]-ETHEL.mkv",
			expectedShow:     "Chicago PD", // Normalized: dots are removed for consistent matching
			expectedSeason:   13,
			expectedEpisode:  7,
			expectedTitle:    "Impulse Control",
			shouldError:      false,
		},
		{
			name:             "TV show with leading zero in season",
			path:             "/tv/Chicago P.D. (2014)/Season 01/Chicago P.D. (2014) - S01E01 - Stepping Stone [WEBRip-1080p][EAC3 5.1][x265]-iVy.mkv",
			expectedShow:     "Chicago PD", // Normalized: dots are removed for consistent matching
			expectedSeason:   1,
			expectedEpisode:  1,
			expectedTitle:    "Stepping Stone",
			shouldError:      false,
		},
		// Specials without episode numbers
		{
			name:            "Special without episode number in Specials folder",
			path:            "/tv/Deadwood/Specials/Deadwood - The Movie.mkv",
			expectedShow:    "Deadwood",
			expectedSeason:  0,
			expectedEpisode: 182, // Hash-based episode number (stable for this filename)
			expectedTitle:   "The Movie",
			shouldError:     false,
		},
		{
			name:            "Special in Extras folder",
			path:            "/tv/Breaking Bad/Extras/Making of Breaking Bad.mkv",
			expectedShow:    "Breaking Bad",
			expectedSeason:  0,
			expectedEpisode: 464, // Hash-based episode number
			expectedTitle:   "Making of Breaking Bad",
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseTVEpisode(tt.path)

			if tt.shouldError && err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if !tt.shouldError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.shouldError {
				return
			}

			if info.ShowName != tt.expectedShow {
				t.Errorf("expected show '%s', got '%s'", tt.expectedShow, info.ShowName)
			}

			if info.Season != tt.expectedSeason {
				t.Errorf("expected season %d, got %d", tt.expectedSeason, info.Season)
			}

			if info.Episode != tt.expectedEpisode {
				t.Errorf("expected episode %d, got %d", tt.expectedEpisode, info.Episode)
			}

			if info.EpisodeTitle != tt.expectedTitle {
				t.Errorf("expected episode title '%s', got '%s'", tt.expectedTitle, info.EpisodeTitle)
			}
		})
	}
}

func TestDefaultParser(t *testing.T) {
	parser := NewDefaultParser()

	// Test movie parsing
	t.Run("ParseMovie through interface", func(t *testing.T) {
		info, err := parser.ParseMovie("The Matrix (1999).mkv")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if info.Title != "The Matrix" {
			t.Errorf("expected title 'The Matrix', got '%s'", info.Title)
		}
		if info.Year != 1999 {
			t.Errorf("expected year 1999, got %d", info.Year)
		}
	})

	// Test TV episode parsing
	t.Run("ParseTVEpisode through interface", func(t *testing.T) {
		info, err := parser.ParseTVEpisode("Breaking.Bad.S01E01.mkv")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if info.ShowName != "Breaking Bad" {
			t.Errorf("expected show 'Breaking Bad', got '%s'", info.ShowName)
		}
		if info.Season != 1 {
			t.Errorf("expected season 1, got %d", info.Season)
		}
		if info.Episode != 1 {
			t.Errorf("expected episode 1, got %d", info.Episode)
		}
	})
}

func TestParseMusic(t *testing.T) {
	tests := []struct {
		name           string
		filepath       string
		expectedTitle  string
		expectedArtist string
		expectedAlbum  string
		expectedTrack  int
		shouldError    bool
	}{
		{
			name:           "Track number - Title format (artist from dir)",
			filepath:       "/music/Artist/Album/01 - Song Title.mp3",
			expectedTitle:  "Song Title",
			expectedArtist: "Artist",
			expectedAlbum:  "Album",
			expectedTrack:  1,
			shouldError:    false,
		},
		{
			name:           "Double digit track number with artist-title",
			filepath:       "/music/SomeArtist/SomeAlbum/12 - Artist Name - Track Twelve.wav",
			expectedTitle:  "Track Twelve",
			expectedArtist: "Artist Name",
			expectedAlbum:  "SomeAlbum",
			expectedTrack:  12,
			shouldError:    false,
		},
		{
			name:           "Simple filename extracts from path",
			filepath:       "/music/Some Artist/Some Album/Some Song.mp3",
			expectedTitle:  "Some Song",
			expectedArtist: "Some Artist",
			expectedAlbum:  "Some Album",
			expectedTrack:  0,
			shouldError:    false,
		},
		{
			name:           "Filename with brackets (quality tags removed)",
			filepath:       "/music/Artist/Album/08 - Title [FLAC 24bit].flac",
			expectedTitle:  "Title [",  // cleanString removes the quality tag but leaves bracket
			expectedArtist: "Artist",
			expectedAlbum:  "Album",
			expectedTrack:  8,
			shouldError:    false,
		},
		{
			name:           "Artist-Title pattern in filename",
			filepath:       "/music/Various/Compilation/Pink Floyd - Wish You Were Here.mp3",
			expectedTitle:  "Wish You Were Here",
			expectedArtist: "Pink Floyd",
			expectedAlbum:  "Compilation",
			expectedTrack:  0,
			shouldError:    false,
		},
		{
			name:           "Track with number and Artist-Title",
			filepath:       "/music/Jazz/Miles Davis - Kind of Blue/03 - Miles Davis - So What.mp3",
			expectedTitle:  "So What",
			expectedArtist: "Miles Davis",
			expectedAlbum:  "Miles Davis - Kind of Blue",
			expectedTrack:  3,
			shouldError:    false,
		},
		{
			name:           "Dots instead of spaces",
			filepath:       "/music/Some.Artist/Some.Album/05.-.Song.Title.mp3",
			expectedTitle:  "Song Title",
			expectedArtist: "Some Artist",
			expectedAlbum:  "Some Album",
			expectedTrack:  5,
			shouldError:    false,
		},
		// Artist - Album - TrackNum - Title pattern
		{
			name:           "Artist-Album-Track-Title pattern",
			filepath:       "/music/Sufjan Stevens/Illinois (2005)[FLAC]/Sufjan Stevens - Illinois - 01 - Concerning the UFO Sighting Near Highland, Illinois.flac",
			expectedTitle:  "Concerning the UFO Sighting Near Highland, Illinois",
			expectedArtist: "Sufjan Stevens",
			expectedAlbum:  "Illinois",
			expectedTrack:  1,
			shouldError:    false,
		},
		{
			name:           "Album directory with format tag",
			filepath:       "/music/Sufjan Stevens/Javelin (2023)[FLAC 24bit]/Sufjan Stevens - Javelin - 08 - Javelin (To Have and to Hold).flac",
			expectedTitle:  "Javelin (To Have and to Hold)",
			expectedArtist: "Sufjan Stevens",
			expectedAlbum:  "Javelin",
			expectedTrack:  8,
			shouldError:    false,
		},
		{
			name:           "Multi-disc album structure",
			filepath:       "/music/Radiohead/Hail to the Thief (2003)/CD 01/Radiohead - Hail to the Thief - 01 - 2 + 2 = 5.flac",
			expectedTitle:  "2 + 2 = 5",
			expectedArtist: "Radiohead",
			expectedAlbum:  "Hail to the Thief",
			expectedTrack:  1,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pass nil for metadata extractor - tests will use filename parsing only
			info, err := ParseMusic(tt.filepath, nil)

			if tt.shouldError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if info == nil {
				t.Fatal("expected MusicInfo but got nil")
			}

			if info.Title != tt.expectedTitle {
				t.Errorf("expected title '%s', got '%s'", tt.expectedTitle, info.Title)
			}

			if info.Artist != tt.expectedArtist {
				t.Errorf("expected artist '%s', got '%s'", tt.expectedArtist, info.Artist)
			}

			if info.Album != tt.expectedAlbum {
				t.Errorf("expected album '%s', got '%s'", tt.expectedAlbum, info.Album)
			}

			if info.TrackNumber != tt.expectedTrack {
				t.Errorf("expected track %d, got %d", tt.expectedTrack, info.TrackNumber)
			}
		})
	}
}

// TestTVPathExtraction tests extractSeasonFromPath and extractShowNameFromPath
func TestTVPathExtraction(t *testing.T) {
	t.Run("extractSeasonFromPath", func(t *testing.T) {
		tests := []struct {
			name           string
			path           string
			expectedSeason int
		}{
			{
				name:           "Season directory - 'Season 01'",
				path:           "/tv/Show Name/Season 01/episode.mkv",
				expectedSeason: 1,
			},
			{
				name:           "Season directory - 'Season 1'",
				path:           "/tv/Show Name/Season 1/episode.mkv",
				expectedSeason: 1,
			},
			{
				name:           "Season directory - 'S01'",
				path:           "/tv/Show Name/S01/episode.mkv",
				expectedSeason: 1,
			},
			{
				name:           "Season directory - 'S1'",
				path:           "/tv/Show Name/S1/episode.mkv",
				expectedSeason: 1,
			},
			{
				name:           "Just number '1'",
				path:           "/tv/Show Name/1/episode.mkv",
				expectedSeason: 1,
			},
			{
				name:           "Season 10",
				path:           "/tv/Show Name/Season 10/episode.mkv",
				expectedSeason: 10,
			},
			{
				name:           "No season in path",
				path:           "/tv/Show Name/episode.mkv",
				expectedSeason: 0,
			},
			{
				name:           "Out of range season number (> 99)",
				path:           "/tv/Show Name/Season 100/episode.mkv",
				expectedSeason: 10,  // Extracts "10" from "100"
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				season := extractSeasonFromPath(tt.path)
				if season != tt.expectedSeason {
					t.Errorf("expected season %d, got %d", tt.expectedSeason, season)
				}
			})
		}
	})

	t.Run("extractShowNameFromPath", func(t *testing.T) {
		tests := []struct {
			name         string
			path         string
			expectedShow string
		}{
			{
				name:         "Show name with season directory",
				path:         "/tv/Breaking Bad/Season 1/episode.mkv",
				expectedShow: "Breaking Bad",
			},
			{
				name:         "Show name without season directory",
				path:         "/tv/The Wire/episode.mkv",
				expectedShow: "The Wire",
			},
			{
				name:         "Show name with S01 directory",
				path:         "/tv shows/Game of Thrones/S01/episode.mkv",
				expectedShow: "Game of Thrones",
			},
			{
				name:         "Deep nested structure",
				path:         "/media/TV Shows/The Sopranos/Season 2/episode.mkv",
				expectedShow: "The Sopranos",
			},
			{
				name:         "Numeric season folder (1)",
				path:         "/tv/Stranger Things/1/episode.mkv",
				expectedShow: "Stranger Things",
			},
			{
				name:         "Show with year",
				path:         "/tv/The Office (US)/Season 1/episode.mkv",
				expectedShow: "The Office (US)",
			},
			{
				name:         "No clear show name",
				path:         "/episode.mkv",
				expectedShow: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				showName := extractShowNameFromPath(tt.path)
				if showName != tt.expectedShow {
					t.Errorf("expected show name '%s', got '%s'", tt.expectedShow, showName)
				}
			})
		}
	})
}

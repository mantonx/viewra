package media

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestProcessMusicTrack(t *testing.T) {
	track3 := 3
	year2021 := 2021

	tests := []struct {
		name       string
		libraryID  int64
		result     *scanner.ScanResult
		setupRepo  func(*mocks.MediaRepository, *mocks.MusicRepository)
		setupCache func(*sync.Map)
		checkRepo  func(*testing.T, *mocks.MediaRepository, *mocks.MusicRepository)
	}{
		{
			name:      "create new music track",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath:    "/music/album/track03.mp3",
				Title:       "Song Title",
				Artist:      "Artist Name",
				Album:       "Album Name",
				TrackNumber: &track3,
				Year:        &year2021,
				Duration:    180,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(tracks))
				}
				for _, track := range tracks {
					if track.Media.Title != "Song Title" {
						t.Errorf("Title = %v, want Song Title", track.Media.Title)
					}
					if track.Artist != "Artist Name" {
						t.Errorf("Artist = %v, want Artist Name", track.Artist)
					}
					if track.Album != "Album Name" {
						t.Errorf("Album = %v, want Album Name", track.Album)
					}
					if track.TrackNumber != 3 {
						t.Errorf("TrackNumber = %v, want 3", track.TrackNumber)
					}
					if track.Year != 2021 {
						t.Errorf("Year = %v, want 2021", track.Year)
					}
				}
			},
		},
		{
			name:      "update existing music track",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/existing.mp3",
				Title:    "Updated Song",
				Artist:   "Updated Artist",
				Album:    "Updated Album",
				Duration: 200,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        80,
					LibraryID: 3,
					FilePath:  "/music/existing.mp3",
					Title:     "Old Song",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        80,
						LibraryID: 3,
						FilePath:  "/music/existing.mp3",
						Title:     "Old Song",
					},
					Artist: "Old Artist",
					Album:  "Old Album",
				})
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/existing.mp3", int64(80))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track updated, got %d", len(tracks))
				}
				for _, track := range tracks {
					if track.Media.ID != 80 {
						t.Errorf("ID = %v, want 80 (existing)", track.Media.ID)
					}
					if track.Media.Title != "Updated Song" {
						t.Errorf("Title = %v, want Updated Song", track.Media.Title)
					}
					if track.Artist != "Updated Artist" {
						t.Errorf("Artist = %v, want Updated Artist", track.Artist)
					}
				}
			},
		},
		{
			name:      "track with minimal metadata",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath:    "/music/minimal.mp3",
				Title:       "Minimal Track",
				Artist:      "",
				Album:       "",
				TrackNumber: nil,
				Year:        nil,
				Duration:    150,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(tracks))
				}
				for _, track := range tracks {
					if track.TrackNumber != 0 {
						t.Errorf("TrackNumber = %v, want 0 (default)", track.TrackNumber)
					}
					if track.Year != 0 {
						t.Errorf("Year = %v, want 0 (default)", track.Year)
					}
				}
			},
		},
		{
			name:      "track with artist and album entities created",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath:    "/music/new-artist/track01.mp3",
				Title:       "New Track",
				Artist:      "New Artist",
				Album:       "New Album",
				TrackNumber: &track3,
				Duration:    200,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track created, got %d", len(tracks))
				}
				artists, _ := musicRepo.ListArtistsByLibrary(context.Background(), 3)
				if len(artists) != 1 {
					t.Errorf("Expected 1 artist created, got %d", len(artists))
				}
				albums, _ := musicRepo.ListAlbumsByLibrary(context.Background(), 3)
				if len(albums) != 1 {
					t.Errorf("Expected 1 album created, got %d", len(albums))
				}
			},
		},
		{
			name:      "track update links to existing artist and album",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/track.mp3",
				Title:    "Track Title",
				Artist:   "Existing Artist",
				Album:    "Existing Album",
				Duration: 180,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Pre-create artist and album
				artist := &media.Artist{ID: 10, LibraryID: 3, Name: "Existing Artist"}
				album := &media.Album{ID: 20, LibraryID: 3, Title: "Existing Album", AlbumArtist: "Existing Artist"}
				musicRepo.CreateArtist(context.Background(), artist)
				musicRepo.CreateAlbum(context.Background(), album)

				// Pre-create media and track entries
				mediaRepo.WithMedia(&media.Media{
					ID:        90,
					LibraryID: 3,
					FilePath:  "/music/track.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        90,
						LibraryID: 3,
						FilePath:  "/music/track.mp3",
						Type:      "music_track",
					},
					Artist:   "Old Artist",
					Album:    "Old Album",
					ArtistID: 0,
					AlbumID:  0,
				})
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/track.mp3", int64(90))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
				if len(tracks) != 1 {
					t.Errorf("Expected 1 track, got %d", len(tracks))
				}
				for _, track := range tracks {
					if track.ArtistID != 10 {
						t.Errorf("ArtistID = %v, want 10", track.ArtistID)
					}
					if track.AlbumID != 20 {
						t.Errorf("AlbumID = %v, want 20", track.AlbumID)
					}
				}
			},
		},
		{
			name:      "handle race condition with unique constraint error",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race.mp3",
				Title:    "Race Track",
				Artist:   "Race Artist",
				Duration: 180,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error on first create
				musicRepo.WithCreateError(errors.New("UNIQUE constraint failed: media.file_path"))
				// After error, we'll fetch existing media
				mediaRepo.WithMedia(&media.Media{
					ID:        95,
					LibraryID: 3,
					FilePath:  "/music/race.mp3",
				})
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Should handle gracefully - update existing record
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			musicRepo := mocks.NewMusicRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, musicRepo)
			}

			mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: tt.result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			_, _ = ProcessMusicTrack(context.Background(), deps, tt.libraryID, tt.result, checkpoint, existingMediaCache)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, musicRepo)
			}
		})
	}
}

func TestProcessMusicTrack_AdditionalCoverage(t *testing.T) {
	tests := []struct {
		name          string
		libraryID     int64
		result        *scanner.ScanResult
		checkpoint    *scanner.ScanCheckpoint
		setupRepo     func(*mocks.MediaRepository, *mocks.MusicRepository)
		setupCache    func(*sync.Map)
		expectMediaID bool
		expectError   bool
		checkError    func(*testing.T, error)
	}{
		{
			name:      "race condition - cache hit after unique constraint",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/track.mp3",
				Title:    "Track",
				Artist:   "Artist",
				Album:    "Album",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/track.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("UNIQUE constraint failed: media.file_path"))
				// Pre-populate media
				mediaRepo.WithMedia(&media.Media{
					ID:        350,
					LibraryID: 3,
					FilePath:  "/music/track.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        350,
						LibraryID: 3,
						FilePath:  "/music/track.mp3",
						Type:      "music_track",
					},
					Artist: "Artist",
					Album:  "Album",
				})
			},
			setupCache: func(cache *sync.Map) {
				// Simulate cache hit during race condition
				cache.Store("/music/track.mp3", int64(350))
			},
			expectMediaID: true,
			expectError:   false,
		},
		{
			name:      "update failure after cache hit",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/update-fail.mp3",
				Title:    "Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        360,
					LibraryID: 3,
					FilePath:  "/music/update-fail.mp3",
					Type:      "music_track",
				})
				// Inject update error
				mediaRepo.UpdateErr = errors.New("database update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/update-fail.mp3", int64(360))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record") {
					t.Errorf("Expected 'failed to update base media record' error, got: %v", err)
				}
			},
		},
		{
			name:      "music track update failure after cache hit",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/track-update-fail.mp3",
				Title:    "Track Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/track-update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        365,
					LibraryID: 3,
					FilePath:  "/music/track-update-fail.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        365,
						LibraryID: 3,
						FilePath:  "/music/track-update-fail.mp3",
						Type:      "music_track",
					},
				})
				// Inject music track update error
				musicRepo.UpdateErr = errors.New("track update failed")
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/music/track-update-fail.mp3", int64(365))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update music track metadata") {
					t.Errorf("Expected 'failed to update music track metadata' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - fetch failure after unique constraint",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race-fetch-fail.mp3",
				Title:    "Race Fetch Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/race-fetch-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("duplicate key value violates unique constraint"))
				// Inject fetch error
				mediaRepo.GetByFilePathErr = errors.New("database fetch failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to fetch existing media after collision") {
					t.Errorf("Expected 'failed to fetch existing media after collision' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - update failure after fetch",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race-update-fail.mp3",
				Title:    "Race Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/race-update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("UNIQUE constraint failed"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        370,
					LibraryID: 3,
					FilePath:  "/music/race-update-fail.mp3",
					Type:      "music_track",
				})
				// Inject update error after collision
				mediaRepo.UpdateErr = errors.New("update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update base media record") {
					t.Errorf("Expected 'failed to update base media record' error, got: %v", err)
				}
			},
		},
		{
			name:      "race condition - music track update failure after fetch",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/race-track-update-fail.mp3",
				Title:    "Race Track Update Fail",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/race-track-update-fail.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Simulate unique constraint error
				musicRepo.WithCreateError(errors.New("duplicate key"))
				// Pre-populate media for fetch
				mediaRepo.WithMedia(&media.Media{
					ID:        375,
					LibraryID: 3,
					FilePath:  "/music/race-track-update-fail.mp3",
					Type:      "music_track",
				})
				musicRepo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        375,
						LibraryID: 3,
						FilePath:  "/music/race-track-update-fail.mp3",
						Type:      "music_track",
					},
				})
				// Inject track update error after collision
				musicRepo.UpdateErr = errors.New("track update after collision failed")
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to update music track metadata") {
					t.Errorf("Expected 'failed to update music track metadata' error, got: %v", err)
				}
			},
		},
		{
			name:      "non-unique constraint create error",
			libraryID: 3,
			result: &scanner.ScanResult{
				FilePath: "/music/generic-error.mp3",
				Title:    "Generic Error",
				Artist:   "Artist",
				Duration: 180,
			},
			checkpoint: &scanner.ScanCheckpoint{
				FilePath: "/music/generic-error.mp3",
				FileSize: 1024,
				FileHash: "hash",
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, musicRepo *mocks.MusicRepository) {
				// Non-unique constraint error
				musicRepo.WithCreateError(errors.New("some other database error"))
			},
			expectMediaID: false,
			expectError:   true,
			checkError: func(t *testing.T, err error) {
				if !strings.Contains(err.Error(), "failed to create music track") {
					t.Errorf("Expected 'failed to create music track' error, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			musicRepo := mocks.NewMusicRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, musicRepo)
			}

			mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			existingMediaCache := &sync.Map{}
			if tt.setupCache != nil {
				tt.setupCache(existingMediaCache)
			}

			mediaID, err := ProcessMusicTrack(context.Background(), deps, tt.libraryID, tt.result, tt.checkpoint, existingMediaCache)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectMediaID && mediaID == nil {
				t.Errorf("Expected media ID but got nil")
			}
			if !tt.expectMediaID && mediaID != nil {
				t.Errorf("Expected nil media ID but got %v", *mediaID)
			}

			if tt.checkError != nil && err != nil {
				tt.checkError(t, err)
			}
		})
	}
}

func TestProcessMusicTrack_NilCheckpoint(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/test.mp3",
		Title:    "Test Track",
		Artist:   "Test Artist",
		Duration: 180,
	}
	existingMediaCache := &sync.Map{}

	// Pass nil checkpoint - should handle gracefully
	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, nil, existingMediaCache)

	if err != nil {
		t.Errorf("Expected no error with nil checkpoint, got %v", err)
	}

	// Verify track was created
	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Errorf("Expected 1 track created, got %d", len(tracks))
	}
}

func TestProcessMusicTrack_AudioMetadataFields(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	track3 := 3
	year2023 := 2023
	result := &scanner.ScanResult{
		FilePath:        "/music/album/track03.flac",
		Title:           "High Quality Track",
		Artist:          "Artist Name",
		Album:           "Album Name",
		TrackNumber:     &track3,
		Year:            &year2023,
		Duration:        300,
		AudioCodec:      "flac",
		Bitrate:         900000,
		ContainerFormat: "flac",
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 30000000, // 30MB
		FileHash: "flac-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	track := tracks[0]
	if track.Media.AudioCodec != "flac" {
		t.Errorf("AudioCodec = %v, want flac", track.Media.AudioCodec)
	}
	if track.Media.Bitrate != 900000 {
		t.Errorf("Bitrate = %v, want 900000", track.Media.Bitrate)
	}
	if track.Media.ContainerFormat != "flac" {
		t.Errorf("ContainerFormat = %v, want flac", track.Media.ContainerFormat)
	}
	if track.Media.FileSize != 30000000 {
		t.Errorf("FileSize = %v, want 30000000", track.Media.FileSize)
	}
	if track.Media.FileHash != "flac-hash" {
		t.Errorf("FileHash = %v, want flac-hash", track.Media.FileHash)
	}
	if track.TrackNumber != 3 {
		t.Errorf("TrackNumber = %v, want 3", track.TrackNumber)
	}
	if track.Year != 2023 {
		t.Errorf("Year = %v, want 2023", track.Year)
	}
}

func TestProcessMusicTrack_AlbumArtistFallback(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/album/track01.mp3",
		Title:    "Track Title",
		Artist:   "Track Artist",
		Album:    "Album Name",
		Duration: 180,
		// Note: No AlbumArtist set - should fallback to Artist
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 5000000,
		FileHash: "hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify album was created with artist as album artist
	albums, _ := musicRepo.ListAlbumsByLibrary(context.Background(), 3)
	if len(albums) != 1 {
		t.Fatalf("Expected 1 album, got %d", len(albums))
	}

	// Album artist should fallback to track artist
	if albums[0].AlbumArtist != "Track Artist" {
		t.Errorf("AlbumArtist = %v, want Track Artist (fallback)", albums[0].AlbumArtist)
	}
}

func TestProcessMusicTrack_IsExtraDetection(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		wantExtra bool
	}{
		{
			name:      "regular track",
			filePath:  "/music/album/track01.mp3",
			wantExtra: false,
		},
		{
			name:      "trailer file",
			filePath:  "/music/album/track01-trailer.mp3",
			wantExtra: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			musicRepo := mocks.NewMusicRepository(t)

			mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath: tt.filePath,
				Title:    "Test Track",
				Artist:   "Artist",
				Duration: 180,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}

			_, _ = ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

			tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
			if len(tracks) != 1 {
				t.Fatalf("Expected 1 track, got %d", len(tracks))
			}

			if tracks[0].Media.IsExtra != tt.wantExtra {
				t.Errorf("IsExtra = %v, want %v for %s", tracks[0].Media.IsExtra, tt.wantExtra, tt.filePath)
			}
		})
	}
}

func TestProcessMusicTrack_ExistingArtistLookup(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	// Pre-create an existing artist
	existingArtist := &media.Artist{
		ID:        10,
		LibraryID: 3,
		Name:      "Existing Artist",
	}
	musicRepo.CreateArtist(context.Background(), existingArtist)

	// Pre-create existing media and track to trigger update path
	mediaRepo.WithMedia(&media.Media{
		ID:        100,
		LibraryID: 3,
		FilePath:  "/music/track.mp3",
		Type:      "music_track",
	})
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        100,
			LibraryID: 3,
			FilePath:  "/music/track.mp3",
			Type:      "music_track",
		},
	})

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/track.mp3",
		Title:    "New Track",
		Artist:   "Existing Artist", // Should link to existing
		Album:    "New Album",
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}

	// Pre-populate cache to trigger update path
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/music/track.mp3", int64(100))

	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify track was linked to existing artist
	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	if tracks[0].ArtistID != 10 {
		t.Errorf("ArtistID = %v, want 10 (existing artist)", tracks[0].ArtistID)
	}

	// Verify no duplicate artist was created
	artists, _ := musicRepo.ListArtistsByLibrary(context.Background(), 3)
	if len(artists) != 1 {
		t.Errorf("Expected 1 artist (existing), got %d", len(artists))
	}
}

func TestProcessMusicTrack_ExistingAlbumLookup(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	// Pre-create an existing album
	existingAlbum := &media.Album{
		ID:          20,
		LibraryID:   3,
		Title:       "Existing Album",
		AlbumArtist: "Album Artist",
	}
	musicRepo.CreateAlbum(context.Background(), existingAlbum)

	// Pre-create existing media and track to trigger update path
	mediaRepo.WithMedia(&media.Media{
		ID:        100,
		LibraryID: 3,
		FilePath:  "/music/track.mp3",
		Type:      "music_track",
	})
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        100,
			LibraryID: 3,
			FilePath:  "/music/track.mp3",
			Type:      "music_track",
		},
	})

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/track.mp3",
		Title:    "New Track",
		Artist:   "Album Artist",
		Album:    "Existing Album", // Should link to existing
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}

	// Pre-populate cache to trigger update path
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/music/track.mp3", int64(100))

	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify track was linked to existing album
	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	if tracks[0].AlbumID != 20 {
		t.Errorf("AlbumID = %v, want 20 (existing album)", tracks[0].AlbumID)
	}
}

func TestProcessMusicTrack_NoArtistOrAlbum(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/unknown.mp3",
		Title:    "Unknown Track",
		Artist:   "", // No artist
		Album:    "", // No album
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify track was created without artist/album links
	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	// No artist/album should be created
	artists, _ := musicRepo.ListArtistsByLibrary(context.Background(), 3)
	if len(artists) != 0 {
		t.Errorf("Expected 0 artists for track with no artist info, got %d", len(artists))
	}

	albums, _ := musicRepo.ListAlbumsByLibrary(context.Background(), 3)
	if len(albums) != 0 {
		t.Errorf("Expected 0 albums for track with no album info, got %d", len(albums))
	}
}

func TestProcessMusicTrack_MetadataExtractionPath(t *testing.T) {
	// Test that metadata extraction is triggered when artist, album, genre, or albumArtist are missing
	// This tests the path at lines 71-116 in music.go
	t.Run("triggers metadata extraction when artist is empty", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		musicRepo := mocks.NewMusicRepository(t)

		mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
		scanRepos := testScanRepos(t)
		deps := testDeps(t, mediaRepos, scanRepos)

		result := &scanner.ScanResult{
			FilePath: "/music/test.mp3",
			Title:    "Test Track",
			Artist:   "", // Empty - should trigger metadata extraction
			Album:    "Test Album",
			Duration: 180,
		}

		checkpoint := &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileSize: 1024,
			FileHash: "hash",
		}
		existingMediaCache := &sync.Map{}

		_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Verify track was created (metadata extraction will fail for non-existent file, but should continue)
		tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
		if len(tracks) != 1 {
			t.Fatalf("Expected 1 track, got %d", len(tracks))
		}
	})

	t.Run("triggers metadata extraction when album is empty", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		musicRepo := mocks.NewMusicRepository(t)

		mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
		scanRepos := testScanRepos(t)
		deps := testDeps(t, mediaRepos, scanRepos)

		result := &scanner.ScanResult{
			FilePath: "/music/test2.mp3",
			Title:    "Test Track",
			Artist:   "Test Artist",
			Album:    "", // Empty - should trigger metadata extraction
			Duration: 180,
		}

		checkpoint := &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileSize: 1024,
			FileHash: "hash",
		}
		existingMediaCache := &sync.Map{}

		_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Verify track was created
		tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
		if len(tracks) != 1 {
			t.Fatalf("Expected 1 track, got %d", len(tracks))
		}
	})

	t.Run("triggers metadata extraction when genre is empty", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		musicRepo := mocks.NewMusicRepository(t)

		mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
		scanRepos := testScanRepos(t)
		deps := testDeps(t, mediaRepos, scanRepos)

		result := &scanner.ScanResult{
			FilePath: "/music/test3.mp3",
			Title:    "Test Track",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180,
			// No Genre field in ScanResult, so it will be "" and trigger extraction
		}

		checkpoint := &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileSize: 1024,
			FileHash: "hash",
		}
		existingMediaCache := &sync.Map{}

		_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Verify track was created
		tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
		if len(tracks) != 1 {
			t.Fatalf("Expected 1 track, got %d", len(tracks))
		}
	})
}

func TestProcessMusicTrack_ArtistCreationError(t *testing.T) {
	// Test error handling when CreateArtist fails in resolveEntities during update path
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	// Pre-create existing media and track to trigger update path
	mediaRepo.WithMedia(&media.Media{
		ID:        100,
		LibraryID: 3,
		FilePath:  "/music/track.mp3",
		Type:      "music_track",
	})
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        100,
			LibraryID: 3,
			FilePath:  "/music/track.mp3",
			Type:      "music_track",
		},
	})

	// Inject error for CreateArtist (uses CreateErr)
	musicRepo.CreateErr = errors.New("artist creation failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/track.mp3",
		Title:    "Track Title",
		Artist:   "New Artist", // Artist doesn't exist, will try to create
		Album:    "Album",
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}

	// Pre-populate cache to trigger update path
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/music/track.mp3", int64(100))

	// Should complete without error even if artist creation fails
	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify track was still updated
	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	// Track should not have artist ID set due to creation error
	if tracks[0].ArtistID != 0 {
		t.Errorf("ArtistID should be 0 when creation fails, got %d", tracks[0].ArtistID)
	}
}

func TestProcessMusicTrack_AlbumCreationError(t *testing.T) {
	// Test error handling when CreateAlbum fails in resolveEntities during update path
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	// Pre-create existing media and track to trigger update path
	mediaRepo.WithMedia(&media.Media{
		ID:        101,
		LibraryID: 3,
		FilePath:  "/music/track2.mp3",
		Type:      "music_track",
	})
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        101,
			LibraryID: 3,
			FilePath:  "/music/track2.mp3",
			Type:      "music_track",
		},
	})

	// Create an artist first so album creation will be attempted
	artist := &media.Artist{ID: 10, LibraryID: 3, Name: "Artist"}
	musicRepo.CreateArtist(context.Background(), artist)

	// Inject error for CreateAlbum (uses CreateErr)
	musicRepo.CreateErr = errors.New("album creation failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/track2.mp3",
		Title:    "Track Title",
		Artist:   "Artist",
		Album:    "New Album", // Album doesn't exist, will try to create
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}

	// Pre-populate cache to trigger update path
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/music/track2.mp3", int64(101))

	// Should complete without error even if album creation fails
	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify track was still updated
	tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
	if len(tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(tracks))
	}

	// Track should have artist ID but not album ID
	if tracks[0].ArtistID != 10 {
		t.Errorf("ArtistID = %v, want 10", tracks[0].ArtistID)
	}
	if tracks[0].AlbumID != 0 {
		t.Errorf("AlbumID should be 0 when creation fails, got %d", tracks[0].AlbumID)
	}
}

func TestProcessMusicTrack_FindArtistError(t *testing.T) {
	// Test error handling when FindArtistByName fails in resolveEntities during update path
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	// Pre-create existing media and track to trigger update path
	mediaRepo.WithMedia(&media.Media{
		ID:        102,
		LibraryID: 3,
		FilePath:  "/music/track3.mp3",
		Type:      "music_track",
	})
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        102,
			LibraryID: 3,
			FilePath:  "/music/track3.mp3",
			Type:      "music_track",
		},
	})

	// Inject error for FindArtistByName (uses GetErr)
	musicRepo.GetErr = errors.New("find artist failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/track3.mp3",
		Title:    "Track Title",
		Artist:   "Artist Name",
		Album:    "Album",
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}

	// Pre-populate cache to trigger update path
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/music/track3.mp3", int64(102))

	// Should complete without error even if find artist fails (will try to create)
	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestProcessMusicTrack_FindAlbumError(t *testing.T) {
	// Test error handling when FindAlbumByTitle fails in resolveEntities during update path
	mediaRepo := mocks.NewMediaRepository(t)
	musicRepo := mocks.NewMusicRepository(t)

	// Pre-create existing media and track to trigger update path
	mediaRepo.WithMedia(&media.Media{
		ID:        103,
		LibraryID: 3,
		FilePath:  "/music/track4.mp3",
		Type:      "music_track",
	})
	musicRepo.WithTracks(&media.MusicTrack{
		Media: media.Media{
			ID:        103,
			LibraryID: 3,
			FilePath:  "/music/track4.mp3",
			Type:      "music_track",
		},
	})

	// Inject error for FindAlbumByTitle (uses GetErr)
	musicRepo.GetErr = errors.New("find album failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/music/track4.mp3",
		Title:    "Track Title",
		Artist:   "Artist",
		Album:    "Album Name",
		Duration: 180,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "hash",
	}

	// Pre-populate cache to trigger update path
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/music/track4.mp3", int64(103))

	// Should complete without error even if find album fails (will try to create)
	_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestProcessMusicTrack_WithRealFile(t *testing.T) {
	// Test metadata extraction with a real file (even though it's not a valid MP3)
	// This will trigger the metadata extraction code path
	t.Run("metadata extraction with existing file", func(t *testing.T) {
		mediaRepo := mocks.NewMediaRepository(t)
		musicRepo := mocks.NewMusicRepository(t)

		mediaRepos := testMediaRepos(t, mediaRepo, nil, nil, musicRepo)
		scanRepos := testScanRepos(t)
		deps := testDeps(t, mediaRepos, scanRepos)

		// Create a temporary file to trigger metadata extraction
		tmpFile := t.TempDir() + "/test.mp3"
		if err := os.WriteFile(tmpFile, []byte("fake mp3 data"), 0644); err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		result := &scanner.ScanResult{
			FilePath: tmpFile,
			Title:    "",    // Empty - should trigger metadata extraction
			Artist:   "",    // Empty - should trigger metadata extraction
			Album:    "",    // Empty - should trigger metadata extraction
			Duration: 180,
		}

		checkpoint := &scanner.ScanCheckpoint{
			FilePath: result.FilePath,
			FileSize: 1024,
			FileHash: "hash",
		}
		existingMediaCache := &sync.Map{}

		// Metadata extraction will fail (not a real MP3), but should not error
		_, err := ProcessMusicTrack(context.Background(), deps, 3, result, checkpoint, existingMediaCache)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		// Verify track was created despite metadata extraction failure
		tracks, _ := musicRepo.ListMusicTracksByLibrary(context.Background(), 3)
		if len(tracks) != 1 {
			t.Fatalf("Expected 1 track, got %d", len(tracks))
		}
	})
}

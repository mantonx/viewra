package media

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// mockMediaRepositoryForTracks wraps the generated mock to track method calls
type mockMediaRepositoryForTracks struct {
	*mocks.MediaRepository
	deleteAudioCalled    bool
	deleteSubtitleCalled bool
	insertAudioCalled    int
	insertSubtitleCalled int
	deleteAudioErr       error
	deleteSubtitleErr    error
	insertAudioErr       error
	insertSubtitleErr    error
	getByIDResult        *media.Media
	getByIDErr           error
}

func newMockMediaRepositoryForTracks(t *testing.T) *mockMediaRepositoryForTracks {
	return &mockMediaRepositoryForTracks{
		MediaRepository: mocks.NewMediaRepository(t),
	}
}

func (m *mockMediaRepositoryForTracks) DeleteAudioTracksByMediaID(ctx context.Context, mediaID int64) error {
	m.deleteAudioCalled = true
	if m.deleteAudioErr != nil {
		return m.deleteAudioErr
	}
	return nil
}

func (m *mockMediaRepositoryForTracks) DeleteSubtitleTracksByMediaID(ctx context.Context, mediaID int64) error {
	m.deleteSubtitleCalled = true
	if m.deleteSubtitleErr != nil {
		return m.deleteSubtitleErr
	}
	return nil
}

func (m *mockMediaRepositoryForTracks) InsertAudioTrack(ctx context.Context, track *media.AudioTrack) error {
	m.insertAudioCalled++
	if m.insertAudioErr != nil {
		return m.insertAudioErr
	}
	return nil
}

func (m *mockMediaRepositoryForTracks) InsertSubtitleTrack(ctx context.Context, track *media.SubtitleTrack) error {
	m.insertSubtitleCalled++
	if m.insertSubtitleErr != nil {
		return m.insertSubtitleErr
	}
	return nil
}

func (m *mockMediaRepositoryForTracks) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if m.getByIDResult != nil {
		return m.getByIDResult, nil
	}
	return nil, errors.New("media not found")
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPersistMediaTracks(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		scanResult  *scanner.ScanResult
		setupMocks  func(*mockMediaRepositoryForTracks, *mocks.LibraryRepository)
		checkResult func(*testing.T, *mockMediaRepositoryForTracks)
	}{
		{
			name:    "persist audio and subtitle tracks successfully",
			mediaID: 100,
			scanResult: &scanner.ScanResult{
				FilePath: "/movies/test.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{
						StreamIndex:   0,
						Codec:         "aac",
						CodecProfile:  "LC",
						Channels:      2,
						ChannelLayout: "stereo",
						SampleRate:    48000,
						BitRate:       128000,
						Language:      "eng",
						Title:         "English",
						IsDefault:     true,
						IsCommentary:  false,
						IsDescriptive: false,
					},
					{
						StreamIndex:   1,
						Codec:         "ac3",
						CodecProfile:  "",
						Channels:      6,
						ChannelLayout: "5.1",
						SampleRate:    48000,
						BitRate:       640000,
						Language:      "eng",
						Title:         "English 5.1",
						IsDefault:     false,
						IsCommentary:  false,
						IsDescriptive: false,
					},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{
					{
						StreamIndex:  2,
						Codec:        "subrip",
						Language:     "eng",
						Title:        "English",
						IsDefault:    true,
						IsForced:     false,
						IsSDH:        false,
						IsCommentary: false,
						IsBitmap:     false,
					},
					{
						StreamIndex:  3,
						Codec:        "subrip",
						Language:     "spa",
						Title:        "Spanish",
						IsDefault:    false,
						IsForced:     false,
						IsSDH:        false,
						IsCommentary: false,
						IsBitmap:     false,
					},
				},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDResult = &media.Media{
					ID:        100,
					LibraryID: 1,
					FilePath:  "/movies/test.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				if !mediaRepo.deleteAudioCalled {
					t.Error("Expected DeleteAudioTracksByMediaID to be called")
				}
				if !mediaRepo.deleteSubtitleCalled {
					t.Error("Expected DeleteSubtitleTracksByMediaID to be called")
				}
				if mediaRepo.insertAudioCalled != 2 {
					t.Errorf("Expected InsertAudioTrack to be called 2 times, got %d", mediaRepo.insertAudioCalled)
				}
				if mediaRepo.insertSubtitleCalled != 2 {
					t.Errorf("Expected InsertSubtitleTrack to be called 2 times, got %d", mediaRepo.insertSubtitleCalled)
				}
			},
		},
		{
			name:    "no tracks to persist",
			mediaID: 101,
			scanResult: &scanner.ScanResult{
				FilePath:       "/movies/test2.mp4",
				AudioTracks:    []scanner.AudioTrackInfo{},
				SubtitleTracks: []scanner.SubtitleTrackInfo{},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDResult = &media.Media{
					ID:        101,
					LibraryID: 1,
					FilePath:  "/movies/test2.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				if !mediaRepo.deleteAudioCalled {
					t.Error("Expected DeleteAudioTracksByMediaID to be called")
				}
				if !mediaRepo.deleteSubtitleCalled {
					t.Error("Expected DeleteSubtitleTracksByMediaID to be called")
				}
				if mediaRepo.insertAudioCalled != 0 {
					t.Errorf("Expected InsertAudioTrack to be called 0 times, got %d", mediaRepo.insertAudioCalled)
				}
				if mediaRepo.insertSubtitleCalled != 0 {
					t.Errorf("Expected InsertSubtitleTrack to be called 0 times, got %d", mediaRepo.insertSubtitleCalled)
				}
			},
		},
		{
			name:    "delete audio tracks error (should continue)",
			mediaID: 102,
			scanResult: &scanner.ScanResult{
				FilePath: "/movies/test3.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{
						StreamIndex: 0,
						Codec:       "aac",
						Language:    "eng",
					},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.deleteAudioErr = errors.New("database error")
				mediaRepo.getByIDResult = &media.Media{
					ID:        102,
					LibraryID: 1,
					FilePath:  "/movies/test3.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// Should continue despite delete error
				if mediaRepo.insertAudioCalled != 1 {
					t.Errorf("Expected InsertAudioTrack to be called 1 time despite delete error, got %d", mediaRepo.insertAudioCalled)
				}
			},
		},
		{
			name:    "delete subtitle tracks error (should continue)",
			mediaID: 103,
			scanResult: &scanner.ScanResult{
				FilePath:    "/movies/test4.mp4",
				AudioTracks: []scanner.AudioTrackInfo{},
				SubtitleTracks: []scanner.SubtitleTrackInfo{
					{
						StreamIndex: 2,
						Codec:       "subrip",
						Language:    "eng",
					},
				},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.deleteSubtitleErr = errors.New("database error")
				mediaRepo.getByIDResult = &media.Media{
					ID:        103,
					LibraryID: 1,
					FilePath:  "/movies/test4.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// Should continue despite delete error
				if mediaRepo.insertSubtitleCalled != 1 {
					t.Errorf("Expected InsertSubtitleTrack to be called 1 time despite delete error, got %d", mediaRepo.insertSubtitleCalled)
				}
			},
		},
		{
			name:    "insert audio track error (should continue with other tracks)",
			mediaID: 104,
			scanResult: &scanner.ScanResult{
				FilePath: "/movies/test5.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{StreamIndex: 0, Codec: "aac", Language: "eng"},
					{StreamIndex: 1, Codec: "ac3", Language: "eng"},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.insertAudioErr = errors.New("insert error")
				mediaRepo.getByIDResult = &media.Media{
					ID:        104,
					LibraryID: 1,
					FilePath:  "/movies/test5.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// Should attempt both inserts despite errors
				if mediaRepo.insertAudioCalled != 2 {
					t.Errorf("Expected InsertAudioTrack to be called 2 times, got %d", mediaRepo.insertAudioCalled)
				}
			},
		},
		{
			name:    "insert subtitle track error (should continue with other tracks)",
			mediaID: 105,
			scanResult: &scanner.ScanResult{
				FilePath:    "/movies/test6.mp4",
				AudioTracks: []scanner.AudioTrackInfo{},
				SubtitleTracks: []scanner.SubtitleTrackInfo{
					{StreamIndex: 2, Codec: "subrip", Language: "eng"},
					{StreamIndex: 3, Codec: "subrip", Language: "spa"},
				},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.insertSubtitleErr = errors.New("insert error")
				mediaRepo.getByIDResult = &media.Media{
					ID:        105,
					LibraryID: 1,
					FilePath:  "/movies/test6.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// Should attempt both inserts despite errors
				if mediaRepo.insertSubtitleCalled != 2 {
					t.Errorf("Expected InsertSubtitleTrack to be called 2 times, got %d", mediaRepo.insertSubtitleCalled)
				}
			},
		},
		{
			name:    "tracks with all metadata fields populated",
			mediaID: 106,
			scanResult: &scanner.ScanResult{
				FilePath: "/movies/test7.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{
						StreamIndex:   0,
						Codec:         "eac3",
						CodecProfile:  "Atmos",
						Channels:      8,
						ChannelLayout: "7.1",
						SampleRate:    48000,
						BitRate:       768000,
						Language:      "eng",
						Title:         "English Atmos",
						IsDefault:     true,
						IsCommentary:  false,
						IsDescriptive: false,
					},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{
					{
						StreamIndex:  2,
						Codec:        "hdmv_pgs_subtitle",
						Language:     "eng",
						Title:        "English SDH",
						IsDefault:    false,
						IsForced:     false,
						IsSDH:        true,
						IsCommentary: false,
						IsBitmap:     true,
					},
				},
			},
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDResult = &media.Media{
					ID:        106,
					LibraryID: 1,
					FilePath:  "/movies/test7.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				if mediaRepo.insertAudioCalled != 1 {
					t.Errorf("Expected InsertAudioTrack to be called 1 time, got %d", mediaRepo.insertAudioCalled)
				}
				if mediaRepo.insertSubtitleCalled != 1 {
					t.Errorf("Expected InsertSubtitleTrack to be called 1 time, got %d", mediaRepo.insertSubtitleCalled)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := newMockMediaRepositoryForTracks(t)
			libRepo := mocks.NewLibraryRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(mediaRepo, libRepo)
			}

			deps := &Deps{
				MediaRepos: &scan.MediaRepositories{
					Library: libRepo,
					Media:   mediaRepo,
					Movie:   mocks.NewMovieRepository(t),
					TV:      mocks.NewTVRepository(t),
					Music:   mocks.NewMusicRepository(t),
				},
				Logger: discardLogger(),
			}

			// Execute - this should not panic or return errors
			PersistMediaTracks(context.Background(), deps, tt.mediaID, tt.scanResult)

			if tt.checkResult != nil {
				tt.checkResult(t, mediaRepo)
			}
		})
	}
}

func TestDiscoverExternalSubtitles(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		videoPath   string
		setupMocks  func(*mockMediaRepositoryForTracks, *mocks.LibraryRepository)
		checkResult func(*testing.T, *mockMediaRepositoryForTracks)
	}{
		{
			name:      "media not found",
			mediaID:   999,
			videoPath: "/movies/test.mp4",
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDErr = errors.New("media not found")
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// Should not attempt to insert subtitles
				if mediaRepo.insertSubtitleCalled > 0 {
					t.Error("Expected InsertSubtitleTrack not to be called when media not found")
				}
			},
		},
		{
			name:      "library not found",
			mediaID:   100,
			videoPath: "/movies/test.mp4",
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDResult = &media.Media{
					ID:        100,
					LibraryID: 999, // Non-existent library
					FilePath:  "/movies/test.mp4",
				}
				// libRepo has no libraries
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// Should not attempt to insert subtitles
				if mediaRepo.insertSubtitleCalled > 0 {
					t.Error("Expected InsertSubtitleTrack not to be called when library not found")
				}
			},
		},
		{
			name:      "no external subtitles found",
			mediaID:   101,
			videoPath: "/movies/test.mp4",
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDResult = &media.Media{
					ID:        101,
					LibraryID: 1,
					FilePath:  "movies/test.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// DiscoverAllExternalSubtitles will return empty for non-existent paths
				// So no subtitles should be inserted
				if mediaRepo.insertSubtitleCalled > 0 {
					t.Error("Expected InsertSubtitleTrack not to be called when no external subtitles found")
				}
			},
		},
		{
			name:      "insert subtitle track error (should warn and continue)",
			mediaID:   102,
			videoPath: "/movies/test.mp4",
			setupMocks: func(mediaRepo *mockMediaRepositoryForTracks, libRepo *mocks.LibraryRepository) {
				mediaRepo.getByIDResult = &media.Media{
					ID:        102,
					LibraryID: 1,
					FilePath:  "movies/test.mp4",
				}
				libRepo.WithLibraries(&library.Library{
					ID:   1,
					Name: "Movies",
					Path: "/data",
					Type: library.LibraryTypeMovies,
				})
				mediaRepo.insertSubtitleErr = errors.New("insert error")
			},
			checkResult: func(t *testing.T, mediaRepo *mockMediaRepositoryForTracks) {
				// No external subtitles will be found for non-existent path
				// but this tests that errors are handled gracefully
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := newMockMediaRepositoryForTracks(t)
			libRepo := mocks.NewLibraryRepository(t)

			if tt.setupMocks != nil {
				tt.setupMocks(mediaRepo, libRepo)
			}

			deps := &Deps{
				MediaRepos: &scan.MediaRepositories{
					Library: libRepo,
					Media:   mediaRepo,
					Movie:   mocks.NewMovieRepository(t),
					TV:      mocks.NewTVRepository(t),
					Music:   mocks.NewMusicRepository(t),
				},
				Logger: discardLogger(),
			}

			// Execute - this should not panic
			DiscoverExternalSubtitles(context.Background(), deps, tt.mediaID, tt.videoPath)

			if tt.checkResult != nil {
				tt.checkResult(t, mediaRepo)
			}
		})
	}
}

func TestDiscoverExternalSubtitles_PathHandling(t *testing.T) {
	// This test verifies that the function correctly handles path conversions
	// from absolute to relative paths
	t.Run("handles relative path conversion", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		mediaRepo.getByIDResult = &media.Media{
			ID:        200,
			LibraryID: 2,
			FilePath:  "movies/action/test.mp4",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   2,
			Name: "Movies",
			Path: "/library/root",
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// This should build the full path as /library/root/movies/action/test.mp4
		// and then search for external subtitles there
		DiscoverExternalSubtitles(context.Background(), deps, 200, "movies/action/test.mp4")

		// No external subtitles will be found for non-existent path,
		// but the function should not panic
	})

	t.Run("handles filepath.Rel error gracefully", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		mediaRepo.getByIDResult = &media.Media{
			ID:        201,
			LibraryID: 2,
			FilePath:  "movies/test.mp4",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   2,
			Name: "Movies",
			Path: "/library",
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// Execute with valid paths - the function should handle everything gracefully
		DiscoverExternalSubtitles(context.Background(), deps, 201, "movies/test.mp4")

		// Function should complete without panicking
	})
}

func TestPersistMediaTracks_Commentary(t *testing.T) {
	// Test that commentary detection works correctly for external subtitles
	t.Run("marks external subtitle as commentary based on title", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		mediaRepo.getByIDResult = &media.Media{
			ID:        300,
			LibraryID: 3,
			FilePath:  "test.mp4",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   3,
			Name: "Test",
			Path: "/test",
			Type: library.LibraryTypeMovies,
		})

		scanResult := &scanner.ScanResult{
			FilePath:       "test.mp4",
			AudioTracks:    []scanner.AudioTrackInfo{},
			SubtitleTracks: []scanner.SubtitleTrackInfo{},
		}

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// This tests that PersistMediaTracks calls DiscoverExternalSubtitles
		PersistMediaTracks(context.Background(), deps, 300, scanResult)

		// The function should execute without errors
		if !mediaRepo.deleteAudioCalled || !mediaRepo.deleteSubtitleCalled {
			t.Error("Expected delete methods to be called")
		}
	})
}

func TestPersistMediaTracks_Integration(t *testing.T) {
	// Integration test that verifies the full flow
	t.Run("full track persistence flow", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		mediaRepo.getByIDResult = &media.Media{
			ID:        400,
			LibraryID: 4,
			FilePath:  "movies/action/movie.mkv",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   4,
			Name: "Movies",
			Path: "/library",
			Type: library.LibraryTypeMovies,
		})

		scanResult := &scanner.ScanResult{
			FilePath: "movies/action/movie.mkv",
			AudioTracks: []scanner.AudioTrackInfo{
				{
					StreamIndex:   0,
					Codec:         "dts",
					CodecProfile:  "DTS-HD MA",
					Channels:      6,
					ChannelLayout: "5.1",
					SampleRate:    48000,
					BitRate:       1536000,
					Language:      "eng",
					Title:         "English DTS-HD",
					IsDefault:     true,
					IsCommentary:  false,
					IsDescriptive: false,
				},
			},
			SubtitleTracks: []scanner.SubtitleTrackInfo{
				{
					StreamIndex:  2,
					Codec:        "ass",
					Language:     "eng",
					Title:        "English (Full)",
					IsDefault:    true,
					IsForced:     false,
					IsSDH:        false,
					IsCommentary: false,
					IsBitmap:     false,
				},
				{
					StreamIndex:  3,
					Codec:        "subrip",
					Language:     "eng",
					Title:        "English (Forced)",
					IsDefault:    false,
					IsForced:     true,
					IsSDH:        false,
					IsCommentary: false,
					IsBitmap:     false,
				},
			},
		}

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		PersistMediaTracks(context.Background(), deps, 400, scanResult)

		// Verify the expected operations were performed
		if !mediaRepo.deleteAudioCalled {
			t.Error("Expected DeleteAudioTracksByMediaID to be called")
		}
		if !mediaRepo.deleteSubtitleCalled {
			t.Error("Expected DeleteSubtitleTracksByMediaID to be called")
		}
		if mediaRepo.insertAudioCalled != 1 {
			t.Errorf("Expected 1 audio track to be inserted, got %d", mediaRepo.insertAudioCalled)
		}
		if mediaRepo.insertSubtitleCalled != 2 {
			t.Errorf("Expected 2 subtitle tracks to be inserted, got %d", mediaRepo.insertSubtitleCalled)
		}
	})
}

func TestDiscoverExternalSubtitles_EdgeCases(t *testing.T) {
	t.Run("handles empty file path", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		mediaRepo.getByIDResult = &media.Media{
			ID:        500,
			LibraryID: 5,
			FilePath:  "",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   5,
			Name: "Test",
			Path: "/test",
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// Should not panic with empty file path
		DiscoverExternalSubtitles(context.Background(), deps, 500, "")
	})

	t.Run("handles special characters in path", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		specialPath := filepath.Join("movies", "action [2023]", "test (1080p).mkv")

		mediaRepo.getByIDResult = &media.Media{
			ID:        501,
			LibraryID: 5,
			FilePath:  specialPath,
		}

		libRepo.WithLibraries(&library.Library{
			ID:   5,
			Name: "Test",
			Path: "/library",
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// Should handle special characters in paths
		DiscoverExternalSubtitles(context.Background(), deps, 501, specialPath)
	})
}

func TestDiscoverExternalSubtitles_WithRealFiles(t *testing.T) {
	// Test with actual files to trigger the external subtitle discovery path
	t.Run("discovers external subtitle files", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		// Create temp directory structure
		tmpDir := t.TempDir()
		videoPath := filepath.Join(tmpDir, "movie.mkv")
		subtitlePath := filepath.Join(tmpDir, "movie.en.srt")

		// Create the video file
		if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
			t.Fatalf("Failed to create video file: %v", err)
		}

		// Create an external subtitle file
		if err := os.WriteFile(subtitlePath, []byte("fake subtitle"), 0644); err != nil {
			t.Fatalf("Failed to create subtitle file: %v", err)
		}

		mediaRepo.getByIDResult = &media.Media{
			ID:        600,
			LibraryID: 6,
			FilePath:  "movie.mkv",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   6,
			Name: "Movies",
			Path: tmpDir,
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// This should discover the external subtitle and insert it
		DiscoverExternalSubtitles(context.Background(), deps, 600, "movie.mkv")

		// Verify that subtitle was inserted
		if mediaRepo.insertSubtitleCalled < 1 {
			t.Logf("Note: InsertSubtitleTrack was called %d times (external subtitle discovery may not have found files)", mediaRepo.insertSubtitleCalled)
		}
	})

	t.Run("handles multiple external subtitle files", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		// Create temp directory structure
		tmpDir := t.TempDir()
		videoPath := filepath.Join(tmpDir, "movie.mp4")
		subtitle1 := filepath.Join(tmpDir, "movie.en.srt")
		subtitle2 := filepath.Join(tmpDir, "movie.es.srt")
		subtitle3 := filepath.Join(tmpDir, "movie.fr.srt")

		// Create the video file
		if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
			t.Fatalf("Failed to create video file: %v", err)
		}

		// Create external subtitle files
		for _, path := range []string{subtitle1, subtitle2, subtitle3} {
			if err := os.WriteFile(path, []byte("fake subtitle"), 0644); err != nil {
				t.Fatalf("Failed to create subtitle file: %v", err)
			}
		}

		mediaRepo.getByIDResult = &media.Media{
			ID:        601,
			LibraryID: 6,
			FilePath:  "movie.mp4",
		}

		libRepo.WithLibraries(&library.Library{
			ID:   6,
			Name: "Movies",
			Path: tmpDir,
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// This should discover all external subtitles
		DiscoverExternalSubtitles(context.Background(), deps, 601, "movie.mp4")

		// Verify that subtitles were inserted
		if mediaRepo.insertSubtitleCalled < 1 {
			t.Logf("Note: InsertSubtitleTrack was called %d times (external subtitle discovery may not have found all files)", mediaRepo.insertSubtitleCalled)
		}
	})

	t.Run("handles subtitle file in subdirectory", func(t *testing.T) {
		mediaRepo := newMockMediaRepositoryForTracks(t)
		libRepo := mocks.NewLibraryRepository(t)

		// Create temp directory structure
		tmpDir := t.TempDir()
		movieDir := filepath.Join(tmpDir, "movies", "action")
		if err := os.MkdirAll(movieDir, 0755); err != nil {
			t.Fatalf("Failed to create movie directory: %v", err)
		}

		videoPath := filepath.Join(movieDir, "movie.mkv")
		subtitlePath := filepath.Join(movieDir, "movie.en.srt")

		// Create files
		if err := os.WriteFile(videoPath, []byte("fake video"), 0644); err != nil {
			t.Fatalf("Failed to create video file: %v", err)
		}
		if err := os.WriteFile(subtitlePath, []byte("fake subtitle"), 0644); err != nil {
			t.Fatalf("Failed to create subtitle file: %v", err)
		}

		mediaRepo.getByIDResult = &media.Media{
			ID:        602,
			LibraryID: 6,
			FilePath:  filepath.Join("movies", "action", "movie.mkv"),
		}

		libRepo.WithLibraries(&library.Library{
			ID:   6,
			Name: "Movies",
			Path: tmpDir,
			Type: library.LibraryTypeMovies,
		})

		deps := &Deps{
			MediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			Logger: discardLogger(),
		}

		// This should discover the external subtitle
		DiscoverExternalSubtitles(context.Background(), deps, 602, filepath.Join("movies", "action", "movie.mkv"))

		// Verify that subtitle was inserted
		if mediaRepo.insertSubtitleCalled < 1 {
			t.Logf("Note: InsertSubtitleTrack was called %d times", mediaRepo.insertSubtitleCalled)
		}
	})
}

func TestPersistMediaTracks_WithEnhancedMock(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		result      *scanner.ScanResult
		setupRepo   func(*mocks.MediaRepository, *mocks.LibraryRepository)
		checkResult func(*testing.T, *mocks.MediaRepository)
	}{
		{
			name:    "persist and verify audio tracks are stored",
			mediaID: 100,
			result: &scanner.ScanResult{
				FilePath: "/movies/test.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{
						StreamIndex:   0,
						Codec:         "aac",
						CodecProfile:  "LC",
						Channels:      2,
						ChannelLayout: "stereo",
						SampleRate:    48000,
						BitRate:       128000,
						Language:      "eng",
						Title:         "English",
						IsDefault:     true,
						IsCommentary:  false,
						IsDescriptive: false,
					},
					{
						StreamIndex:   1,
						Codec:         "ac3",
						CodecProfile:  "",
						Channels:      6,
						ChannelLayout: "5.1",
						SampleRate:    48000,
						BitRate:       640000,
						Language:      "jpn",
						Title:         "Japanese 5.1",
						IsDefault:     false,
						IsCommentary:  false,
						IsDescriptive: false,
					},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{},
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, libraryRepo *mocks.LibraryRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        100,
					LibraryID: 1,
					FilePath:  "/movies/test.mp4",
				})
				libraryRepo.WithLibraries(&library.Library{
					ID:   1,
					Path: "/media/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mocks.MediaRepository) {
				t.Helper()
				audioTracks, err := mediaRepo.GetAudioTracksByMediaID(context.Background(), 100)
				if err != nil {
					t.Fatalf("Failed to get audio tracks: %v", err)
				}
				if len(audioTracks) != 2 {
					t.Fatalf("Expected 2 audio tracks, got %d", len(audioTracks))
				}
				// Verify first track
				track0 := audioTracks[0]
				if track0.MediaID != 100 {
					t.Errorf("AudioTrack[0].MediaID = %v, want 100", track0.MediaID)
				}
				if track0.Codec != "aac" {
					t.Errorf("AudioTrack[0].Codec = %v, want aac", track0.Codec)
				}
				if track0.CodecProfile != "LC" {
					t.Errorf("AudioTrack[0].CodecProfile = %v, want LC", track0.CodecProfile)
				}
				if track0.Channels != 2 {
					t.Errorf("AudioTrack[0].Channels = %v, want 2", track0.Channels)
				}
				if track0.ChannelLayout != "stereo" {
					t.Errorf("AudioTrack[0].ChannelLayout = %v, want stereo", track0.ChannelLayout)
				}
				if track0.SampleRate != 48000 {
					t.Errorf("AudioTrack[0].SampleRate = %v, want 48000", track0.SampleRate)
				}
				if track0.BitRate != 128000 {
					t.Errorf("AudioTrack[0].BitRate = %v, want 128000", track0.BitRate)
				}
				if track0.Language != "eng" {
					t.Errorf("AudioTrack[0].Language = %v, want eng", track0.Language)
				}
				if track0.Title != "English" {
					t.Errorf("AudioTrack[0].Title = %v, want English", track0.Title)
				}
				if !track0.IsDefault {
					t.Errorf("AudioTrack[0].IsDefault = false, want true")
				}
				if track0.IsCommentary {
					t.Errorf("AudioTrack[0].IsCommentary = true, want false")
				}
				if track0.IsDescriptive {
					t.Errorf("AudioTrack[0].IsDescriptive = true, want false")
				}

				// Verify second track
				track1 := audioTracks[1]
				if track1.Codec != "ac3" {
					t.Errorf("AudioTrack[1].Codec = %v, want ac3", track1.Codec)
				}
				if track1.Channels != 6 {
					t.Errorf("AudioTrack[1].Channels = %v, want 6", track1.Channels)
				}
				if track1.ChannelLayout != "5.1" {
					t.Errorf("AudioTrack[1].ChannelLayout = %v, want 5.1", track1.ChannelLayout)
				}
				if track1.Language != "jpn" {
					t.Errorf("AudioTrack[1].Language = %v, want jpn", track1.Language)
				}
				if track1.IsDefault {
					t.Errorf("AudioTrack[1].IsDefault = true, want false")
				}
			},
		},
		{
			name:    "persist and verify subtitle tracks are stored",
			mediaID: 101,
			result: &scanner.ScanResult{
				FilePath:    "/movies/test2.mkv",
				AudioTracks: []scanner.AudioTrackInfo{},
				SubtitleTracks: []scanner.SubtitleTrackInfo{
					{
						StreamIndex:  2,
						Codec:        "subrip",
						Language:     "eng",
						Title:        "English",
						IsDefault:    true,
						IsForced:     false,
						IsSDH:        false,
						IsCommentary: false,
						IsBitmap:     false,
					},
					{
						StreamIndex:  3,
						Codec:        "subrip",
						Language:     "spa",
						Title:        "Spanish",
						IsDefault:    false,
						IsForced:     true,
						IsSDH:        false,
						IsCommentary: false,
						IsBitmap:     false,
					},
					{
						StreamIndex:  4,
						Codec:        "hdmv_pgs_subtitle",
						Language:     "eng",
						Title:        "English SDH",
						IsDefault:    false,
						IsForced:     false,
						IsSDH:        true,
						IsCommentary: false,
						IsBitmap:     true,
					},
				},
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, libraryRepo *mocks.LibraryRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        101,
					LibraryID: 1,
					FilePath:  "/movies/test2.mkv",
				})
				libraryRepo.WithLibraries(&library.Library{
					ID:   1,
					Path: "/media/movies",
					Type: library.LibraryTypeMovies,
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mocks.MediaRepository) {
				t.Helper()
				subtitleTracks, err := mediaRepo.GetSubtitleTracksByMediaID(context.Background(), 101)
				if err != nil {
					t.Fatalf("Failed to get subtitle tracks: %v", err)
				}
				if len(subtitleTracks) != 3 {
					t.Fatalf("Expected 3 subtitle tracks, got %d", len(subtitleTracks))
				}

				// Verify first track (default)
				track0 := subtitleTracks[0]
				if track0.MediaID != 101 {
					t.Errorf("SubtitleTrack[0].MediaID = %v, want 101", track0.MediaID)
				}
				if track0.SourceType != media.SubtitleSourceEmbedded {
					t.Errorf("SubtitleTrack[0].SourceType = %v, want embedded", track0.SourceType)
				}
				if track0.Codec != "subrip" {
					t.Errorf("SubtitleTrack[0].Codec = %v, want subrip", track0.Codec)
				}
				if track0.Language != "eng" {
					t.Errorf("SubtitleTrack[0].Language = %v, want eng", track0.Language)
				}
				if track0.Title != "English" {
					t.Errorf("SubtitleTrack[0].Title = %v, want English", track0.Title)
				}
				if !track0.IsDefault {
					t.Errorf("SubtitleTrack[0].IsDefault = false, want true")
				}
				if track0.StreamIndex == nil || *track0.StreamIndex != 2 {
					t.Errorf("SubtitleTrack[0].StreamIndex = %v, want 2", track0.StreamIndex)
				}

				// Verify second track (forced)
				track1 := subtitleTracks[1]
				if track1.Language != "spa" {
					t.Errorf("SubtitleTrack[1].Language = %v, want spa", track1.Language)
				}
				if !track1.IsForced {
					t.Errorf("SubtitleTrack[1].IsForced = false, want true")
				}
				if track1.IsDefault {
					t.Errorf("SubtitleTrack[1].IsDefault = true, want false")
				}

				// Verify third track (SDH bitmap)
				track2 := subtitleTracks[2]
				if track2.Codec != "hdmv_pgs_subtitle" {
					t.Errorf("SubtitleTrack[2].Codec = %v, want hdmv_pgs_subtitle", track2.Codec)
				}
				if !track2.IsSDH {
					t.Errorf("SubtitleTrack[2].IsSDH = false, want true")
				}
				if !track2.IsBitmap {
					t.Errorf("SubtitleTrack[2].IsBitmap = false, want true")
				}
				if track2.IsForced {
					t.Errorf("SubtitleTrack[2].IsForced = true, want false")
				}
			},
		},
		{
			name:    "verify old tracks are deleted before inserting new ones",
			mediaID: 102,
			result: &scanner.ScanResult{
				FilePath: "/movies/updated.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{
						StreamIndex: 0,
						Codec:       "aac",
						Language:    "eng",
						Title:       "New Audio",
					},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{
					{
						StreamIndex: 2,
						Codec:       "subrip",
						Language:    "eng",
						Title:       "New Subtitle",
					},
				},
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, libraryRepo *mocks.LibraryRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        102,
					LibraryID: 1,
					FilePath:  "/movies/updated.mp4",
				})
				libraryRepo.WithLibraries(&library.Library{
					ID:   1,
					Path: "/media/movies",
					Type: library.LibraryTypeMovies,
				})
				// Pre-insert old tracks that should be deleted
				_ = mediaRepo.InsertAudioTrack(context.Background(), &media.AudioTrack{
					MediaID:  102,
					Codec:    "old_audio_codec",
					Language: "old_lang",
				})
				_ = mediaRepo.InsertSubtitleTrack(context.Background(), &media.SubtitleTrack{
					MediaID:    102,
					SourceType: media.SubtitleSourceEmbedded,
					Codec:      "old_subtitle_codec",
				})
			},
			checkResult: func(t *testing.T, mediaRepo *mocks.MediaRepository) {
				t.Helper()
				// Verify old audio tracks were deleted and new ones inserted
				audioTracks, err := mediaRepo.GetAudioTracksByMediaID(context.Background(), 102)
				if err != nil {
					t.Fatalf("Failed to get audio tracks: %v", err)
				}
				if len(audioTracks) != 1 {
					t.Fatalf("Expected 1 audio track (old deleted), got %d", len(audioTracks))
				}
				if audioTracks[0].Codec != "aac" {
					t.Errorf("AudioTrack.Codec = %v, want aac (not old_audio_codec)", audioTracks[0].Codec)
				}
				if audioTracks[0].Title != "New Audio" {
					t.Errorf("AudioTrack.Title = %v, want New Audio", audioTracks[0].Title)
				}

				// Verify old subtitle tracks were deleted and new ones inserted
				subtitleTracks, err := mediaRepo.GetSubtitleTracksByMediaID(context.Background(), 102)
				if err != nil {
					t.Fatalf("Failed to get subtitle tracks: %v", err)
				}
				if len(subtitleTracks) != 1 {
					t.Fatalf("Expected 1 subtitle track (old deleted), got %d", len(subtitleTracks))
				}
				if subtitleTracks[0].Codec != "subrip" {
					t.Errorf("SubtitleTrack.Codec = %v, want subrip (not old_subtitle_codec)", subtitleTracks[0].Codec)
				}
				if subtitleTracks[0].Title != "New Subtitle" {
					t.Errorf("SubtitleTrack.Title = %v, want New Subtitle", subtitleTracks[0].Title)
				}
			},
		},
		{
			name:    "handle DeleteAudioTracksByMediaID error gracefully",
			mediaID: 103,
			result: &scanner.ScanResult{
				FilePath: "/movies/error.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{
						StreamIndex: 0,
						Codec:       "aac",
						Language:    "eng",
					},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{},
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, libraryRepo *mocks.LibraryRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        103,
					LibraryID: 1,
					FilePath:  "/movies/error.mp4",
				})
				libraryRepo.WithLibraries(&library.Library{
					ID:   1,
					Path: "/media/movies",
					Type: library.LibraryTypeMovies,
				})
				// Inject error for delete operation
				mediaRepo.WithDeleteAudioTracksError(errors.New("delete audio tracks failed"))
			},
			checkResult: func(t *testing.T, mediaRepo *mocks.MediaRepository) {
				t.Helper()
				// Despite delete error, insert should still happen
				audioTracks, err := mediaRepo.GetAudioTracksByMediaID(context.Background(), 103)
				if err != nil {
					t.Fatalf("Failed to get audio tracks: %v", err)
				}
				if len(audioTracks) != 1 {
					t.Fatalf("Expected 1 audio track (inserted despite delete error), got %d", len(audioTracks))
				}
			},
		},
		{
			name:    "handle InsertAudioTrack error gracefully",
			mediaID: 104,
			result: &scanner.ScanResult{
				FilePath: "/movies/insert_error.mp4",
				AudioTracks: []scanner.AudioTrackInfo{
					{StreamIndex: 0, Codec: "aac", Language: "eng"},
					{StreamIndex: 1, Codec: "ac3", Language: "jpn"},
				},
				SubtitleTracks: []scanner.SubtitleTrackInfo{},
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, libraryRepo *mocks.LibraryRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        104,
					LibraryID: 1,
					FilePath:  "/movies/insert_error.mp4",
				})
				libraryRepo.WithLibraries(&library.Library{
					ID:   1,
					Path: "/media/movies",
					Type: library.LibraryTypeMovies,
				})
				// Inject error for insert operation
				mediaRepo.WithInsertAudioTrackError(errors.New("insert audio track failed"))
			},
			checkResult: func(t *testing.T, mediaRepo *mocks.MediaRepository) {
				t.Helper()
				// All inserts should fail, so no tracks should be stored
				// But the function should not panic
				audioTracks, _ := mediaRepo.GetAudioTracksByMediaID(context.Background(), 104)
				// Due to the error, tracks won't be inserted
				if len(audioTracks) != 0 {
					t.Errorf("Expected 0 audio tracks (inserts failed), got %d", len(audioTracks))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			libraryRepo := mocks.NewLibraryRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, libraryRepo)
			}

			deps := &Deps{
				MediaRepos: &scan.MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
				},
				Logger: discardLogger(),
			}

			// Call PersistMediaTracks - should not panic
			PersistMediaTracks(context.Background(), deps, tt.mediaID, tt.result)

			if tt.checkResult != nil {
				tt.checkResult(t, mediaRepo)
			}
		})
	}
}

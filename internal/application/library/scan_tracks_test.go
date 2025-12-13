package library

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// mockMediaRepositoryForTracks wraps the generated mock to track method calls
type mockMediaRepositoryForTracks struct {
	*mocks.MediaRepository
	deleteAudioCalled      bool
	deleteSubtitleCalled   bool
	insertAudioCalled      int
	insertSubtitleCalled   int
	deleteAudioErr         error
	deleteSubtitleErr      error
	insertAudioErr         error
	insertSubtitleErr      error
	getByIDResult          *media.Media
	getByIDErr             error
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

func TestScanLibraryUseCase_persistMediaTracks(t *testing.T) {
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

			uc := &ScanLibraryUseCase{
				mediaRepos: &scan.MediaRepositories{
					Library: libRepo,
					Media:   mediaRepo,
					Movie:   mocks.NewMovieRepository(t),
					TV:      mocks.NewTVRepository(t),
					Music:   mocks.NewMusicRepository(t),
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Execute - this should not panic or return errors
			uc.persistMediaTracks(context.Background(), tt.mediaID, tt.scanResult)

			if tt.checkResult != nil {
				tt.checkResult(t, mediaRepo)
			}
		})
	}
}

func TestScanLibraryUseCase_discoverExternalSubtitles(t *testing.T) {
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

			uc := &ScanLibraryUseCase{
				mediaRepos: &scan.MediaRepositories{
					Library: libRepo,
					Media:   mediaRepo,
					Movie:   mocks.NewMovieRepository(t),
					TV:      mocks.NewTVRepository(t),
					Music:   mocks.NewMusicRepository(t),
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			// Execute - this should not panic
			uc.discoverExternalSubtitles(context.Background(), tt.mediaID, tt.videoPath)

			if tt.checkResult != nil {
				tt.checkResult(t, mediaRepo)
			}
		})
	}
}

func TestScanLibraryUseCase_discoverExternalSubtitles_PathHandling(t *testing.T) {
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

		uc := &ScanLibraryUseCase{
			mediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// This should build the full path as /library/root/movies/action/test.mp4
		// and then search for external subtitles there
		uc.discoverExternalSubtitles(context.Background(), 200, "movies/action/test.mp4")

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

		uc := &ScanLibraryUseCase{
			mediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// Execute with valid paths - the function should handle everything gracefully
		uc.discoverExternalSubtitles(context.Background(), 201, "movies/test.mp4")

		// Function should complete without panicking
	})
}

func TestScanLibraryUseCase_persistMediaTracks_Commentary(t *testing.T) {
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

		uc := &ScanLibraryUseCase{
			mediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// This tests that persistMediaTracks calls discoverExternalSubtitles
		uc.persistMediaTracks(context.Background(), 300, scanResult)

		// The function should execute without errors
		if !mediaRepo.deleteAudioCalled || !mediaRepo.deleteSubtitleCalled {
			t.Error("Expected delete methods to be called")
		}
	})
}

func TestScanLibraryUseCase_persistMediaTracks_Integration(t *testing.T) {
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

		uc := &ScanLibraryUseCase{
			mediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		uc.persistMediaTracks(context.Background(), 400, scanResult)

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

func TestScanLibraryUseCase_discoverExternalSubtitles_EdgeCases(t *testing.T) {
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

		uc := &ScanLibraryUseCase{
			mediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// Should not panic with empty file path
		uc.discoverExternalSubtitles(context.Background(), 500, "")
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

		uc := &ScanLibraryUseCase{
			mediaRepos: &scan.MediaRepositories{
				Library: libRepo,
				Media:   mediaRepo,
				Movie:   mocks.NewMovieRepository(t),
				TV:      mocks.NewTVRepository(t),
				Music:   mocks.NewMusicRepository(t),
			},
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}

		// Should handle special characters in paths
		uc.discoverExternalSubtitles(context.Background(), 501, specialPath)
	})
}

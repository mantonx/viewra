package media

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestProcessTVEpisode(t *testing.T) {
	season1 := 1
	episode5 := 5

	tests := []struct {
		name       string
		libraryID  int64
		result     *scanner.ScanResult
		setupRepo  func(*mocks.MediaRepository, *mocks.TVRepository)
		setupCache func(*sync.Map)
		checkRepo  func(*testing.T, *mocks.MediaRepository, *mocks.TVRepository)
	}{
		{
			name:      "create new TV episode",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/The Show/Season 01/The Show - S01E05 - Episode Title.mp4",
				Title:         "Episode Title",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2700,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode created, got %d", len(episodes))
				}
				for _, ep := range episodes {
					if ep.Media.Title != "Episode Title" {
						t.Errorf("Title = %v, want Episode Title", ep.Media.Title)
					}
					if ep.Season != 1 {
						t.Errorf("Season = %v, want 1", ep.Season)
					}
					if ep.Episode != 5 {
						t.Errorf("Episode = %v, want 5", ep.Episode)
					}
				}
			},
		},
		{
			name:      "update existing TV episode",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4",
				Title:         "Updated Episode",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2800,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        70,
					LibraryID: 2,
					FilePath:  "/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4",
					Title:     "Old Title",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        70,
						LibraryID: 2,
						FilePath:  "/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4",
						Title:     "Old Title",
					},
					Season:  1,
					Episode: 5,
				})
			},
			setupCache: func(cache *sync.Map) {
				cache.Store("/tv/My Show/Season 01/My Show - S01E05 - Updated Episode.mp4", int64(70))
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode updated, got %d", len(episodes))
				}
				for _, ep := range episodes {
					if ep.Media.ID != 70 {
						t.Errorf("ID = %v, want 70 (existing)", ep.Media.ID)
					}
					if ep.Media.Title != "Updated Episode" {
						t.Errorf("Title = %v, want Updated Episode", ep.Media.Title)
					}
				}
			},
		},
		{
			name:      "episode with nil season/episode numbers",
			libraryID: 2,
			result: &scanner.ScanResult{
				FilePath:      "/tv/Another Show/Season 1/Another Show - S01E01 - No Numbers.mp4",
				Title:         "No Numbers",
				SeasonNumber:  nil,
				EpisodeNumber: nil,
				Duration:      2700,
			},
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
			},
			checkRepo: func(t *testing.T, mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
				if len(episodes) != 1 {
					t.Errorf("Expected 1 episode created, got %d", len(episodes))
				}
				for _, ep := range episodes {
					// Parser will extract S01E01 from filename
					if ep.Season != 1 {
						t.Errorf("Season = %v, want 1 (parsed from filename)", ep.Season)
					}
					if ep.Episode != 1 {
						t.Errorf("Episode = %v, want 1 (parsed from filename)", ep.Episode)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
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

			_, _ = ProcessTVEpisode(context.Background(), deps, tt.libraryID, tt.result, checkpoint, existingMediaCache)

			if tt.checkRepo != nil {
				tt.checkRepo(t, mediaRepo, tvRepo)
			}
		})
	}
}

func TestProcessTVEpisode_WithShowTitle(t *testing.T) {
	season1 := 1
	episode1 := 1

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:      "/tv/New Show/Season 01/New Show - S01E01 - Pilot.mp4",
		Title:         "Pilot",
		SeasonNumber:  &season1,
		EpisodeNumber: &episode1,
		Duration:      2700,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	mediaID, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify media ID was returned
	if mediaID == nil {
		t.Errorf("Expected mediaID to be returned")
	}

	// Verify episode was created with correct data
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Errorf("Expected 1 episode created, got %d", len(episodes))
	}
	if len(episodes) > 0 {
		ep := episodes[0]
		if ep.Season != 1 {
			t.Errorf("Season = %d, want 1", ep.Season)
		}
		if ep.Episode != 1 {
			t.Errorf("Episode = %d, want 1", ep.Episode)
		}
	}
}

func TestProcessMultiEpisodeFile(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02 - Double Episode.mp4",
		Title:    "Double Episode",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "multi-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"Double Episode",
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify both episodes were created
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 2 {
		t.Errorf("Expected 2 episodes created for multi-episode file, got %d", len(episodes))
	}

	// Verify episodes have correct numbers
	episodeNums := make(map[int]bool)
	for _, ep := range episodes {
		episodeNums[ep.Episode] = true
	}
	if !episodeNums[1] || !episodeNums[2] {
		t.Errorf("Expected episodes 1 and 2, got %v", episodeNums)
	}
}

func TestProcessMultiEpisodeFile_VirtualPaths(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E03 - Triple.mp4",
		Title:    "Triple",
		Duration: 8100,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 3072,
		FileHash: "triple-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		3,
		"Triple",
	)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 3 {
		t.Fatalf("Expected 3 episodes, got %d", len(episodes))
	}

	// First episode should have real path, others should have virtual paths
	pathCounts := make(map[string]int)
	for _, ep := range episodes {
		if ep.Media.FilePath == result.FilePath {
			pathCounts["real"]++
		} else if len(ep.Media.FilePath) > len(result.FilePath) {
			pathCounts["virtual"]++
		}
	}

	if pathCounts["real"] != 1 {
		t.Errorf("Expected 1 episode with real path, got %d", pathCounts["real"])
	}
	if pathCounts["virtual"] != 2 {
		t.Errorf("Expected 2 episodes with virtual paths, got %d", pathCounts["virtual"])
	}
}

func TestProcessTVEpisode_MultiEpisodeDetection(t *testing.T) {
	season1 := 1
	episode1 := 1

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	// Multi-episode file
	result := &scanner.ScanResult{
		FilePath:      "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4",
		Title:         "Double",
		SeasonNumber:  &season1,
		EpisodeNumber: &episode1,
		Duration:      5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "multi-hash",
	}
	existingMediaCache := &sync.Map{}

	mediaID, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if mediaID == nil {
		t.Error("Expected mediaID to be returned for multi-episode file")
	}

	// Should create multiple episodes
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) < 2 {
		t.Errorf("Expected at least 2 episodes for multi-episode file, got %d", len(episodes))
	}
}

func TestProcessMultiEpisodeFile_CacheHit(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Pre-populate existing episodes and media
	mediaRepo.WithMedia(
		&media.Media{
			ID:        100,
			LibraryID: 2,
			FilePath:  "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4",
			Type:      "tv_episode",
		},
		&media.Media{
			ID:        101,
			LibraryID: 2,
			FilePath:  "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4#ep2",
			Type:      "tv_episode",
		},
	)
	tvRepo.WithEpisodes(
		&media.TVEpisode{
			Media: media.Media{
				ID:        100,
				LibraryID: 2,
				FilePath:  "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4",
				Type:      "tv_episode",
			},
			ShowTitle: "Show",
			Season:    1,
			Episode:   1,
		},
		&media.TVEpisode{
			Media: media.Media{
				ID:        101,
				LibraryID: 2,
				FilePath:  "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4#ep2",
				Type:      "tv_episode",
			},
			ShowTitle: "Show",
			Season:    1,
			Episode:   2,
		},
	)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4",
		Title:    "Updated Title",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "multi-hash",
	}

	// Pre-populate cache with existing episodes
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4", int64(100))
	existingMediaCache.Store("/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4#ep2", int64(101))

	mediaID, err := ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"Updated Title",
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if mediaID == nil {
		t.Error("Expected non-nil mediaID")
	}
	if mediaID != nil && *mediaID != 100 {
		t.Errorf("Expected first mediaID to be 100, got %d", *mediaID)
	}
}

func TestProcessMultiEpisodeFile_UpdateError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Setup update error on media repo
	mediaRepo.UpdateErr = errors.New("update failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02.mp4",
		Title:    "Test",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "hash",
	}

	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/tv/Show/Season 01/Show - S01E01-E02.mp4", int64(100))

	// Should not panic on update error
	ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"Test",
	)
}

func TestProcessMultiEpisodeFile_TVUpdateError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Setup TV update error
	tvRepo.UpdateErr = errors.New("tv update failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02.mp4",
		Title:    "Test",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "hash",
	}

	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/tv/Show/Season 01/Show - S01E01-E02.mp4", int64(100))

	// Should not panic on TV update error
	ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"Test",
	)
}

func TestProcessMultiEpisodeFile_CreateError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Setup create error (non-constraint)
	tvRepo.CreateErr = errors.New("create failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02.mp4",
		Title:    "Test",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "hash",
	}

	existingMediaCache := &sync.Map{}

	// Should not panic on create error
	ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"Test",
	)
}

func TestProcessMultiEpisodeFile_ConstraintError(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Setup constraint error
	tvRepo.CreateErr = errors.New("UNIQUE constraint failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02.mp4",
		Title:    "Test",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "hash",
	}

	existingMediaCache := &sync.Map{}

	// Should handle constraint error gracefully (skip duplicate)
	mediaID, err := ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"Test",
	)

	// No error returned, but no mediaID either since all creates failed
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if mediaID != nil {
		t.Errorf("Expected nil mediaID when all creates fail with constraint error")
	}
}

func TestProcessMultiEpisodeFile_NoEpisodeTitle(t *testing.T) {
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath: "/tv/Show/Season 01/Show - S01E01-E02.mp4",
		Title:    "",
		Duration: 5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "hash",
	}

	existingMediaCache := &sync.Map{}

	// Test with empty episode title - should not add "Part X" suffix
	mediaID, err := ProcessMultiEpisodeFile(
		context.Background(),
		deps,
		2,
		result,
		checkpoint,
		existingMediaCache,
		"Show",
		1,
		1,
		2,
		"", // empty episode title
	)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if mediaID == nil {
		t.Error("Expected non-nil mediaID")
	}

	// Verify episodes were created with empty titles (no Part suffix)
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	for _, ep := range episodes {
		if ep.EpisodeTitle != "" {
			t.Errorf("Expected empty episode title, got %q", ep.EpisodeTitle)
		}
	}
}

func TestProcessTVEpisode_SkipsAudioFiles(t *testing.T) {
	audioExtensions := []string{".mp3", ".flac", ".m4a", ".wav", ".ogg"}

	for _, ext := range audioExtensions {
		t.Run("skips "+ext, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)

			mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath: "/tv/Show/Season 01/theme" + ext,
				Title:    "Theme",
				Duration: 180,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}

			mediaID, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

			if err != nil {
				t.Errorf("Expected no error for audio file, got %v", err)
			}
			if mediaID != nil {
				t.Errorf("Expected nil mediaID for skipped audio file, got %v", *mediaID)
			}

			// Verify no episode was created
			episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
			if len(episodes) != 0 {
				t.Errorf("Expected 0 episodes for audio file, got %d", len(episodes))
			}
		})
	}
}

func TestProcessTVEpisode_NilCheckpoint(t *testing.T) {
	season1 := 1
	episode1 := 1

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:      "/tv/Show/Season 01/Show - S01E01.mp4",
		Title:         "Pilot",
		SeasonNumber:  &season1,
		EpisodeNumber: &episode1,
		Duration:      2700,
	}
	existingMediaCache := &sync.Map{}

	// Pass nil checkpoint - should handle gracefully
	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, nil, existingMediaCache)

	if err != nil {
		t.Errorf("Expected no error with nil checkpoint, got %v", err)
	}

	// Verify episode was created
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Errorf("Expected 1 episode created, got %d", len(episodes))
	}
}

func TestProcessTVEpisode_ParseFallback(t *testing.T) {
	// Test when ShowName is empty and we need to fallback to parsing
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		// ShowName is empty - will need to parse
		FilePath: "/tv/Breaking Bad/Season 01/Breaking Bad - S01E01 - Pilot.mkv",
		Title:    "Pilot",
		ShowName: "", // Empty to trigger fallback
		Duration: 2700,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	mediaID, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if mediaID == nil {
		t.Error("Expected mediaID to be returned")
	}

	// Verify episode was created with parsed show name
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Errorf("Expected 1 episode created, got %d", len(episodes))
	}
	if len(episodes) > 0 {
		if episodes[0].ShowTitle != "Breaking Bad" {
			t.Errorf("ShowTitle = %q, want Breaking Bad", episodes[0].ShowTitle)
		}
	}
}

func TestProcessTVEpisode_VideoMetadataFields(t *testing.T) {
	season1 := 1
	episode1 := 1

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:        "/tv/Show/Season 01/Show - S01E01 - Pilot.mkv",
		Title:           "Pilot",
		SeasonNumber:    &season1,
		EpisodeNumber:   &episode1,
		Duration:        2700,
		Width:           1920,
		Height:          1080,
		VideoCodec:      "hevc",
		AudioCodec:      "truehd",
		Bitrate:         15000000,
		FrameRate:       23.976,
		ContainerFormat: "mkv",
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2147483648, // 2GB
		FileHash: "episode-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Fatalf("Expected 1 episode, got %d", len(episodes))
	}

	ep := episodes[0]
	if ep.Media.Width != 1920 {
		t.Errorf("Width = %v, want 1920", ep.Media.Width)
	}
	if ep.Media.Height != 1080 {
		t.Errorf("Height = %v, want 1080", ep.Media.Height)
	}
	if ep.Media.VideoCodec != "hevc" {
		t.Errorf("VideoCodec = %v, want hevc", ep.Media.VideoCodec)
	}
	if ep.Media.AudioCodec != "truehd" {
		t.Errorf("AudioCodec = %v, want truehd", ep.Media.AudioCodec)
	}
	if ep.Media.Bitrate != 15000000 {
		t.Errorf("Bitrate = %v, want 15000000", ep.Media.Bitrate)
	}
	if ep.Media.FrameRate != 23.976 {
		t.Errorf("FrameRate = %v, want 23.976", ep.Media.FrameRate)
	}
	if ep.Media.ContainerFormat != "mkv" {
		t.Errorf("ContainerFormat = %v, want mkv", ep.Media.ContainerFormat)
	}
	if ep.Media.FileSize != 2147483648 {
		t.Errorf("FileSize = %v, want 2147483648", ep.Media.FileSize)
	}
	if ep.Media.FileHash != "episode-hash" {
		t.Errorf("FileHash = %v, want episode-hash", ep.Media.FileHash)
	}
}

func TestProcessTVEpisode_IsExtraDetection(t *testing.T) {
	tests := []struct {
		name      string
		filePath  string
		wantExtra bool
	}{
		{
			name:      "regular episode",
			filePath:  "/tv/Show/Season 01/Show - S01E01 - Pilot.mkv",
			wantExtra: false,
		},
		{
			name:      "trailer file with episode pattern",
			filePath:  "/tv/Show/Season 01/Show - S01E01-trailer.mp4",
			wantExtra: true,
		},
		{
			name:      "deleted scenes with episode pattern",
			filePath:  "/tv/Show/Season 01/Show - S01E01-deleted.mp4",
			wantExtra: true,
		},
		{
			name:      "featurette with episode pattern",
			filePath:  "/tv/Show/Season 01/Show - S01E01-featurette.mp4",
			wantExtra: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			season1 := 1
			episode1 := 1

			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)

			mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath:      tt.filePath,
				Title:         "Test",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode1,
				Duration:      2700,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}

			_, _ = ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

			episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
			if len(episodes) != 1 {
				t.Fatalf("Expected 1 episode, got %d", len(episodes))
			}

			if episodes[0].Media.IsExtra != tt.wantExtra {
				t.Errorf("IsExtra = %v, want %v for %s", episodes[0].Media.IsExtra, tt.wantExtra, tt.filePath)
			}
		})
	}
}

func TestProcessTVEpisode_EpisodeEndNumber(t *testing.T) {
	season1 := 1
	episode1 := 1
	episodeEnd2 := 2

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	// Test with EpisodeEndNumber set - should create multi-episode records
	result := &scanner.ScanResult{
		FilePath:         "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4",
		Title:            "Double Feature",
		SeasonNumber:     &season1,
		EpisodeNumber:    &episode1,
		EpisodeEndNumber: &episodeEnd2,
		Duration:         5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "multi-hash",
	}
	existingMediaCache := &sync.Map{}

	mediaID, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if mediaID == nil {
		t.Error("Expected mediaID to be returned")
	}

	// Should create 2 episode records
	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) < 2 {
		t.Errorf("Expected at least 2 episodes for multi-episode file with EpisodeEndNumber, got %d", len(episodes))
	}
}

func TestProcessTVEpisode_NFOMetadata(t *testing.T) {
	// NFO metadata enrichment is now handled asynchronously by the enrichment pipeline.
	// See internal/application/enrichment/builtin/nfo.go for the NFO enricher tests.
	t.Skip("NFO parsing moved to async enrichment pipeline")

	// Create temp directory for NFO files
	tmpDir := t.TempDir()

	// Create episode file and NFO file
	episodePath := tmpDir + "/Show/Season 01/Show - S01E05 - Episode Title.mkv"
	nfoPath := tmpDir + "/Show/Season 01/Show - S01E05 - Episode Title.nfo"

	if err := os.MkdirAll(tmpDir+"/Show/Season 01", 0755); err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	if err := os.WriteFile(episodePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create episode file: %v", err)
	}

	// Create NFO file with metadata
	nfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<episodedetails>
  <title>NFO Episode Title</title>
  <showtitle>NFO Show Title</showtitle>
  <season>1</season>
  <episode>5</episode>
  <plot>This is the episode plot from NFO.</plot>
  <aired>2024-05-15</aired>
  <runtime>45</runtime>
  <uniqueid type="imdb">tt9876543</uniqueid>
  <uniqueid type="tvdb">123456</uniqueid>
</episodedetails>`

	if err := os.WriteFile(nfoPath, []byte(nfoContent), 0644); err != nil {
		t.Fatalf("Failed to create NFO file: %v", err)
	}

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	season1 := 1
	episode5 := 5
	result := &scanner.ScanResult{
		FilePath:      episodePath,
		Title:         "Scan Result Title", // Should be overridden by NFO
		ShowName:      "Scan Show Name",
		SeasonNumber:  &season1,
		EpisodeNumber: &episode5,
		Duration:      2700,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: episodePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Fatalf("Expected 1 episode, got %d", len(episodes))
	}

	ep := episodes[0]
	if ep.EpisodeTitle != "NFO Episode Title" {
		t.Errorf("EpisodeTitle = %v, want 'NFO Episode Title'", ep.EpisodeTitle)
	}
	if ep.ShowTitle != "NFO Show Title" {
		t.Errorf("ShowTitle = %v, want 'NFO Show Title'", ep.ShowTitle)
	}
	if ep.Description != "This is the episode plot from NFO." {
		t.Errorf("Description = %v, want 'This is the episode plot from NFO.'", ep.Description)
	}
	if ep.AirDate != "2024-05-15" {
		t.Errorf("AirDate = %v, want '2024-05-15'", ep.AirDate)
	}
	// Runtime 45 min * 60 = 2700 seconds
	if ep.Media.Duration != 2700 {
		t.Errorf("Duration = %v, want 2700", ep.Media.Duration)
	}
}

func TestProcessTVEpisode_FallbackParser(t *testing.T) {
	// Test fallback parsing when ShowName is empty
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	season1 := 1
	episode3 := 3
	result := &scanner.ScanResult{
		FilePath:      "/tv/Breaking Bad/Season 01/Breaking Bad - S01E03 - Episode Title.mp4",
		Title:         "Episode Title",
		ShowName:      "", // Empty - should trigger fallback parser
		SeasonNumber:  &season1,
		EpisodeNumber: &episode3,
		Duration:      2700,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Fatalf("Expected 1 episode, got %d", len(episodes))
	}

	// Fallback parser should extract ShowName from path
	ep := episodes[0]
	if ep.ShowTitle == "" {
		t.Error("Expected ShowTitle to be populated by fallback parser")
	}
}

func TestProcessTVEpisode_FallbackParserNoSeasonEpisode(t *testing.T) {
	// Test fallback parsing when ShowName, Season, and Episode are all empty/zero
	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:      "/tv/The Office/Season 02/The Office - S02E07 - The Client.mp4",
		Title:         "The Client",
		ShowName:      "", // Empty - should trigger fallback parser
		SeasonNumber:  nil,
		EpisodeNumber: nil,
		Duration:      2700,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	episodes, _ := tvRepo.ListTVEpisodesByLibrary(context.Background(), 2)
	if len(episodes) != 1 {
		t.Fatalf("Expected 1 episode, got %d", len(episodes))
	}

	// Fallback parser should extract all info from path
	ep := episodes[0]
	if ep.Season != 2 {
		t.Errorf("Season = %v, want 2", ep.Season)
	}
	if ep.Episode != 7 {
		t.Errorf("Episode = %v, want 7", ep.Episode)
	}
}

func TestProcessTVEpisode_UpdateErrors(t *testing.T) {
	season1 := 1
	episode5 := 5

	tests := []struct {
		name        string
		setupRepo   func(*mocks.MediaRepository, *mocks.TVRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name: "media update error",
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        100,
					LibraryID: 2,
					FilePath:  "/tv/Show/Season 01/Show - S01E05 - Test.mp4",
				})
				mediaRepo.UpdateErr = errors.New("media update failed")
			},
			expectError: true,
			errorMsg:    "failed to update base media record",
		},
		{
			name: "episode update error",
			setupRepo: func(mediaRepo *mocks.MediaRepository, tvRepo *mocks.TVRepository) {
				mediaRepo.WithMedia(&media.Media{
					ID:        100,
					LibraryID: 2,
					FilePath:  "/tv/Show/Season 01/Show - S01E05 - Test.mp4",
				})
				tvRepo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        100,
						LibraryID: 2,
						FilePath:  "/tv/Show/Season 01/Show - S01E05 - Test.mp4",
					},
				})
				tvRepo.UpdateErr = errors.New("episode update failed")
			},
			expectError: true,
			errorMsg:    "failed to update TV episode metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mediaRepo := mocks.NewMediaRepository(t)
			tvRepo := mocks.NewTVRepository(t)

			if tt.setupRepo != nil {
				tt.setupRepo(mediaRepo, tvRepo)
			}

			mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
			scanRepos := testScanRepos(t)
			deps := testDeps(t, mediaRepos, scanRepos)

			result := &scanner.ScanResult{
				FilePath:      "/tv/Show/Season 01/Show - S01E05 - Test.mp4",
				Title:         "Test",
				ShowName:      "Show",
				SeasonNumber:  &season1,
				EpisodeNumber: &episode5,
				Duration:      2700,
			}

			checkpoint := &scanner.ScanCheckpoint{
				FilePath: result.FilePath,
				FileSize: 1024,
				FileHash: "test-hash",
			}
			existingMediaCache := &sync.Map{}
			existingMediaCache.Store(result.FilePath, int64(100))

			_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			}
		})
	}
}

func TestProcessTVEpisode_CreateError(t *testing.T) {
	season1 := 1
	episode5 := 5

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)
	tvRepo.CreateErr = errors.New("create episode failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:      "/tv/Show/Season 01/Show - S01E05 - Test.mp4",
		Title:         "Test",
		ShowName:      "Show",
		SeasonNumber:  &season1,
		EpisodeNumber: &episode5,
		Duration:      2700,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 1024,
		FileHash: "test-hash",
	}
	existingMediaCache := &sync.Map{}

	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

	if err == nil {
		t.Error("Expected error but got none")
	}
	if !contains(err.Error(), "failed to create TV episode") {
		t.Errorf("Expected error containing 'failed to create TV episode', got: %v", err)
	}
}

func TestProcessMultiEpisodeFile_UpdateExisting(t *testing.T) {
	season1 := 1
	episode1 := 1
	episodeEnd3 := 3

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Pre-populate existing episodes
	mediaRepo.WithMedia(&media.Media{
		ID:        200,
		LibraryID: 2,
		FilePath:  "/tv/Show/Season 01/Show - S01E01-E03 - Triple.mp4",
	})
	tvRepo.WithEpisodes(&media.TVEpisode{
		Media: media.Media{
			ID:        200,
			LibraryID: 2,
			FilePath:  "/tv/Show/Season 01/Show - S01E01-E03 - Triple.mp4",
		},
	})

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:         "/tv/Show/Season 01/Show - S01E01-E03 - Triple.mp4",
		Title:            "Triple Feature",
		ShowName:         "Show",
		SeasonNumber:     &season1,
		EpisodeNumber:    &episode1,
		EpisodeEndNumber: &episodeEnd3,
		Duration:         8100,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 3072,
		FileHash: "triple-hash",
	}

	// Pre-populate cache with first episode
	existingMediaCache := &sync.Map{}
	existingMediaCache.Store("/tv/Show/Season 01/Show - S01E01-E03 - Triple.mp4", int64(200))

	mediaID, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if mediaID == nil {
		t.Error("Expected mediaID to be returned")
	}
}

func TestProcessMultiEpisodeFile_ConstraintErrorViaProcessTVEpisode(t *testing.T) {
	season1 := 1
	episode1 := 1
	episodeEnd2 := 2

	mediaRepo := mocks.NewMediaRepository(t)
	tvRepo := mocks.NewTVRepository(t)

	// Simulate constraint error on second episode
	tvRepo.CreateErr = errors.New("UNIQUE constraint failed")

	mediaRepos := testMediaRepos(t, mediaRepo, nil, tvRepo, nil)
	scanRepos := testScanRepos(t)
	deps := testDeps(t, mediaRepos, scanRepos)

	result := &scanner.ScanResult{
		FilePath:         "/tv/Show/Season 01/Show - S01E01-E02 - Double.mp4",
		Title:            "Double Feature",
		ShowName:         "Show",
		SeasonNumber:     &season1,
		EpisodeNumber:    &episode1,
		EpisodeEndNumber: &episodeEnd2,
		Duration:         5400,
	}

	checkpoint := &scanner.ScanCheckpoint{
		FilePath: result.FilePath,
		FileSize: 2048,
		FileHash: "double-hash",
	}
	existingMediaCache := &sync.Map{}

	// Should not error - constraint errors are handled gracefully
	_, err := ProcessTVEpisode(context.Background(), deps, 2, result, checkpoint, existingMediaCache)

	if err != nil {
		t.Errorf("Expected no error (constraint errors handled), got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

package library

import (
	"context"
	"errors"
	"testing"

	domainLibrary "github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// TestPersistMediaTracks_WithEnhancedMock demonstrates using the enhanced mock repository
// to test persistMediaTracks with actual track storage verification
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
				libraryRepo.WithLibraries(&domainLibrary.Library{
					ID:   1,
					Path: "/media/movies",
					Type: domainLibrary.LibraryTypeMovies,
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
				libraryRepo.WithLibraries(&domainLibrary.Library{
					ID:   1,
					Path: "/media/movies",
					Type: domainLibrary.LibraryTypeMovies,
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
				libraryRepo.WithLibraries(&domainLibrary.Library{
					ID:   1,
					Path: "/media/movies",
					Type: domainLibrary.LibraryTypeMovies,
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
				libraryRepo.WithLibraries(&domainLibrary.Library{
					ID:   1,
					Path: "/media/movies",
					Type: domainLibrary.LibraryTypeMovies,
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
				libraryRepo.WithLibraries(&domainLibrary.Library{
					ID:   1,
					Path: "/media/movies",
					Type: domainLibrary.LibraryTypeMovies,
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

			uc := &ScanLibraryUseCase{
				mediaRepos: &MediaRepositories{
					Library: libraryRepo,
					Media:   mediaRepo,
				},
				logger: discardLogger(),
			}

			// Call persistMediaTracks - should not panic
			uc.persistMediaTracks(context.Background(), tt.mediaID, tt.result)

			if tt.checkResult != nil {
				tt.checkResult(t, mediaRepo)
			}
		})
	}
}

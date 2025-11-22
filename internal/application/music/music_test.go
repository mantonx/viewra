package music

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestGetTrackUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		trackID   int64
		setupRepo func(*mocks.MusicRepository)
		wantErr   bool
	}{
		{
			name:    "successful get track",
			trackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Bohemian Rhapsody",
						FilePath:  "music/queen/bohemian-rhapsody.mp3",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Artist:      "Queen",
					Album:       "A Night at the Opera",
					AlbumArtist: "Queen",
					Year:        1975,
					TrackNumber: 11,
				})
			},
			wantErr: false,
		},
		{
			name:    "track not found",
			trackID: 999,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Empty repository
			},
			wantErr: true,
		},
		{
			name:    "repository error",
			trackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMusicRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewGetTrackUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.trackID)

			if tt.wantErr {
				if err == nil {
					t.Error("Execute() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp == nil {
				t.Error("Execute() returned nil response")
				return
			}

			if resp.ID != tt.trackID {
				t.Errorf("Execute() got ID = %d, want %d", resp.ID, tt.trackID)
			}
		})
	}
}

func TestListArtistsUseCase_Execute(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		setupRepo  func(*mocks.MusicRepository)
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "successful list artists",
			libraryID: 100,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Add tracks from multiple artists
				repo.WithTracks(
					&media.MusicTrack{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Song 1",
							FilePath:  "music/artist1/song1.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      "Artist 1",
						AlbumArtist: "Artist 1",
						Album:       "Album 1",
					},
					&media.MusicTrack{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "Song 2",
							FilePath:  "music/artist2/song2.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      "Artist 2",
						AlbumArtist: "Artist 2",
						Album:       "Album 2",
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "empty library",
			libraryID: 100,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Empty repository
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "count error",
			libraryID: 100,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.CountErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMusicRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListArtistsUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Error("Execute() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("Execute() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Artists) != tt.wantCount {
				t.Errorf("Execute() got %d artists, want %d", len(resp.Artists), tt.wantCount)
			}
		})
	}
}

func TestListArtistsUseCase_ExecuteWithPagination(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		pagination *common.PaginationParams
		setupRepo  func(*mocks.MusicRepository)
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "paginated list - first 2 artists",
			libraryID: 100,
			pagination: &common.PaginationParams{
				Limit:  2,
				Offset: 0,
			},
			setupRepo: func(repo *mocks.MusicRepository) {
				// Create 5 different artists
				for i := 1; i <= 5; i++ {
					artistName := fmt.Sprintf("Artist %d", i)
					repo.WithTracks(&media.MusicTrack{
						Media: media.Media{
							ID:        int64(i),
							LibraryID: 100,
							Title:     "Song",
							FilePath:  "music/song.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      artistName,
						AlbumArtist: artistName,
						Album:       "Album",
					})
				}
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:       "nil pagination uses defaults",
			libraryID:  100,
			pagination: nil,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Song",
						FilePath:  "music/song.mp3",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Artist:      "Artist",
					AlbumArtist: "Artist",
					Album:       "Album",
				})
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "count error",
			libraryID: 100,
			pagination: &common.PaginationParams{
				Limit:  10,
				Offset: 0,
			},
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.CountErr = errors.New("count error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMusicRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListArtistsUseCase(repo)
			resp, err := uc.ExecuteWithPagination(context.Background(), tt.libraryID, tt.pagination)

			if tt.wantErr {
				if err == nil {
					t.Error("ExecuteWithPagination() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteWithPagination() unexpected error = %v", err)
				return
			}

			if len(resp.Artists) != tt.wantCount {
				t.Errorf("ExecuteWithPagination() got %d artists, want %d", len(resp.Artists), tt.wantCount)
			}
		})
	}
}

func TestSearchTracksUseCase_Execute(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		query      string
		setupRepo  func(*mocks.MusicRepository)
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "successful search by title",
			libraryID: 100,
			query:     "Bohemian",
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.WithTracks(
					&media.MusicTrack{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Bohemian Rhapsody",
							FilePath:  "music/queen/bohemian.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist: "Queen",
						Album:  "A Night at the Opera",
					},
					&media.MusicTrack{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "Another Song",
							FilePath:  "music/other/song.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist: "Other Artist",
						Album:  "Other Album",
					},
				)
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:       "empty query",
			libraryID:  100,
			query:      "",
			setupRepo:  func(repo *mocks.MusicRepository) {},
			wantCount:  0,
			wantErr:    true,
		},
		{
			name:      "no results",
			libraryID: 100,
			query:     "NonExistent",
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Song",
						FilePath:  "music/song.mp3",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Artist: "Artist",
					Album:  "Album",
				})
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "repository error",
			libraryID: 100,
			query:     "test",
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.SearchErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMusicRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewSearchTracksUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.libraryID, tt.query)

			if tt.wantErr {
				if err == nil {
					t.Error("Execute() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("Execute() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Tracks) != tt.wantCount {
				t.Errorf("Execute() got %d tracks, want %d", len(resp.Tracks), tt.wantCount)
			}
		})
	}
}

func TestListAlbumsByArtistIDUseCase_Execute(t *testing.T) {
	tests := []struct {
		name          string
		artistTrackID int64
		setupRepo     func(*mocks.MusicRepository)
		wantCount     int
		wantErr       bool
	}{
		{
			name:          "successful list albums for artist",
			artistTrackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Add tracks from same artist across multiple albums
				repo.WithTracks(
					&media.MusicTrack{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Song 1",
							FilePath:  "music/artist/album1/song1.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      "Queen",
						AlbumArtist: "Queen",
						Album:       "Album 1",
						Year:        1975,
					},
					&media.MusicTrack{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "Song 2",
							FilePath:  "music/artist/album2/song2.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      "Queen",
						AlbumArtist: "Queen",
						Album:       "Album 2",
						Year:        1976,
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:          "track not found",
			artistTrackID: 999,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Empty repository
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:          "track has no artist information",
			artistTrackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Song",
						FilePath:  "music/song.mp3",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Artist:      "", // No artist
					AlbumArtist: "", // No album artist
					Album:       "Album",
				})
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:          "repository error on get track",
			artistTrackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMusicRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListAlbumsByArtistIDUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.artistTrackID)

			if tt.wantErr {
				if err == nil {
					t.Error("Execute() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("Execute() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Albums) != tt.wantCount {
				t.Errorf("Execute() got %d albums, want %d", len(resp.Albums), tt.wantCount)
			}
		})
	}
}

func TestListTracksByAlbumIDUseCase_Execute(t *testing.T) {
	tests := []struct {
		name         string
		albumTrackID int64
		setupRepo    func(*mocks.MusicRepository)
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "successful list tracks for album",
			albumTrackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Add multiple tracks from same album
				repo.WithTracks(
					&media.MusicTrack{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Track 1",
							FilePath:  "music/album/track1.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      "Queen",
						AlbumArtist: "Queen",
						Album:       "A Night at the Opera",
						TrackNumber: 1,
					},
					&media.MusicTrack{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "Track 2",
							FilePath:  "music/album/track2.mp3",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						Artist:      "Queen",
						AlbumArtist: "Queen",
						Album:       "A Night at the Opera",
						TrackNumber: 2,
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:         "track not found",
			albumTrackID: 999,
			setupRepo: func(repo *mocks.MusicRepository) {
				// Empty repository
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:         "track has no album information",
			albumTrackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.WithTracks(&media.MusicTrack{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Song",
						FilePath:  "music/song.mp3",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Artist: "Artist",
					Album:  "", // No album
				})
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:         "repository error on get track",
			albumTrackID: 1,
			setupRepo: func(repo *mocks.MusicRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewMusicRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListTracksByAlbumIDUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.albumTrackID)

			if tt.wantErr {
				if err == nil {
					t.Error("Execute() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("Execute() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Tracks) != tt.wantCount {
				t.Errorf("Execute() got %d tracks, want %d", len(resp.Tracks), tt.wantCount)
			}
		})
	}
}

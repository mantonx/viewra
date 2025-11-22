package tv

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/media"
	domainprogress "github.com/mantonx/viewra/internal/domain/progress"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestGetTVEpisodeUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		episodeID int64
		setupRepo func(*mocks.TVRepository)
		wantErr   bool
	}{
		{
			name:      "successful get episode",
			episodeID: 1,
			setupRepo: func(repo *mocks.TVRepository) {
				repo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Breaking Bad S01E01",
						FilePath:  "tv/breaking-bad/s01e01.mp4",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					ShowTitle:    "Breaking Bad",
					EpisodeTitle: "Pilot",
					Season:       1,
					Episode:      1,
				})
			},
			wantErr: false,
		},
		{
			name:      "episode not found",
			episodeID: 999,
			setupRepo: func(repo *mocks.TVRepository) {
				// Empty repository
			},
			wantErr: true,
		},
		{
			name:      "repository error",
			episodeID: 1,
			setupRepo: func(repo *mocks.TVRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewTVRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewGetTVEpisodeUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.episodeID)

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

			if resp.ID != tt.episodeID {
				t.Errorf("Execute() got ID = %d, want %d", resp.ID, tt.episodeID)
			}
		})
	}
}

func TestListTVEpisodesUseCase_ExecuteByShow(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		showTitle  string
		setupRepo  func(*mocks.TVRepository)
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "successful list episodes by show",
			libraryID: 100,
			showTitle: "Breaking Bad",
			setupRepo: func(repo *mocks.TVRepository) {
				repo.WithEpisodes(
					&media.TVEpisode{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Breaking Bad S01E01",
							FilePath:  "tv/breaking-bad/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Breaking Bad",
						EpisodeTitle: "Pilot",
						Season:       1,
						Episode:      1,
					},
					&media.TVEpisode{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "Breaking Bad S01E02",
							FilePath:  "tv/breaking-bad/s01e02.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Breaking Bad",
						EpisodeTitle: "Cat's in the Bag...",
						Season:       1,
						Episode:      2,
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "no episodes found for show",
			libraryID: 100,
			showTitle: "NonExistent",
			setupRepo: func(repo *mocks.TVRepository) {
				repo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Breaking Bad S01E01",
						FilePath:  "tv/breaking-bad/s01e01.mp4",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					ShowTitle:    "Breaking Bad",
					EpisodeTitle: "Pilot",
					Season:       1,
					Episode:      1,
				})
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "repository error",
			libraryID: 100,
			showTitle: "Breaking Bad",
			setupRepo: func(repo *mocks.TVRepository) {
				repo.ListErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewTVRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListTVEpisodesUseCase(repo)
			resp, err := uc.ExecuteByShow(context.Background(), tt.libraryID, tt.showTitle)

			if tt.wantErr {
				if err == nil {
					t.Error("ExecuteByShow() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteByShow() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("ExecuteByShow() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Episodes) != tt.wantCount {
				t.Errorf("ExecuteByShow() got %d episodes, want %d", len(resp.Episodes), tt.wantCount)
			}
		})
	}
}

func TestListTVEpisodesUseCase_ExecuteByShowID(t *testing.T) {
	tests := []struct {
		name      string
		showID    int64
		setupRepo func(*mocks.TVRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "successful list episodes by show ID",
			showID: 1,
			setupRepo: func(repo *mocks.TVRepository) {
				// Add the show first
				repo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 100,
					Title:     "Test Show",
				})
				// Then add episodes for that show
				repo.WithEpisodes(
					&media.TVEpisode{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Episode 1",
							FilePath:  "tv/show/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
								ShowTitle:    "Test Show",
						EpisodeTitle: "Pilot",
						Season:       1,
						Episode:      1,
					},
					&media.TVEpisode{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "Episode 2",
							FilePath:  "tv/show/s01e02.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
								ShowTitle:    "Test Show",
						EpisodeTitle: "Episode 2",
						Season:       1,
						Episode:      2,
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:   "no episodes for show ID",
			showID: 999,
			setupRepo: func(repo *mocks.TVRepository) {
				// Add a show with ID=1
				repo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 100,
					Title:     "Test Show",
				})
				// Episode belongs to show ID=1, not 999
				repo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Episode 1",
						FilePath:  "tv/show/s01e01.mp4",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					ShowTitle:    "Test Show",
					EpisodeTitle: "Pilot",
					Season:       1,
					Episode:      1,
				})
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:   "repository error",
			showID: 1,
			setupRepo: func(repo *mocks.TVRepository) {
				repo.ListErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewTVRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListTVEpisodesUseCase(repo)
			resp, err := uc.ExecuteByShowID(context.Background(), tt.showID)

			if tt.wantErr {
				if err == nil {
					t.Error("ExecuteByShowID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteByShowID() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("ExecuteByShowID() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Episodes) != tt.wantCount {
				t.Errorf("ExecuteByShowID() got %d episodes, want %d", len(resp.Episodes), tt.wantCount)
			}
		})
	}
}

func TestListTVEpisodesUseCase_ExecuteByLibrary(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		setupRepo  func(*mocks.TVRepository)
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "successful list episodes by library",
			libraryID: 100,
			setupRepo: func(repo *mocks.TVRepository) {
				for i := 1; i <= 3; i++ {
					repo.WithEpisodes(&media.TVEpisode{
						Media: media.Media{
							ID:        int64(i),
							LibraryID: 100,
							Title:     "Episode",
							FilePath:  "tv/show/episode.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Test Show",
						EpisodeTitle: "Episode",
						Season:       1,
						Episode:      i,
					})
				}
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "empty library",
			libraryID: 100,
			setupRepo: func(repo *mocks.TVRepository) {
				// Empty repository
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "repository error",
			libraryID: 100,
			setupRepo: func(repo *mocks.TVRepository) {
				repo.ListErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewTVRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewListTVEpisodesUseCase(repo)
			resp, err := uc.ExecuteByLibrary(context.Background(), tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Error("ExecuteByLibrary() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteByLibrary() unexpected error = %v", err)
				return
			}

			if resp.Total != tt.wantCount {
				t.Errorf("ExecuteByLibrary() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Episodes) != tt.wantCount {
				t.Errorf("ExecuteByLibrary() got %d episodes, want %d", len(resp.Episodes), tt.wantCount)
			}
		})
	}
}


func TestSearchTVEpisodesUseCase_Execute(t *testing.T) {
	tests := []struct {
		name       string
		libraryID  int64
		query      string
		setupRepo  func(*mocks.TVRepository)
		wantCount  int
		wantErr    bool
	}{
		{
			name:      "successful search by show title",
			libraryID: 100,
			query:     "Breaking",
			setupRepo: func(repo *mocks.TVRepository) {
				repo.WithEpisodes(
					&media.TVEpisode{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "Breaking Bad S01E01",
							FilePath:  "tv/breaking-bad/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Breaking Bad",
						EpisodeTitle: "Pilot",
						Season:       1,
						Episode:      1,
					},
					&media.TVEpisode{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "The Wire S01E01",
							FilePath:  "tv/wire/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "The Wire",
						EpisodeTitle: "The Target",
						Season:       1,
						Episode:      1,
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
			setupRepo:  func(repo *mocks.TVRepository) {},
			wantCount:  0,
			wantErr:    true,
		},
		{
			name:      "no results",
			libraryID: 100,
			query:     "NonExistent",
			setupRepo: func(repo *mocks.TVRepository) {
				repo.WithEpisodes(&media.TVEpisode{
					Media: media.Media{
						ID:        1,
						LibraryID: 100,
						Title:     "Breaking Bad S01E01",
						FilePath:  "tv/breaking-bad/s01e01.mp4",
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					ShowTitle:    "Breaking Bad",
					EpisodeTitle: "Pilot",
					Season:       1,
					Episode:      1,
				})
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "repository error",
			libraryID: 100,
			query:     "test",
			setupRepo: func(repo *mocks.TVRepository) {
				repo.SearchErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewTVRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewSearchTVEpisodesUseCase(repo)
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

			if len(resp.Episodes) != tt.wantCount {
				t.Errorf("Execute() got %d episodes, want %d", len(resp.Episodes), tt.wantCount)
			}
		})
	}
}

func TestGetNextEpisodeUseCase_Execute(t *testing.T) {
	tests := []struct {
		name         string
		showID       int64
		setupTV      func(*mocks.TVRepository)
		setupProgress func(*mocks.ProgressRepository)
		wantEpisodeID int64
		wantErr      bool
	}{
		{
			name:   "return in-progress episode",
			showID: 1,
			setupTV: func(repo *mocks.TVRepository) {
				repo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 100,
					Title:     "Test Show",
				})
				repo.WithEpisodes(
					&media.TVEpisode{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "S01E01",
							FilePath:  "tv/show/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Test Show",
						EpisodeTitle: "Episode 1",
						Season:       1,
						Episode:      1,
					},
					&media.TVEpisode{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "S01E02",
							FilePath:  "tv/show/s01e02.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Test Show",
						EpisodeTitle: "Episode 2",
						Season:       1,
						Episode:      2,
					},
				)
			},
			setupProgress: func(repo *mocks.ProgressRepository) {
				// Episode 2 is in progress
				repo.WithProgress(&domainprogress.WatchProgress{
					MediaID:         2,
					UserID:          1,
					ProgressSeconds: 300,
					IsWatched:       false,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				})
			},
			wantEpisodeID: 2,
			wantErr:       false,
		},
		{
			name:   "return first unwatched episode",
			showID: 1,
			setupTV: func(repo *mocks.TVRepository) {
				repo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 100,
					Title:     "Test Show",
				})
				repo.WithEpisodes(
					&media.TVEpisode{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "S01E01",
							FilePath:  "tv/show/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Test Show",
						EpisodeTitle: "Episode 1",
						Season:       1,
						Episode:      1,
					},
					&media.TVEpisode{
						Media: media.Media{
							ID:        2,
							LibraryID: 100,
							Title:     "S01E02",
							FilePath:  "tv/show/s01e02.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Test Show",
						EpisodeTitle: "Episode 2",
						Season:       1,
						Episode:      2,
					},
				)
			},
			setupProgress: func(repo *mocks.ProgressRepository) {
				// Episode 1 is watched
				repo.WithProgress(&domainprogress.WatchProgress{
					MediaID:         1,
					UserID:          1,
					ProgressSeconds: 3600,
					IsWatched:       true,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				})
			},
			wantEpisodeID: 2,
			wantErr:       false,
		},
		{
			name:   "all watched - return first episode",
			showID: 1,
			setupTV: func(repo *mocks.TVRepository) {
				repo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 100,
					Title:     "Test Show",
				})
				repo.WithEpisodes(
					&media.TVEpisode{
						Media: media.Media{
							ID:        1,
							LibraryID: 100,
							Title:     "S01E01",
							FilePath:  "tv/show/s01e01.mp4",
							CreatedAt: time.Now(),
							UpdatedAt: time.Now(),
						},
						ShowTitle:    "Test Show",
						EpisodeTitle: "Episode 1",
						Season:       1,
						Episode:      1,
					},
				)
			},
			setupProgress: func(repo *mocks.ProgressRepository) {
				// All episodes watched
				repo.WithProgress(&domainprogress.WatchProgress{
					MediaID:         1,
					UserID:          1,
					ProgressSeconds: 3600,
					IsWatched:       true,
					CreatedAt:       time.Now(),
					UpdatedAt:       time.Now(),
				})
			},
			wantEpisodeID: 1,
			wantErr:       false,
		},
		{
			name:   "no episodes found",
			showID: 999,
			setupTV: func(repo *mocks.TVRepository) {
				repo.WithShows(media.TVShow{
					ID:        1,
					LibraryID: 100,
					Title:     "Test Show",
				})
			},
			setupProgress: func(repo *mocks.ProgressRepository) {},
			wantEpisodeID: 0,
			wantErr:       true,
		},
		{
			name:   "repository error",
			showID: 1,
			setupTV: func(repo *mocks.TVRepository) {
				repo.ListErr = errors.New("database error")
			},
			setupProgress: func(repo *mocks.ProgressRepository) {},
			wantEpisodeID: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvRepo := mocks.NewTVRepository(t)
			progressRepo := mocks.NewProgressRepository(t)

			if tt.setupTV != nil {
				tt.setupTV(tvRepo)
			}
			if tt.setupProgress != nil {
				tt.setupProgress(progressRepo)
			}

			uc := NewGetNextEpisodeUseCase(tvRepo, progressRepo)
			resp, err := uc.Execute(context.Background(), tt.showID)

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

			if resp.ID != tt.wantEpisodeID {
				t.Errorf("Execute() got episode ID = %d, want %d", resp.ID, tt.wantEpisodeID)
			}
		})
	}
}

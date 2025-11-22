package images

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestGetImageUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		imageID   int
		setupRepo func(*mocks.ImageRepository)
		wantErr   bool
	}{
		{
			name:    "successful get image",
			imageID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.WithImages(&images.Image{
					ID:        1,
					MediaType: images.MediaTypeMovie,
					EntityID:  100,
					ImageType: images.ImageTypePoster,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			},
			wantErr: false,
		},
		{
			name:    "image not found",
			imageID: 999,
			setupRepo: func(repo *mocks.ImageRepository) {
				// Empty repository
			},
			wantErr: true,
		},
		{
			name:    "repository error",
			imageID: 1,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewImageRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewGetImageUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.imageID)

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

			if resp.ID != tt.imageID {
				t.Errorf("Execute() got ID = %d, want %d", resp.ID, tt.imageID)
			}
		})
	}
}

func TestGetMediaImagesUseCase_Execute(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int
		setupRepo   func(*mocks.ImageRepository)
		wantCount   int
		wantErr     bool
	}{
		{
			name:    "successful get media images - multiple images",
			mediaID: 100,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.WithImages(
					&images.Image{
						ID:        1,
						MediaID:   intPtr(100),
						MediaType: images.MediaTypeMovie,
						EntityID:  100,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					&images.Image{
						ID:        2,
						MediaID:   intPtr(100),
						MediaType: images.MediaTypeMovie,
						EntityID:  100,
						ImageType: images.ImageTypeFanart,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:    "no images found for media",
			mediaID: 999,
			setupRepo: func(repo *mocks.ImageRepository) {
				// Empty repository
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:    "repository error",
			mediaID: 100,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewImageRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewGetMediaImagesUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.mediaID)

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

			if resp.Total != tt.wantCount {
				t.Errorf("Execute() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Images) != tt.wantCount {
				t.Errorf("Execute() got %d images, want %d", len(resp.Images), tt.wantCount)
			}
		})
	}
}

func TestGetEntityImagesUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		mediaType images.MediaType
		entityID  int
		setupRepo func(*mocks.ImageRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "successful get TV show images",
			mediaType: images.MediaTypeTVShow,
			entityID:  10,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.WithImages(
					&images.Image{
						ID:        1,
						MediaType: images.MediaTypeTVShow,
						EntityID:  10,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					&images.Image{
						ID:        2,
						MediaType: images.MediaTypeTVShow,
						EntityID:  10,
						ImageType: images.ImageTypeBanner,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "no images found for entity",
			mediaType: images.MediaTypeTVShow,
			entityID:  999,
			setupRepo: func(repo *mocks.ImageRepository) {
				// Empty repository
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "repository error",
			mediaType: images.MediaTypeTVShow,
			entityID:  10,
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.GetErr = errors.New("database error")
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewImageRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewGetEntityImagesUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.mediaType, tt.entityID)

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

			if resp.Total != tt.wantCount {
				t.Errorf("Execute() got Total = %d, want %d", resp.Total, tt.wantCount)
			}

			if len(resp.Images) != tt.wantCount {
				t.Errorf("Execute() got %d images, want %d", len(resp.Images), tt.wantCount)
			}
		})
	}
}

func TestGetBatchMediaImagesUseCase_Execute(t *testing.T) {
	tests := []struct {
		name      string
		mediaIDs  []int
		mediaType string
		entityIDs []int
		setupRepo func(*mocks.ImageRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:     "successful batch get with media IDs",
			mediaIDs: []int{100, 101, 102},
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.WithImages(
					&images.Image{
						ID:        1,
						MediaID:   intPtr(100),
						MediaType: images.MediaTypeMovie,
						EntityID:  100,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					&images.Image{
						ID:        2,
						MediaID:   intPtr(101),
						MediaType: images.MediaTypeMovie,
						EntityID:  101,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					&images.Image{
						ID:        3,
						MediaID:   intPtr(102),
						MediaType: images.MediaTypeMovie,
						EntityID:  102,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				)
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "successful batch get with entity IDs",
			entityIDs: []int{10, 20},
			mediaType: "tv_show",
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.WithImages(
					&images.Image{
						ID:        1,
						MediaType: images.MediaTypeTVShow,
						EntityID:  10,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					&images.Image{
						ID:        2,
						MediaType: images.MediaTypeTVShow,
						EntityID:  20,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				)
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:     "batch with partial errors - continues processing",
			mediaIDs: []int{100, 999},
			setupRepo: func(repo *mocks.ImageRepository) {
				repo.WithImages(
					&images.Image{
						ID:        1,
						MediaID:   intPtr(100),
						MediaType: images.MediaTypeMovie,
						EntityID:  100,
						ImageType: images.ImageTypePoster,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				)
			},
			wantCount: 2, // Should return entries for both (100 with images, 999 with empty array)
			wantErr:   false,
		},
		{
			name:      "invalid media type for entity batch",
			entityIDs: []int{10},
			mediaType: "invalid_type",
			setupRepo: func(repo *mocks.ImageRepository) {
				// Setup is irrelevant for invalid media type
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "empty request",
			setupRepo: func(repo *mocks.ImageRepository) {},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mocks.NewImageRepository(t)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			uc := NewGetBatchMediaImagesUseCase(repo)
			resp, err := uc.Execute(context.Background(), tt.mediaIDs, tt.mediaType, tt.entityIDs)

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

			if len(resp.MediaImages) != tt.wantCount {
				t.Errorf("Execute() got %d entries in batch, want %d", len(resp.MediaImages), tt.wantCount)
			}
		})
	}
}

// Helper function to create int pointers
func intPtr(i int) *int {
	return &i
}

package media

import (
	"context"
	"testing"
	"time"

	"github.com/viewra/viewra/internal/domain/media"
)

// mockMediaRepository is a mock implementation of media.Repository for testing
type mockMediaRepository struct {
	mediaItems map[int64]*media.Media
	nextID     int64
}

func newMockMediaRepository() *mockMediaRepository {
	return &mockMediaRepository{
		mediaItems: make(map[int64]*media.Media),
		nextID:     1,
	}
}

func (m *mockMediaRepository) Create(ctx context.Context, med *media.Media) error {
	med.ID = m.nextID
	m.nextID++
	m.mediaItems[med.ID] = med
	return nil
}

func (m *mockMediaRepository) GetByID(ctx context.Context, id int64) (*media.Media, error) {
	med, ok := m.mediaItems[id]
	if !ok {
		return nil, media.ErrMediaNotFound
	}
	return med, nil
}

func (m *mockMediaRepository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*media.Media, error) {
	for _, med := range m.mediaItems {
		if med.LibraryID == libraryID && med.FilePath == filePath {
			return med, nil
		}
	}
	return nil, media.ErrMediaNotFound
}

func (m *mockMediaRepository) ListAll(ctx context.Context) ([]*media.Media, error) {
	result := make([]*media.Media, 0, len(m.mediaItems))
	for _, med := range m.mediaItems {
		result = append(result, med)
	}
	return result, nil
}

func (m *mockMediaRepository) ListByLibrary(ctx context.Context, libraryID int64) ([]*media.Media, error) {
	var result []*media.Media
	for _, med := range m.mediaItems {
		if med.LibraryID == libraryID {
			result = append(result, med)
		}
	}
	return result, nil
}

func (m *mockMediaRepository) ListByType(ctx context.Context, libraryID int64, mediaType media.MediaType) ([]*media.Media, error) {
	// For simplicity, return all media in library (type filtering would require additional field)
	return m.ListByLibrary(ctx, libraryID)
}

func (m *mockMediaRepository) Update(ctx context.Context, med *media.Media) error {
	if _, ok := m.mediaItems[med.ID]; !ok {
		return media.ErrMediaNotFound
	}
	m.mediaItems[med.ID] = med
	return nil
}

func (m *mockMediaRepository) Delete(ctx context.Context, id int64) error {
	if _, ok := m.mediaItems[id]; !ok {
		return media.ErrMediaNotFound
	}
	delete(m.mediaItems, id)
	return nil
}

func (m *mockMediaRepository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	_, err := m.GetByFilePath(ctx, libraryID, filePath)
	return err == nil, nil
}

func (m *mockMediaRepository) Count(ctx context.Context, libraryID int64) (int64, error) {
	var count int64
	for _, med := range m.mediaItems {
		if med.LibraryID == libraryID {
			count++
		}
	}
	return count, nil
}

func (m *mockMediaRepository) CountByType(ctx context.Context, libraryID int64, mediaType media.MediaType) (int64, error) {
	return m.Count(ctx, libraryID)
}

func TestGetMediaUseCase_Execute(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		setup       func(*mockMediaRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "existing media",
			mediaID: 1,
			setup: func(repo *mockMediaRepository) {
				_ = repo.Create(context.Background(), &media.Media{
					LibraryID: 1,
					Title:     "Test Movie",
					FilePath:  "movies/test.mp4",
					FileSize:  1000000,
					Duration:  7200,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				})
			},
			wantErr: false,
		},
		{
			name:        "non-existent media",
			mediaID:     999,
			setup:       func(repo *mockMediaRepository) {},
			wantErr:     true,
			expectedErr: media.ErrMediaNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockMediaRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewGetMediaUseCase(repo)

			resp, err := uc.Execute(context.Background(), tt.mediaID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
					return
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.ID != tt.mediaID {
				t.Errorf("Execute() ID = %v, want %v", resp.ID, tt.mediaID)
			}
		})
	}
}

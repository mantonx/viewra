package media

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// mockRepository implements Repository for testing
type mockRepository struct {
	media     map[int64]*Media
	nextID    int64
	pathIndex map[int64]map[string]bool // libraryID -> filePath -> exists
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	existsErr error
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		media:     make(map[int64]*Media),
		nextID:    1,
		pathIndex: make(map[int64]map[string]bool),
	}
}

func (m *mockRepository) Create(ctx context.Context, media *Media) error {
	if m.createErr != nil {
		return m.createErr
	}
	media.ID = m.nextID
	m.nextID++
	m.media[media.ID] = media

	if m.pathIndex[media.LibraryID] == nil {
		m.pathIndex[media.LibraryID] = make(map[string]bool)
	}
	m.pathIndex[media.LibraryID][media.FilePath] = true
	return nil
}

func (m *mockRepository) GetByID(ctx context.Context, id int64) (*Media, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	media, ok := m.media[id]
	if !ok {
		return nil, ErrMediaNotFound
	}
	return media, nil
}

func (m *mockRepository) GetByFilePath(ctx context.Context, libraryID int64, filePath string) (*Media, error) {
	for _, media := range m.media {
		if media.FilePath == filePath && media.LibraryID == libraryID {
			return media, nil
		}
	}
	return nil, ErrMediaNotFound
}

func (m *mockRepository) ListAll(ctx context.Context) ([]*Media, error) {
	result := make([]*Media, 0, len(m.media))
	for _, media := range m.media {
		result = append(result, media)
	}
	return result, nil
}

func (m *mockRepository) ListByLibrary(ctx context.Context, libraryID int64) ([]*Media, error) {
	result := make([]*Media, 0)
	for _, media := range m.media {
		if media.LibraryID == libraryID {
			result = append(result, media)
		}
	}
	return result, nil
}

func (m *mockRepository) Update(ctx context.Context, media *Media) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.media[media.ID]; !ok {
		return ErrMediaNotFound
	}
	m.media[media.ID] = media
	return nil
}

func (m *mockRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	media, ok := m.media[id]
	if !ok {
		return ErrMediaNotFound
	}
	delete(m.media, id)
	if m.pathIndex[media.LibraryID] != nil {
		delete(m.pathIndex[media.LibraryID], media.FilePath)
	}
	return nil
}

func (m *mockRepository) DeleteByLibrary(ctx context.Context, libraryID int64) error {
	for id, media := range m.media {
		if media.LibraryID == libraryID {
			delete(m.media, id)
		}
	}
	delete(m.pathIndex, libraryID)
	return nil
}

func (m *mockRepository) ExistsInLibrary(ctx context.Context, libraryID int64, filePath string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	if m.pathIndex[libraryID] == nil {
		return false, nil
	}
	return m.pathIndex[libraryID][filePath], nil
}

func (m *mockRepository) Count(ctx context.Context, libraryID int64) (int64, error) {
	var count int64
	for _, media := range m.media {
		if media.LibraryID == libraryID {
			count++
		}
	}
	return count, nil
}

func (m *mockRepository) CountByType(ctx context.Context, libraryID int64, mediaType MediaType) (int64, error) {
	return 0, nil
}

func (m *mockRepository) ListByType(ctx context.Context, libraryID int64, mediaType MediaType) ([]*Media, error) {
	return nil, nil
}

func (m *mockRepository) GetFilePathCache(ctx context.Context, libraryID int64) (map[string]int64, error) {
	cache := make(map[string]int64)
	for id, media := range m.media {
		if media.LibraryID == libraryID {
			cache[media.FilePath] = id
		}
	}
	return cache, nil
}

func (m *mockRepository) InsertAudioTrack(ctx context.Context, track *AudioTrack) error {
	return nil
}

func (m *mockRepository) GetAudioTracksByMediaID(ctx context.Context, mediaID int64) ([]*AudioTrack, error) {
	return nil, nil
}

func (m *mockRepository) DeleteAudioTracksByMediaID(ctx context.Context, mediaID int64) error {
	return nil
}

func (m *mockRepository) InsertSubtitleTrack(ctx context.Context, track *SubtitleTrack) error {
	return nil
}

func (m *mockRepository) GetSubtitleTracksByMediaID(ctx context.Context, mediaID int64) ([]*SubtitleTrack, error) {
	return nil, nil
}

func (m *mockRepository) DeleteSubtitleTracksByMediaID(ctx context.Context, mediaID int64) error {
	return nil
}

func (m *mockRepository) DeleteExternalSubtitlesByMediaID(ctx context.Context, mediaID int64) error {
	return nil
}

// Transaction-aware methods (delegate to non-tx versions for testing)
func (m *mockRepository) DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error {
	return m.Delete(ctx, id)
}

func (m *mockRepository) ListByLibraryWithTx(ctx context.Context, tx *sql.Tx, libraryID int64) ([]*Media, error) {
	return m.ListByLibrary(ctx, libraryID)
}

func TestNewService(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
	if service.repo == nil {
		t.Error("NewService() repo is nil")
	}
}

func TestService_CreateMedia(t *testing.T) {
	// Create temp directory for test files
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		media       *Media
		libraryPath string
		setupRepo   func(*mockRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid media creation",
			media: &Media{
				Title:     "Test Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo:   func(m *mockRepository) {},
			wantErr:     false,
		},
		{
			name: "invalid media - validation error",
			media: &Media{
				Title:     "", // Invalid: empty title
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo:   func(m *mockRepository) {},
			wantErr:     true,
			expectedErr: ErrEmptyTitle,
		},
		{
			name: "file does not exist",
			media: &Media{
				Title:     "Test Movie",
				FilePath:  "nonexistent.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo:   func(m *mockRepository) {},
			wantErr:     true,
			expectedErr: ErrEmptyFilePath,
		},
		{
			name: "duplicate file path",
			media: &Media{
				Title:     "Test Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				// Add existing media with same path
				m.pathIndex[1] = map[string]bool{"test.mp4": true}
			},
			wantErr:     true,
			expectedErr: ErrDuplicateFilePath,
		},
		{
			name: "file size auto-detected",
			media: &Media{
				Title:     "Test Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
				FileSize:  0, // Should be auto-detected
			},
			libraryPath: tempDir,
			setupRepo:   func(m *mockRepository) {},
			wantErr:     false,
		},
		{
			name: "repository create error",
			media: &Media{
				Title:     "Test Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				m.createErr = errors.New("database error")
			},
			wantErr: true,
		},
		{
			name: "repository exists check error",
			media: &Media{
				Title:     "Test Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				m.existsErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			tt.setupRepo(repo)
			service := NewService(repo)

			err := service.CreateMedia(context.Background(), tt.media, tt.libraryPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateMedia() expected error, got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					// Check if error message contains expected error
					if !containsError(err, tt.expectedErr) {
						t.Errorf("CreateMedia() error = %v, want %v", err, tt.expectedErr)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("CreateMedia() unexpected error = %v", err)
				return
			}

			if tt.media.ID == 0 {
				t.Error("CreateMedia() did not set media ID")
			}

			// Verify file size was detected if it was 0
			if tt.name == "file size auto-detected" && tt.media.FileSize == 0 {
				t.Error("CreateMedia() did not auto-detect file size")
			}
		})
	}
}

func TestService_CreateMedia_DirectoryPath(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	repo := newMockRepository()
	service := NewService(repo)

	media := &Media{
		Title:     "Test",
		FilePath:  "subdir", // This is a directory, not a file
		LibraryID: 1,
	}

	err := service.CreateMedia(context.Background(), media, tempDir)
	if err == nil {
		t.Error("CreateMedia() should error when path is a directory")
	}
	// Directory paths fail validation (missing extension) or file check
	if err == nil {
		t.Error("CreateMedia() should error when path is a directory")
	}
}

func TestService_GetMediaByID(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		setupRepo   func(*mockRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "get existing media",
			mediaID: 1,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{
					ID:       1,
					Title:    "Test Movie",
					FilePath: "/path/to/movie.mp4",
				}
			},
			wantErr: false,
		},
		{
			name:    "media not found",
			mediaID: 999,
			setupRepo: func(m *mockRepository) {
				// No media created
			},
			wantErr:     true,
			expectedErr: ErrMediaNotFound,
		},
		{
			name:    "repository error",
			mediaID: 1,
			setupRepo: func(m *mockRepository) {
				m.getErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			tt.setupRepo(repo)
			service := NewService(repo)

			media, err := service.GetMediaByID(context.Background(), tt.mediaID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetMediaByID() expected error, got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && !containsError(err, tt.expectedErr) {
					t.Errorf("GetMediaByID() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("GetMediaByID() unexpected error = %v", err)
				return
			}

			if media == nil {
				t.Error("GetMediaByID() returned nil media")
			}
			if media.ID != tt.mediaID {
				t.Errorf("GetMediaByID() ID = %v, want %v", media.ID, tt.mediaID)
			}
		})
	}
}

func TestService_ListMediaByLibrary(t *testing.T) {
	tests := []struct {
		name      string
		libraryID int64
		setupRepo func(*mockRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "list media in library with items",
			libraryID: 1,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1, LibraryID: 1, Title: "Movie 1"}
				m.media[2] = &Media{ID: 2, LibraryID: 1, Title: "Movie 2"}
				m.media[3] = &Media{ID: 3, LibraryID: 2, Title: "Movie 3"}
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "list media in empty library",
			libraryID: 1,
			setupRepo: func(m *mockRepository) {
				// No media
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "list media in library with no matching items",
			libraryID: 999,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1, LibraryID: 1, Title: "Movie 1"}
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			tt.setupRepo(repo)
			service := NewService(repo)

			mediaList, err := service.ListMediaByLibrary(context.Background(), tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ListMediaByLibrary() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ListMediaByLibrary() unexpected error = %v", err)
				return
			}

			if len(mediaList) != tt.wantCount {
				t.Errorf("ListMediaByLibrary() returned %d items, want %d", len(mediaList), tt.wantCount)
			}
		})
	}
}

func TestService_UpdateMedia(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		media       *Media
		libraryPath string
		setupRepo   func(*mockRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "valid media update",
			media: &Media{
				ID:        1,
				Title:     "Updated Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1, Title: "Original"}
			},
			wantErr: false,
		},
		{
			name: "invalid media - validation error",
			media: &Media{
				ID:        1,
				Title:     "", // Invalid
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1}
			},
			wantErr:     true,
			expectedErr: ErrEmptyTitle,
		},
		{
			name: "file does not exist",
			media: &Media{
				ID:        1,
				Title:     "Updated Movie",
				FilePath:  "nonexistent.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1}
			},
			wantErr:     true,
			expectedErr: ErrEmptyFilePath,
		},
		{
			name: "repository update error",
			media: &Media{
				ID:        1,
				Title:     "Updated Movie",
				FilePath:  "test.mp4",
				LibraryID: 1,
			},
			libraryPath: tempDir,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1}
				m.updateErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			tt.setupRepo(repo)
			service := NewService(repo)

			err := service.UpdateMedia(context.Background(), tt.media, tt.libraryPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateMedia() expected error, got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && !containsError(err, tt.expectedErr) {
					t.Errorf("UpdateMedia() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("UpdateMedia() unexpected error = %v", err)
			}
		})
	}
}

func TestService_DeleteMedia(t *testing.T) {
	tests := []struct {
		name        string
		mediaID     int64
		setupRepo   func(*mockRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name:    "delete existing media",
			mediaID: 1,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1, Title: "Test Movie"}
			},
			wantErr: false,
		},
		{
			name:    "delete non-existent media",
			mediaID: 999,
			setupRepo: func(m *mockRepository) {
				// No media created
			},
			wantErr:     true,
			expectedErr: ErrMediaNotFound,
		},
		{
			name:    "repository delete error",
			mediaID: 1,
			setupRepo: func(m *mockRepository) {
				m.media[1] = &Media{ID: 1}
				m.deleteErr = errors.New("database error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepository()
			tt.setupRepo(repo)
			service := NewService(repo)

			err := service.DeleteMedia(context.Background(), tt.mediaID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteMedia() expected error, got nil")
					return
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) && !containsError(err, tt.expectedErr) {
					t.Errorf("DeleteMedia() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("DeleteMedia() unexpected error = %v", err)
				return
			}

			// Verify media was deleted
			if _, ok := repo.media[tt.mediaID]; ok {
				t.Error("DeleteMedia() did not remove media from repository")
			}
		})
	}
}

// containsError checks if err's error string contains targetErr's error string
func containsError(err, targetErr error) bool {
	if err == nil || targetErr == nil {
		return false
	}
	return errors.Is(err, targetErr)
}

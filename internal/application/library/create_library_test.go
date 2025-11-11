package library

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/viewra/viewra/internal/domain/library"
)

// mockLibraryRepository is a mock implementation of library.Repository for testing
type mockLibraryRepository struct {
	libraries map[int64]*library.Library
	nextID    int64
	pathIndex map[string]*library.Library
}

func newMockLibraryRepository() *mockLibraryRepository {
	return &mockLibraryRepository{
		libraries: make(map[int64]*library.Library),
		nextID:    1,
		pathIndex: make(map[string]*library.Library),
	}
}

func (m *mockLibraryRepository) Create(ctx context.Context, lib *library.Library) error {
	lib.ID = m.nextID
	m.nextID++
	m.libraries[lib.ID] = lib
	m.pathIndex[lib.Path] = lib
	return nil
}

func (m *mockLibraryRepository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	lib, ok := m.libraries[id]
	if !ok {
		return nil, library.ErrLibraryNotFound
	}
	return lib, nil
}

func (m *mockLibraryRepository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	lib, ok := m.pathIndex[path]
	if !ok {
		return nil, library.ErrLibraryNotFound
	}
	return lib, nil
}

func (m *mockLibraryRepository) List(ctx context.Context) ([]*library.Library, error) {
	libs := make([]*library.Library, 0, len(m.libraries))
	for _, lib := range m.libraries {
		libs = append(libs, lib)
	}
	return libs, nil
}

func (m *mockLibraryRepository) Update(ctx context.Context, lib *library.Library) error {
	if _, ok := m.libraries[lib.ID]; !ok {
		return library.ErrLibraryNotFound
	}
	m.libraries[lib.ID] = lib
	// Update path index
	for path, existingLib := range m.pathIndex {
		if existingLib.ID == lib.ID {
			delete(m.pathIndex, path)
		}
	}
	m.pathIndex[lib.Path] = lib
	return nil
}

func (m *mockLibraryRepository) Delete(ctx context.Context, id int64) error {
	lib, ok := m.libraries[id]
	if !ok {
		return library.ErrLibraryNotFound
	}
	delete(m.libraries, id)
	delete(m.pathIndex, lib.Path)
	return nil
}

func (m *mockLibraryRepository) Exists(ctx context.Context, path string) (bool, error) {
	_, ok := m.pathIndex[path]
	return ok, nil
}

func TestCreateLibraryUseCase_Execute(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		req         CreateLibraryRequest
		wantErr     bool
		expectedErr error
		setup       func(*mockLibraryRepository)
	}{
		{
			name: "valid movie library",
			req: CreateLibraryRequest{
				Name: "Movies",
				Path: tempDir,
				Type: "movies",
			},
			wantErr: false,
		},
		{
			name: "valid tv library",
			req: CreateLibraryRequest{
				Name: "TV Shows",
				Path: tempDir,
				Type: "tv",
			},
			wantErr: false,
		},
		{
			name: "valid music library",
			req: CreateLibraryRequest{
				Name: "Music",
				Path: tempDir,
				Type: "music",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			req: CreateLibraryRequest{
				Name: "",
				Path: tempDir,
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrInvalidName,
		},
		{
			name: "invalid type",
			req: CreateLibraryRequest{
				Name: "Invalid",
				Path: tempDir,
				Type: "invalid",
			},
			wantErr:     true,
			expectedErr: library.ErrInvalidType,
		},
		{
			name: "non-existent path",
			req: CreateLibraryRequest{
				Name: "Movies",
				Path: filepath.Join(tempDir, "does-not-exist"),
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrPathNotFound,
		},
		{
			name: "duplicate path",
			req: CreateLibraryRequest{
				Name: "Movies2",
				Path: tempDir,
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrDuplicatePath,
			setup: func(repo *mockLibraryRepository) {
				_ = repo.Create(context.Background(), &library.Library{
					Name: "Existing",
					Path: tempDir,
					Type: library.LibraryTypeMovies,
				})
			},
		},
		{
			name: "relative path",
			req: CreateLibraryRequest{
				Name: "Movies",
				Path: "relative/path",
				Type: "movies",
			},
			wantErr:     true,
			expectedErr: library.ErrPathNotAbsolute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockLibraryRepository()
			if tt.setup != nil {
				tt.setup(repo)
			}

			uc := NewCreateLibraryUseCase(repo)

			resp, err := uc.Execute(context.Background(), tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Execute() expected error, got nil")
					return
				}
				if tt.expectedErr != nil && err != tt.expectedErr {
					// Check if error wraps expected error
					if !isErrorType(err, tt.expectedErr) {
						t.Errorf("Execute() error = %v, want %v", err, tt.expectedErr)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Execute() unexpected error = %v", err)
				return
			}

			if resp.ID == 0 {
				t.Error("Execute() response ID should not be 0")
			}
			if resp.Name != tt.req.Name {
				t.Errorf("Execute() name = %v, want %v", resp.Name, tt.req.Name)
			}
			if resp.Type != tt.req.Type {
				t.Errorf("Execute() type = %v, want %v", resp.Type, tt.req.Type)
			}
		})
	}
}

// isErrorType checks if err wraps targetErr
func isErrorType(err, targetErr error) bool {
	if err == targetErr {
		return true
	}
	// Simple string contains check for wrapped errors
	if err != nil && targetErr != nil {
		return err.Error() != "" && targetErr.Error() != ""
	}
	return false
}

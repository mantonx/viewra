package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	libraries map[int64]*Library
	pathIndex map[string]int64
	nextID    int64

	// Error injection for testing
	createErr    error
	getByIDErr   error
	getByPathErr error
	listErr      error
	updateErr    error
	deleteErr    error
	existsErr    error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		libraries: make(map[int64]*Library),
		pathIndex: make(map[string]int64),
		nextID:    1,
	}
}

func (m *MockRepository) Create(ctx context.Context, lib *Library) error {
	if m.createErr != nil {
		return m.createErr
	}
	lib.ID = m.nextID
	m.nextID++
	// Store a copy to avoid modifications affecting stored data
	libCopy := *lib
	m.libraries[lib.ID] = &libCopy
	m.pathIndex[lib.Path] = lib.ID
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, id int64) (*Library, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	lib, exists := m.libraries[id]
	if !exists {
		return nil, ErrLibraryNotFound
	}
	// Return a copy to avoid modifications affecting stored data
	libCopy := *lib
	return &libCopy, nil
}

func (m *MockRepository) GetByPath(ctx context.Context, path string) (*Library, error) {
	if m.getByPathErr != nil {
		return nil, m.getByPathErr
	}
	id, exists := m.pathIndex[path]
	if !exists {
		return nil, ErrLibraryNotFound
	}
	return m.libraries[id], nil
}

func (m *MockRepository) List(ctx context.Context) ([]*Library, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*Library, 0, len(m.libraries))
	for _, lib := range m.libraries {
		result = append(result, lib)
	}
	return result, nil
}

func (m *MockRepository) Update(ctx context.Context, lib *Library) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	existing, exists := m.libraries[lib.ID]
	if !exists {
		return ErrLibraryNotFound
	}

	// Update path index if path changed
	if existing.Path != lib.Path {
		delete(m.pathIndex, existing.Path)
		m.pathIndex[lib.Path] = lib.ID
	}

	// Store a copy to avoid modifications affecting stored data
	libCopy := *lib
	m.libraries[lib.ID] = &libCopy
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	lib, exists := m.libraries[id]
	if !exists {
		return ErrLibraryNotFound
	}
	delete(m.pathIndex, lib.Path)
	delete(m.libraries, id)
	return nil
}

func (m *MockRepository) Exists(ctx context.Context, path string) (bool, error) {
	if m.existsErr != nil {
		return false, m.existsErr
	}
	_, exists := m.pathIndex[path]
	return exists, nil
}

// Transaction-aware methods (delegate to non-tx versions for testing)
func (m *MockRepository) CreateWithTx(ctx context.Context, tx *sql.Tx, lib *Library) error {
	return m.Create(ctx, lib)
}

func (m *MockRepository) GetByIDWithTx(ctx context.Context, tx *sql.Tx, id int64) (*Library, error) {
	return m.GetByID(ctx, id)
}

func (m *MockRepository) DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error {
	return m.Delete(ctx, id)
}

func (m *MockRepository) ExistsWithTx(ctx context.Context, tx *sql.Tx, path string) (bool, error) {
	return m.Exists(ctx, path)
}

func TestNewService(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)

	if service == nil {
		t.Fatal("Expected non-nil service")
	}
	if service.repo != repo {
		t.Error("Expected service to have correct repository")
	}
}

func TestServiceCreate(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)
	ctx := context.Background()

	// Create temp directory for testing
	tmpDir := t.TempDir()

	t.Run("successful creation", func(t *testing.T) {
		lib := &Library{
			Name: "Test Library",
			Path: tmpDir,
			Type: LibraryTypeMovies,
		}

		err := service.Create(ctx, lib)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		if lib.ID == 0 {
			t.Error("Expected ID to be set after creation")
		}
	})

	t.Run("validation error - empty name", func(t *testing.T) {
		lib := &Library{
			Name: "",
			Path: tmpDir,
			Type: LibraryTypeMovies,
		}

		err := service.Create(ctx, lib)
		if err == nil {
			t.Error("Expected validation error")
		}
		if !errors.Is(err, ErrInvalidName) {
			t.Errorf("Expected ErrInvalidName, got: %v", err)
		}
	})

	t.Run("path not found", func(t *testing.T) {
		lib := &Library{
			Name: "Test Library",
			Path: "/nonexistent/path",
			Type: LibraryTypeMovies,
		}

		err := service.Create(ctx, lib)
		if err == nil {
			t.Error("Expected path error")
		}
		if !errors.Is(err, ErrPathNotFound) {
			t.Errorf("Expected ErrPathNotFound, got: %v", err)
		}
	})

	t.Run("duplicate path", func(t *testing.T) {
		lib1 := &Library{
			Name: "Library 1",
			Path: tmpDir,
			Type: LibraryTypeMovies,
		}

		// This should fail because we already created a library with this path
		err := service.Create(ctx, lib1)
		if err == nil {
			t.Error("Expected duplicate path error")
		}
		if !errors.Is(err, ErrDuplicatePath) {
			t.Errorf("Expected ErrDuplicatePath, got: %v", err)
		}
	})

	t.Run("repository error on exists check", func(t *testing.T) {
		repo.existsErr = errors.New("database error")
		defer func() { repo.existsErr = nil }()

		tmpDir2 := t.TempDir()
		lib := &Library{
			Name: "Test Library",
			Path: tmpDir2,
			Type: LibraryTypeMovies,
		}

		err := service.Create(ctx, lib)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})

	t.Run("repository error on create", func(t *testing.T) {
		repo.createErr = errors.New("database error")
		defer func() { repo.createErr = nil }()

		tmpDir2 := t.TempDir()
		lib := &Library{
			Name: "Test Library",
			Path: tmpDir2,
			Type: LibraryTypeMovies,
		}

		err := service.Create(ctx, lib)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})
}

func TestServiceGetByID(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a library first
	lib := &Library{
		Name: "Test Library",
		Path: tmpDir,
		Type: LibraryTypeMovies,
	}
	_ = service.Create(ctx, lib)

	t.Run("successful get", func(t *testing.T) {
		retrieved, err := service.GetByID(ctx, lib.ID)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if retrieved.Name != lib.Name {
			t.Errorf("Expected name %s, got %s", lib.Name, retrieved.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetByID(ctx, 999)
		if err == nil {
			t.Error("Expected not found error")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo.getByIDErr = errors.New("database error")
		defer func() { repo.getByIDErr = nil }()

		_, err := service.GetByID(ctx, lib.ID)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})
}

func TestServiceGetByPath(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)
	ctx := context.Background()
	tmpDir := t.TempDir()

	lib := &Library{
		Name: "Test Library",
		Path: tmpDir,
		Type: LibraryTypeMovies,
	}
	_ = service.Create(ctx, lib)

	t.Run("successful get", func(t *testing.T) {
		retrieved, err := service.GetByPath(ctx, tmpDir)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if retrieved.Name != lib.Name {
			t.Errorf("Expected name %s, got %s", lib.Name, retrieved.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetByPath(ctx, "/nonexistent")
		if err == nil {
			t.Error("Expected not found error")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo.getByPathErr = errors.New("database error")
		defer func() { repo.getByPathErr = nil }()

		_, err := service.GetByPath(ctx, tmpDir)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})
}

func TestServiceList(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)
	ctx := context.Background()

	t.Run("empty list", func(t *testing.T) {
		libraries, err := service.List(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(libraries) != 0 {
			t.Errorf("Expected empty list, got %d libraries", len(libraries))
		}
	})

	t.Run("list with libraries", func(t *testing.T) {
		tmpDir1 := t.TempDir()
		tmpDir2 := t.TempDir()

		_ = service.Create(ctx, &Library{Name: "Lib1", Path: tmpDir1, Type: LibraryTypeMovies})
		_ = service.Create(ctx, &Library{Name: "Lib2", Path: tmpDir2, Type: LibraryTypeTV})

		libraries, err := service.List(ctx)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		if len(libraries) != 2 {
			t.Errorf("Expected 2 libraries, got %d", len(libraries))
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo.listErr = errors.New("database error")
		defer func() { repo.listErr = nil }()

		_, err := service.List(ctx)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})
}

func TestServiceUpdate(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)
	ctx := context.Background()
	tmpDir := t.TempDir()

	lib := &Library{
		Name: "Original Name",
		Path: tmpDir,
		Type: LibraryTypeMovies,
	}
	_ = service.Create(ctx, lib)

	t.Run("successful update", func(t *testing.T) {
		lib.Name = "Updated Name"
		err := service.Update(ctx, lib)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		retrieved, _ := service.GetByID(ctx, lib.ID)
		if retrieved.Name != "Updated Name" {
			t.Errorf("Expected updated name, got %s", retrieved.Name)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		invalid := &Library{
			ID:   lib.ID,
			Name: "",
			Path: tmpDir,
			Type: LibraryTypeMovies,
		}

		err := service.Update(ctx, invalid)
		if err == nil {
			t.Error("Expected validation error")
		}
	})

	t.Run("library not found", func(t *testing.T) {
		nonExistent := &Library{
			ID:   999,
			Name: "Test",
			Path: tmpDir,
			Type: LibraryTypeMovies,
		}

		err := service.Update(ctx, nonExistent)
		if err == nil {
			t.Error("Expected not found error")
		}
		if !errors.Is(err, ErrLibraryNotFound) {
			t.Errorf("Expected ErrLibraryNotFound, got: %v", err)
		}
	})

	t.Run("duplicate path on update", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		lib2 := &Library{
			Name: "Library 2",
			Path: tmpDir2,
			Type: LibraryTypeMovies,
		}
		_ = service.Create(ctx, lib2)

		tmpDir3 := t.TempDir()
		lib3 := &Library{
			Name: "Library 3",
			Path: tmpDir3,
			Type: LibraryTypeMovies,
		}
		_ = service.Create(ctx, lib3)

		// Try to update lib3 to use lib2's path
		lib3.Path = tmpDir2
		err := service.Update(ctx, lib3)

		if err == nil {
			t.Error("Expected duplicate path error")
		}
		if !errors.Is(err, ErrDuplicatePath) {
			t.Errorf("Expected ErrDuplicatePath, got: %v", err)
		}
	})

	t.Run("path not found", func(t *testing.T) {
		lib.Path = "/nonexistent/path"
		err := service.Update(ctx, lib)
		lib.Path = tmpDir // Reset

		if err == nil {
			t.Error("Expected path error")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo.updateErr = errors.New("database error")
		defer func() { repo.updateErr = nil }()

		err := service.Update(ctx, lib)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})
}

func TestServiceDelete(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)
	ctx := context.Background()
	tmpDir := t.TempDir()

	lib := &Library{
		Name: "Test Library",
		Path: tmpDir,
		Type: LibraryTypeMovies,
	}
	_ = service.Create(ctx, lib)

	t.Run("successful delete", func(t *testing.T) {
		err := service.Delete(ctx, lib.ID)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		_, err = service.GetByID(ctx, lib.ID)
		if err == nil {
			t.Error("Library should have been deleted")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		repo.deleteErr = errors.New("database error")
		defer func() { repo.deleteErr = nil }()

		err := service.Delete(ctx, 1)
		if err == nil {
			t.Error("Expected error from repository")
		}
	})
}

func TestValidatePathExists(t *testing.T) {
	repo := NewMockRepository()
	service := NewService(repo)

	t.Run("valid directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		err := service.validatePathExists(tmpDir)
		if err != nil {
			t.Errorf("Expected no error for valid directory, got: %v", err)
		}
	})

	t.Run("path not found", func(t *testing.T) {
		err := service.validatePathExists("/nonexistent/path")
		if !errors.Is(err, ErrPathNotFound) {
			t.Errorf("Expected ErrPathNotFound, got: %v", err)
		}
	})

	t.Run("path is a file not directory", func(t *testing.T) {
		tmpFile := t.TempDir() + "/file.txt"
		_ = os.WriteFile(tmpFile, []byte("test"), 0644)

		err := service.validatePathExists(tmpFile)
		if !errors.Is(err, ErrPathNotDirectory) {
			t.Errorf("Expected ErrPathNotDirectory, got: %v", err)
		}
	})
}

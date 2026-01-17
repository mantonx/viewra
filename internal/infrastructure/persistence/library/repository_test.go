package library

import (
	"context"
	"database/sql"
	"testing"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database with schema
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create schema matching production migrations
	schema := `
	CREATE TABLE IF NOT EXISTS libraries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL CHECK(type IN ('movies', 'tv', 'music')),
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		monitoring_enabled INTEGER NOT NULL DEFAULT 1,
		monitoring_config TEXT,
		preferred_audio_lang TEXT DEFAULT 'eng',
		preferred_subtitle_lang TEXT DEFAULT 'eng',
		auto_enable_subtitles TEXT DEFAULT 'foreign_only'
			CHECK(auto_enable_subtitles IN ('always', 'foreign_only', 'never')),
		last_scanned_at DATETIME
	);
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

// setupTestRepo creates a test repository with an in-memory database
func setupTestRepo(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	db := setupTestDB(t)
	baseRepo := common.NewBaseRepository(db, "sqlite")
	return NewRepository(baseRepo), db
}

func TestRepository_Create(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	lib := &library.Library{
		Name: "Test Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}

	// Test creation
	if err := repo.Create(ctx, lib); err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	if lib.ID == 0 {
		t.Error("Expected ID to be set after creation")
	}
	if lib.Name != "Test Library" {
		t.Errorf("Expected name 'Test Library', got %s", lib.Name)
	}
	if lib.Path != "/test/path" {
		t.Errorf("Expected path '/test/path', got %s", lib.Path)
	}
	if lib.Type != library.LibraryTypeMovies {
		t.Errorf("Expected type Local, got %v", lib.Type)
	}
	if lib.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
	if lib.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}

	// Test duplicate path error
	lib2 := &library.Library{
		Name: "Another Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}
	if err := repo.Create(ctx, lib2); err == nil {
		t.Error("Expected error for duplicate path, got nil")
	}
}

func TestRepository_GetByID(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	// Create a library first
	lib := &library.Library{
		Name: "Test Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}
	if err := repo.Create(ctx, lib); err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Test retrieval
	found, err := repo.GetByID(ctx, lib.ID)
	if err != nil {
		t.Fatalf("Failed to get library: %v", err)
	}

	if found.ID != lib.ID {
		t.Errorf("Expected ID %d, got %d", lib.ID, found.ID)
	}
	if found.Name != "Test Library" {
		t.Errorf("Expected name 'Test Library', got %s", found.Name)
	}

	// Test not found
	_, err = repo.GetByID(ctx, 99999)
	if err != library.ErrLibraryNotFound {
		t.Errorf("Expected ErrLibraryNotFound, got %v", err)
	}
}

func TestRepository_GetByPath(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	// Create a library first
	lib := &library.Library{
		Name: "Test Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}
	if err := repo.Create(ctx, lib); err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Test retrieval by path
	found, err := repo.GetByPath(ctx, "/test/path")
	if err != nil {
		t.Fatalf("Failed to get library by path: %v", err)
	}

	if found.ID != lib.ID {
		t.Errorf("Expected ID %d, got %d", lib.ID, found.ID)
	}

	// Test not found
	_, err = repo.GetByPath(ctx, "/nonexistent/path")
	if err != library.ErrLibraryNotFound {
		t.Errorf("Expected ErrLibraryNotFound, got %v", err)
	}
}

func TestRepository_List(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	// Create multiple libraries
	lib1 := &library.Library{Name: "Library 1", Path: "/path/1", Type: library.LibraryTypeMovies}
	lib2 := &library.Library{Name: "Library 2", Path: "/path/2", Type: library.LibraryTypeTV}
	lib3 := &library.Library{Name: "Library 3", Path: "/path/3", Type: library.LibraryTypeMovies}

	if err := repo.Create(ctx, lib1); err != nil {
		t.Fatalf("Failed to create library 1: %v", err)
	}
	if err := repo.Create(ctx, lib2); err != nil {
		t.Fatalf("Failed to create library 2: %v", err)
	}
	if err := repo.Create(ctx, lib3); err != nil {
		t.Fatalf("Failed to create library 3: %v", err)
	}

	// Test list all
	libraries, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list libraries: %v", err)
	}

	if len(libraries) != 3 {
		t.Errorf("Expected 3 libraries, got %d", len(libraries))
	}
}

func TestRepository_Update(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	// Create a library first
	lib := &library.Library{
		Name: "Original Name",
		Path: "/original/path",
		Type: library.LibraryTypeMovies,
	}
	if err := repo.Create(ctx, lib); err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Update the library
	lib.Name = "Updated Name"
	lib.Path = "/updated/path"

	if err := repo.Update(ctx, lib); err != nil {
		t.Fatalf("Failed to update library: %v", err)
	}

	if lib.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got %s", lib.Name)
	}
	if lib.Path != "/updated/path" {
		t.Errorf("Expected path '/updated/path', got %s", lib.Path)
	}

	// Verify persistence
	found, err := repo.GetByID(ctx, lib.ID)
	if err != nil {
		t.Fatalf("Failed to get updated library: %v", err)
	}

	if found.Name != "Updated Name" {
		t.Errorf("Expected persisted name 'Updated Name', got %s", found.Name)
	}
}

func TestRepository_Delete(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	// Create a library first
	lib := &library.Library{
		Name: "Test Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}
	if err := repo.Create(ctx, lib); err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Delete the library
	if err := repo.Delete(ctx, lib.ID); err != nil {
		t.Fatalf("Failed to delete library: %v", err)
	}

	// Verify deletion
	_, err := repo.GetByID(ctx, lib.ID)
	if err != library.ErrLibraryNotFound {
		t.Errorf("Expected ErrLibraryNotFound after deletion, got %v", err)
	}

	// Test deleting non-existent library (should not error)
	if err := repo.Delete(ctx, 99999); err != nil {
		t.Errorf("Expected no error deleting non-existent library, got %v", err)
	}
}

func TestRepository_Exists(t *testing.T) {
	repo, db := setupTestRepo(t)
	defer db.Close()
	ctx := context.Background()

	// Create a library first
	lib := &library.Library{
		Name: "Test Library",
		Path: "/test/path",
		Type: library.LibraryTypeMovies,
	}
	if err := repo.Create(ctx, lib); err != nil {
		t.Fatalf("Failed to create library: %v", err)
	}

	// Test exists by path
	exists, err := repo.Exists(ctx, "/test/path")
	if err != nil {
		t.Fatalf("Failed to check existence by path: %v", err)
	}
	if !exists {
		t.Error("Expected library to exist by path")
	}

	// Test non-existent by path
	exists, err = repo.Exists(ctx, "/nonexistent/path")
	if err != nil {
		t.Fatalf("Failed to check non-existence by path: %v", err)
	}
	if exists {
		t.Error("Expected library not to exist by non-existent path")
	}
}

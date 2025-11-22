package scanjob

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// setupTestDB creates an in-memory SQLite database with schema
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Create libraries table (required for foreign key)
	librariesSchema := `
	CREATE TABLE libraries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		type TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(librariesSchema); err != nil {
		t.Fatalf("Failed to create libraries schema: %v", err)
	}

	// Create scan_jobs table
	scanJobsSchema := `
	CREATE TABLE scan_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		library_id INTEGER NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('pending', 'running', 'paused', 'completed', 'failed')),
		progress REAL DEFAULT 0.0,
		files_found INTEGER DEFAULT 0,
		files_processed INTEGER DEFAULT 0,
		bytes_processed INTEGER DEFAULT 0,
		error_count INTEGER DEFAULT 0,
		started_at DATETIME,
		completed_at DATETIME,
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (library_id) REFERENCES libraries(id) ON DELETE CASCADE
	);

	CREATE INDEX idx_scan_jobs_library_id ON scan_jobs(library_id);
	CREATE INDEX idx_scan_jobs_status ON scan_jobs(status);
	CREATE INDEX idx_scan_jobs_created_at ON scan_jobs(created_at);
	`

	if _, err := db.Exec(scanJobsSchema); err != nil {
		t.Fatalf("Failed to create scan_jobs schema: %v", err)
	}

	// Insert test library
	_, err = db.Exec("INSERT INTO libraries (id, name, path, type) VALUES (1, 'Test Library', '/test/path', 'movies')")
	if err != nil {
		t.Fatalf("Failed to insert test library: %v", err)
	}

	return db
}

func TestNewRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		name   string
		driver string
	}{
		{"sqlite driver", "sqlite"},
		{"sqlite3 driver", "sqlite3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(db, tt.driver)
			if repo == nil {
				t.Error("Expected repository to be created, got nil")
			}
			if repo.db != db {
				t.Error("Expected db to be set")
			}
		})
	}
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name    string
		job     *scanner.ScanJob
		wantErr bool
		check   func(*testing.T, *scanner.ScanJob)
	}{
		{
			name: "create scan job with all fields",
			job: &scanner.ScanJob{
				LibraryID:      1,
				Status:         scanner.ScanStatusPending,
				Progress:       0.0,
				FilesFound:     0,
				FilesProcessed: 0,
				BytesProcessed: 0,
				ErrorCount:     0,
				StartedAt:      now,
			},
			wantErr: false,
			check: func(t *testing.T, job *scanner.ScanJob) {
				if job.ID == 0 {
					t.Error("Expected ID to be set after creation")
				}
				if job.LibraryID != 1 {
					t.Errorf("LibraryID = %d, want 1", job.LibraryID)
				}
				if job.Status != scanner.ScanStatusPending {
					t.Errorf("Status = %v, want %v", job.Status, scanner.ScanStatusPending)
				}
				if job.CreatedAt.IsZero() {
					t.Error("Expected CreatedAt to be set")
				}
				if job.UpdatedAt.IsZero() {
					t.Error("Expected UpdatedAt to be set")
				}
			},
		},
		{
			name: "create running scan job",
			job: &scanner.ScanJob{
				LibraryID:      1,
				Status:         scanner.ScanStatusRunning,
				Progress:       25.5,
				FilesFound:     100,
				FilesProcessed: 25,
				BytesProcessed: 1024000,
				ErrorCount:     2,
				StartedAt:      now,
			},
			wantErr: false,
			check: func(t *testing.T, job *scanner.ScanJob) {
				if job.Progress != 25.5 {
					t.Errorf("Progress = %f, want 25.5", job.Progress)
				}
				if job.FilesFound != 100 {
					t.Errorf("FilesFound = %d, want 100", job.FilesFound)
				}
				if job.FilesProcessed != 25 {
					t.Errorf("FilesProcessed = %d, want 25", job.FilesProcessed)
				}
			},
		},
		{
			name: "create with invalid library ID",
			job: &scanner.ScanJob{
				LibraryID: 999,
				Status:    scanner.ScanStatusPending,
				StartedAt: now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(ctx, tt.job)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, tt.job)
			}
		})
	}
}

func TestRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create test job
	job := &scanner.ScanJob{
		LibraryID:      1,
		Status:         scanner.ScanStatusRunning,
		Progress:       50.0,
		FilesFound:     200,
		FilesProcessed: 100,
		BytesProcessed: 2048000,
		ErrorCount:     5,
		StartedAt:      now,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	tests := []struct {
		name    string
		id      int64
		wantErr bool
		check   func(*testing.T, *scanner.ScanJob)
	}{
		{
			name:    "get existing job",
			id:      job.ID,
			wantErr: false,
			check: func(t *testing.T, result *scanner.ScanJob) {
				if result.ID != job.ID {
					t.Errorf("ID = %d, want %d", result.ID, job.ID)
				}
				if result.LibraryID != 1 {
					t.Errorf("LibraryID = %d, want 1", result.LibraryID)
				}
				if result.Status != scanner.ScanStatusRunning {
					t.Errorf("Status = %v, want %v", result.Status, scanner.ScanStatusRunning)
				}
				if result.Progress != 50.0 {
					t.Errorf("Progress = %f, want 50.0", result.Progress)
				}
				if result.FilesFound != 200 {
					t.Errorf("FilesFound = %d, want 200", result.FilesFound)
				}
			},
		},
		{
			name:    "get non-existent job",
			id:      999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByID(ctx, tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if err != scanner.ErrNotFound {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestRepository_GetLatestByLibrary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create multiple jobs for library 1
	older := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusCompleted,
		StartedAt: now.Add(-2 * time.Hour),
	}
	if err := repo.Create(ctx, older); err != nil {
		t.Fatalf("Failed to create older job: %v", err)
	}

	// Manually set older created_at to simulate an older job
	olderTime := now.Add(-1 * time.Hour)
	_, err := db.Exec("UPDATE scan_jobs SET created_at = ? WHERE id = ?", olderTime, older.ID)
	if err != nil {
		t.Fatalf("Failed to update created_at for older job: %v", err)
	}

	newer := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		StartedAt: now,
	}
	if err := repo.Create(ctx, newer); err != nil {
		t.Fatalf("Failed to create newer job: %v", err)
	}

	tests := []struct {
		name      string
		libraryID int64
		wantErr   bool
		check     func(*testing.T, *scanner.ScanJob)
	}{
		{
			name:      "get latest job for library with multiple scans",
			libraryID: 1,
			wantErr:   false,
			check: func(t *testing.T, result *scanner.ScanJob) {
				// The newer job (created second) should be returned
				// since the query orders by created_at DESC
				if result.Status != scanner.ScanStatusRunning {
					t.Errorf("Status = %v, want %v (newer job)", result.Status, scanner.ScanStatusRunning)
				}
				// Verify it's actually the newer job by checking created_at
				if !result.CreatedAt.After(older.CreatedAt) && !result.CreatedAt.Equal(older.CreatedAt) {
					t.Errorf("Expected newer job, but got older one")
				}
			},
		},
		{
			name:      "no scans for library",
			libraryID: 999,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetLatestByLibrary(ctx, tt.libraryID)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if err != scanner.ErrNotFound {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestRepository_ListByLibrary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create multiple jobs
	for i := 0; i < 5; i++ {
		job := &scanner.ScanJob{
			LibraryID: 1,
			Status:    scanner.ScanStatusCompleted,
			StartedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := repo.Create(ctx, job); err != nil {
			t.Fatalf("Failed to create job %d: %v", i, err)
		}
		time.Sleep(5 * time.Millisecond) // Ensure different timestamps
	}

	tests := []struct {
		name      string
		libraryID int64
		limit     int32
		wantErr   bool
		wantCount int
	}{
		{
			name:      "list all jobs",
			libraryID: 1,
			limit:     10,
			wantErr:   false,
			wantCount: 5,
		},
		{
			name:      "list with limit",
			libraryID: 1,
			limit:     3,
			wantErr:   false,
			wantCount: 3,
		},
		{
			name:      "list for library with no jobs",
			libraryID: 999,
			limit:     10,
			wantErr:   false,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.ListByLibrary(ctx, tt.libraryID, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("Got %d jobs, want %d", len(result), tt.wantCount)
			}

			// Verify jobs are ordered by created_at DESC (newest first)
			if len(result) > 1 {
				for i := 0; i < len(result)-1; i++ {
					if result[i].CreatedAt.Before(result[i+1].CreatedAt) {
						t.Error("Jobs are not ordered by created_at DESC")
						break
					}
				}
			}
		})
	}
}

func TestRepository_ListRunning(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create jobs with different statuses
	statuses := []scanner.ScanStatus{
		scanner.ScanStatusCompleted,
		scanner.ScanStatusRunning,
		scanner.ScanStatusRunning,
		scanner.ScanStatusFailed,
		scanner.ScanStatusPending,
	}

	for _, status := range statuses {
		job := &scanner.ScanJob{
			LibraryID: 1,
			Status:    status,
			StartedAt: now,
		}
		if err := repo.Create(ctx, job); err != nil {
			t.Fatalf("Failed to create job with status %v: %v", status, err)
		}
	}

	result, err := repo.ListRunning(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Got %d running jobs, want 2", len(result))
	}

	for _, job := range result {
		if job.Status != scanner.ScanStatusRunning {
			t.Errorf("Expected running status, got %v", job.Status)
		}
	}
}

func TestRepository_UpdateProgress(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create test job
	job := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		StartedAt: now,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	// Update progress
	progress := &scanner.Progress{
		FilesFound:     500,
		FilesProcessed: 250,
		BytesProcessed: 10240000,
		ErrorCount:     10,
	}

	if err := repo.UpdateProgress(ctx, job.ID, progress); err != nil {
		t.Fatalf("Failed to update progress: %v", err)
	}

	// Verify the update
	updated, err := repo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("Failed to get updated job: %v", err)
	}

	if updated.FilesFound != progress.FilesFound {
		t.Errorf("FilesFound = %d, want %d", updated.FilesFound, progress.FilesFound)
	}
	if updated.FilesProcessed != progress.FilesProcessed {
		t.Errorf("FilesProcessed = %d, want %d", updated.FilesProcessed, progress.FilesProcessed)
	}
	if updated.BytesProcessed != progress.BytesProcessed {
		t.Errorf("BytesProcessed = %d, want %d", updated.BytesProcessed, progress.BytesProcessed)
	}
	if updated.ErrorCount != progress.ErrorCount {
		t.Errorf("ErrorCount = %d, want %d", updated.ErrorCount, progress.ErrorCount)
	}

	// Test update on non-existent job (should not error in SQLite, just no-op)
	nonExistentProgress := &scanner.Progress{FilesFound: 100}
	if err := repo.UpdateProgress(ctx, 999, nonExistentProgress); err != nil {
		t.Errorf("Unexpected error for non-existent job: %v", err)
	}
}

func TestRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create test job
	job := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusPending,
		StartedAt: now,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	// Update status to running
	if err := repo.UpdateStatus(ctx, job.ID, scanner.ScanStatusRunning); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Verify the update
	updated, err := repo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("Failed to get updated job: %v", err)
	}

	if updated.Status != scanner.ScanStatusRunning {
		t.Errorf("Status = %v, want %v", updated.Status, scanner.ScanStatusRunning)
	}

	// Test update on non-existent job (should not error in SQLite, just no-op)
	if err := repo.UpdateStatus(ctx, 999, scanner.ScanStatusCompleted); err != nil {
		t.Errorf("Unexpected error for non-existent job: %v", err)
	}
}

func TestRepository_Complete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create test job
	job := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		StartedAt: now,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	completedTime := now.Add(5 * time.Minute)

	// Complete the job
	completedJob := &scanner.ScanJob{
		ID:             job.ID,
		Status:         scanner.ScanStatusCompleted,
		Progress:       100.0,
		FilesFound:     1000,
		FilesProcessed: 1000,
		BytesProcessed: 50000000,
		ErrorCount:     0,
		CompletedAt:    &completedTime,
	}

	if err := repo.Complete(ctx, completedJob); err != nil {
		t.Fatalf("Failed to complete job: %v", err)
	}

	// Verify the completion
	completed, err := repo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("Failed to get completed job: %v", err)
	}

	if completed.Status != scanner.ScanStatusCompleted {
		t.Errorf("Status = %v, want %v", completed.Status, scanner.ScanStatusCompleted)
	}
	if completed.Progress != 100.0 {
		t.Errorf("Progress = %f, want 100.0", completed.Progress)
	}
	if completed.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}

	// Test complete on non-existent job (should not error in SQLite, just no-op)
	nonExistentJob := &scanner.ScanJob{
		ID:     999,
		Status: scanner.ScanStatusCompleted,
	}
	if err := repo.Complete(ctx, nonExistentJob); err != nil {
		t.Errorf("Unexpected error for non-existent job: %v", err)
	}
}

func TestRepository_Complete_WithError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create test job
	job := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		StartedAt: now,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	completedTime := now.Add(2 * time.Minute)

	// Complete with error
	job.Status = scanner.ScanStatusFailed
	job.Progress = 45.0
	job.FilesFound = 500
	job.FilesProcessed = 225
	job.ErrorCount = 50
	job.CompletedAt = &completedTime
	job.ErrorMessage = "filesystem error: permission denied"

	if err := repo.Complete(ctx, job); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the error completion
	completed, err := repo.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("Failed to get completed job: %v", err)
	}

	if completed.Status != scanner.ScanStatusFailed {
		t.Errorf("Status = %v, want %v", completed.Status, scanner.ScanStatusFailed)
	}
	if completed.ErrorMessage != "filesystem error: permission denied" {
		t.Errorf("ErrorMessage = %q, want %q", completed.ErrorMessage, "filesystem error: permission denied")
	}
	if completed.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
}

func TestRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create test job
	job := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusCompleted,
		StartedAt: now,
	}
	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("Failed to create test job: %v", err)
	}

	tests := []struct {
		name    string
		id      int64
		wantErr bool
	}{
		{
			name:    "delete existing job",
			id:      job.ID,
			wantErr: false,
		},
		{
			name:    "delete non-existent job (no error)",
			id:      999,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Delete(ctx, tt.id)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify deletion (only for valid job)
			if tt.id == job.ID {
				_, err := repo.GetByID(ctx, tt.id)
				if err != scanner.ErrNotFound {
					t.Errorf("Expected ErrNotFound after deletion, got %v", err)
				}
			}
		})
	}
}

func TestRepository_DeleteOld(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db, "sqlite")
	ctx := context.Background()
	now := time.Now()

	// Create jobs with different completion times
	oldCompleted := now.Add(-10 * 24 * time.Hour)
	recentCompleted := now.Add(-3 * 24 * time.Hour)

	// Old completed job (should be deleted)
	oldJob := &scanner.ScanJob{
		LibraryID:   1,
		Status:      scanner.ScanStatusCompleted,
		StartedAt:   oldCompleted,
		CompletedAt: &oldCompleted,
	}
	if err := repo.Create(ctx, oldJob); err != nil {
		t.Fatalf("Failed to create old job: %v", err)
	}

	// Manually update created_at to simulate an old job
	// The DELETE query filters by created_at, not completed_at
	_, err := db.Exec("UPDATE scan_jobs SET created_at = ? WHERE id = ?", oldCompleted, oldJob.ID)
	if err != nil {
		t.Fatalf("Failed to update created_at for old job: %v", err)
	}

	// Recent completed job (should be kept)
	recentJob := &scanner.ScanJob{
		LibraryID:   1,
		Status:      scanner.ScanStatusCompleted,
		StartedAt:   recentCompleted,
		CompletedAt: &recentCompleted,
	}
	if err := repo.Create(ctx, recentJob); err != nil {
		t.Fatalf("Failed to create recent job: %v", err)
	}

	// Update created_at to simulate a recent job
	_, err = db.Exec("UPDATE scan_jobs SET created_at = ? WHERE id = ?", recentCompleted, recentJob.ID)
	if err != nil {
		t.Fatalf("Failed to update created_at for recent job: %v", err)
	}

	// Running job (should be kept)
	runningJob := &scanner.ScanJob{
		LibraryID: 1,
		Status:    scanner.ScanStatusRunning,
		StartedAt: now,
	}
	if err := repo.Create(ctx, runningJob); err != nil {
		t.Fatalf("Failed to create running job: %v", err)
	}

	// Delete old jobs (older than 7 days)
	if err := repo.DeleteOld(ctx, 1, 7); err != nil {
		t.Fatalf("Failed to delete old jobs: %v", err)
	}

	// Verify old job is deleted
	_, err = repo.GetByID(ctx, oldJob.ID)
	if err != scanner.ErrNotFound {
		t.Errorf("Expected old job to be deleted, got error: %v", err)
	}

	// Verify recent job still exists
	_, err = repo.GetByID(ctx, recentJob.ID)
	if err != nil {
		t.Errorf("Expected recent job to exist, got error: %v", err)
	}

	// Verify running job still exists
	_, err = repo.GetByID(ctx, runningJob.ID)
	if err != nil {
		t.Errorf("Expected running job to exist, got error: %v", err)
	}
}

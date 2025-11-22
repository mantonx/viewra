package library

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

// Mock library repository for StartScan tests
type mockLibraryRepoForScan struct {
	getByIDFunc func(ctx context.Context, id int64) (*library.Library, error)
}

func (m *mockLibraryRepoForScan) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, library.ErrLibraryNotFound
}

func (m *mockLibraryRepoForScan) Create(ctx context.Context, lib *library.Library) error {
	return nil
}

func (m *mockLibraryRepoForScan) List(ctx context.Context) ([]*library.Library, error) {
	return nil, nil
}

func (m *mockLibraryRepoForScan) Update(ctx context.Context, lib *library.Library) error {
	return nil
}

func (m *mockLibraryRepoForScan) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockLibraryRepoForScan) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	return nil, library.ErrLibraryNotFound
}

func (m *mockLibraryRepoForScan) Exists(ctx context.Context, path string) (bool, error) {
	return false, nil
}

// Transaction-aware methods (delegate to non-transactional versions for mock)
func (m *mockLibraryRepoForScan) CreateWithTx(ctx context.Context, tx *sql.Tx, lib *library.Library) error {
	return m.Create(ctx, lib)
}

func (m *mockLibraryRepoForScan) GetByIDWithTx(ctx context.Context, tx *sql.Tx, id int64) (*library.Library, error) {
	return m.GetByID(ctx, id)
}

func (m *mockLibraryRepoForScan) DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error {
	return m.Delete(ctx, id)
}

func (m *mockLibraryRepoForScan) ExistsWithTx(ctx context.Context, tx *sql.Tx, path string) (bool, error) {
	return m.Exists(ctx, path)
}

func TestScanLibraryUseCase_StartScan(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		libraryID     int64
		setupMocks    func(*mockLibraryRepoForScan, *mockScanJobRepository)
		wantErr       bool
		checkErr      func(*testing.T, error)
		checkResponse func(*testing.T, StartScanResponse)
	}{
		{
			name:      "successful scan start",
			libraryID: 1,
			setupMocks: func(libRepo *mockLibraryRepoForScan, scanRepo *mockScanJobRepository) {
				libRepo.getByIDFunc = func(ctx context.Context, id int64) (*library.Library, error) {
					return &library.Library{
						ID:   1,
						Name: "Movies",
						Path: "/movies",
						Type: library.LibraryTypeMovies,
					}, nil
				}

				// No running scans
				scanRepo.jobs = make(map[int64]*scanner.ScanJob)
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp StartScanResponse) {
				if resp.LibraryID != 1 {
					t.Errorf("Expected LibraryID 1, got %d", resp.LibraryID)
				}
				if resp.Status != string(scanner.ScanStatusRunning) {
					t.Errorf("Expected status 'running', got '%s'", resp.Status)
				}
			},
		},
		{
			name:      "library not found",
			libraryID: 999,
			setupMocks: func(libRepo *mockLibraryRepoForScan, scanRepo *mockScanJobRepository) {
				libRepo.getByIDFunc = func(ctx context.Context, id int64) (*library.Library, error) {
					return nil, library.ErrLibraryNotFound
				}
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				if !errors.Is(err, library.ErrLibraryNotFound) {
					t.Errorf("Expected ErrLibraryNotFound, got %v", err)
				}
			},
		},
		{
			name:      "scan already running for same library",
			libraryID: 1,
			setupMocks: func(libRepo *mockLibraryRepoForScan, scanRepo *mockScanJobRepository) {
				libRepo.getByIDFunc = func(ctx context.Context, id int64) (*library.Library, error) {
					return &library.Library{
						ID:   1,
						Name: "Movies",
						Path: "/movies",
						Type: library.LibraryTypeMovies,
					}, nil
				}

				// Create existing running scan for same library
				scanRepo.jobs = map[int64]*scanner.ScanJob{
					1: {
						ID:        1,
						LibraryID: 1,
						Status:    scanner.ScanStatusRunning,
						StartedAt: now,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
			},
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				if !errors.Is(err, scanner.ErrAlreadyRunning) {
					t.Errorf("Expected ErrAlreadyRunning, got %v", err)
				}
			},
		},
		{
			name:      "scan running for different library (should succeed)",
			libraryID: 2,
			setupMocks: func(libRepo *mockLibraryRepoForScan, scanRepo *mockScanJobRepository) {
				libRepo.getByIDFunc = func(ctx context.Context, id int64) (*library.Library, error) {
					if id == 2 {
						return &library.Library{
							ID:   2,
							Name: "TV Shows",
							Path: "/tv",
							Type: library.LibraryTypeTV,
						}, nil
					}
					return nil, library.ErrLibraryNotFound
				}

				// Create existing running scan for different library (ID=1)
				scanRepo.jobs = map[int64]*scanner.ScanJob{
					1: {
						ID:        1,
						LibraryID: 1,
						Status:    scanner.ScanStatusRunning,
						StartedAt: now,
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp StartScanResponse) {
				if resp.LibraryID != 2 {
					t.Errorf("Expected LibraryID 2, got %d", resp.LibraryID)
				}
				if resp.Status != string(scanner.ScanStatusRunning) {
					t.Errorf("Expected status 'running', got '%s'", resp.Status)
				}
			},
		},
		{
			name:      "multiple completed scans exist (should succeed)",
			libraryID: 1,
			setupMocks: func(libRepo *mockLibraryRepoForScan, scanRepo *mockScanJobRepository) {
				libRepo.getByIDFunc = func(ctx context.Context, id int64) (*library.Library, error) {
					return &library.Library{
						ID:   1,
						Name: "Movies",
						Path: "/movies",
						Type: library.LibraryTypeMovies,
					}, nil
				}

				completedAt := now.Add(-1 * time.Hour)
				// Only completed scans exist
				scanRepo.jobs = map[int64]*scanner.ScanJob{
					1: {
						ID:          1,
						LibraryID:   1,
						Status:      scanner.ScanStatusCompleted,
						StartedAt:   now.Add(-2 * time.Hour),
						CompletedAt: &completedAt,
						CreatedAt:   now.Add(-2 * time.Hour),
						UpdatedAt:   completedAt,
					},
				}
			},
			wantErr: false,
			checkResponse: func(t *testing.T, resp StartScanResponse) {
				if resp.LibraryID != 1 {
					t.Errorf("Expected LibraryID 1, got %d", resp.LibraryID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			libRepo := &mockLibraryRepoForScan{}
			scanRepo := newMockScanJobRepository()
			mediaRepo := mocks.NewMediaRepository(t)
			movieRepo := mocks.NewMovieRepository(t)
			tvRepo := mocks.NewTVRepository(t)
			musicRepo := mocks.NewMusicRepository(t)

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks(libRepo, scanRepo)
			}

			// Create use case
			uc := &ScanLibraryUseCase{
				libraryRepo: libRepo,
				mediaRepo:   mediaRepo,
				movieRepo:   movieRepo,
				tvRepo:      tvRepo,
				musicRepo:   musicRepo,
				scanJobRepo: scanRepo,
			}

			// Execute
			resp, err := uc.StartScan(context.Background(), tt.libraryID)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
					return
				}
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
				return
			}

			// Check success expectations
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, resp)
			}

			// Verify a scan job was created
			if len(scanRepo.jobs) == 0 {
				t.Error("Expected scan job to be created")
			}

			// Give the goroutine a moment to start
			time.Sleep(10 * time.Millisecond)
		})
	}
}

func TestScanLibraryUseCase_StartScan_CreationError(t *testing.T) {
	// Test case where scan job creation fails
	libRepo := &mockLibraryRepoForScan{
		getByIDFunc: func(ctx context.Context, id int64) (*library.Library, error) {
			return &library.Library{
				ID:   1,
				Name: "Movies",
				Path: "/movies",
				Type: library.LibraryTypeMovies,
			}, nil
		},
	}

	// Create a custom mock that returns error on Create
	scanRepo := &mockScanJobRepositoryWithError{
		mockScanJobRepository: newMockScanJobRepository(),
		createErr:             errors.New("database error"),
	}

	uc := &ScanLibraryUseCase{
		libraryRepo: libRepo,
		mediaRepo:   mocks.NewMediaRepository(t),
		movieRepo:   mocks.NewMovieRepository(t),
		tvRepo:      mocks.NewTVRepository(t),
		musicRepo:   mocks.NewMusicRepository(t),
		scanJobRepo: scanRepo,
	}

	_, err := uc.StartScan(context.Background(), 1)
	if err == nil {
		t.Error("Expected error when job creation fails, got nil")
	}
	if !errors.Is(err, scanRepo.createErr) {
		t.Errorf("Expected error to contain database error, got %v", err)
	}
}

// mockScanJobRepositoryWithError wraps the mock to return errors
type mockScanJobRepositoryWithError struct {
	*mockScanJobRepository
	createErr      error
	listRunningErr error
}

func (m *mockScanJobRepositoryWithError) Create(ctx context.Context, job *scanner.ScanJob) error {
	if m.createErr != nil {
		return m.createErr
	}
	return m.mockScanJobRepository.Create(ctx, job)
}

func (m *mockScanJobRepositoryWithError) ListRunning(ctx context.Context) ([]*scanner.ScanJob, error) {
	if m.listRunningErr != nil {
		return nil, m.listRunningErr
	}
	return m.mockScanJobRepository.ListRunning(ctx)
}

func TestScanLibraryUseCase_StartScan_ListRunningError(t *testing.T) {
	// Test case where ListRunning fails
	libRepo := &mockLibraryRepoForScan{
		getByIDFunc: func(ctx context.Context, id int64) (*library.Library, error) {
			return &library.Library{
				ID:   1,
				Name: "Movies",
				Path: "/movies",
				Type: library.LibraryTypeMovies,
			}, nil
		},
	}

	// Create a custom mock that returns error on ListRunning
	scanRepo := &mockScanJobRepositoryWithError{
		mockScanJobRepository: newMockScanJobRepository(),
		listRunningErr:        errors.New("database connection error"),
	}

	uc := &ScanLibraryUseCase{
		libraryRepo: libRepo,
		mediaRepo:   mocks.NewMediaRepository(t),
		movieRepo:   mocks.NewMovieRepository(t),
		tvRepo:      mocks.NewTVRepository(t),
		musicRepo:   mocks.NewMusicRepository(t),
		scanJobRepo: scanRepo,
	}

	_, err := uc.StartScan(context.Background(), 1)
	if err == nil {
		t.Error("Expected error when ListRunning fails, got nil")
	}
}

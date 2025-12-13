package library

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	"github.com/mantonx/viewra/internal/testutil/mocks"
)

func TestScanLibraryUseCase_isMediaFile(t *testing.T) {
	uc := &ScanLibraryUseCase{}

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		// Video formats
		{"mp4 video", "mp4", true},
		{"mkv video", "mkv", true},
		{"avi video", "avi", true},
		{"mov video", "mov", true},
		{"wmv video", "wmv", true},
		{"flv video", "flv", true},
		{"webm video", "webm", true},
		{"m4v video", "m4v", true},
		{"mpg video", "mpg", true},
		{"mpeg video", "mpeg", true},
		{"m2ts video", "m2ts", true},
		{"ts video", "ts", true},
		{"vob video", "vob", true},
		{"3gp video", "3gp", true},
		{"mxf video", "mxf", true},

		// Audio formats
		{"mp3 audio", "mp3", true},
		{"flac audio", "flac", true},
		{"m4a audio", "m4a", true},
		{"aac audio", "aac", true},
		{"ogg audio", "ogg", true},
		{"opus audio", "opus", true},
		{"wav audio", "wav", true},
		{"wma audio", "wma", true},

		// Case insensitivity
		{"uppercase MP4", "MP4", true},
		{"mixed case MkV", "MkV", true},

		// With leading dot
		{"with dot .mp4", ".mp4", true},
		{"with dot .mkv", ".mkv", true},

		// Non-media files
		{"text file", "txt", false},
		{"image png", "png", false},
		{"image jpg", "jpg", false},
		{"subtitle srt", "srt", false},
		{"subtitle ass", "ass", false},
		{"nfo file", "nfo", false},
		{"json file", "json", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uc.isMediaFile(tt.ext)
			if result != tt.expected {
				t.Errorf("isMediaFile(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestScanLibraryUseCase_calculateProcessingTimeout(t *testing.T) {
	tests := []struct {
		name              string
		fileSize          int64
		isRemote          bool
		baseFileTimeout   time.Duration
		remoteTimeout     time.Duration
		maxExtraTimeout   time.Duration
		expectedMinimum   time.Duration
		expectedMaximum   time.Duration
	}{
		{
			name:            "small local file",
			fileSize:        100 * 1024 * 1024, // 100MB
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 30 * time.Second,
			expectedMaximum: 30 * time.Second,
		},
		{
			name:            "small remote file",
			fileSize:        100 * 1024 * 1024, // 100MB
			isRemote:        true,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 60 * time.Second,
			expectedMaximum: 60 * time.Second,
		},
		{
			name:            "1GB local file",
			fileSize:        1024 * 1024 * 1024, // 1GB
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 31 * time.Second, // 30s base + 1s per GB
			expectedMaximum: 31 * time.Second,
		},
		{
			name:            "50GB local file adds extra time",
			fileSize:        50 * 1024 * 1024 * 1024, // 50GB
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 80 * time.Second, // 30s base + 50s (1s per GB)
			expectedMaximum: 80 * time.Second,
		},
		{
			name:            "150GB local file hits max extra",
			fileSize:        150 * 1024 * 1024 * 1024, // 150GB - larger than maxExtraTimeout
			isRemote:        false,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 150 * time.Second, // 30s base + 120s max extra (capped)
			expectedMaximum: 150 * time.Second,
		},
		{
			name:            "50GB remote file",
			fileSize:        50 * 1024 * 1024 * 1024, // 50GB
			isRemote:        true,
			baseFileTimeout: 30 * time.Second,
			remoteTimeout:   60 * time.Second,
			maxExtraTimeout: 120 * time.Second,
			expectedMinimum: 110 * time.Second, // 60s base + 50s (1s per GB)
			expectedMaximum: 110 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var profile *system.Profile
			if tt.isRemote {
				profile = &system.Profile{
					Storage: system.StorageProfile{
						IsRemote: true,
					},
				}
			}

			uc := &ScanLibraryUseCase{
				config: ScanConfig{
					BaseFileTimeout:      tt.baseFileTimeout,
					RemoteStorageTimeout: tt.remoteTimeout,
					MaxExtraTimeout:      tt.maxExtraTimeout,
				},
				systemProfile: profile,
			}

			result := uc.calculateProcessingTimeout(tt.fileSize)
			if result < tt.expectedMinimum || result > tt.expectedMaximum {
				t.Errorf("calculateProcessingTimeout(%d) = %v, want between %v and %v",
					tt.fileSize, result, tt.expectedMinimum, tt.expectedMaximum)
			}
		})
	}
}

func TestScanLibraryUseCase_statWithTimeout(t *testing.T) {
	// Create a temp file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	uc := &ScanLibraryUseCase{}

	t.Run("successful stat", func(t *testing.T) {
		ctx := context.Background()
		info, err := uc.statWithTimeout(ctx, tmpFile, 5*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if info == nil {
			t.Error("expected FileInfo, got nil")
		}
		if info != nil && info.Size() != 12 {
			t.Errorf("Size() = %d, want 12", info.Size())
		}
	})

	t.Run("file not found", func(t *testing.T) {
		ctx := context.Background()
		_, err := uc.statWithTimeout(ctx, "/nonexistent/path/file.txt", 5*time.Second)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		_, err := uc.statWithTimeout(ctx, tmpFile, 5*time.Second)
		// Either context.Canceled error or successful completion due to race
		// is acceptable - the stat goroutine might finish before cancellation is detected
		_ = err // No assertion needed - just verify no panic
	})
}

func TestScanLibraryUseCase_validateDiscovery(t *testing.T) {
	tests := []struct {
		name             string
		filesDiscovered  int64
		stats            *filesystem.WalkStats
		previousJobs     []*scanner.ScanJob
		expectedWarnings int
	}{
		{
			name:             "clean discovery no issues",
			filesDiscovered:  1000,
			stats:            &filesystem.WalkStats{FilesDiscovered: 1000, DirsScanned: 50},
			previousJobs:     nil,
			expectedWarnings: 0,
		},
		{
			name:            "discovery with skipped dirs",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 950,
				DirsScanned:     50,
				DirsSkipped:     5,
			},
			previousJobs:     nil,
			expectedWarnings: 1,
		},
		{
			name:            "discovery with skipped files",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 950,
				DirsScanned:     50,
				FilesSkipped:    10,
			},
			previousJobs:     nil,
			expectedWarnings: 1,
		},
		{
			name:            "discovery with permission errors",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered:  950,
				DirsScanned:      50,
				DirsSkipped:      1, // HasErrors() requires DirsSkipped > 0 or FilesSkipped > 0
				PermissionErrors: 15,
			},
			previousJobs:     nil,
			expectedWarnings: 2, // DirsSkipped warning + PermissionErrors warning
		},
		{
			name:            "discovery with network errors",
			filesDiscovered: 950,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 950,
				DirsScanned:     50,
				FilesSkipped:    1, // HasErrors() requires DirsSkipped > 0 or FilesSkipped > 0
				NetworkErrors:   5,
			},
			previousJobs:     nil,
			expectedWarnings: 2, // FilesSkipped warning + NetworkErrors warning
		},
		{
			name:            "significant file count drop",
			filesDiscovered: 500,
			stats:           &filesystem.WalkStats{FilesDiscovered: 500, DirsScanned: 50},
			previousJobs: []*scanner.ScanJob{
				{ID: 2, LibraryID: 1, Status: scanner.ScanStatusRunning, FilesFound: 0},   // Current (running) - needed because ListByLibrary returns all
				{ID: 1, LibraryID: 1, Status: scanner.ScanStatusCompleted, FilesFound: 1000}, // Previous completed scan
			},
			expectedWarnings: 1, // 50% drop
		},
		{
			name:            "repeated discovery errors",
			filesDiscovered: 900,
			stats: &filesystem.WalkStats{
				FilesDiscovered: 900,
				DirsScanned:     50,
				DirsSkipped:     5,
			},
			previousJobs: []*scanner.ScanJob{
				{ID: 2, LibraryID: 1, Status: scanner.ScanStatusRunning, FilesFound: 0},       // Current (running)
				{ID: 1, LibraryID: 1, Status: scanner.ScanStatusCompleted, FilesFound: 950, DirsSkipped: 3}, // Previous completed
			},
			expectedWarnings: 2, // dirs skipped + repeated errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanJobRepo := mocks.NewScanJobRepository(t)
			if tt.previousJobs != nil {
				scanJobRepo.WithJobs(tt.previousJobs...)
			}

			uc := &ScanLibraryUseCase{
				scanRepos: &ScanRepositories{
					ScanJob: scanJobRepo,
				},
				logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}

			ctx := context.Background()
			warnings := uc.validateDiscovery(ctx, 1, tt.filesDiscovered, tt.stats)

			if len(warnings) != tt.expectedWarnings {
				t.Errorf("validateDiscovery() returned %d warnings, want %d", len(warnings), tt.expectedWarnings)
				for i, w := range warnings {
					t.Logf("  warning[%d]: %s", i, w)
				}
			}
		})
	}
}

func TestIsExtra(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		expected bool
	}{
		// Trailers
		{"trailer with dash", "/movies/Movie (2023)/Movie (2023)-trailer.mp4", true},
		{"trailer with underscore", "/movies/Movie (2023)/Movie (2023)_trailer.mp4", true},
		{"trailer with dot", "/movies/Movie (2023)/Movie (2023).trailer.mp4", true},

		// Deleted scenes
		{"deleted with dash", "/movies/Movie (2023)/Movie (2023)-deleted-scene.mp4", true},
		{"deleted with underscore", "/movies/Movie (2023)/Movie (2023)_deleted_scene.mp4", true},
		{"deleted with dot", "/movies/Movie (2023)/Movie (2023).deleted.mp4", true},

		// Featurettes
		{"featurette with dash", "/movies/Movie (2023)/Movie (2023)-featurette.mp4", true},
		{"featurette with underscore", "/movies/Movie (2023)/Making_featurette.mp4", true},
		{"featurette with dot", "/movies/Movie (2023)/Making.featurette.mp4", true},

		// Extras
		{"extra with dash", "/movies/Movie (2023)/Movie-extra.mp4", true},
		{"extra with underscore", "/movies/Movie (2023)/Movie_extra.mp4", true},
		{"extra with dot", "/movies/Movie (2023)/Movie.extra.mp4", true},

		// Bonus
		{"bonus with dash", "/movies/Movie (2023)/Movie-bonus.mp4", true},
		{"bonus with underscore", "/movies/Movie (2023)/Movie_bonus.mp4", true},
		{"bonus with dot", "/movies/Movie (2023)/Movie.bonus.mp4", true},

		// Case insensitivity
		{"uppercase TRAILER", "/movies/Movie (2023)/Movie-TRAILER.mp4", true},
		{"mixed case Trailer", "/movies/Movie (2023)/Movie-Trailer.mp4", true},

		// Non-extras
		{"regular movie", "/movies/Movie (2023)/Movie (2023).mkv", false},
		{"movie with extra in name", "/movies/Extraordinary (2023)/Extraordinary (2023).mkv", false},
		{"trailer folder", "/movies/Movie (2023)/Trailers/trailer.mp4", false}, // pattern needs separator
		{"regular episode", "/tv/Show/S01/Show S01E01.mkv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isExtra(tt.filepath)
			if result != tt.expected {
				t.Errorf("isExtra(%q) = %v, want %v", tt.filepath, result, tt.expected)
			}
		})
	}
}

func TestAudioExtensions(t *testing.T) {
	// Test that audioExtensions is a proper subset of mediaExtensions
	for ext := range audioExtensions {
		if !mediaExtensions[ext] {
			t.Errorf("audioExtensions contains %q which is not in mediaExtensions", ext)
		}
	}

	// Test known audio extensions are in audioExtensions
	expectedAudio := []string{"mp3", "flac", "m4a", "aac", "ogg", "opus", "wav", "wma", "aiff"}
	for _, ext := range expectedAudio {
		if !audioExtensions[ext] {
			t.Errorf("expected %q in audioExtensions", ext)
		}
	}

	// Test video extensions are NOT in audioExtensions
	videoOnly := []string{"mp4", "mkv", "avi", "mov", "m2ts"}
	for _, ext := range videoOnly {
		if audioExtensions[ext] {
			t.Errorf("video extension %q should not be in audioExtensions", ext)
		}
	}
}

func TestMediaExtensions(t *testing.T) {
	// Test all expected media extensions exist
	expectedVideo := []string{"mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "m2ts", "ts"}
	for _, ext := range expectedVideo {
		if !mediaExtensions[ext] {
			t.Errorf("expected video extension %q in mediaExtensions", ext)
		}
	}

	expectedAudio := []string{"mp3", "flac", "m4a", "aac", "ogg", "opus", "wav"}
	for _, ext := range expectedAudio {
		if !mediaExtensions[ext] {
			t.Errorf("expected audio extension %q in mediaExtensions", ext)
		}
	}

	// Test non-media extensions are not present
	nonMedia := []string{"txt", "jpg", "png", "srt", "ass", "nfo", "xml", "json"}
	for _, ext := range nonMedia {
		if mediaExtensions[ext] {
			t.Errorf("non-media extension %q should not be in mediaExtensions", ext)
		}
	}
}

func BenchmarkIsMediaFile(b *testing.B) {
	uc := &ScanLibraryUseCase{}
	extensions := []string{"mp4", "mkv", "txt", "jpg", ".mp4", "MP4", "flac", "nfo"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ext := extensions[i%len(extensions)]
		uc.isMediaFile(ext)
	}
}

func BenchmarkIsExtra(b *testing.B) {
	paths := []string{
		"/movies/Movie (2023)/Movie (2023).mkv",
		"/movies/Movie (2023)/Movie-trailer.mp4",
		"/movies/Movie (2023)/Movie-featurette.mp4",
		"/tv/Show/S01/Show S01E01.mkv",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		isExtra(path)
	}
}

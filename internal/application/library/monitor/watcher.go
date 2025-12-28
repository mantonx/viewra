package monitor

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mantonx/viewra/internal/domain/library"
)

// WatcherMode indicates whether to use fsnotify or polling.
type WatcherMode string

const (
	WatcherModeFsnotify WatcherMode = "fsnotify"
	WatcherModePolling  WatcherMode = "polling"
)

// LibraryWatcher watches a single library for filesystem changes.
// It uses fsnotify for local drives and polling for network drives.
type LibraryWatcher struct {
	library   *library.Library
	config    *library.MonitoringConfig
	mode      WatcherMode
	debouncer *Debouncer
	logger    *slog.Logger

	// fsnotify watcher (nil if using polling)
	fsWatcher *fsnotify.Watcher

	// Polling state
	pollTicker    *time.Ticker
	lastPollState map[string]time.Time // path -> modtime

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	// Stats
	stats WatcherStats
}

// WatcherStats contains statistics about the watcher.
type WatcherStats struct {
	Mode           WatcherMode
	StartedAt      time.Time
	EventsReceived int64
	EventsEmitted  int64
	LastEventAt    time.Time
	WatchedDirs    int
	LastPollAt     time.Time
	LastPollFiles  int
	Errors         int64
	LastError      error
	LastErrorAt    time.Time
}

// NewLibraryWatcher creates a new watcher for a library.
func NewLibraryWatcher(
	lib *library.Library,
	debouncer *Debouncer,
	logger *slog.Logger,
) *LibraryWatcher {
	config := lib.GetEffectiveConfig()

	return &LibraryWatcher{
		library:       lib,
		config:        config,
		debouncer:     debouncer,
		logger:        logger.With("library_id", lib.ID, "library_path", lib.Path),
		lastPollState: make(map[string]time.Time),
	}
}

// Start begins watching the library.
func (w *LibraryWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.ctx != nil {
		return nil // Already started
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.stats.StartedAt = time.Now()

	// Try fsnotify first, fall back to polling if it fails
	// (e.g., on network drives where fsnotify doesn't work)
	if err := w.startFsnotify(); err != nil {
		w.logger.Warn("Failed to start fsnotify, falling back to polling",
			"error", err)
		w.mode = WatcherModePolling
		w.stats.Mode = WatcherModePolling
		w.startPolling()
	} else {
		w.mode = WatcherModeFsnotify
		w.stats.Mode = WatcherModeFsnotify
	}

	pollingInterval := time.Duration(w.config.PollingIntervalMinutes) * time.Minute
	w.logger.Info("Library watcher started",
		"mode", w.mode,
		"polling_interval_minutes", w.config.PollingIntervalMinutes,
		"polling_interval", pollingInterval)

	return nil
}

// Stop stops watching the library.
func (w *LibraryWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}

	if w.fsWatcher != nil {
		w.fsWatcher.Close()
		w.fsWatcher = nil
	}

	if w.pollTicker != nil {
		w.pollTicker.Stop()
		w.pollTicker = nil
	}

	w.wg.Wait()
	w.logger.Info("Library watcher stopped")
}

// Stats returns current watcher statistics.
func (w *LibraryWatcher) Stats() WatcherStats {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.stats
}

// Library returns the library being watched.
func (w *LibraryWatcher) Library() *library.Library {
	return w.library
}

// startFsnotify initializes fsnotify-based watching.
func (w *LibraryWatcher) startFsnotify() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Walk the library path and add all directories
	dirsAdded := 0
	err = filepath.Walk(w.library.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories we can't access
		}
		if info.IsDir() {
			// Skip hidden directories
			if filepath.Base(path) != "." && filepath.Base(path)[0] == '.' {
				return filepath.SkipDir
			}
			if err := watcher.Add(path); err != nil {
				w.logger.Debug("Failed to watch directory", "path", path, "error", err)
				return nil
			}
			dirsAdded++
		}
		return nil
	})
	if err != nil {
		watcher.Close()
		return err
	}

	w.fsWatcher = watcher
	w.stats.WatchedDirs = dirsAdded

	w.logger.Debug("Added directories to fsnotify", "count", dirsAdded)

	// Start event processing goroutine
	w.wg.Add(1)
	go w.processFsnotifyEvents()

	return nil
}

// processFsnotifyEvents handles events from fsnotify.
func (w *LibraryWatcher) processFsnotifyEvents() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			w.mu.Lock()
			w.stats.EventsReceived++
			w.stats.LastEventAt = time.Now()
			w.mu.Unlock()

			// Convert fsnotify event to our event type
			var op FileOp
			switch {
			case event.Op&fsnotify.Create != 0:
				op = OpCreate
				// If a new directory was created, add it to the watcher
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if err := w.fsWatcher.Add(event.Name); err == nil {
						w.mu.Lock()
						w.stats.WatchedDirs++
						w.mu.Unlock()
					}
				}
			case event.Op&fsnotify.Write != 0:
				op = OpWrite
			case event.Op&fsnotify.Remove != 0:
				op = OpRemove
			case event.Op&fsnotify.Rename != 0:
				op = OpRename
			default:
				continue
			}

			w.debouncer.Add(FileEvent{
				Path:      event.Name,
				Op:        op,
				Timestamp: time.Now(),
			})

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			w.mu.Lock()
			w.stats.Errors++
			w.stats.LastError = err
			w.stats.LastErrorAt = time.Now()
			w.mu.Unlock()
			w.logger.Warn("fsnotify error", "error", err)
		}
	}
}

// startPolling initializes polling-based watching.
func (w *LibraryWatcher) startPolling() {
	intervalMinutes := w.config.PollingIntervalMinutes
	if intervalMinutes < 1 {
		intervalMinutes = 1 // Minimum 1 minute
	}
	interval := time.Duration(intervalMinutes) * time.Minute

	w.pollTicker = time.NewTicker(interval)

	// Do initial poll
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		// Initial poll to establish baseline
		w.poll(true)

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-w.pollTicker.C:
				w.poll(false)
			}
		}
	}()
}

// poll scans the library for changes.
func (w *LibraryWatcher) poll(initial bool) {
	startTime := time.Now()
	currentState := make(map[string]time.Time)
	filesScanned := 0

	err := filepath.Walk(w.library.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories (we only care about files)
		if info.IsDir() {
			// Skip hidden directories
			if filepath.Base(path) != "." && filepath.Base(path)[0] == '.' {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files we should ignore
		if ShouldIgnoreFile(path) {
			return nil
		}

		// Only track files we care about
		changeType := ClassifyFile(path)
		if changeType == FileChangeUnknown {
			return nil
		}

		filesScanned++
		modTime := info.ModTime()
		currentState[path] = modTime

		if !initial {
			lastModTime, existed := w.lastPollState[path]
			if !existed {
				// New file
				w.debouncer.Add(FileEvent{
					Path:      path,
					Op:        OpCreate,
					Timestamp: modTime,
				})
				w.mu.Lock()
				w.stats.EventsReceived++
				w.stats.LastEventAt = time.Now()
				w.mu.Unlock()
			} else if modTime.After(lastModTime) {
				// Modified file
				w.debouncer.Add(FileEvent{
					Path:      path,
					Op:        OpWrite,
					Timestamp: modTime,
				})
				w.mu.Lock()
				w.stats.EventsReceived++
				w.stats.LastEventAt = time.Now()
				w.mu.Unlock()
			}
		}

		return nil
	})

	if err != nil {
		w.mu.Lock()
		w.stats.Errors++
		w.stats.LastError = err
		w.stats.LastErrorAt = time.Now()
		w.mu.Unlock()
		w.logger.Warn("Polling error", "error", err)
		return
	}

	// Check for deleted files
	if !initial {
		for path := range w.lastPollState {
			if _, exists := currentState[path]; !exists {
				w.debouncer.Add(FileEvent{
					Path:      path,
					Op:        OpRemove,
					Timestamp: time.Now(),
				})
				w.mu.Lock()
				w.stats.EventsReceived++
				w.stats.LastEventAt = time.Now()
				w.mu.Unlock()
			}
		}
	}

	w.mu.Lock()
	w.lastPollState = currentState
	w.stats.LastPollAt = startTime
	w.stats.LastPollFiles = filesScanned
	w.mu.Unlock()

	if !initial {
		w.logger.Debug("Poll completed",
			"files", filesScanned,
			"duration_ms", time.Since(startTime).Milliseconds())
	}
}

// UpdateConfig updates the watcher configuration.
// If polling interval changes for a polling-mode watcher, the ticker is updated.
func (w *LibraryWatcher) UpdateConfig(config *library.MonitoringConfig) {
	w.mu.Lock()
	defer w.mu.Unlock()

	oldConfig := w.config
	w.config = config

	// If we're in polling mode and interval changed, update the ticker
	if w.mode == WatcherModePolling && w.pollTicker != nil {
		if oldConfig.PollingIntervalMinutes != config.PollingIntervalMinutes {
			intervalMinutes := config.PollingIntervalMinutes
			if intervalMinutes < 1 {
				intervalMinutes = 1
			}
			w.pollTicker.Reset(time.Duration(intervalMinutes) * time.Minute)
			w.logger.Info("Updated polling interval",
				"old_minutes", oldConfig.PollingIntervalMinutes,
				"new_minutes", config.PollingIntervalMinutes)
		}
	}
}

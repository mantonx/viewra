package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	domainevents "github.com/mantonx/viewra/internal/domain/events"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
)

// EventBus combines Publisher and Subscriber for the file monitor.
type EventBus interface {
	domainevents.Publisher
	domainevents.Subscriber
}

// Service manages filesystem monitoring for all libraries.
// It coordinates watchers, debouncers, and event handlers.
type Service struct {
	libraryRepo library.Repository
	mediaRepo   media.Repository
	enqueuer    EnrichmentEnqueuer
	eventBus    EventBus
	logger      *slog.Logger

	// Per-library watchers
	watchers   map[int64]*LibraryWatcher
	debouncers map[int64]*Debouncer
	watchersMu sync.RWMutex

	// Shared handler
	handler *Handler

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Scan tracking for auto-pause
	activeScans   map[int64]struct{} // libraryID -> struct{}
	activeScansMu sync.Mutex

	// Status
	running bool
	paused  bool
	mu      sync.RWMutex
}

// ServiceConfig contains configuration for the monitor service.
type ServiceConfig struct {
	// DebouncerConfig for all library debouncers
	DebouncerConfig DebouncerConfig
}

// DefaultServiceConfig returns the default service configuration.
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		DebouncerConfig: DefaultDebouncerConfig(),
	}
}

// NewService creates a new file monitor service.
func NewService(
	libraryRepo library.Repository,
	mediaRepo media.Repository,
	enqueuer EnrichmentEnqueuer,
	eventBus EventBus,
	logger *slog.Logger,
) *Service {
	return &Service{
		libraryRepo: libraryRepo,
		mediaRepo:   mediaRepo,
		enqueuer:    enqueuer,
		eventBus:    eventBus,
		logger:      logger.With("component", "file_monitor"),
		watchers:    make(map[int64]*LibraryWatcher),
		debouncers:  make(map[int64]*Debouncer),
		activeScans: make(map[int64]struct{}),
		handler: NewHandler(
			libraryRepo,
			mediaRepo,
			enqueuer,
			eventBus,
			logger,
		),
	}
}

// Start begins monitoring all libraries that have monitoring enabled.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true

	// Load all monitored libraries
	libs, err := s.libraryRepo.ListMonitored(ctx)
	if err != nil {
		s.logger.Error("Failed to list monitored libraries", "error", err)
		return err
	}

	s.logger.Info("Starting file monitor service", "library_count", len(libs))

	// Start a watcher for each library
	for _, lib := range libs {
		if err := s.startLibraryWatcher(lib); err != nil {
			s.logger.Error("Failed to start watcher for library",
				"library_id", lib.ID,
				"library_path", lib.Path,
				"error", err)
			// Continue with other libraries
		}
	}

	// Subscribe to scan events for auto-pause during scans
	if s.eventBus != nil {
		s.wg.Add(1)
		go s.watchScanEvents()
	}

	return nil
}

// Stop stops all monitoring.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("Stopping file monitor service")

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Stop all watchers
	s.watchersMu.Lock()
	for id, watcher := range s.watchers {
		watcher.Stop()
		delete(s.watchers, id)
	}
	for id, debouncer := range s.debouncers {
		debouncer.Stop()
		delete(s.debouncers, id)
	}
	s.watchersMu.Unlock()

	s.wg.Wait()
	s.running = false
}

// Pause temporarily pauses all monitoring (e.g., during a scan).
func (s *Service) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.paused {
		return
	}

	s.logger.Info("Pausing file monitor service")
	s.paused = true

	// Stop all watchers but keep debouncers to flush pending events
	s.watchersMu.Lock()
	for _, watcher := range s.watchers {
		watcher.Stop()
	}
	s.watchersMu.Unlock()
}

// Resume resumes monitoring after a pause.
func (s *Service) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.paused {
		return
	}

	s.logger.Info("Resuming file monitor service")
	s.paused = false

	// Flush any pending events before resuming
	s.watchersMu.RLock()
	for _, debouncer := range s.debouncers {
		debouncer.Flush()
	}
	s.watchersMu.RUnlock()

	// Restart watchers
	s.watchersMu.Lock()
	for id, watcher := range s.watchers {
		if err := watcher.Start(s.ctx); err != nil {
			s.logger.Error("Failed to restart watcher",
				"library_id", id,
				"error", err)
		}
	}
	s.watchersMu.Unlock()
}

// AddLibrary starts monitoring a new library.
func (s *Service) AddLibrary(lib *library.Library) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running || s.paused {
		return nil
	}

	if !lib.MonitoringEnabled {
		return nil
	}

	return s.startLibraryWatcher(lib)
}

// RemoveLibrary stops monitoring a library.
func (s *Service) RemoveLibrary(libraryID int64) {
	s.watchersMu.Lock()
	defer s.watchersMu.Unlock()

	if watcher, ok := s.watchers[libraryID]; ok {
		watcher.Stop()
		delete(s.watchers, libraryID)
	}

	if debouncer, ok := s.debouncers[libraryID]; ok {
		debouncer.Stop()
		delete(s.debouncers, libraryID)
	}
}

// UpdateLibrary updates monitoring configuration for a library.
func (s *Service) UpdateLibrary(lib *library.Library) error {
	s.watchersMu.Lock()
	defer s.watchersMu.Unlock()

	// Check if monitoring state changed
	watcher, exists := s.watchers[lib.ID]

	if lib.MonitoringEnabled && !exists {
		// Monitoring was enabled - start watcher
		s.watchersMu.Unlock()
		err := s.AddLibrary(lib)
		s.watchersMu.Lock()
		return err
	}

	if !lib.MonitoringEnabled && exists {
		// Monitoring was disabled - stop watcher
		watcher.Stop()
		delete(s.watchers, lib.ID)
		if debouncer, ok := s.debouncers[lib.ID]; ok {
			debouncer.Stop()
			delete(s.debouncers, lib.ID)
		}
		return nil
	}

	if exists {
		// Update configuration
		watcher.UpdateConfig(lib.GetEffectiveConfig())
	}

	return nil
}

// GetStatus returns the current status of the monitor service.
func (s *Service) GetStatus() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.watchersMu.RLock()
	defer s.watchersMu.RUnlock()

	status := ServiceStatus{
		Running:   s.running,
		Paused:    s.paused,
		Libraries: make(map[int64]LibraryStatus),
	}

	for id, watcher := range s.watchers {
		stats := watcher.Stats()
		status.Libraries[id] = LibraryStatus{
			LibraryID:      id,
			Mode:           stats.Mode,
			StartedAt:      stats.StartedAt,
			EventsReceived: stats.EventsReceived,
			LastEventAt:    stats.LastEventAt,
			WatchedDirs:    stats.WatchedDirs,
			Errors:         stats.Errors,
			LastError:      stats.LastError,
		}
	}

	return status
}

// ServiceStatus contains the current status of the monitor service.
type ServiceStatus struct {
	Running   bool
	Paused    bool
	Libraries map[int64]LibraryStatus
}

// LibraryStatus contains the monitoring status for a single library.
type LibraryStatus struct {
	LibraryID      int64
	Mode           WatcherMode
	StartedAt      time.Time
	EventsReceived int64
	LastEventAt    time.Time
	WatchedDirs    int
	Errors         int64
	LastError      error
}

// startLibraryWatcher creates and starts a watcher for a library.
func (s *Service) startLibraryWatcher(lib *library.Library) error {
	s.watchersMu.Lock()
	defer s.watchersMu.Unlock()

	// Check if already watching
	if _, exists := s.watchers[lib.ID]; exists {
		return nil
	}

	// Create debouncer for this library
	config := lib.GetEffectiveConfig()
	debouncerCfg := DebouncerConfig{
		DebounceDelay: time.Duration(config.DebounceSeconds) * time.Second,
		MaxDelay:      30 * time.Second,
	}
	debouncer := NewDebouncer(debouncerCfg)
	s.debouncers[lib.ID] = debouncer

	// Create and start watcher
	watcher := NewLibraryWatcher(lib, debouncer, s.logger)
	s.watchers[lib.ID] = watcher

	if err := watcher.Start(s.ctx); err != nil {
		delete(s.watchers, lib.ID)
		delete(s.debouncers, lib.ID)
		debouncer.Stop()
		return err
	}

	// Start event processing goroutine for this library
	s.wg.Add(1)
	go s.processLibraryEvents(lib, debouncer)

	s.logger.Info("Started monitoring library",
		"library_id", lib.ID,
		"library_path", lib.Path,
		"mode", watcher.mode)

	return nil
}

// processLibraryEvents processes debounced events for a library.
func (s *Service) processLibraryEvents(lib *library.Library, debouncer *Debouncer) {
	defer s.wg.Done()

	for event := range debouncer.Events() {
		// Check if we're paused or this library is being scanned
		s.mu.RLock()
		paused := s.paused
		s.mu.RUnlock()

		s.activeScansMu.Lock()
		_, scanning := s.activeScans[lib.ID]
		s.activeScansMu.Unlock()

		if paused || scanning {
			continue
		}

		// Handle the event
		if err := s.handler.HandleEvent(s.ctx, event, lib); err != nil {
			s.logger.Error("Failed to handle file event",
				"library_id", lib.ID,
				"path", event.Path,
				"error", err)
		}
	}
}

// watchScanEvents subscribes to scan events and pauses monitoring during scans.
func (s *Service) watchScanEvents() {
	defer s.wg.Done()

	sub := s.eventBus.Subscribe(
		domainevents.WithEventTypes(
			domainevents.EventScanStarted,
			domainevents.EventScanCompleted,
			domainevents.EventScanFailed,
		),
	)
	defer sub.Close()

	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-sub.Events():
			if !ok {
				return
			}

			// Extract library_id from event data
			libraryIDRaw, ok := event.Data["library_id"]
			if !ok {
				continue
			}
			libraryID, ok := libraryIDRaw.(int64)
			if !ok {
				// Try float64 (JSON unmarshaling may produce float64)
				if f, ok := libraryIDRaw.(float64); ok {
					libraryID = int64(f)
				} else {
					continue
				}
			}

			switch event.Type {
			case domainevents.EventScanStarted:
				s.activeScansMu.Lock()
				s.activeScans[libraryID] = struct{}{}
				s.activeScansMu.Unlock()
				s.logger.Debug("Pausing monitoring for library during scan",
					"library_id", libraryID)

			case domainevents.EventScanCompleted, domainevents.EventScanFailed:
				s.activeScansMu.Lock()
				delete(s.activeScans, libraryID)
				s.activeScansMu.Unlock()
				s.logger.Debug("Resuming monitoring for library after scan",
					"library_id", libraryID)
			}
		}
	}
}

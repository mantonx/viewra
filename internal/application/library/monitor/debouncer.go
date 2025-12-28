package monitor

import (
	"sync"
	"time"
)

// FileEvent represents a filesystem change event.
type FileEvent struct {
	Path      string
	Op        FileOp
	Timestamp time.Time
}

// FileOp represents the type of filesystem operation.
type FileOp int

const (
	OpCreate FileOp = iota
	OpWrite
	OpRemove
	OpRename
)

func (op FileOp) String() string {
	switch op {
	case OpCreate:
		return "create"
	case OpWrite:
		return "write"
	case OpRemove:
		return "remove"
	case OpRename:
		return "rename"
	default:
		return "unknown"
	}
}

// DebouncedEvent represents a batched event after debouncing.
// Multiple rapid changes to the same file result in a single DebouncedEvent.
type DebouncedEvent struct {
	Path       string
	ChangeType FileChangeType
	LastOp     FileOp
	FirstSeen  time.Time
	LastSeen   time.Time
	EventCount int
}

// Debouncer batches rapid filesystem events into single events per file.
// This prevents triggering multiple enrichments when a file is being copied
// (which generates many write events) or when multiple related files change together.
type Debouncer struct {
	mu            sync.Mutex
	pending       map[string]*pendingEvent
	debounceDelay time.Duration
	maxDelay      time.Duration
	output        chan DebouncedEvent
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

type pendingEvent struct {
	path       string
	changeType FileChangeType
	lastOp     FileOp
	firstSeen  time.Time
	lastSeen   time.Time
	eventCount int
	timer      *time.Timer
}

// DebouncerConfig contains configuration for the debouncer.
type DebouncerConfig struct {
	// DebounceDelay is how long to wait after the last event before emitting.
	// Default: 2 seconds
	DebounceDelay time.Duration

	// MaxDelay is the maximum time to wait before emitting, even if events
	// are still coming. This prevents indefinite delays during long copies.
	// Default: 30 seconds
	MaxDelay time.Duration

	// OutputBufferSize is the size of the output channel buffer.
	// Default: 100
	OutputBufferSize int
}

// DefaultDebouncerConfig returns the default debouncer configuration.
func DefaultDebouncerConfig() DebouncerConfig {
	return DebouncerConfig{
		DebounceDelay:    2 * time.Second,
		MaxDelay:         30 * time.Second,
		OutputBufferSize: 100,
	}
}

// NewDebouncer creates a new debouncer with the given configuration.
func NewDebouncer(cfg DebouncerConfig) *Debouncer {
	if cfg.DebounceDelay == 0 {
		cfg.DebounceDelay = 2 * time.Second
	}
	if cfg.MaxDelay == 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.OutputBufferSize == 0 {
		cfg.OutputBufferSize = 100
	}

	return &Debouncer{
		pending:       make(map[string]*pendingEvent),
		debounceDelay: cfg.DebounceDelay,
		maxDelay:      cfg.MaxDelay,
		output:        make(chan DebouncedEvent, cfg.OutputBufferSize),
		stopCh:        make(chan struct{}),
	}
}

// Events returns the channel that emits debounced events.
func (d *Debouncer) Events() <-chan DebouncedEvent {
	return d.output
}

// Add adds a new event to be debounced.
func (d *Debouncer) Add(event FileEvent) {
	// Ignore files that should be filtered
	if ShouldIgnoreFile(event.Path) {
		return
	}

	// Classify the file
	changeType := ClassifyFile(event.Path)
	if changeType == FileChangeUnknown {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if we're stopped
	select {
	case <-d.stopCh:
		return
	default:
	}

	existing, ok := d.pending[event.Path]
	if ok {
		// Update existing pending event
		existing.lastSeen = event.Timestamp
		existing.lastOp = event.Op
		existing.eventCount++

		// Check if we've exceeded max delay
		if event.Timestamp.Sub(existing.firstSeen) >= d.maxDelay {
			// Force emit now
			existing.timer.Stop()
			d.emitLocked(event.Path)
			return
		}

		// Reset the debounce timer
		existing.timer.Reset(d.debounceDelay)
	} else {
		// Create new pending event
		pe := &pendingEvent{
			path:       event.Path,
			changeType: changeType,
			lastOp:     event.Op,
			firstSeen:  event.Timestamp,
			lastSeen:   event.Timestamp,
			eventCount: 1,
		}

		// Create timer that will emit after debounce delay
		pe.timer = time.AfterFunc(d.debounceDelay, func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.emitLocked(event.Path)
		})

		d.pending[event.Path] = pe
	}
}

// emitLocked emits a pending event and removes it from the map.
// Must be called with d.mu held.
func (d *Debouncer) emitLocked(path string) {
	pe, ok := d.pending[path]
	if !ok {
		return
	}

	delete(d.pending, path)

	// Don't block if output is full or we're stopped
	select {
	case <-d.stopCh:
		return
	case d.output <- DebouncedEvent{
		Path:       pe.path,
		ChangeType: pe.changeType,
		LastOp:     pe.lastOp,
		FirstSeen:  pe.firstSeen,
		LastSeen:   pe.lastSeen,
		EventCount: pe.eventCount,
	}:
	default:
		// Output channel is full, event is dropped
		// This shouldn't happen with proper buffer sizing
	}
}

// Flush emits all pending events immediately.
func (d *Debouncer) Flush() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for path, pe := range d.pending {
		pe.timer.Stop()
		delete(d.pending, path)

		select {
		case <-d.stopCh:
			return
		case d.output <- DebouncedEvent{
			Path:       pe.path,
			ChangeType: pe.changeType,
			LastOp:     pe.lastOp,
			FirstSeen:  pe.firstSeen,
			LastSeen:   pe.lastSeen,
			EventCount: pe.eventCount,
		}:
		default:
		}
	}
}

// Stop stops the debouncer and closes the output channel.
// Pending events are discarded.
func (d *Debouncer) Stop() {
	d.mu.Lock()

	// Signal stop
	select {
	case <-d.stopCh:
		// Already stopped
		d.mu.Unlock()
		return
	default:
		close(d.stopCh)
	}

	// Cancel all pending timers
	for _, pe := range d.pending {
		pe.timer.Stop()
	}
	d.pending = make(map[string]*pendingEvent)

	d.mu.Unlock()

	// Close output channel after clearing pending
	close(d.output)
}

// PendingCount returns the number of events waiting to be emitted.
func (d *Debouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}

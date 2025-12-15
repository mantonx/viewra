package scanutil

import "sync"

// AtomicDeduplicator provides thread-safe deduplication for processing items exactly once.
// It uses sync.Map internally for lock-free concurrent access during parallel scans.
//
// Usage:
//
//	if dedup.TryMark(key) {
//	    // First time seeing this key - process it
//	}
type AtomicDeduplicator struct {
	seen sync.Map
}

// TryMark atomically checks if a key has been seen and marks it if not.
// Returns true if this is the first time the key is being marked (caller should process).
// Returns false if the key was already marked (caller should skip).
func (d *AtomicDeduplicator) TryMark(key string) bool {
	_, alreadyMarked := d.seen.LoadOrStore(key, struct{}{})
	return !alreadyMarked
}

// Reset clears all marked keys. Call this between scan sessions.
func (d *AtomicDeduplicator) Reset() {
	d.seen = sync.Map{}
}

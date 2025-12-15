package scanutil

import (
	"sync"
	"testing"
)

func TestAtomicDeduplicator_TryMark(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		want     bool
		setupFn  func(d *AtomicDeduplicator)
	}{
		{
			name: "first mark returns true",
			key:  "test-key",
			want: true,
		},
		{
			name: "second mark returns false",
			key:  "test-key",
			want: false,
			setupFn: func(d *AtomicDeduplicator) {
				d.TryMark("test-key")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &AtomicDeduplicator{}
			if tt.setupFn != nil {
				tt.setupFn(d)
			}
			if got := d.TryMark(tt.key); got != tt.want {
				t.Errorf("TryMark() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtomicDeduplicator_TryMark_DifferentKeys(t *testing.T) {
	d := &AtomicDeduplicator{}

	// Different keys should all return true
	if !d.TryMark("key1") {
		t.Error("first key1 should return true")
	}
	if !d.TryMark("key2") {
		t.Error("first key2 should return true")
	}
	if !d.TryMark("key3") {
		t.Error("first key3 should return true")
	}

	// Repeating should return false
	if d.TryMark("key1") {
		t.Error("second key1 should return false")
	}
}

func TestAtomicDeduplicator_Reset(t *testing.T) {
	d := &AtomicDeduplicator{}

	// Mark some keys
	d.TryMark("key1")
	d.TryMark("key2")

	// Reset
	d.Reset()

	// Should be able to mark them again
	if !d.TryMark("key1") {
		t.Error("key1 after reset should return true")
	}
	if !d.TryMark("key2") {
		t.Error("key2 after reset should return true")
	}
}

func TestAtomicDeduplicator_Concurrent(t *testing.T) {
	d := &AtomicDeduplicator{}
	const key = "concurrent-key"
	const numGoroutines = 100

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if d.TryMark(key) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful mark, got %d", successCount)
	}
}

func BenchmarkAtomicDeduplicator_TryMark(b *testing.B) {
	d := &AtomicDeduplicator{}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			d.TryMark(string(rune(i % 1000)))
			i++
		}
	})
}

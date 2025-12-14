package bus

import (
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

func TestSubscription_Events(t *testing.T) {
	ch := make(chan events.Event, 10)
	sub := &subscription{
		id:   1,
		ch:   ch,
		done: make(chan struct{}),
	}

	// Send an event
	ch <- events.Event{Type: events.EventScanStarted, Source: "test"}

	// Should receive via Events()
	select {
	case e := <-sub.Events():
		if e.Type != events.EventScanStarted {
			t.Errorf("expected EventScanStarted, got %s", e.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestSubscription_Close(t *testing.T) {
	ch := make(chan events.Event, 10)
	sub := &subscription{
		id:   1,
		ch:   ch,
		done: make(chan struct{}),
	}

	sub.Close()

	// Channel should be closed
	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Error("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel not closed")
	}

	// Done channel should be closed
	select {
	case <-sub.done:
		// Good
	default:
		t.Error("done channel not closed")
	}
}

func TestSubscription_Close_Idempotent(t *testing.T) {
	ch := make(chan events.Event, 10)
	sub := &subscription{
		id:   1,
		ch:   ch,
		done: make(chan struct{}),
	}

	sub.Close()
	sub.Close() // Should not panic
	sub.Close() // Should not panic
}

func TestSubscription_isClosed(t *testing.T) {
	ch := make(chan events.Event, 10)
	sub := &subscription{
		id:   1,
		ch:   ch,
		done: make(chan struct{}),
	}

	if sub.isClosed() {
		t.Error("expected subscription to be open")
	}

	sub.Close()

	if !sub.isClosed() {
		t.Error("expected subscription to be closed")
	}
}

func TestSubscription_Filter(t *testing.T) {
	filter := func(e events.Event) bool {
		return e.Type == events.EventScanStarted
	}

	sub := &subscription{
		id:     1,
		ch:     make(chan events.Event, 10),
		filter: filter,
		done:   make(chan struct{}),
	}

	// Filter should work
	if !sub.filter(events.Event{Type: events.EventScanStarted}) {
		t.Error("filter should accept EventScanStarted")
	}
	if sub.filter(events.Event{Type: events.EventMediaDiscovered}) {
		t.Error("filter should reject EventMediaDiscovered")
	}
}

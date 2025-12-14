package bus

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/events"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNew(t *testing.T) {
	b := New(100, testLogger())

	if b == nil {
		t.Fatal("expected non-nil bus")
	}

	stats := b.Stats()
	if stats.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers, got %d", stats.SubscriberCount)
	}
	if stats.BufferCapacity != 100 {
		t.Errorf("expected buffer capacity 100, got %d", stats.BufferCapacity)
	}
	if stats.BufferCount != 0 {
		t.Errorf("expected buffer count 0, got %d", stats.BufferCount)
	}
	if stats.TotalPublished != 0 {
		t.Errorf("expected 0 total published, got %d", stats.TotalPublished)
	}
}

func TestBus_Publish(t *testing.T) {
	b := New(100, testLogger())

	e := events.Event{
		Type:   events.EventMediaDiscovered,
		Source: "test",
	}
	b.Publish(e)

	stats := b.Stats()
	if stats.TotalPublished != 1 {
		t.Errorf("expected 1 total published, got %d", stats.TotalPublished)
	}
	if stats.BufferCount != 1 {
		t.Errorf("expected buffer count 1, got %d", stats.BufferCount)
	}
}

func TestBus_Publish_SetsTimestamp(t *testing.T) {
	b := New(100, testLogger())

	before := time.Now()
	e := events.Event{
		Type:   events.EventScanStarted,
		Source: "test",
		// Timestamp not set
	}
	b.Publish(e)
	after := time.Now()

	// Check the event in the buffer has a timestamp
	recent := b.Recent(1)
	if len(recent) != 1 {
		t.Fatal("expected 1 event in buffer")
	}
	if recent[0].Timestamp.Before(before) || recent[0].Timestamp.After(after) {
		t.Error("timestamp not in expected range")
	}
}

func TestBus_Publish_PreservesTimestamp(t *testing.T) {
	b := New(100, testLogger())

	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	e := events.Event{
		Type:      events.EventScanStarted,
		Source:    "test",
		Timestamp: fixedTime,
	}
	b.Publish(e)

	recent := b.Recent(1)
	if len(recent) != 1 {
		t.Fatal("expected 1 event in buffer")
	}
	if !recent[0].Timestamp.Equal(fixedTime) {
		t.Errorf("expected timestamp %v, got %v", fixedTime, recent[0].Timestamp)
	}
}

func TestBus_Subscribe_ReceivesEvents(t *testing.T) {
	b := New(100, testLogger())

	sub := b.Subscribe()
	defer b.Unsubscribe(sub)

	// Publish an event
	e := events.Event{
		Type:   events.EventMediaDiscovered,
		Source: "test",
		Data:   map[string]any{"media_id": int64(42)},
	}
	b.Publish(e)

	// Should receive the event
	select {
	case received := <-sub.Events():
		if received.Type != events.EventMediaDiscovered {
			t.Errorf("expected type %s, got %s", events.EventMediaDiscovered, received.Type)
		}
		if received.Data["media_id"] != int64(42) {
			t.Errorf("expected media_id=42, got %v", received.Data["media_id"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for event")
	}
}

func TestBus_Subscribe_WithBufferSize(t *testing.T) {
	b := New(100, testLogger())

	sub := b.Subscribe(events.WithBufferSize(10))
	defer b.Unsubscribe(sub)

	// Publish 15 events without reading
	for i := 0; i < 15; i++ {
		b.Publish(events.Event{
			Type:   events.EventScanProgress,
			Source: "test",
		})
	}

	// Should have received 10 (buffer size), rest dropped
	count := 0
	for {
		select {
		case <-sub.Events():
			count++
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	if count != 10 {
		t.Errorf("expected 10 events (buffer size), got %d", count)
	}
}

func TestBus_Subscribe_WithFilter(t *testing.T) {
	b := New(100, testLogger())

	// Only receive scan events
	sub := b.Subscribe(events.WithFilter(func(e events.Event) bool {
		return string(e.Type)[:5] == "scan."
	}))
	defer b.Unsubscribe(sub)

	// Publish various events
	b.Publish(events.Event{Type: events.EventMediaDiscovered, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})
	b.Publish(events.Event{Type: events.EventEnrichmentQueued, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanCompleted, Source: "test"})

	// Should only receive scan events
	received := []events.EventType{}
	for {
		select {
		case e := <-sub.Events():
			received = append(received, e.Type)
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	if len(received) != 2 {
		t.Errorf("expected 2 scan events, got %d", len(received))
	}
	for _, et := range received {
		if string(et)[:5] != "scan." {
			t.Errorf("expected scan event, got %s", et)
		}
	}
}

func TestBus_Subscribe_WithEventTypes(t *testing.T) {
	b := New(100, testLogger())

	sub := b.Subscribe(events.WithEventTypes(
		events.EventScanStarted,
		events.EventScanCompleted,
	))
	defer b.Unsubscribe(sub)

	// Publish various events
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanProgress, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanCompleted, Source: "test"})
	b.Publish(events.Event{Type: events.EventMediaDiscovered, Source: "test"})

	received := []events.EventType{}
	for {
		select {
		case e := <-sub.Events():
			received = append(received, e.Type)
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0] != events.EventScanStarted {
		t.Errorf("expected EventScanStarted, got %s", received[0])
	}
	if received[1] != events.EventScanCompleted {
		t.Errorf("expected EventScanCompleted, got %s", received[1])
	}
}

func TestBus_Subscribe_WithEventPrefix(t *testing.T) {
	b := New(100, testLogger())

	sub := b.Subscribe(events.WithEventPrefix("enrichment."))
	defer b.Unsubscribe(sub)

	// Publish various events
	b.Publish(events.Event{Type: events.EventEnrichmentQueued, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})
	b.Publish(events.Event{Type: events.EventEnrichmentComplete, Source: "test"})
	b.Publish(events.Event{Type: events.EventMediaDiscovered, Source: "test"})

	received := []events.EventType{}
	for {
		select {
		case e := <-sub.Events():
			received = append(received, e.Type)
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	if len(received) != 2 {
		t.Fatalf("expected 2 enrichment events, got %d", len(received))
	}
}

func TestBus_Subscribe_WithReplayLast(t *testing.T) {
	b := New(100, testLogger())

	// Publish some events before subscribing
	for i := 0; i < 5; i++ {
		b.Publish(events.Event{
			Type:   events.EventScanProgress,
			Source: "test",
			Data:   map[string]any{"index": i},
		})
	}

	// Subscribe with replay of last 3
	sub := b.Subscribe(events.WithReplayLast(3))
	defer b.Unsubscribe(sub)

	// Should receive the last 3 events immediately
	received := []int{}
	for i := 0; i < 3; i++ {
		select {
		case e := <-sub.Events():
			received = append(received, e.Data["index"].(int))
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("timed out waiting for replayed event %d", i)
		}
	}

	// Should have received indices 2, 3, 4 (last 3)
	expected := []int{2, 3, 4}
	for i, v := range received {
		if v != expected[i] {
			t.Errorf("position %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestBus_Subscribe_WithReplayLast_FilteredReplay(t *testing.T) {
	b := New(100, testLogger())

	// Publish various events
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})
	b.Publish(events.Event{Type: events.EventMediaDiscovered, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanProgress, Source: "test"})
	b.Publish(events.Event{Type: events.EventEnrichmentQueued, Source: "test"})
	b.Publish(events.Event{Type: events.EventScanCompleted, Source: "test"})

	// Subscribe with filter and replay
	sub := b.Subscribe(
		events.WithEventPrefix("scan."),
		events.WithReplayLast(10),
	)
	defer b.Unsubscribe(sub)

	// Should only replay scan events
	received := []events.EventType{}
	for {
		select {
		case e := <-sub.Events():
			received = append(received, e.Type)
		case <-time.After(50 * time.Millisecond):
			goto done
		}
	}
done:
	if len(received) != 3 {
		t.Fatalf("expected 3 scan events from replay, got %d", len(received))
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	b := New(100, testLogger())

	sub := b.Subscribe()

	// Verify subscriber count
	stats := b.Stats()
	if stats.SubscriberCount != 1 {
		t.Errorf("expected 1 subscriber, got %d", stats.SubscriberCount)
	}

	b.Unsubscribe(sub)

	// Verify subscriber removed
	stats = b.Stats()
	if stats.SubscriberCount != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", stats.SubscriberCount)
	}

	// Channel should be closed
	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Error("expected channel to be closed")
		}
	case <-time.After(50 * time.Millisecond):
		t.Error("channel not closed after unsubscribe")
	}
}

// fakeSubscription is a fake that doesn't match our internal subscription type
type fakeSubscription struct{}

func (f *fakeSubscription) Events() <-chan events.Event { return nil }
func (f *fakeSubscription) Close()                      {}

func TestBus_Unsubscribe_InvalidSubscription(t *testing.T) {
	b := New(100, testLogger())

	// Should not panic with nil
	b.Unsubscribe(nil)

	// Should not panic with wrong type (implements interface but not our internal type)
	b.Unsubscribe(&fakeSubscription{})
}

func TestBus_Close(t *testing.T) {
	b := New(100, testLogger())

	sub1 := b.Subscribe()
	sub2 := b.Subscribe()

	stats := b.Stats()
	if stats.SubscriberCount != 2 {
		t.Errorf("expected 2 subscribers, got %d", stats.SubscriberCount)
	}

	b.Close()

	// Both subscriptions should be closed
	for i, sub := range []events.Subscription{sub1, sub2} {
		select {
		case _, ok := <-sub.Events():
			if ok {
				t.Errorf("subscription %d: expected channel to be closed", i)
			}
		case <-time.After(50 * time.Millisecond):
			t.Errorf("subscription %d: channel not closed after bus close", i)
		}
	}

	// Publishing after close should not panic
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})
}

func TestBus_Close_Idempotent(t *testing.T) {
	b := New(100, testLogger())

	b.Close()
	b.Close() // Should not panic
}

func TestBus_Recent(t *testing.T) {
	b := New(100, testLogger())

	for i := 0; i < 5; i++ {
		b.Publish(events.Event{
			Type:   events.EventScanProgress,
			Source: "test",
			Data:   map[string]any{"index": i},
		})
	}

	recent := b.Recent(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(recent))
	}

	// Should be in chronological order (oldest to newest)
	for i, e := range recent {
		expected := i + 2 // indices 2, 3, 4
		if e.Data["index"] != expected {
			t.Errorf("position %d: expected index %d, got %v", i, expected, e.Data["index"])
		}
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	b := New(100, testLogger())

	sub1 := b.Subscribe()
	sub2 := b.Subscribe()
	sub3 := b.Subscribe(events.WithEventPrefix("scan."))
	defer func() {
		b.Unsubscribe(sub1)
		b.Unsubscribe(sub2)
		b.Unsubscribe(sub3)
	}()

	// Publish an event
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})

	// All subscribers should receive it
	for i, sub := range []events.Subscription{sub1, sub2, sub3} {
		select {
		case <-sub.Events():
			// Good
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d timed out", i)
		}
	}

	// Publish non-scan event
	b.Publish(events.Event{Type: events.EventMediaDiscovered, Source: "test"})

	// sub1 and sub2 should receive, sub3 should not
	for _, sub := range []events.Subscription{sub1, sub2} {
		select {
		case <-sub.Events():
			// Good
		case <-time.After(100 * time.Millisecond):
			t.Error("subscriber timed out for media event")
		}
	}

	// sub3 should not receive
	select {
	case <-sub3.Events():
		t.Error("filtered subscriber should not receive media event")
	case <-time.After(50 * time.Millisecond):
		// Good - no event received
	}
}

func TestBus_Concurrency(t *testing.T) {
	b := New(1000, testLogger())
	var wg sync.WaitGroup

	// Start multiple subscribers
	subs := make([]events.Subscription, 5)
	for i := range subs {
		subs[i] = b.Subscribe(events.WithBufferSize(500))
	}

	// Concurrent publishers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				b.Publish(events.Event{
					Type:   events.EventScanProgress,
					Source: "test",
					Data:   map[string]any{"publisher": id, "seq": j},
				})
			}
		}(i)
	}

	// Concurrent readers
	for i, sub := range subs {
		wg.Add(1)
		go func(id int, s events.Subscription) {
			defer wg.Done()
			count := 0
			timeout := time.After(2 * time.Second)
			for {
				select {
				case _, ok := <-s.Events():
					if !ok {
						return
					}
					count++
				case <-timeout:
					return
				}
			}
		}(i, sub)
	}

	wg.Wait()

	// Cleanup
	for _, sub := range subs {
		b.Unsubscribe(sub)
	}

	stats := b.Stats()
	if stats.TotalPublished != 1000 {
		t.Errorf("expected 1000 total published, got %d", stats.TotalPublished)
	}
}

func TestBus_ClosedSubscriber_NotDelivered(t *testing.T) {
	b := New(100, testLogger())

	sub := b.Subscribe()
	b.Unsubscribe(sub) // Close the subscription

	// Publish after close
	b.Publish(events.Event{Type: events.EventScanStarted, Source: "test"})

	// Event should be in buffer but not delivered to closed subscriber
	stats := b.Stats()
	if stats.BufferCount != 1 {
		t.Errorf("expected 1 event in buffer, got %d", stats.BufferCount)
	}
}

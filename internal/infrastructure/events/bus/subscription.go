package bus

import (
	"sync"

	"github.com/mantonx/viewra/internal/domain/events"
)

// subscription implements the events.Subscription interface.
type subscription struct {
	id     uint64
	ch     chan events.Event
	filter func(events.Event) bool
	done   chan struct{}

	closeOnce sync.Once
}

// Events returns the channel of events for this subscription.
func (s *subscription) Events() <-chan events.Event {
	return s.ch
}

// Close terminates the subscription and closes the events channel.
// Safe to call multiple times.
func (s *subscription) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		close(s.ch)
	})
}

// isClosed returns true if the subscription has been closed.
func (s *subscription) isClosed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

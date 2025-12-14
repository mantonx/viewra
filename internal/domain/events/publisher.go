package events

// Publisher is the interface for publishing events.
// Components depend on this interface rather than a concrete implementation.
type Publisher interface {
	Publish(e Event)
}

// Subscriber provides access to event streams.
type Subscriber interface {
	// Subscribe creates a new subscription to the event stream.
	// Options can filter events and configure replay behavior.
	Subscribe(opts ...SubscribeOption) Subscription
}

// Subscription represents an active subscription to the event bus.
type Subscription interface {
	// Events returns the channel of events for this subscription.
	Events() <-chan Event

	// Close terminates the subscription and releases resources.
	Close()
}

// SubscribeOption configures subscription behavior.
type SubscribeOption func(*SubscribeConfig)

// SubscribeConfig holds subscription configuration.
type SubscribeConfig struct {
	BufferSize int
	Filter     func(Event) bool
	ReplayLast int
}

// WithBufferSize sets the subscription's event buffer size.
func WithBufferSize(size int) SubscribeOption {
	return func(c *SubscribeConfig) {
		c.BufferSize = size
	}
}

// WithFilter sets a filter function for events.
func WithFilter(fn func(Event) bool) SubscribeOption {
	return func(c *SubscribeConfig) {
		c.Filter = fn
	}
}

// WithReplayLast enables replay of recent events for late subscribers.
func WithReplayLast(n int) SubscribeOption {
	return func(c *SubscribeConfig) {
		c.ReplayLast = n
	}
}

// WithEventTypes is a convenience filter for specific event types.
func WithEventTypes(types ...EventType) SubscribeOption {
	typeSet := make(map[EventType]struct{}, len(types))
	for _, t := range types {
		typeSet[t] = struct{}{}
	}
	return WithFilter(func(e Event) bool {
		_, ok := typeSet[e.Type]
		return ok
	})
}

// WithEventPrefix is a convenience filter for event type prefixes.
// Example: WithEventPrefix("scan.") matches all scan events.
func WithEventPrefix(prefix string) SubscribeOption {
	return WithFilter(func(e Event) bool {
		return len(e.Type) >= len(prefix) && string(e.Type)[:len(prefix)] == prefix
	})
}

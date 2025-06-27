package modulemanager

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/services"
)

// ModuleEventHandler handles event-based communication between modules
type ModuleEventHandler struct {
	eventBus      events.EventBus
	subscriptions map[string][]*events.Subscription // moduleID -> []*Subscription
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewModuleEventHandler creates a new module event handler
func NewModuleEventHandler(eventBus events.EventBus) *ModuleEventHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModuleEventHandler{
		eventBus:      eventBus,
		subscriptions: make(map[string][]*events.Subscription),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// RegisterModuleEventHandlers registers common event handlers for module communication
func (h *ModuleEventHandler) RegisterModuleEventHandlers() error {
	// Handle service registration events
	_, err := h.eventBus.Subscribe(h.ctx, events.EventFilter{
		Types: []events.EventType{events.EventServiceRegistered},
	}, func(event events.Event) error {
		log.Printf("Service registered: %s by module %s",
			event.Data["service_name"],
			event.Data["module_name"])

		// Notify modules waiting for this service
		h.notifyServiceAvailable(event.Data["service_name"].(string))
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to service registration events: %w", err)
	}

	// Handle transcoding request events
	_, err = h.eventBus.Subscribe(h.ctx, events.EventFilter{
		Types: []events.EventType{events.EventTranscodeRequested},
	}, func(event events.Event) error {
		// The transcoding module will handle this event
		log.Printf("Transcoding requested for media file: %s", event.Data["media_file_id"])
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to transcode request events: %w", err)
	}

	// Handle segment ready events for manifest updates
	_, err = h.eventBus.Subscribe(h.ctx, events.EventFilter{
		Types: []events.EventType{events.EventSegmentReady},
	}, func(event events.Event) error {
		// Notify interested modules about new segments
		log.Printf("Segment %d ready for session %s",
			event.Data["segment_id"],
			event.Data["session_id"])
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to segment ready events: %w", err)
	}

	// Handle media scanned events for enrichment
	_, err = h.eventBus.Subscribe(h.ctx, events.EventFilter{
		Types: []events.EventType{events.EventMediaScanned},
	}, func(event events.Event) error {
		// The enrichment module will handle this event
		log.Printf("Media scanned: %s", event.Data["media_file_id"])
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to media scanned events: %w", err)
	}

	return nil
}

// SubscribeModule subscribes a module to specific event types
func (h *ModuleEventHandler) SubscribeModule(moduleID string, filter events.EventFilter, handler events.EventHandler) (*events.Subscription, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create subscription
	subscription, err := h.eventBus.Subscribe(h.ctx, filter, func(event events.Event) error {
		// Add module context to the handler
		log.Printf("Module %s handling event: %s", moduleID, event.Type)
		return handler(event)
	})
	if err != nil {
		return nil, err
	}

	// Track subscription
	h.subscriptions[moduleID] = append(h.subscriptions[moduleID], subscription)

	return subscription, nil
}

// UnsubscribeModule unsubscribes all event handlers for a module
func (h *ModuleEventHandler) UnsubscribeModule(moduleID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Unsubscribe all handlers for this module
	if subs, exists := h.subscriptions[moduleID]; exists {
		for _, sub := range subs {
			h.eventBus.Unsubscribe(sub.ID)
		}
		delete(h.subscriptions, moduleID)
	}
}

// PublishModuleEvent publishes an event from a module
func (h *ModuleEventHandler) PublishModuleEvent(moduleID string, event events.Event) {
	// Add module source if not set
	if event.Source == "" {
		event.Source = fmt.Sprintf("module:%s", moduleID)
	}

	// Publish event
	h.eventBus.PublishAsync(event)
}

// notifyServiceAvailable notifies modules waiting for a service
func (h *ModuleEventHandler) notifyServiceAvailable(serviceName string) {
	// Publish service available event
	event := events.Event{
		Type:     events.EventServiceAvailable,
		Source:   "system",
		Title:    "Service Available",
		Message:  fmt.Sprintf("Service '%s' is now available", serviceName),
		Priority: events.PriorityNormal,
		Data: map[string]interface{}{
			"service_name": serviceName,
		},
	}

	h.eventBus.PublishAsync(event)
}

// Module communication helpers

// RequestTranscode publishes a transcode request event
func (h *ModuleEventHandler) RequestTranscode(moduleID, mediaFileID, container string, options map[string]interface{}) {
	event := events.NewTranscodeEvent(
		events.EventTranscodeRequested,
		"", // Session ID will be assigned by transcoding module
		mediaFileID,
		"requested",
	)

	// Add additional options
	event.Data["container"] = container
	event.Data["requester"] = moduleID
	for k, v := range options {
		event.Data[k] = v
	}

	h.PublishModuleEvent(moduleID, event)
}

// NotifyMediaScanned publishes a media scanned event
func (h *ModuleEventHandler) NotifyMediaScanned(moduleID, mediaFileID string, metadata map[string]interface{}) {
	event := events.NewMediaProcessingEvent(
		events.EventMediaScanned,
		mediaFileID,
		moduleID,
		"scanned",
	)

	// Add metadata
	event.Data["metadata"] = metadata

	h.PublishModuleEvent(moduleID, event)
}

// NotifyEnrichmentReady publishes an enrichment ready event
func (h *ModuleEventHandler) NotifyEnrichmentReady(moduleID, mediaFileID string, enrichments map[string]interface{}) {
	event := events.NewMediaProcessingEvent(
		events.EventMediaEnrichmentReady,
		mediaFileID,
		moduleID,
		"enriched",
	)

	// Add enrichment data
	event.Data["enrichments"] = enrichments

	h.PublishModuleEvent(moduleID, event)
}

// WaitForService subscribes to service available events and calls handler when service is ready
func (h *ModuleEventHandler) WaitForService(moduleID, serviceName string, handler func()) (*events.Subscription, error) {
	// Check if service is already available
	if _, err := services.Get(serviceName); err == nil {
		// Service already available, call handler immediately
		handler()
		return nil, nil
	}

	// Subscribe to service available events
	return h.SubscribeModule(moduleID, events.EventFilter{
		Types: []events.EventType{events.EventServiceAvailable},
	}, func(event events.Event) error {
		if event.Data["service_name"] == serviceName {
			handler()
		}
		return nil
	})
}

// Global module event handler instance
var globalEventHandler *ModuleEventHandler
var eventHandlerOnce sync.Once

// GetModuleEventHandler returns the global module event handler
func GetModuleEventHandler() *ModuleEventHandler {
	eventHandlerOnce.Do(func() {
		eventBus := events.GetGlobalEventBus()
		globalEventHandler = NewModuleEventHandler(eventBus)
		if err := globalEventHandler.RegisterModuleEventHandlers(); err != nil {
			log.Printf("Failed to register module event handlers: %v", err)
		}
	})
	return globalEventHandler
}

// Stop stops the module event handler
func (h *ModuleEventHandler) Stop() {
	h.cancel()
}

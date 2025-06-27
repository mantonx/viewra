package events

import (
	"fmt"
	"time"
)

// Module-specific event types
const (
	// Service registration events
	EventServiceRegistered   EventType = "service.registered"
	EventServiceUnregistered EventType = "service.unregistered"
	EventServiceRequested    EventType = "service.requested"
	EventServiceAvailable    EventType = "service.available"

	// Module lifecycle events
	EventModuleInitialized EventType = "module.initialized"
	EventModuleStarted     EventType = "module.started"
	EventModuleStopped     EventType = "module.stopped"
	EventModuleError       EventType = "module.error"

	// Cross-module communication events
	EventTranscodeRequested EventType = "transcode.requested"
	EventTranscodeCompleted EventType = "transcode.completed"
	EventTranscodeFailed    EventType = "transcode.failed"
	EventSegmentReady       EventType = "transcode.segment.ready"
	EventManifestUpdated    EventType = "transcode.manifest.updated"

	// Media processing events
	EventMediaScanned         EventType = "media.scanned"
	EventMediaAnalyzed        EventType = "media.analyzed"
	EventMediaEnrichmentReady EventType = "media.enrichment.ready"

	// Asset management events
	EventAssetRequested EventType = "asset.requested"
	EventAssetReady     EventType = "asset.ready"
	EventAssetMissing   EventType = "asset.missing"
)

// ServiceRegisteredData represents data for service.registered event
type ServiceRegisteredData struct {
	ServiceName string    `json:"service_name"`
	ModuleID    string    `json:"module_id"`
	ModuleName  string    `json:"module_name"`
	Timestamp   time.Time `json:"timestamp"`
}

// ServiceRequestedData represents data for service.requested event
type ServiceRequestedData struct {
	ServiceName string    `json:"service_name"`
	RequesterID string    `json:"requester_id"`
	RequiredBy  []string  `json:"required_by"`
	Timestamp   time.Time `json:"timestamp"`
}

// ModuleLifecycleData represents data for module lifecycle events
type ModuleLifecycleData struct {
	ModuleID   string                 `json:"module_id"`
	ModuleName string                 `json:"module_name"`
	State      string                 `json:"state"`
	Services   []string               `json:"services,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// TranscodeEventData represents data for transcoding events
type TranscodeEventData struct {
	SessionID   string                 `json:"session_id"`
	MediaFileID string                 `json:"media_file_id"`
	Container   string                 `json:"container"`
	Status      string                 `json:"status"`
	Progress    float64                `json:"progress,omitempty"`
	ContentHash string                 `json:"content_hash,omitempty"`
	ManifestURL string                 `json:"manifest_url,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// SegmentEventData represents data for segment-related events
type SegmentEventData struct {
	SessionID   string    `json:"session_id"`
	SegmentID   int       `json:"segment_id"`
	SegmentPath string    `json:"segment_path"`
	Duration    float64   `json:"duration"`
	Size        int64     `json:"size"`
	Timestamp   time.Time `json:"timestamp"`
}

// MediaProcessingEventData represents data for media processing events
type MediaProcessingEventData struct {
	MediaFileID string                 `json:"media_file_id"`
	MediaType   string                 `json:"media_type"`
	Path        string                 `json:"path"`
	ProcessorID string                 `json:"processor_id"`
	Status      string                 `json:"status"`
	Results     map[string]interface{} `json:"results,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// AssetEventData represents data for asset-related events
type AssetEventData struct {
	AssetType   string    `json:"asset_type"`
	AssetID     string    `json:"asset_id"`
	MediaID     string    `json:"media_id,omitempty"`
	URL         string    `json:"url,omitempty"`
	Size        int64     `json:"size,omitempty"`
	RequesterID string    `json:"requester_id,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Helper functions to create module events

// NewServiceRegisteredEvent creates a new service registered event
func NewServiceRegisteredEvent(serviceName, moduleID, moduleName string) Event {
	return Event{
		ID:       fmt.Sprintf("srv-reg-%d", time.Now().UnixNano()),
		Type:     EventServiceRegistered,
		Source:   fmt.Sprintf("module:%s", moduleID),
		Title:    "Service Registered",
		Message:  fmt.Sprintf("Service '%s' registered by module '%s'", serviceName, moduleName),
		Priority: PriorityNormal,
		Tags:     []string{"service", "registration", serviceName},
		Data: map[string]interface{}{
			"service_name": serviceName,
			"module_id":    moduleID,
			"module_name":  moduleName,
		},
		Timestamp: time.Now(),
	}
}

// NewModuleLifecycleEvent creates a new module lifecycle event
func NewModuleLifecycleEvent(eventType EventType, moduleID, moduleName, state string) Event {
	return Event{
		ID:       fmt.Sprintf("mod-lc-%d", time.Now().UnixNano()),
		Type:     eventType,
		Source:   fmt.Sprintf("module:%s", moduleID),
		Title:    "Module Lifecycle",
		Message:  fmt.Sprintf("Module '%s' %s", moduleName, state),
		Priority: PriorityNormal,
		Tags:     []string{"module", "lifecycle", state},
		Data: map[string]interface{}{
			"module_id":   moduleID,
			"module_name": moduleName,
			"state":       state,
		},
		Timestamp: time.Now(),
	}
}

// NewTranscodeEvent creates a new transcoding event
func NewTranscodeEvent(eventType EventType, sessionID, mediaFileID, status string) Event {
	return Event{
		ID:       fmt.Sprintf("transcode-%d", time.Now().UnixNano()),
		Type:     eventType,
		Source:   "module:transcoding",
		Title:    "Transcoding Event",
		Message:  fmt.Sprintf("Transcoding %s for session %s", status, sessionID),
		Priority: PriorityNormal,
		Tags:     []string{"transcoding", status},
		Data: map[string]interface{}{
			"session_id":    sessionID,
			"media_file_id": mediaFileID,
			"status":        status,
		},
		Timestamp: time.Now(),
	}
}

// NewSegmentReadyEvent creates a new segment ready event
func NewSegmentReadyEvent(sessionID string, segmentID int, segmentPath string, duration float64) Event {
	return Event{
		ID:       fmt.Sprintf("segment-%d-%d", time.Now().UnixNano(), segmentID),
		Type:     EventSegmentReady,
		Source:   "module:transcoding",
		Title:    "Segment Ready",
		Message:  fmt.Sprintf("Segment %d ready for session %s", segmentID, sessionID),
		Priority: PriorityNormal,
		Tags:     []string{"segment", "streaming"},
		Data: map[string]interface{}{
			"session_id":   sessionID,
			"segment_id":   segmentID,
			"segment_path": segmentPath,
			"duration":     duration,
		},
		Timestamp: time.Now(),
	}
}

// NewMediaProcessingEvent creates a new media processing event
func NewMediaProcessingEvent(eventType EventType, mediaFileID, processorID, status string) Event {
	return Event{
		ID:       fmt.Sprintf("media-proc-%d", time.Now().UnixNano()),
		Type:     eventType,
		Source:   fmt.Sprintf("processor:%s", processorID),
		Title:    "Media Processing",
		Message:  fmt.Sprintf("Media file %s %s", mediaFileID, status),
		Priority: PriorityNormal,
		Tags:     []string{"media", "processing", status},
		Data: map[string]interface{}{
			"media_file_id": mediaFileID,
			"processor_id":  processorID,
			"status":        status,
		},
		Timestamp: time.Now(),
	}
}

// NewAssetEvent creates a new asset event
func NewAssetEvent(eventType EventType, assetType, assetID, status string) Event {
	return Event{
		ID:       fmt.Sprintf("asset-%d", time.Now().UnixNano()),
		Type:     eventType,
		Source:   "module:asset",
		Title:    "Asset Event",
		Message:  fmt.Sprintf("Asset %s/%s %s", assetType, assetID, status),
		Priority: PriorityNormal,
		Tags:     []string{"asset", assetType, status},
		Data: map[string]interface{}{
			"asset_type": assetType,
			"asset_id":   assetID,
			"status":     status,
		},
		Timestamp: time.Now(),
	}
}

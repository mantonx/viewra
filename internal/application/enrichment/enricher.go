// Package enrichment provides the unified Enricher interface for all
// enrichment stages in the media enrichment pipeline.
//
// Both built-in (in-process) and third-party (gRPC) plugins implement this interface.
// The Pipeline Manager treats all enrichers uniformly regardless of execution model.
//
// This package uses proto types directly to avoid conversion overhead between
// internal types and gRPC wire format.
package enrichment

import (
	"context"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// Enricher is the unified interface for all enrichment stages.
// Both built-in (in-process) and third-party (gRPC) plugins implement this.
//
// Built-in enrichers (NFO, Local Images) run in-process and can directly
// import ViewRA's internal libraries.
//
// Third-party enrichers communicate via gRPC and access data through
// Host Services only.
type Enricher interface {
	// Stage returns the unique identifier for this enrichment stage.
	// Examples: "nfo", "local_images", "tmdb", "musicbrainz"
	Stage() string

	// Capabilities describes what this enricher provides and requires.
	// Used by the Pipeline Manager to configure worker pools.
	Capabilities() EnricherCapabilities

	// Enrich processes a single media item.
	// Returns EnrichResponse with metadata, discovered IDs, and images.
	//
	// Implementations should:
	// - Return Skipped=true (not an error) if there's nothing to do
	// - Return errors only for actual failures (network, parsing, etc.)
	// - Populate DiscoveredIds for any external IDs found
	Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error)
}

// EnricherCapabilities is a convenience wrapper around the proto type
// that provides helper methods. The underlying data uses proto types directly.
type EnricherCapabilities struct {
	*pluginv1.EnricherCapabilities
}

// NewCapabilities creates EnricherCapabilities from proto.
func NewCapabilities(caps *pluginv1.EnricherCapabilities) EnricherCapabilities {
	return EnricherCapabilities{caps}
}

// SupportsMediaType returns true if this enricher supports the given media type.
func (c EnricherCapabilities) SupportsMediaType(mt enrichment.MediaType) bool {
	for _, supported := range c.MediaTypes {
		if supported == string(mt) {
			return true
		}
	}
	return false
}

// IsLocalEnricher returns true if this enricher operates locally (high concurrency).
func (c EnricherCapabilities) IsLocalEnricher() bool {
	return c.IsLocal
}

// GetRateLimitPerMinute returns requests per minute (0 = unlimited).
func (c EnricherCapabilities) GetRateLimitPerMinute() int {
	return int(c.RateLimit)
}

// --- Request/Response helpers ---

// HasExternalID returns true if the request has the specified external ID.
func HasExternalID(req *pluginv1.EnrichRequest, provider string) bool {
	if req.ExistingIds == nil {
		return false
	}
	_, ok := req.ExistingIds[provider]
	return ok
}

// GetExternalID returns the external ID for the given provider, or empty string if not found.
func GetExternalID(req *pluginv1.EnrichRequest, provider string) string {
	if req.ExistingIds == nil {
		return ""
	}
	return req.ExistingIds[provider]
}

// Skip creates a skipped response with the given reason.
func Skip(reason string) *pluginv1.EnrichResponse {
	return &pluginv1.EnrichResponse{
		Skipped:    true,
		SkipReason: reason,
	}
}

// NoMatch creates a response indicating no match was found.
func NoMatch() *pluginv1.EnrichResponse {
	return &pluginv1.EnrichResponse{
		Matched: false,
	}
}

// Match creates a successful match response.
// Caller should populate Metadata, DiscoveredIds, and Images as needed.
func Match() *pluginv1.EnrichResponse {
	return &pluginv1.EnrichResponse{
		Matched:       true,
		DiscoveredIds: make(map[string]string),
	}
}

// AddDiscoveredID adds an external ID to the response.
// Initializes the map if nil.
func AddDiscoveredID(resp *pluginv1.EnrichResponse, provider, id string) {
	if resp.DiscoveredIds == nil {
		resp.DiscoveredIds = make(map[string]string)
	}
	resp.DiscoveredIds[provider] = id
}

// AddImage adds an image to the response.
func AddImage(resp *pluginv1.EnrichResponse, img *pluginv1.EnrichedImage) {
	resp.Images = append(resp.Images, img)
}

// --- Metadata helpers ---

// Ptr returns a pointer to the value. Useful for setting optional proto fields.
func Ptr[T any](v T) *T {
	return &v
}

// NewMetadata creates a new EnrichedMetadata proto.
func NewMetadata() *pluginv1.EnrichedMetadata {
	return &pluginv1.EnrichedMetadata{}
}

// --- Capabilities builder ---

// CapabilitiesBuilder helps construct EnricherCapabilities.
type CapabilitiesBuilder struct {
	caps *pluginv1.EnricherCapabilities
}

// NewCapabilitiesBuilder creates a new builder.
func NewCapabilitiesBuilder() *CapabilitiesBuilder {
	return &CapabilitiesBuilder{
		caps: &pluginv1.EnricherCapabilities{},
	}
}

// WithMediaTypes sets supported media types.
func (b *CapabilitiesBuilder) WithMediaTypes(types ...enrichment.MediaType) *CapabilitiesBuilder {
	for _, t := range types {
		b.caps.MediaTypes = append(b.caps.MediaTypes, string(t))
	}
	return b
}

// WithProvides sets what this enricher provides.
func (b *CapabilitiesBuilder) WithProvides(provides ...string) *CapabilitiesBuilder {
	b.caps.Provides = provides
	return b
}

// AsLocal marks this as a local enricher (high concurrency, no rate limit).
func (b *CapabilitiesBuilder) AsLocal() *CapabilitiesBuilder {
	b.caps.IsLocal = true
	return b
}

// AsRemote marks this as a remote enricher with a rate limit.
func (b *CapabilitiesBuilder) AsRemote(ratePerMinute int) *CapabilitiesBuilder {
	b.caps.IsLocal = false
	b.caps.RateLimit = int32(ratePerMinute)
	return b
}

// WithRequires sets required external IDs.
func (b *CapabilitiesBuilder) WithRequires(requires ...string) *CapabilitiesBuilder {
	b.caps.Requires = requires
	return b
}

// Build returns the constructed capabilities.
func (b *CapabilitiesBuilder) Build() EnricherCapabilities {
	return EnricherCapabilities{b.caps}
}

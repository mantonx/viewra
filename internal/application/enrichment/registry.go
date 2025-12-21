package enrichment

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
)

// EnricherMetadata provides optional metadata for enrichers.
// Built-in enrichers can implement this to provide display info.
type EnricherMetadata interface {
	// Metadata returns display information about the enricher.
	Metadata() (name, version string)
}

// EnricherSource indicates where an enricher comes from.
type EnricherSource string

const (
	SourceBuiltin  EnricherSource = "builtin"
	SourceExternal EnricherSource = "external"
)

// EnricherInfo holds metadata about a registered enricher.
type EnricherInfo struct {
	// Stage is the unique identifier (e.g., "nfo", "tmdb", "musicbrainz")
	Stage string

	// Name is the human-readable name (e.g., "NFO Parser", "TMDb Enricher")
	Name string

	// Version is the enricher version (e.g., "1.0.0")
	Version string

	// Source indicates if this is builtin or external
	Source EnricherSource

	// Enricher is the actual enricher implementation
	Enricher Enricher

	// Provides lists what this enricher provides (metadata, artwork, external_ids)
	Provides []string

	// MediaTypes lists supported media types
	MediaTypes []string

	// IsLocal indicates if this runs locally (high concurrency) or remotely
	IsLocal bool

	// RateLimit is requests per minute for remote enrichers (0 = unlimited)
	RateLimit int

	// BuildTime is the time when the plugin binary was built (external plugins only)
	BuildTime time.Time
}

// Registry manages all enrichers (both built-in and external).
type Registry struct {
	mu        sync.RWMutex
	enrichers map[string]*EnricherInfo
	order     []string // maintains registration order
}

// NewRegistry creates a new enricher registry.
func NewRegistry() *Registry {
	return &Registry{
		enrichers: make(map[string]*EnricherInfo),
		order:     make([]string, 0),
	}
}

// Register adds an enricher to the registry.
func (r *Registry) Register(info *EnricherInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if info.Stage == "" {
		return fmt.Errorf("enricher stage is required")
	}

	if _, exists := r.enrichers[info.Stage]; exists {
		return fmt.Errorf("enricher already registered: %s", info.Stage)
	}

	// Extract capabilities from enricher if not provided
	if info.Enricher != nil {
		caps := info.Enricher.Capabilities()
		if len(info.Provides) == 0 {
			info.Provides = caps.Provides
		}
		if len(info.MediaTypes) == 0 {
			info.MediaTypes = caps.MediaTypes
		}
		if !info.IsLocal && caps.IsLocal {
			info.IsLocal = caps.IsLocal
		}
		if info.RateLimit == 0 {
			info.RateLimit = int(caps.RateLimit)
		}
	}

	r.enrichers[info.Stage] = info
	r.order = append(r.order, info.Stage)
	return nil
}

// Get returns an enricher by stage name.
func (r *Registry) Get(stage string) (*EnricherInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.enrichers[stage]
	return info, ok
}

// All returns all registered enrichers in registration order.
func (r *Registry) All() []*EnricherInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*EnricherInfo, 0, len(r.order))
	for _, stage := range r.order {
		if info, ok := r.enrichers[stage]; ok {
			result = append(result, info)
		}
	}
	return result
}

// BySource returns enrichers filtered by source.
func (r *Registry) BySource(source EnricherSource) []*EnricherInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*EnricherInfo, 0)
	for _, stage := range r.order {
		if info, ok := r.enrichers[stage]; ok && info.Source == source {
			result = append(result, info)
		}
	}
	return result
}

// Count returns the total number of registered enrichers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.enrichers)
}

// PrintTable writes a formatted table of all registered enrichers.
func (r *Registry) PrintTable(w io.Writer, title string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.enrichers) == 0 {
		fmt.Fprintln(w, "No enrichers registered")
		return
	}

	// Print title if provided
	if title != "" {
		fmt.Fprintf(w, "\n%s\n", title)
	}

	// Create table with Unicode box-drawing style
	table := tablewriter.NewTable(w,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Borders: tw.Border{
				Left:   tw.On,
				Right:  tw.On,
				Top:    tw.On,
				Bottom: tw.On,
			},
			Symbols: tw.NewSymbols(tw.StyleLight),
			Settings: tw.Settings{
				Separators: tw.Separators{
					BetweenColumns: tw.On,
				},
				Lines: tw.Lines{
					ShowHeaderLine: tw.On,
				},
			},
		})),
	)

	table.Header("Name", "Version", "Type", "Provides", "Media Types", "Built")

	for _, stage := range r.order {
		info := r.enrichers[stage]

		// Format type column
		typeStr := string(info.Source)
		if info.IsLocal {
			typeStr += " (local)"
		} else if info.RateLimit > 0 {
			typeStr += fmt.Sprintf(" (%d/min)", info.RateLimit)
		}

		// Format provides column
		provides := strings.Join(info.Provides, ", ")

		// Format media types (abbreviate if too long)
		mediaTypes := formatMediaTypes(info.MediaTypes)

		// Format build time (only for external plugins)
		buildTimeStr := "-"
		if info.Source == SourceExternal && !info.BuildTime.IsZero() {
			buildTimeStr = info.BuildTime.Format("2006-01-02 15:04")
		}

		table.Append([]string{
			info.Name,
			info.Version,
			typeStr,
			provides,
			mediaTypes,
			buildTimeStr,
		})
	}

	table.Render()
}

// RegisterBuiltin is a helper to register a built-in enricher.
// It extracts metadata if the enricher implements EnricherMetadata.
func (r *Registry) RegisterBuiltin(enricher Enricher) error {
	info := &EnricherInfo{
		Stage:    enricher.Stage(),
		Source:   SourceBuiltin,
		Enricher: enricher,
	}

	// Try to get metadata from the enricher
	if meta, ok := enricher.(EnricherMetadata); ok {
		info.Name, info.Version = meta.Metadata()
	} else {
		// Fallback to stage name
		info.Name = enricher.Stage()
		info.Version = "builtin"
	}

	return r.Register(info)
}

// RegisterExternal is a helper to register an external plugin enricher.
func (r *Registry) RegisterExternal(enricher Enricher, name, version string, buildTime time.Time) error {
	info := &EnricherInfo{
		Stage:     enricher.Stage(),
		Name:      name,
		Version:   version,
		Source:    SourceExternal,
		Enricher:  enricher,
		BuildTime: buildTime,
	}

	return r.Register(info)
}

// formatMediaTypes abbreviates media type names for display.
func formatMediaTypes(types []string) string {
	if len(types) == 0 {
		return "-"
	}

	// Abbreviate common prefixes
	abbrev := make([]string, 0, len(types))
	for _, t := range types {
		switch t {
		case "movie":
			abbrev = append(abbrev, "mov")
		case "tv":
			abbrev = append(abbrev, "tv")
		case "tv_show":
			abbrev = append(abbrev, "show")
		case "tv_season":
			abbrev = append(abbrev, "season")
		case "music":
			abbrev = append(abbrev, "track")
		case "music_album":
			abbrev = append(abbrev, "album")
		case "music_artist":
			abbrev = append(abbrev, "artist")
		default:
			abbrev = append(abbrev, t)
		}
	}

	return strings.Join(abbrev, ", ")
}

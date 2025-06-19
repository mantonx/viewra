package plugins

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// DashboardSectionBuilder provides a fluent interface for building dashboard sections
type DashboardSectionBuilder struct {
	section *DashboardSection
}

// NewDashboardSection creates a new dashboard section builder
func NewDashboardSection(id, title, pluginType string) *DashboardSectionBuilder {
	return &DashboardSectionBuilder{
		section: &DashboardSection{
			ID:       id,
			Type:     pluginType,
			Title:    title,
			Priority: 0,
			Config: DashboardSectionConfig{
				RefreshInterval:  30, // Default 30 seconds
				SupportsRealtime: false,
				HasNerdPanel:     true,
				RequiresAuth:     false,
			},
			Manifest: DashboardManifest{
				ComponentType: "builtin",
				DataEndpoints: make(map[string]DataEndpoint),
				Actions:       []DashboardAction{},
			},
		},
	}
}

// WithDescription sets the section description
func (b *DashboardSectionBuilder) WithDescription(description string) *DashboardSectionBuilder {
	b.section.Description = description
	return b
}

// WithIcon sets the section icon
func (b *DashboardSectionBuilder) WithIcon(icon string) *DashboardSectionBuilder {
	b.section.Icon = icon
	return b
}

// WithPriority sets the section priority (higher = more important)
func (b *DashboardSectionBuilder) WithPriority(priority int) *DashboardSectionBuilder {
	b.section.Priority = priority
	return b
}

// WithRefreshInterval sets how often the section should refresh
func (b *DashboardSectionBuilder) WithRefreshInterval(interval time.Duration) *DashboardSectionBuilder {
	b.section.Config.RefreshInterval = int(interval.Seconds())
	return b
}

// WithAction adds an action to the section
func (b *DashboardSectionBuilder) WithAction(id, label, actionType string) *DashboardSectionBuilder {
	action := DashboardAction{
		ID:       id,
		Label:    label,
		Icon:     "activity",
		Style:    "primary",
		Endpoint: fmt.Sprintf("/api/v1/dashboard/sections/%s/actions/%s", b.section.ID, id),
		Method:   "POST",
		Confirm:  false,
	}
	b.section.Manifest.Actions = append(b.section.Manifest.Actions, action)
	return b
}

// Build returns the completed dashboard section
func (b *DashboardSectionBuilder) Build() *DashboardSection {
	return b.section
}

// Common dashboard data structures and helpers

// MetricValue represents a single metric with formatting
type MetricValue struct {
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`         // "MB", "GB", "fps", "%", etc.
	DisplayName string  `json:"display_name"` // Human readable name
	Trend       string  `json:"trend"`        // "up", "down", "stable"
	IsGood      bool    `json:"is_good"`      // Whether current value is considered good
}

// QuickStats provides a standard way to show key metrics
type QuickStats struct {
	Primary   *MetricValue `json:"primary"`   // Most important metric
	Secondary *MetricValue `json:"secondary"` // Secondary metric
	Status    string       `json:"status"`    // Overall status: "healthy", "warning", "error"
}

// SessionSummary provides a standard way to show session information
type SessionSummary struct {
	ID           string            `json:"id"`
	DisplayName  string            `json:"display_name"` // User-friendly name
	Status       string            `json:"status"`       // "active", "queued", "completed", "failed"
	Progress     float64           `json:"progress"`     // 0-100
	StartTime    time.Time         `json:"start_time"`
	Duration     time.Duration     `json:"duration"`
	ClientInfo   *ClientInfo       `json:"client_info"`
	Metadata     map[string]string `json:"metadata"`
	CanInterrupt bool              `json:"can_interrupt"` // Whether session can be stopped
}

// ClientInfo provides standardized client information
type ClientInfo struct {
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Device    string `json:"device"`   // "Chrome", "Firefox", "Roku", etc.
	Platform  string `json:"platform"` // "Windows", "macOS", "Android", etc.
}

// Helper functions for common dashboard patterns

// FormatBytes converts bytes to human readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration converts duration to human readable format
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// CalculatePercentage safely calculates percentage with division by zero protection
func CalculatePercentage(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

// CreateHealthStatus creates a standardized health status
func CreateHealthStatus(healthy, warning, error int) *StatusIndicator {
	total := healthy + warning + error
	if error > 0 {
		return &StatusIndicator{
			Key:     "health_status",
			Value:   fmt.Sprintf("%d errors, %d warnings", error, warning),
			Color:   "red",
			Tooltip: fmt.Sprintf("Total: %d services", total),
		}
	}
	if warning > 0 {
		return &StatusIndicator{
			Key:     "health_status",
			Value:   fmt.Sprintf("%d warnings", warning),
			Color:   "yellow",
			Tooltip: fmt.Sprintf("Total: %d services", total),
		}
	}
	return &StatusIndicator{
		Key:     "health_status",
		Value:   "All systems operational",
		Color:   "green",
		Tooltip: fmt.Sprintf("Total: %d services", total),
	}
}

// Base dashboard data provider with common functionality
type BaseDashboardProvider struct {
	sectionID   string
	refreshRate time.Duration
}

// NewBaseDashboardProvider creates a new base provider
func NewBaseDashboardProvider(sectionID string, refreshRate time.Duration) *BaseDashboardProvider {
	return &BaseDashboardProvider{
		sectionID:   sectionID,
		refreshRate: refreshRate,
	}
}

// GetRefreshRate returns the refresh rate for this section
func (b *BaseDashboardProvider) GetRefreshRate() time.Duration {
	return b.refreshRate
}

// SectionID returns the section ID
func (b *BaseDashboardProvider) SectionID() string {
	return b.sectionID
}

// Common action types
const (
	ActionTypeButton = "button"
	ActionTypeToggle = "toggle"
	ActionTypeMenu   = "menu"
	ActionTypeModal  = "modal"
)

// Common action IDs
const (
	ActionRestart    = "restart"
	ActionStop       = "stop"
	ActionPause      = "pause"
	ActionResume     = "resume"
	ActionClearCache = "clear_cache"
	ActionRefresh    = "refresh"
	ActionViewLogs   = "view_logs"
	ActionConfigure  = "configure"
)

// Action builder helpers
func CreateRestartAction() *DashboardAction {
	return &DashboardAction{
		ID:       ActionRestart,
		Label:    "Restart Service",
		Icon:     "refresh-cw",
		Style:    "danger",
		Endpoint: "",
		Method:   "POST",
		Confirm:  true,
	}
}

func CreateStopAction() *DashboardAction {
	return &DashboardAction{
		ID:       ActionStop,
		Label:    "Stop Service",
		Icon:     "stop-circle",
		Style:    "danger",
		Endpoint: "",
		Method:   "POST",
		Confirm:  true,
	}
}

func CreateClearCacheAction() *DashboardAction {
	return &DashboardAction{
		ID:       ActionClearCache,
		Label:    "Clear Cache",
		Icon:     "trash-2",
		Style:    "secondary",
		Endpoint: "",
		Method:   "POST",
		Confirm:  false,
	}
}

// Session helper functions

// FilterActiveSessions filters sessions to only show active ones
func FilterActiveSessions(sessions []*SessionSummary) []*SessionSummary {
	var active []*SessionSummary
	for _, session := range sessions {
		if session.Status == "active" || session.Status == "running" {
			active = append(active, session)
		}
	}
	return active
}

// SortSessionsByStartTime sorts sessions by start time (newest first)
func SortSessionsByStartTime(sessions []*SessionSummary) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
}

// Interface for plugins that provide dashboard data using helpers
type EnhancedDashboardProvider interface {
	DashboardDataProvider

	// GetQuickStats returns the most important metrics for this plugin
	GetQuickStats(ctx context.Context) (*QuickStats, error)

	// GetActiveSessions returns current active sessions
	GetActiveSessions(ctx context.Context) ([]*SessionSummary, error)

	// GetHealthStatus returns overall health status
	GetHealthStatus(ctx context.Context) (*StatusIndicator, error)
}

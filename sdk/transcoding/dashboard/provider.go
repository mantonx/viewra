// Package dashboard provides dashboard functionality for transcoding providers.
// This package enables transcoding providers to expose monitoring data, statistics,
// and control actions through a unified dashboard interface. It supports multiple
// dashboard sections, real-time data updates, and provider-specific actions.
//
// The dashboard system allows providers to:
// - Display real-time statistics and metrics
// - Show active session information
// - Provide control actions (stop, pause, resume)
// - Monitor resource usage
// - Track performance metrics
//
// Dashboard sections can be of various types:
// - Stats: Key-value statistics display
// - Table: Tabular data for sessions or jobs
// - Chart: Time-series or categorical data visualization
// - Actions: Interactive controls
//
// Example usage:
//   provider := dashboard.NewProvider(logger)
//   provider.RegisterStatsSection("overview", getOverviewStats)
//   provider.RegisterTableSection("sessions", getSessionTable)
//   provider.RegisterAction("stop_all", stopAllSessions)
//   
//   // Get dashboard data
//   data, err := provider.GetSectionData("overview")
package dashboard

import (
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Provider manages dashboard functionality for a transcoding provider
type Provider struct {
	logger      types.Logger
	sections    map[string]*Section
	actions     map[string]ActionHandler
	dataFuncs   map[string]DataFunc
	mutex       sync.RWMutex
}

// Section represents a dashboard section
type Section struct {
	ID          string
	Title       string
	Type        string
	Description string
	RefreshRate int // seconds, 0 for manual refresh
}

// DataFunc is a function that returns dashboard data
type DataFunc func() (interface{}, error)

// ActionHandler handles dashboard actions
type ActionHandler func(params map[string]interface{}) error

// NewProvider creates a new dashboard provider
func NewProvider(logger types.Logger) *Provider {
	return &Provider{
		logger:    logger,
		sections:  make(map[string]*Section),
		actions:   make(map[string]ActionHandler),
		dataFuncs: make(map[string]DataFunc),
	}
}

// RegisterStatsSection registers a statistics section
func (p *Provider) RegisterStatsSection(id, title, description string, dataFunc DataFunc) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.sections[id] = &Section{
		ID:          id,
		Title:       title,
		Type:        "stats",
		Description: description,
		RefreshRate: 5, // Default 5 second refresh
	}
	p.dataFuncs[id] = dataFunc
}

// RegisterTableSection registers a table section
func (p *Provider) RegisterTableSection(id, title, description string, dataFunc DataFunc) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.sections[id] = &Section{
		ID:          id,
		Title:       title,
		Type:        "table",
		Description: description,
		RefreshRate: 10, // Default 10 second refresh
	}
	p.dataFuncs[id] = dataFunc
}

// RegisterChartSection registers a chart section
func (p *Provider) RegisterChartSection(id, title, description string, dataFunc DataFunc) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.sections[id] = &Section{
		ID:          id,
		Title:       title,
		Type:        "chart",
		Description: description,
		RefreshRate: 30, // Default 30 second refresh
	}
	p.dataFuncs[id] = dataFunc
}

// RegisterAction registers a dashboard action
func (p *Provider) RegisterAction(id string, handler ActionHandler) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.actions[id] = handler
}

// GetSections returns all registered dashboard sections
func (p *Provider) GetSections() []types.DashboardSection {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	sections := make([]types.DashboardSection, 0, len(p.sections))
	for _, section := range p.sections {
		sections = append(sections, types.DashboardSection{
			ID:          section.ID,
			Title:       section.Title,
			Type:        section.Type,
			Description: section.Description,
		})
	}

	return sections
}

// GetSectionData returns data for a specific section
func (p *Provider) GetSectionData(sectionID string) (interface{}, error) {
	p.mutex.RLock()
	section, exists := p.sections[sectionID]
	if !exists {
		p.mutex.RUnlock()
		return nil, fmt.Errorf("unknown section: %s", sectionID)
	}

	dataFunc, hasFunc := p.dataFuncs[sectionID]
	p.mutex.RUnlock()

	if !hasFunc {
		return nil, fmt.Errorf("no data function for section: %s", sectionID)
	}

	// Call the data function
	data, err := dataFunc()
	if err != nil {
		if p.logger != nil {
			p.logger.Error("failed to get dashboard data",
				"section", sectionID,
				"error", err,
			)
		}
		return nil, err
	}

	// Wrap data with metadata
	return map[string]interface{}{
		"section":      section,
		"data":         data,
		"timestamp":    getCurrentTimestamp(),
		"refreshRate":  section.RefreshRate,
	}, nil
}

// ExecuteAction executes a dashboard action
func (p *Provider) ExecuteAction(actionID string, params map[string]interface{}) error {
	p.mutex.RLock()
	handler, exists := p.actions[actionID]
	p.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("unknown action: %s", actionID)
	}

	// Log action execution
	if p.logger != nil {
		p.logger.Info("executing dashboard action",
			"action", actionID,
			"params", params,
		)
	}

	// Execute the action
	err := handler(params)
	if err != nil {
		if p.logger != nil {
			p.logger.Error("dashboard action failed",
				"action", actionID,
				"error", err,
			)
		}
		return err
	}

	return nil
}

// SetSectionRefreshRate updates the refresh rate for a section
func (p *Provider) SetSectionRefreshRate(sectionID string, seconds int) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	section, exists := p.sections[sectionID]
	if !exists {
		return fmt.Errorf("unknown section: %s", sectionID)
	}

	section.RefreshRate = seconds
	return nil
}

// RemoveSection removes a dashboard section
func (p *Provider) RemoveSection(sectionID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	delete(p.sections, sectionID)
	delete(p.dataFuncs, sectionID)
}

// RemoveAction removes a dashboard action
func (p *Provider) RemoveAction(actionID string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	delete(p.actions, actionID)
}

// getCurrentTimestamp returns the current timestamp in ISO format
func getCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
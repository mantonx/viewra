package plugins

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	infraplugins "github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// toSummary converts a database row to a PluginSummary.
func (s *Service) toSummary(row Plugin) PluginSummary {
	summary := PluginSummary{
		ID:           row.ID,
		Name:         row.Name,
		Version:      row.Version,
		Description:  row.Description,
		Author:       row.Author,
		Capabilities: parseCapabilities(row.Capabilities),
		Enabled:      row.Enabled,
		IsBuiltin:    row.IsBuiltin,
		Health:       "unknown",
		HasSettings:  row.SettingsSchema != "" && row.SettingsSchema != "{}",
	}

	if row.HealthStatus != "" {
		summary.Health = row.HealthStatus
	}

	// Get runtime info if plugin is loaded
	if instance, ok := s.manager.GetPlugin(row.ID); ok {
		summary.Health = instance.Health.Status.String()

		// Extract x-viewra-meta from settings schema
		if meta := s.getPluginMeta(instance); meta != nil {
			summary.Meta = meta
		}

		// Check if running plugin has settings schema (even if not cached in DB)
		if !summary.HasSettings && instance.CoreClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
			cancel()
			if err == nil && schemaResp != nil && len(schemaResp.JsonSchema) > 0 {
				summary.HasSettings = true
				// Cache it for next time
				if cacheErr := s.queries.UpdatePluginSettingsSchema(context.Background(), row.ID, string(schemaResp.JsonSchema)); cacheErr != nil {
					s.logger.Warn("failed to cache settings schema", "plugin", row.ID, "error", cacheErr)
				}
			}
		}

		// Get capabilities from manifest
		if instance.Manifest != nil {
			summary.Capabilities = instance.Manifest.Capabilities
		}

		// Check for missing dependencies from manifest
		if instance.Manifest != nil && len(instance.Manifest.Requires) > 0 {
			summary.MissingDependencies = s.checkMissingDependencies(instance.Manifest.Requires)
		}
	}

	// Get capabilities from capability registry (for route-based capabilities)
	if capRegistry := s.manager.GetCapabilityRegistry(); capRegistry != nil {
		routeCaps := capRegistry.GetCapabilitiesForPlugin(row.ID)
		if len(routeCaps) > 0 {
			// Merge with manifest capabilities, avoiding duplicates
			for _, cap := range routeCaps {
				if !contains(summary.Capabilities, cap) {
					summary.Capabilities = append(summary.Capabilities, cap)
				}
			}
		}
	}

	// Get provider ID if this is a provider plugin
	if providerRegistry := s.manager.GetProviderRegistry(); providerRegistry != nil {
		summary.ProviderID = providerRegistry.GetByPluginID(row.ID)
	}

	// Get manifest-defined display category if available
	var manifestCategory string
	if instance, ok := s.manager.GetPlugin(row.ID); ok && instance.Manifest != nil {
		manifestCategory = instance.Manifest.DisplayCategory
	}

	// Compute display category for UI grouping
	summary.DisplayCategory = computeDisplayCategory(manifestCategory, summary.IsBuiltin)

	return summary
}

// toDetail converts a database row to a PluginDetail.
func (s *Service) toDetail(row Plugin) *PluginDetail {
	summary := s.toSummary(row)

	detail := &PluginDetail{
		PluginSummary: summary,
		License:       row.License,
		Homepage:      row.Homepage,
		RestartCount:  row.RestartCount,
		InstalledAt:   row.InstalledAt,
		UpdatedAt:     row.UpdatedAt,
		LastHeartbeat: row.LastHeartbeat,
	}

	return detail
}

// checkMissingDependencies returns which required capabilities are not currently available.
func (s *Service) checkMissingDependencies(requires []string) []string {
	hostPluginsServer := s.manager.GetHostPluginsServer()
	if hostPluginsServer == nil {
		// If no capability broker, all dependencies are missing
		return requires
	}

	var missing []string
	for _, req := range requires {
		if !hostPluginsServer.HasCapability(req) {
			missing = append(missing, req)
		}
	}
	return missing
}

// getPluginMeta extracts x-viewra-meta from a plugin's settings schema.
func (s *Service) getPluginMeta(instance *infraplugins.PluginInstance) map[string]any {
	if instance.CoreClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil
	}

	if len(schemaResp.JsonSchema) == 0 {
		return nil
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaResp.JsonSchema, &schema); err != nil {
		return nil
	}

	meta, ok := schema["x-viewra-meta"].(map[string]any)
	if !ok {
		return nil
	}

	return meta
}

// getEnricherCapabilities extracts enricher capabilities from a plugin instance.
func (s *Service) getEnricherCapabilities(instance *infraplugins.PluginInstance) *EnricherCapabilities {
	if instance.EnricherClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	caps, err := instance.EnricherClient.GetCapabilities(ctx, &pluginv1.Empty{})
	if err != nil {
		s.logger.Warn("failed to get enricher capabilities", "plugin", instance.ID, "error", err)
		return nil
	}

	return &EnricherCapabilities{
		MediaTypes: caps.MediaTypes,
		Provides:   caps.Provides,
		IsLocal:    caps.IsLocal,
		RateLimit:  int(caps.RateLimit),
		Requires:   caps.Requires,
	}
}

// parseCapabilities parses capabilities from JSON array or comma-separated string.
func parseCapabilities(caps string) []string {
	if caps == "" {
		return nil
	}

	// Try parsing as JSON array first
	caps = strings.TrimSpace(caps)
	if strings.HasPrefix(caps, "[") {
		var result []string
		if err := json.Unmarshal([]byte(caps), &result); err == nil {
			return result
		}
	}

	// Fall back to comma-separated format
	parts := strings.Split(caps, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

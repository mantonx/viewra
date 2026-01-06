package home

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
)

// PluginLookup provides access to plugin instances.
type PluginLookup interface {
	// GetPlugin returns a plugin instance by ID.
	GetPlugin(id string) (*types.Instance, bool)
}

// PluginWidgetFetcher fetches widget data from plugins via gRPC.
type PluginWidgetFetcher struct {
	plugins PluginLookup
	logger  *slog.Logger
}

// NewPluginWidgetFetcher creates a new widget data fetcher.
func NewPluginWidgetFetcher(plugins PluginLookup, logger *slog.Logger) *PluginWidgetFetcher {
	return &PluginWidgetFetcher{
		plugins: plugins,
		logger:  logger,
	}
}

// FetchWidgetData fetches data for a widget from a plugin.
// The path is the widget's configured endpoint (e.g., "/recommendations/for-you").
// The params map contains additional query parameters from endpoint_params config.
func (f *PluginWidgetFetcher) FetchWidgetData(ctx context.Context, pluginID, path, userID, clientType string, params map[string]string) (map[string]any, error) {
	// Get the plugin instance
	instance, ok := f.plugins.GetPlugin(pluginID)
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	if instance.CoreClient == nil {
		return nil, fmt.Errorf("plugin %s has no core client", pluginID)
	}

	// Build the gRPC request
	req := &pluginv1.PluginHTTPRequest{
		Method:  "GET",
		Path:    path,
		Headers: make(map[string]string),
		Query:   make(map[string]string),
		UserId:  userID,
	}

	// Add endpoint_params as query parameters
	for k, v := range params {
		req.Query[k] = v
	}

	// Add client type as query parameter if provided
	if clientType != "" {
		req.Query["client_type"] = clientType
	}

	f.logger.Debug("fetching widget data from plugin",
		"plugin", pluginID,
		"path", path,
		"user_id", userID,
	)

	// Call the plugin
	resp, err := instance.CoreClient.HandleHTTP(ctx, req)
	if err != nil {
		f.logger.Error("plugin HandleHTTP failed",
			"plugin", pluginID,
			"path", path,
			"error", err,
		)
		return nil, fmt.Errorf("plugin request failed: %w", err)
	}

	// Check for non-2xx status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		f.logger.Warn("plugin returned non-success status",
			"plugin", pluginID,
			"path", path,
			"status", resp.StatusCode,
		)
		return nil, fmt.Errorf("plugin returned status %d", resp.StatusCode)
	}

	// Parse the JSON response
	var data map[string]any
	if err := json.Unmarshal(resp.Body, &data); err != nil {
		f.logger.Error("failed to parse plugin response",
			"plugin", pluginID,
			"path", path,
			"error", err,
			"body", string(resp.Body),
		)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	f.logger.Debug("received widget data from plugin",
		"plugin", pluginID,
		"path", path,
		"data_keys", getMapKeys(data),
	)

	return data, nil
}

// getMapKeys returns the keys of a map for logging.
func getMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

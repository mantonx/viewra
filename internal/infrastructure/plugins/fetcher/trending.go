// Package fetcher provides implementations for fetching data from plugins.
package fetcher

import (
	"context"
	"fmt"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/types"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// PluginLookup provides access to plugin instances.
type PluginLookup interface {
	GetPlugin(id string) (*types.Instance, bool)
}

// TrendingDataFetcher fetches trending data from plugins via gRPC.
type TrendingDataFetcher struct {
	plugins PluginLookup
	logger  *slog.Logger
}

// NewTrendingDataFetcher creates a new trending data fetcher.
func NewTrendingDataFetcher(plugins PluginLookup, logger *slog.Logger) *TrendingDataFetcher {
	return &TrendingDataFetcher{
		plugins: plugins,
		logger:  logger,
	}
}

// FetchTrending fetches trending data from a plugin.
func (f *TrendingDataFetcher) FetchTrending(ctx context.Context, pluginID string, req *sdk.TrendingRequest) (*sdk.TrendingResponse, error) {
	instance, ok := f.plugins.GetPlugin(pluginID)
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}

	if instance.TrendingClient == nil {
		return nil, fmt.Errorf("plugin %s does not provide trending service", pluginID)
	}

	// Convert SDK request to proto request
	protoReq := &pluginv1.TrendingRequest{
		MediaType: req.MediaType,
		Window:    req.Window,
		Limit:     int32(req.Limit),
		Region:    req.Region,
	}

	// Call the plugin's GetTrending RPC
	protoResp, err := instance.TrendingClient.GetTrending(ctx, protoReq)
	if err != nil {
		f.logger.Error("trending RPC failed",
			"plugin", pluginID,
			"error", err)
		return nil, fmt.Errorf("trending RPC failed: %w", err)
	}

	// Convert proto response to SDK response
	return protoToSDKTrendingResponse(protoResp), nil
}

// protoToSDKTrendingResponse converts a proto TrendingResponse to SDK TrendingResponse.
func protoToSDKTrendingResponse(resp *pluginv1.TrendingResponse) *sdk.TrendingResponse {
	items := make([]sdk.TrendingItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		sdkItem := sdk.TrendingItem{
			ExternalID:   item.ExternalId,
			MediaType:    item.MediaType,
			Title:        item.Title,
			Year:         int(item.Year),
			Popularity:   item.Popularity,
			PosterPath:   item.PosterPath,
			Overview:     item.Overview,
			LocalMatched: item.LocalMatched,
		}
		if item.LocalId != 0 {
			localID := item.LocalId
			sdkItem.LocalID = &localID
		}
		items = append(items, sdkItem)
	}

	return &sdk.TrendingResponse{
		Items:    items,
		Window:   resp.Window,
		Source:   resp.Source,
		CachedAt: resp.CachedAt,
	}
}

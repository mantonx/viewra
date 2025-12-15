package plugins

import (
	"context"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/application/enrichment"
)

// GRPCEnricher wraps a gRPC EnricherClient to implement the application Enricher interface.
// This allows external plugins (go-plugin processes) to be used as enrichers.
//
// Since the Enricher interface now uses proto types directly, this wrapper is minimal -
// it just stores capabilities and delegates calls to the gRPC client.
type GRPCEnricher struct {
	pluginID string
	client   pluginv1.EnricherClient
	caps     enrichment.EnricherCapabilities
	logger   *slog.Logger
}

// NewGRPCEnricher creates a new GRPCEnricher wrapping the given client.
func NewGRPCEnricher(pluginID string, client pluginv1.EnricherClient, logger *slog.Logger) (*GRPCEnricher, error) {
	e := &GRPCEnricher{
		pluginID: pluginID,
		client:   client,
		logger:   logger,
	}

	// Fetch capabilities once at construction time
	ctx := context.Background()
	protoCaps, err := client.GetCapabilities(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, err
	}
	e.caps = enrichment.NewCapabilities(protoCaps)

	return e, nil
}

// Stage implements enrichment.Enricher.
func (e *GRPCEnricher) Stage() string {
	return e.pluginID
}

// Capabilities implements enrichment.Enricher.
func (e *GRPCEnricher) Capabilities() enrichment.EnricherCapabilities {
	return e.caps
}

// Enrich implements enrichment.Enricher.
// Since we now use proto types directly, this is just a pass-through.
func (e *GRPCEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	return e.client.Enrich(ctx, req)
}

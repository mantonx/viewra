// Package main implements the Semantic Search plugin for ViewRA.
// This plugin provides semantic search and media indexing capabilities.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/semantic-search/internal"
)

func main() {
	// Use SDK logger for go-plugin compatibility - logs will be forwarded to host
	hclogger, logger := sdk.NewLogger("semantic-search")

	// Create the plugin - it implements sdk.VectorSearchEnricherPlugin
	plugin := internal.NewSemanticSearchPlugin(logger)

	// Use the SDK's serve helper which handles all gRPC boilerplate
	sdk.ServeVectorSearchEnricher(plugin, hclogger)
}

// Package main implements the Voyage AI provider plugin for ViewRA.
// This plugin provides high-quality embedding capabilities via Voyage AI.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/ai-provider-voyage/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("ai-provider-voyage")
	provider := internal.NewVoyageProvider(logger)
	sdk.ServeProvider(provider, hclogger)
}

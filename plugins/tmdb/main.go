// Package main implements the TMDb enricher plugin for ViewRA.
// This plugin fetches movie and TV metadata from The Movie Database (TMDb) API.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/tmdb/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("tmdb")
	plugin := internal.NewTMDbPlugin(logger)
	sdk.ServeEnricher(plugin, hclogger)
}

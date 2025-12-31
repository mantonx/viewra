// Package main implements the recommendations plugin for ViewRA.
// This plugin provides personalized recommendations based on user ratings
// and watch history, with widgets for the home screen.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/recommendations/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("recommendations")
	plugin := internal.NewRecommendationsPlugin(logger)
	sdk.ServeWidget(plugin, hclogger)
}

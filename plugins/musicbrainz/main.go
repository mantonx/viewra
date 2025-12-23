// Package main implements the MusicBrainz enricher plugin for ViewRA.
// This plugin fetches music metadata from MusicBrainz and cover art from Cover Art Archive.
package main

import (
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"github.com/mantonx/viewra/plugins/musicbrainz/internal"
)

func main() {
	hclogger, logger := sdk.NewLogger("musicbrainz")
	plugin := internal.NewMusicBrainzPlugin(logger)
	sdk.ServeEnricher(plugin, hclogger)
}

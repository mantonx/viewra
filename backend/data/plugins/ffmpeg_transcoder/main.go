package main

import (
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/plugin"
	"github.com/mantonx/viewra/pkg/plugins"
)

func main() {
	// Create and start the plugin
	p := plugin.New()
	plugins.StartPlugin(p)
}

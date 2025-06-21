package main

import (
	"github.com/mantonx/viewra/plugins/ffmpeg_transcoder/internal/plugin"
	plugins "github.com/mantonx/viewra/sdk"
)

func main() {
	// Create and start the plugin
	p := plugin.New()
	plugins.StartPlugin(p)
}

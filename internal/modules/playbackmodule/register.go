package playbackmodule

import (
	"github.com/mantonx/viewra/internal/modules/modulemanager"
)

// Auto-register the module when imported
func init() {
	Register()
}

// Register registers the playback module with the module system
func Register() {
	playbackModule := &Module{}
	modulemanager.Register(playbackModule)
}
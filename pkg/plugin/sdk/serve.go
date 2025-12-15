package sdk

import (
	"github.com/hashicorp/go-plugin"
)

// Handshake is the shared handshake config for all ViewRA plugins.
// Both host and plugin must agree on these values.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "viewra-plugin-v1",
}

// ServeEnricher starts an enricher plugin server.
// Call this from your plugin's main() function.
//
// Example:
//
//	func main() {
//	    sdk.ServeEnricher(&MyEnricherPlugin{})
//	}
func ServeEnricher(impl EnricherPlugin) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"core":     &EnricherGRPCPlugin{Impl: impl},
			"enricher": &EnricherGRPCPlugin{Impl: impl},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

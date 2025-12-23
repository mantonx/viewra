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

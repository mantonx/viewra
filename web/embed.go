//go:build !dev
// +build !dev

package web

import (
	"embed"
	"io/fs"
)

// DistFS embeds the built frontend files for production
// This only happens when building WITHOUT -tags dev
//
//go:embed dist
var embedFS embed.FS

// FS returns the embedded frontend filesystem
func FS() (fs.FS, error) {
	return fs.Sub(embedFS, "dist")
}

// IsEmbedded returns true when frontend is embedded
func IsEmbedded() bool {
	return true
}

//go:build dev
// +build dev

package web

import (
	"errors"
	"io/fs"
)

// FS returns an error in dev mode - frontend should be served by Vite
func FS() (fs.FS, error) {
	return nil, errors.New("frontend not embedded in dev mode - use Vite dev server on :5173")
}

// IsEmbedded returns false in dev mode
func IsEmbedded() bool {
	return false
}

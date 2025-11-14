package transcoding

import (
	"path/filepath"
	"strconv"
)

// GetHLSOutputPath returns the output directory path for HLS transcoding.
// Format: <outputDir>/hls/<mediaID>/<quality>/
func GetHLSOutputPath(outputDir string, mediaID int64, quality string) string {
	return filepath.Join(
		outputDir,
		"hls",
		strconv.FormatInt(mediaID, 10),
		quality,
	)
}

// GetHLSManifestPath returns the full path to the HLS manifest file.
// Format: <outputDir>/hls/<mediaID>/<quality>/playlist.m3u8
func GetHLSManifestPath(outputDir string, mediaID int64, quality string) string {
	return filepath.Join(
		GetHLSOutputPath(outputDir, mediaID, quality),
		"playlist.m3u8",
	)
}

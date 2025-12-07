// Package paths provides HLS output path utilities for transcoding.
package paths

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GetHLSOutputPath returns the output directory path for HLS transcoding.
// Format: <outputDir>/hls/<mediaID>/<quality>/
// Quality is normalized to lowercase for consistency.
func GetHLSOutputPath(outputDir string, mediaID int64, quality string) string {
	return filepath.Join(
		outputDir,
		"hls",
		fmt.Sprintf("%d", mediaID),
		strings.ToLower(quality),
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

// GetHLSSegmentPath returns the full path to a specific HLS segment file.
// Format: <outputDir>/hls/<mediaID>/<quality>/seg_NNNNNN.ts
// segmentFilename is a function that generates the segment filename from segment number.
func GetHLSSegmentPath(outputDir string, mediaID int64, quality string, segmentNum int, segmentFilename func(int) string) string {
	return filepath.Join(
		GetHLSOutputPath(outputDir, mediaID, quality),
		segmentFilename(segmentNum),
	)
}

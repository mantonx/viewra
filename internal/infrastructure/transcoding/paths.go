package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/paths"
)

// GetHLSOutputPath returns the output directory path for HLS transcoding.
// Re-exported from paths subpackage for backward compatibility.
func GetHLSOutputPath(outputDir string, mediaID int64, quality string) string {
	return paths.GetHLSOutputPath(outputDir, mediaID, quality)
}

// GetHLSManifestPath returns the full path to the HLS manifest file.
// Re-exported from paths subpackage for backward compatibility.
func GetHLSManifestPath(outputDir string, mediaID int64, quality string) string {
	return paths.GetHLSManifestPath(outputDir, mediaID, quality)
}

// GetHLSSegmentPath returns the full path to a specific HLS segment file.
// Re-exported from paths subpackage for backward compatibility.
func GetHLSSegmentPath(outputDir string, mediaID int64, quality string, segmentNum int) string {
	return paths.GetHLSSegmentPath(outputDir, mediaID, quality, segmentNum, SegmentFilename)
}

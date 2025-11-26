package media

import "strings"

// CalculateAspectRatio calculates aspect ratio string from width and height
// Returns common aspect ratio labels (16:9, 4:3, 2.39:1, etc.)
// This is domain logic - how we categorize and label aspect ratios
func CalculateAspectRatio(width, height int) string {
	if height == 0 {
		return ""
	}
	ratio := float64(width) / float64(height)

	// Common aspect ratios: [min, max, label]
	type aspectRatio struct {
		min, max float64
		label    string
	}

	ratios := []aspectRatio{
		{2.35, 2.45, "2.39:1"}, // Cinemascope
		{1.85, 1.86, "1.85:1"},
		{1.77, 1.79, "16:9"},
		{1.32, 1.34, "4:3"},
	}

	for _, ar := range ratios {
		if ratio >= ar.min && ratio <= ar.max {
			return ar.label
		}
	}

	return ""
}

// CalculateResolutionLabel returns resolution label from height
// Returns 4K, 1080p, 720p, 480p, or SD
// This is domain logic - how we categorize video resolutions
// Deprecated: Use CalculateResolutionLabelFromDimensions for accurate 4K detection
func CalculateResolutionLabel(height int) string {
	return CalculateResolutionLabelFromDimensions(0, height)
}

// CalculateResolutionLabelFromDimensions returns resolution label from width and height
// Uses width as the primary indicator for 4K (ultrawide content has full 4K width but reduced height)
// Returns 4K, 1080p, 720p, 480p, or SD
func CalculateResolutionLabelFromDimensions(width, height int) string {
	// 4K detection: use width as primary indicator
	// UHD 4K is 3840 wide, DCI 4K is 4096 wide
	// Ultrawide 4K (2.39:1) has 3840x1600 - still 4K content
	if width >= 3840 {
		return "4K"
	}

	// For non-4K, use height-based detection
	resolutions := []struct {
		minHeight int
		label     string
	}{
		{2160, "4K"}, // Fallback for cases where only height is provided
		{1080, "1080p"},
		{720, "720p"},
		{480, "480p"},
	}

	for _, res := range resolutions {
		if height >= res.minHeight {
			return res.label
		}
	}

	return "SD"
}

// DetectSourceType detects source type from filename patterns
// Returns BluRay, WEB-DL, WEBRip, HDTV, DVDRip, DVD, Remux, or empty string
// This is domain logic - how we identify media sources
func DetectSourceType(filename string) string {
	lower := strings.ToLower(filename)

	// Check in priority order (most specific to least specific)
	patterns := []struct {
		keywords   []string
		sourceType string
	}{
		{[]string{"bluray", "blu-ray", "bdrip", "brrip", "bdremux"}, "BluRay"},
		{[]string{"remux"}, "Remux"},
		{[]string{"web-dl", "webdl", "web dl"}, "WEB-DL"},
		{[]string{"webrip", "web-rip", "web rip"}, "WEBRip"},
		{[]string{"hdtv", "pdtv", "dsr", "tvrip"}, "HDTV"},
		{[]string{"dvdrip", "dvd-rip"}, "DVDRip"},
		{[]string{"dvd"}, "DVD"},
		{[]string{"hddvd", "hd-dvd"}, "HD-DVD"},
		{[]string{"cam", "camrip", "ts", "telesync", "tc", "telecine"}, "CAM"},
		{[]string{"screener", "scr", "dvdscr"}, "Screener"},
	}

	for _, p := range patterns {
		for _, keyword := range p.keywords {
			if strings.Contains(lower, keyword) {
				return p.sourceType
			}
		}
	}

	return ""
}

// Detect3D detects if media is 3D and determines stereo mode from filename
// Returns (is3D bool, stereoMode string)
// This is domain logic - how we identify 3D content
func Detect3D(filename string) (bool, string) {
	lower := strings.ToLower(filename)

	// Check for 3D indicators with specific stereo modes
	patterns := []struct {
		keywords   []string
		stereoMode string
	}{
		{[]string{"3d.hsbs", "half.sbs", "h-sbs", "hsbs"}, "half-sbs"},
		{[]string{"3d.sbs", "side.by.side", "sbs"}, "sbs"},
		{[]string{"3d.htab", "half.tab", "h-tab", "htab"}, "half-tab"},
		{[]string{"3d.tab", "top.and.bottom", "tab"}, "tab"},
		{[]string{"3d.mvc", "mvc"}, "mvc"},
	}

	for _, p := range patterns {
		for _, keyword := range p.keywords {
			if strings.Contains(lower, keyword) {
				return true, p.stereoMode
			}
		}
	}

	// Generic 3D check (no specific stereo mode detected)
	if strings.Contains(lower, "3d") || strings.Contains(lower, ".3d.") {
		return true, ""
	}

	return false, ""
}

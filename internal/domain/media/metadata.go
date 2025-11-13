package media

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
func CalculateResolutionLabel(height int) string {
	resolutions := []struct {
		minHeight int
		label     string
	}{
		{2160, "4K"},
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
	// TODO: Implement source type detection from filename patterns
	return ""
}

// Detect3D detects if media is 3D and determines stereo mode from filename
// Returns (is3D bool, stereoMode string)
// This is domain logic - how we identify 3D content
func Detect3D(filename string) (bool, string) {
	// TODO: Implement 3D detection from filename patterns
	return false, ""
}

// Package ffmpeg provides validation for FFmpeg command arguments
package ffmpeg

import (
	"fmt"
	"strings"
)

// ValidateArgs validates FFmpeg arguments before execution
func ValidateArgs(args []string) error {
	// Check for known problematic argument combinations
	for i := 0; i < len(args); i++ {
		arg := args[i]
		
		// Check for ambiguous -profile usage
		if arg == "-profile" && i+1 < len(args) {
			// -profile without :v or :a is ambiguous
			nextArg := args[i+1]
			// Check if this is a DASH/HLS profile option that FFmpeg might misinterpret
			if nextArg == "onDemand" || nextArg == "live" {
				return fmt.Errorf("invalid -profile value '%s': use -manifest_type instead for DASH manifests", nextArg)
			}
			// Ensure it's properly qualified
			return fmt.Errorf("ambiguous -profile: use -profile:v or -profile:a to specify video or audio profile")
		}
		
		// Check for -profile:v with invalid values
		if arg == "-profile:v" && i+1 < len(args) {
			validProfiles := []string{"baseline", "main", "high", "high10", "high422", "high444"}
			nextArg := args[i+1]
			isValid := false
			for _, valid := range validProfiles {
				if nextArg == valid {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid video profile '%s': valid options are %v", nextArg, validProfiles)
			}
		}
		
		// Check for multiple -vf/-filter:v options (FFmpeg only uses the last one)
		if arg == "-vf" || arg == "-filter:v" || strings.HasPrefix(arg, "-filter:v:") || strings.HasPrefix(arg, "-vf:") {
			// Count occurrences for the same stream
			streamID := extractStreamID(arg)
			count := 0
			for j := 0; j < len(args); j++ {
				if j != i {
					otherArg := args[j]
					if (otherArg == "-vf" || otherArg == "-filter:v" || strings.HasPrefix(otherArg, "-filter:v:") || strings.HasPrefix(otherArg, "-vf:")) {
						otherStreamID := extractStreamID(otherArg)
						if streamID == otherStreamID {
							count++
						}
					}
				}
			}
			if count > 0 {
				return fmt.Errorf("multiple video filter options specified for stream %s: FFmpeg will only use the last one", streamID)
			}
		}
		
		// Check for invalid DASH options
		if arg == "-base_url" {
			return fmt.Errorf("invalid option '-base_url': this is not a valid FFmpeg option for DASH")
		}
		
		// Check for manifest_type with invalid values
		if arg == "-manifest_type" && i+1 < len(args) {
			validTypes := []string{"vod", "live"}
			nextArg := args[i+1]
			isValid := false
			for _, valid := range validTypes {
				if nextArg == valid {
					isValid = true
					break
				}
			}
			if !isValid {
				return fmt.Errorf("invalid manifest_type '%s': valid options are %v", nextArg, validTypes)
			}
		}
		
		// Hardware acceleration checks
		if arg == "-hwaccel" && i+1 < len(args) {
			hwaccelType := args[i+1]
			// auto is valid and will gracefully fall back
			if hwaccelType != "auto" {
				// Validate specific hwaccel types
				validTypes := []string{"auto", "cuda", "nvdec", "vaapi", "qsv", "videotoolbox", "d3d11va", "dxva2"}
				isValid := false
				for _, valid := range validTypes {
					if hwaccelType == valid {
						isValid = true
						break
					}
				}
				if !isValid {
					return fmt.Errorf("invalid hwaccel type '%s': valid options are %v", hwaccelType, validTypes)
				}
			}
		}
	}
	
	// Check for required arguments
	hasInput := false
	hasOutput := false
	hasFormat := false
	
	for i := 0; i < len(args); i++ {
		if args[i] == "-i" {
			hasInput = true
		}
		if args[i] == "-f" {
			hasFormat = true
		}
		// The output is typically the last argument
		if i == len(args)-1 && !strings.HasPrefix(args[i], "-") {
			hasOutput = true
		}
	}
	
	if !hasInput {
		return fmt.Errorf("missing input file: -i option is required")
	}
	if !hasOutput {
		return fmt.Errorf("missing output file: output path must be specified")
	}
	if !hasFormat {
		return fmt.Errorf("missing format: -f option is required")
	}
	
	return nil
}

// extractStreamID extracts the stream ID from filter arguments
func extractStreamID(arg string) string {
	// Extract stream ID from arguments like "-vf:2" or "-filter:v:2"
	parts := strings.Split(arg, ":")
	if len(parts) > 2 {
		return parts[2]
	} else if len(parts) > 1 && parts[1] != "v" && parts[1] != "a" {
		return parts[1]
	}
	return "0" // Default stream
}

// SanitizeArgs removes or fixes known problematic arguments
func SanitizeArgs(args []string) []string {
	sanitized := make([]string, 0, len(args))
	
	for i := 0; i < len(args); i++ {
		arg := args[i]
		
		// Skip known problematic arguments
		if arg == "-base_url" {
			i++ // Skip the next argument too
			continue
		}
		
		// Fix ambiguous -profile
		if arg == "-profile" && i+1 < len(args) {
			// Convert to -profile:v by default
			sanitized = append(sanitized, "-profile:v")
			continue
		}
		
		sanitized = append(sanitized, arg)
	}
	
	return sanitized
}
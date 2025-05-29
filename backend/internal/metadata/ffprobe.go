package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FFprobe availability cache
var (
	ffprobeAvailable     *bool
	ffprobeCheckTime     time.Time
	ffprobeCheckMutex    sync.RWMutex
	ffprobeCheckInterval = 5 * time.Minute // Cache for 5 minutes
	
	// Debug flag - set to false to reduce logging in production
	FFProbeDebugLogging = false
)

func debugLog(format string, args ...interface{}) {
	if FFProbeDebugLogging {
		fmt.Printf(format, args...)
	}
}

// FFProbeOutput represents the JSON output from ffprobe
type FFProbeOutput struct {
	Format FFProbeFormat `json:"format"`
	Streams []FFProbeStream `json:"streams"`
}

type FFProbeFormat struct {
	Filename       string            `json:"filename"`
	NbStreams      int               `json:"nb_streams"`
	NbPrograms     int               `json:"nb_programs"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	StartTime      string            `json:"start_time"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	ProbeScore     int               `json:"probe_score"`
	Tags           map[string]string `json:"tags"`
}

type FFProbeStream struct {
	Index              int               `json:"index"`
	CodecName          string            `json:"codec_name"`
	CodecLongName      string            `json:"codec_long_name"`
	Profile            string            `json:"profile"`
	CodecType          string            `json:"codec_type"`
	CodecTagString     string            `json:"codec_tag_string"`
	CodecTag           string            `json:"codec_tag"`
	SampleFmt          string            `json:"sample_fmt"`
	SampleRate         string            `json:"sample_rate"`
	Channels           int               `json:"channels"`
	ChannelLayout      string            `json:"channel_layout"`
	BitsPerSample      int               `json:"bits_per_sample"`
	RFrameRate         string            `json:"r_frame_rate"`
	AvgFrameRate       string            `json:"avg_frame_rate"`
	TimeBase           string            `json:"time_base"`
	StartPts           int               `json:"start_pts"`
	StartTime          string            `json:"start_time"`
	DurationTs         int64             `json:"duration_ts"`
	Duration           string            `json:"duration"`
	BitRate            string            `json:"bit_rate"`
	MaxBitRate         string            `json:"max_bit_rate"`
	BitsPerRawSample   string            `json:"bits_per_raw_sample"`
	NbFrames           string            `json:"nb_frames"`
	Tags               map[string]string `json:"tags"`
}

// AudioTechnicalInfo represents technical audio information extracted from ffprobe
type AudioTechnicalInfo struct {
	Format      string  // File format (flac, mp3, ogg, etc.)
	Bitrate     int     // Bitrate in bits per second
	SampleRate  int     // Sample rate in Hz
	Channels    int     // Number of channels
	Duration    float64 // Duration in seconds
	Codec       string  // Audio codec
	IsLossless  bool    // Whether the format is lossless
}

// ExtractAudioTechnicalInfo uses ffprobe to extract technical audio information
func ExtractAudioTechnicalInfo(filePath string) (*AudioTechnicalInfo, error) {
	// Run ffprobe command with better error handling
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath)

	debugLog("DEBUG: Running ffprobe on: %s\n", filePath)
	
	output, err := cmd.Output()
	if err != nil {
		// Get more detailed error information
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := string(exitError.Stderr)
			fmt.Printf("ERROR: ffprobe failed for %s - Exit code: %d, Stderr: %s\n", filePath, exitError.ExitCode(), stderr)
			return nil, fmt.Errorf("ffprobe command failed with exit code %d: %s - stderr: %s", exitError.ExitCode(), err, stderr)
		}
		fmt.Printf("ERROR: ffprobe command execution failed for %s: %v\n", filePath, err)
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	debugLog("DEBUG: ffprobe output length: %d bytes\n", len(output))

	// Parse JSON output
	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		fmt.Printf("ERROR: Failed to parse ffprobe JSON output for %s: %v\n", filePath, err)
		debugLog("DEBUG: Raw ffprobe output: %s\n", string(output)[:min(500, len(output))])
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	debugLog("DEBUG: Parsed ffprobe output - Format: %s, Streams: %d\n", probeOutput.Format.FormatName, len(probeOutput.Streams))

	// Find the first audio stream
	var audioStream *FFProbeStream
	for i := range probeOutput.Streams {
		if probeOutput.Streams[i].CodecType == "audio" {
			audioStream = &probeOutput.Streams[i]
			debugLog("DEBUG: Found audio stream - Codec: %s, Channels: %d, Sample Rate: %s, Bitrate: %s\n", 
				audioStream.CodecName, audioStream.Channels, audioStream.SampleRate, audioStream.BitRate)
			break
		}
	}

	if audioStream == nil {
		fmt.Printf("WARNING: No audio stream found in file %s\n", filePath)
		return nil, fmt.Errorf("no audio stream found in file")
	}

	// Extract technical information
	info := &AudioTechnicalInfo{}

	// Determine format from format_name (prioritize this over file extension)
	info.Format = determineAudioFormat(probeOutput.Format.FormatName, filePath)
	debugLog("DEBUG: Determined format: %s (from ffprobe: %s)\n", info.Format, probeOutput.Format.FormatName)

	// Extract bitrate (prefer stream bitrate over format bitrate)
	if audioStream.BitRate != "" {
		if bitrate, err := strconv.Atoi(audioStream.BitRate); err == nil {
			info.Bitrate = bitrate
		} else {
			debugLog("WARNING: Failed to parse stream bitrate '%s': %v\n", audioStream.BitRate, err)
		}
	} else if probeOutput.Format.BitRate != "" {
		if bitrate, err := strconv.Atoi(probeOutput.Format.BitRate); err == nil {
			info.Bitrate = bitrate
		} else {
			debugLog("WARNING: Failed to parse format bitrate '%s': %v\n", probeOutput.Format.BitRate, err)
		}
	}

	// Extract sample rate
	if audioStream.SampleRate != "" {
		if sampleRate, err := strconv.Atoi(audioStream.SampleRate); err == nil {
			info.SampleRate = sampleRate
		} else {
			debugLog("WARNING: Failed to parse sample rate '%s': %v\n", audioStream.SampleRate, err)
		}
	}

	// Extract channels
	info.Channels = audioStream.Channels

	// Extract duration
	if probeOutput.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probeOutput.Format.Duration, 64); err == nil {
			info.Duration = duration
		} else {
			debugLog("WARNING: Failed to parse duration '%s': %v\n", probeOutput.Format.Duration, err)
		}
	}

	// Extract codec
	info.Codec = audioStream.CodecName

	// Determine if format is lossless
	info.IsLossless = isLosslessFormat(info.Format, info.Codec)

	debugLog("SUCCESS: FFprobe extraction complete for %s - Format: %s, Bitrate: %d, SampleRate: %d, Channels: %d\n", 
		filePath, info.Format, info.Bitrate, info.SampleRate, info.Channels)

	return info, nil
}

// min function for Go versions that don't have it built-in
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// determineAudioFormat maps ffprobe format names to our standardized format names
func determineAudioFormat(formatName, filePath string) string {
	// FFprobe format_name examples:
	// "flac" -> "flac"
	// "mp3" -> "mp3"
	// "ogg" -> "ogg"
	// "wav" -> "wav"
	// "mp4,m4a,3gp,3g2,mj2" -> "m4a" (for .m4a files)

	formatName = strings.ToLower(formatName)

	// Handle direct matches
	if strings.Contains(formatName, "flac") {
		return "flac"
	}
	if strings.Contains(formatName, "mp3") {
		return "mp3"
	}
	if strings.Contains(formatName, "ogg") {
		return "ogg"
	}
	if strings.Contains(formatName, "wav") {
		return "wav"
	}

	// Handle M4A/AAC (mp4 container with .m4a extension)
	if strings.Contains(formatName, "mp4") || strings.Contains(formatName, "m4a") {
		if strings.HasSuffix(strings.ToLower(filePath), ".m4a") {
			return "m4a"
		}
		return "mp4"
	}

	// Handle other formats
	if strings.Contains(formatName, "aac") {
		return "aac"
	}
	if strings.Contains(formatName, "opus") {
		return "opus"
	}
	if strings.Contains(formatName, "aiff") {
		return "aiff"
	}

	// Fallback to file extension if format name is unclear
	if strings.Contains(filePath, ".") {
		ext := strings.ToLower(filePath[strings.LastIndex(filePath, ".")+1:])
		return ext
	}

	return "unknown"
}

// isLosslessFormat determines if a format/codec combination is lossless
func isLosslessFormat(format, codec string) bool {
	format = strings.ToLower(format)
	codec = strings.ToLower(codec)

	// Known lossless formats
	losslessFormats := map[string]bool{
		"flac": true,
		"wav":  true,
		"aiff": true,
		"ape":  true,
		"wv":   true, // WavPack
	}

	// Known lossless codecs
	losslessCodecs := map[string]bool{
		"flac":    true,
		"pcm_s16le": true,
		"pcm_s24le": true,
		"pcm_s32le": true,
		"pcm_f32le": true,
		"pcm_f64le": true,
		"alac":    true, // Apple Lossless
		"ape":     true,
		"wavpack": true,
	}

	return losslessFormats[format] || losslessCodecs[codec]
}

// IsFFProbeAvailable checks if ffprobe is available on the system (with caching)
func IsFFProbeAvailable() bool {
	ffprobeCheckMutex.RLock()
	if ffprobeAvailable != nil && time.Since(ffprobeCheckTime) < ffprobeCheckInterval {
		result := *ffprobeAvailable
		ffprobeCheckMutex.RUnlock()
		return result
	}
	ffprobeCheckMutex.RUnlock()

	ffprobeCheckMutex.Lock()
	defer ffprobeCheckMutex.Unlock()

	// Double-check in case another goroutine updated it while we waited for the lock
	if ffprobeAvailable != nil && time.Since(ffprobeCheckTime) < ffprobeCheckInterval {
		return *ffprobeAvailable
	}
	
	cmd := exec.Command("ffprobe", "-version")
	output, err := cmd.Output()
	
	available := false
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr := string(exitError.Stderr)
			debugLog("ERROR: ffprobe version check failed - Exit code: %d, Stderr: %s\n", exitError.ExitCode(), stderr)
		} else {
			debugLog("ERROR: ffprobe not found or not executable: %v\n", err)
		}
	} else {
		available = true
		// Parse version information from output
		versionOutput := string(output)
		lines := strings.Split(versionOutput, "\n")
		if len(lines) > 0 {
			debugLog("SUCCESS: ffprobe is available - %s\n", strings.TrimSpace(lines[0]))
		} else {
			debugLog("SUCCESS: ffprobe is available\n")
		}
	}
	
	// Cache the result
	ffprobeAvailable = &available
	ffprobeCheckTime = time.Now()
	
	return available
}

// SetDebugLogging enables or disables debug logging for FFprobe operations
func SetDebugLogging(enabled bool) {
	FFProbeDebugLogging = enabled
}

// InitializeFFProbeLogging checks environment variables and sets appropriate logging level
func InitializeFFProbeLogging() {
	// Check for production environment
	if env := os.Getenv("APP_ENV"); env == "production" || env == "prod" {
		FFProbeDebugLogging = false
	}
	
	// Allow explicit override
	if debugEnv := os.Getenv("FFPROBE_DEBUG"); debugEnv == "false" || debugEnv == "0" {
		FFProbeDebugLogging = false
	} else if debugEnv == "true" || debugEnv == "1" {
		FFProbeDebugLogging = true
	}
} 
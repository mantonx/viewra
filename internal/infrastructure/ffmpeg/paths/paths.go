// Package paths provides FFmpeg binary path resolution and command preparation.
//
// This package handles:
//   - FFmpeg/FFprobe binary discovery (environment or PATH)
//   - Custom library path configuration for patched builds
//   - Command preparation with proper environment setup
package paths

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Paths holds FFmpeg and FFprobe binary paths with optional library path.
// This is the single source of truth for FFmpeg configuration across the application.
type Paths struct {
	FFmpeg  string // Path to FFmpeg binary
	FFprobe string // Path to FFprobe binary
	LibPath string // LD_LIBRARY_PATH for custom builds (empty for system FFmpeg)
}

// New resolves FFmpeg/FFprobe paths from environment or system PATH.
// Priority: VIEWRA_FFMPEG_PATH > system PATH
func New() (*Paths, error) {
	p := &Paths{}

	// FFmpeg path
	p.FFmpeg = os.Getenv("VIEWRA_FFMPEG_PATH")
	if p.FFmpeg == "" {
		path, err := exec.LookPath("ffmpeg")
		if err != nil {
			return nil, fmt.Errorf("ffmpeg not found in PATH and VIEWRA_FFMPEG_PATH not set: %w", err)
		}
		p.FFmpeg = path
	}

	// FFprobe path
	p.FFprobe = os.Getenv("VIEWRA_FFPROBE_PATH")
	if p.FFprobe == "" {
		path, err := exec.LookPath("ffprobe")
		if err != nil {
			return nil, fmt.Errorf("ffprobe not found in PATH and VIEWRA_FFPROBE_PATH not set: %w", err)
		}
		p.FFprobe = path
	}

	// Library path (optional - only needed for custom builds)
	p.LibPath = os.Getenv("VIEWRA_FFMPEG_LIB_PATH")

	return p, nil
}

// IsCustomBuild returns true if using a custom FFmpeg build (has lib path).
func (p *Paths) IsCustomBuild() bool {
	return p.LibPath != ""
}

// Version returns the FFmpeg version string.
func (p *Paths) Version() string {
	cmd := exec.Command(p.FFmpeg, "-version")
	if p.LibPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+p.LibPath)
	}

	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "ffmpeg version ") {
		parts := strings.Fields(lines[0])
		if len(parts) >= 3 {
			return parts[2]
		}
	}
	return "unknown"
}

// PrepareCommand creates an exec.Cmd with LD_LIBRARY_PATH set if needed.
// The name should be "ffmpeg" or "ffprobe" to use the configured paths.
func (p *Paths) PrepareCommand(name string, args ...string) *exec.Cmd {
	var binary string
	switch name {
	case "ffmpeg":
		binary = p.FFmpeg
	case "ffprobe":
		binary = p.FFprobe
	default:
		binary = name
	}

	cmd := exec.Command(binary, args...)
	if p.LibPath != "" {
		cmd.Env = append(os.Environ(), "LD_LIBRARY_PATH="+p.LibPath)
	}
	return cmd
}

// Global singleton for package-level functions.
// This is initialized once via SetGlobal and accessed by CheckFFmpegEncoder/CheckFFmpegFilter.
var (
	globalPaths *Paths
	globalMu    sync.RWMutex
)

// SetGlobal sets the global FFmpeg paths for use by package-level functions.
// This should be called once during application startup after paths are resolved.
// Thread-safe: can be called concurrently with Global().
func SetGlobal(p *Paths) {
	globalMu.Lock()
	globalPaths = p
	globalMu.Unlock()
}

// Global returns the global FFmpeg paths, or nil if not set.
// Thread-safe: can be called concurrently with SetGlobal().
func Global() *Paths {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalPaths
}

// CheckFFmpegEncoder verifies that FFmpeg has support for a specific encoder.
// Uses the global paths set via SetGlobal, or falls back to system "ffmpeg".
func CheckFFmpegEncoder(encoderName string) bool {
	cmd := prepareGlobalCommand("-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return bytes.Contains(output, []byte(encoderName))
}

// CheckFFmpegFilter verifies that FFmpeg has support for a specific filter.
// Uses the global paths set via SetGlobal, or falls back to system "ffmpeg".
func CheckFFmpegFilter(filterName string) bool {
	cmd := prepareGlobalCommand("-filters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return bytes.Contains(output, []byte(filterName))
}

// prepareGlobalCommand creates an FFmpeg command using global paths or fallback.
func prepareGlobalCommand(args ...string) *exec.Cmd {
	p := Global()
	if p != nil {
		return p.PrepareCommand("ffmpeg", args...)
	}
	// Fallback to system ffmpeg
	return exec.Command("ffmpeg", args...)
}

package transcoding

import (
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/ffmpeg"
)

// ProcessManager re-export for tests
type ProcessManager = ffmpeg.ProcessManager

// NewProcessManager creates a process manager - re-exported for test compatibility
// Accepts the parent package's TranscodeConfig and converts it
func NewProcessManager(config *TranscodeConfig) *ProcessManager {
	return ffmpeg.NewProcessManager(convertToFFmpegConfig(config))
}

// createTestVideoInfo creates a VideoInfo for testing tone mapping filters
func createTestVideoInfo(codec string, width, height int, isHDR bool) *VideoInfo {
	return &VideoInfo{
		Codec:  codec,
		Width:  width,
		Height: height,
		IsHDR:  isHDR,
	}
}

// argsContain checks if the args slice contains a flag with the expected value
func argsContain(args []string, flag, expectedValue string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == expectedValue {
			return true
		}
	}
	return false
}

// argsContainFlag checks if the args slice contains a specific flag (without checking value)
func argsContainFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

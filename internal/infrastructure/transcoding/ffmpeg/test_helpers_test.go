package ffmpeg

// createTestProfile creates a minimal AdaptiveProfile for testing
func createTestProfile() *AdaptiveProfile {
	return &AdaptiveProfile{
		Width:        1920,
		Height:       1080,
		VideoBitrate: 8_000_000,
		VideoMaxRate: 8_800_000,
		VideoBufSize: 16_000_000,
		GOPSize:      48,
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

// createTestVideoInfo creates a VideoInfo for testing tone mapping filters
func createTestVideoInfo(codec string, width, height int, isHDR bool) *VideoInfo {
	return &VideoInfo{
		Width:  width,
		Height: height,
		IsHDR:  isHDR,
	}
}

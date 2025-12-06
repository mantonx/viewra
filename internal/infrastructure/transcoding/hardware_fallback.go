package transcoding

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// HardwareFallbackManager handles automatic fallback from hardware to software encoding
// when hardware encoding fails or is unavailable.
type HardwareFallbackManager struct {
	config        *TranscodeConfig
	logger        *slog.Logger
	mu            sync.Mutex // Protects failureCount map
	failureCount  map[HardwareAccel]int
	maxRetries    int
	fallbackChain []HardwareAccel
	testEncoder   *HardwareTestEncoder
}

// NewHardwareFallbackManager creates a new fallback manager with default retry logic.
func NewHardwareFallbackManager(config *TranscodeConfig, logger *slog.Logger) *HardwareFallbackManager {
	return &HardwareFallbackManager{
		config:       config,
		logger:       logger,
		failureCount: make(map[HardwareAccel]int),
		maxRetries:   2, // Allow 2 failures before fallback
		fallbackChain: []HardwareAccel{
			config.HardwareAccel, // Current hardware
			AccelNone,            // Fallback to software
		},
		testEncoder: NewHardwareTestEncoder(),
	}
}

// GetCurrentAccel returns the currently active hardware acceleration method.
func (m *HardwareFallbackManager) GetCurrentAccel() HardwareAccel {
	return m.config.HardwareAccel
}

// RecordFailure records a hardware encoding failure and potentially triggers fallback.
// Returns true if fallback occurred.
func (m *HardwareFallbackManager) RecordFailure(hwAccel HardwareAccel, err error) bool {
	// Check if this is a hardware-related error
	if !m.isHardwareError(err) {
		return false
	}

	m.mu.Lock()
	m.failureCount[hwAccel]++
	count := m.failureCount[hwAccel]
	m.mu.Unlock()

	m.logger.Warn("Hardware encoding failure detected",
		"hardware", hwAccel,
		"failure_count", count,
		"error", err,
	)

	// Check if we should fallback
	if count >= m.maxRetries {
		return m.fallbackToNext(hwAccel)
	}

	return false
}

// fallbackToNext moves to the next acceleration method in the fallback chain.
func (m *HardwareFallbackManager) fallbackToNext(failed HardwareAccel) bool {
	// Find current position in fallback chain
	currentIdx := -1
	for i, accel := range m.fallbackChain {
		if accel == failed {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 || currentIdx >= len(m.fallbackChain)-1 {
		// Already at end of fallback chain (software encoding)
		return false
	}

	// Move to next in chain
	nextAccel := m.fallbackChain[currentIdx+1]
	m.logger.Warn("Falling back to next acceleration method",
		"from", failed,
		"to", nextAccel,
	)

	m.config.HardwareAccel = nextAccel
	return true
}

// isHardwareError checks if an error is related to hardware encoding failure.
func (m *HardwareFallbackManager) isHardwareError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Common hardware error patterns
	hardwareErrorPatterns := []string{
		"cuda",
		"nvenc",
		"qsv",
		"vaapi",
		"videotoolbox",
		"hardware",
		"hwaccel",
		"gpu",
		"device",
		"driver",
		"cannot load",
		"cannot open",
		"not found",
		"initialization failed",
	}

	for _, pattern := range hardwareErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// VerifyHardwareAvailability proactively tests if hardware acceleration is working.
// Returns an error if hardware is not available, allowing early fallback.
func (m *HardwareFallbackManager) VerifyHardwareAvailability(hwAccel HardwareAccel) error {
	if hwAccel == AccelNone {
		return nil // Software encoding always available
	}

	// Get the encoder name for this hardware acceleration
	encoder, _ := m.getEncoderForAccel(hwAccel)
	if encoder == "" {
		return fmt.Errorf("unknown hardware acceleration: %s", hwAccel)
	}

	// Check if FFmpeg has this encoder
	if !CheckFFmpegEncoder(encoder) {
		return fmt.Errorf("ffmpeg does not have %s encoder support", encoder)
	}

	// Perform a quick test encode to verify hardware is functional
	if err := m.testEncoder.TestEncode(hwAccel); err != nil {
		return fmt.Errorf("hardware test encode failed: %w", err)
	}

	m.logger.Info("Hardware acceleration verified", "hardware", hwAccel, "encoder", encoder)
	return nil
}

// getEncoderForAccel returns the FFmpeg encoder name for a hardware acceleration type.
func (m *HardwareFallbackManager) getEncoderForAccel(hwAccel HardwareAccel) (string, string) {
	// Use the unified hardware function
	return GetVideoCodecAndPreset(hwAccel)
}

// ResetFailureCount resets the failure count for successful encodes.
func (m *HardwareFallbackManager) ResetFailureCount(hwAccel HardwareAccel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failureCount[hwAccel] > 0 {
		m.logger.Debug("Resetting failure count", "hardware", hwAccel)
		delete(m.failureCount, hwAccel)
	}
}

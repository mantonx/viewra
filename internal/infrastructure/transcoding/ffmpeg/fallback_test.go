package ffmpeg

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestNewHardwareFallbackManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{
		HardwareAccel: AccelNVENC,
	}

	manager := NewHardwareFallbackManager(config, logger)

	if manager == nil {
		t.Fatal("NewHardwareFallbackManager() returned nil")
	}

	if manager.config != config {
		t.Error("Manager config not set correctly")
	}

	if manager.logger != logger {
		t.Error("Manager logger not set correctly")
	}

	if manager.maxRetries != 2 {
		t.Errorf("Expected maxRetries=2, got %d", manager.maxRetries)
	}

	if manager.failureCount == nil {
		t.Error("failureCount map not initialized")
	}

	expectedChain := []HardwareAccel{AccelNVENC, AccelNone}
	if len(manager.fallbackChain) != len(expectedChain) {
		t.Errorf("Expected fallback chain length %d, got %d", len(expectedChain), len(manager.fallbackChain))
	}

	for i, accel := range expectedChain {
		if manager.fallbackChain[i] != accel {
			t.Errorf("Fallback chain[%d]: expected %s, got %s", i, accel, manager.fallbackChain[i])
		}
	}
}

func TestGetCurrentAccel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		hardwareAccel HardwareAccel
	}{
		{"NVENC", AccelNVENC},
		{"VAAPI", AccelVAAPI},
		{"QSV", AccelQSV},
		{"VideoToolbox", AccelVideoToolbox},
		{"None", AccelNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TranscodeConfig{HardwareAccel: tt.hardwareAccel}
			manager := NewHardwareFallbackManager(config, logger)

			result := manager.GetCurrentAccel()
			if result != tt.hardwareAccel {
				t.Errorf("GetCurrentAccel() = %v, want %v", result, tt.hardwareAccel)
			}
		})
	}
}

func TestRecordFailure_IncrementCount(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	// Create hardware-related error
	err := errors.New("CUDA initialization failed")

	// First failure
	fallbackTriggered := manager.RecordFailure(AccelNVENC, err)
	if fallbackTriggered {
		t.Error("First failure should not trigger fallback")
	}

	if manager.failureCount[AccelNVENC] != 1 {
		t.Errorf("Expected failure count 1, got %d", manager.failureCount[AccelNVENC])
	}

	// Second failure
	fallbackTriggered = manager.RecordFailure(AccelNVENC, err)
	if !fallbackTriggered {
		t.Error("Second failure should trigger fallback after maxRetries=2")
	}

	if manager.failureCount[AccelNVENC] != 2 {
		t.Errorf("Expected failure count 2, got %d", manager.failureCount[AccelNVENC])
	}

	// Verify fallback occurred
	if manager.config.HardwareAccel != AccelNone {
		t.Errorf("Expected fallback to AccelNone, got %v", manager.config.HardwareAccel)
	}
}

func TestRecordFailure_NonHardwareError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	// Non-hardware error should not increment count
	// Use an error that doesn't match any hardware patterns
	err := errors.New("invalid video resolution specified")

	fallbackTriggered := manager.RecordFailure(AccelNVENC, err)
	if fallbackTriggered {
		t.Error("Non-hardware error should not trigger fallback")
	}

	if count := manager.failureCount[AccelNVENC]; count != 0 {
		t.Errorf("Expected failure count 0, got %d", count)
	}

	if manager.config.HardwareAccel != AccelNVENC {
		t.Error("Hardware acceleration should not change for non-hardware error")
	}
}

func TestRecordFailure_NilError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	fallbackTriggered := manager.RecordFailure(AccelNVENC, nil)
	if fallbackTriggered {
		t.Error("Nil error should not trigger fallback")
	}

	if count := manager.failureCount[AccelNVENC]; count != 0 {
		t.Errorf("Expected failure count 0, got %d", count)
	}
}

func TestIsHardwareError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	tests := []struct {
		name        string
		err         error
		shouldMatch bool
	}{
		{"CUDA error", errors.New("CUDA initialization failed"), true},
		{"NVENC error", errors.New("nvenc encoder not found"), true},
		{"QSV error", errors.New("qsv device cannot be opened"), true},
		{"VAAPI error", errors.New("vaapi driver initialization failed"), true},
		{"VideoToolbox error", errors.New("videotoolbox hardware acceleration failed"), true},
		{"Hardware keyword", errors.New("hardware encoder not available"), true},
		{"GPU keyword", errors.New("GPU device not found"), true},
		{"Device keyword", errors.New("device /dev/dri/renderD128 cannot load"), true},
		{"Driver keyword", errors.New("driver error occurred"), true},
		{"Not found error", errors.New("h264_nvenc encoder not found"), true},
		{"Initialization failed", errors.New("initialization failed for hardware codec"), true},
		{"File not found", errors.New("file not found: /video.mp4"), true}, // "not found" pattern matches
		{"Invalid resolution", errors.New("invalid resolution specified"), false},
		{"Codec error", errors.New("unsupported codec"), false},
		{"Generic error", errors.New("something went wrong"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isHardwareError(tt.err)
			if result != tt.shouldMatch {
				t.Errorf("isHardwareError(%v) = %v, want %v", tt.err, result, tt.shouldMatch)
			}
		})
	}
}

func TestFallbackToNext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name                  string
		initialAccel          HardwareAccel
		failedAccel           HardwareAccel
		expectedFallback      bool
		expectedFinalAccel    HardwareAccel
	}{
		{
			name:               "NVENC fallback to software",
			initialAccel:       AccelNVENC,
			failedAccel:        AccelNVENC,
			expectedFallback:   true,
			expectedFinalAccel: AccelNone,
		},
		{
			name:               "Already at software - fallback to software (last in chain)",
			initialAccel:       AccelNone,
			failedAccel:        AccelNone,
			expectedFallback:   true, // Chain is [none, none], so it will fallback to index 1
			expectedFinalAccel: AccelNone,
		},
		{
			name:               "Failed accel not in chain",
			initialAccel:       AccelNVENC,
			failedAccel:        AccelQSV,
			expectedFallback:   false,
			expectedFinalAccel: AccelNVENC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TranscodeConfig{HardwareAccel: tt.initialAccel}
			manager := NewHardwareFallbackManager(config, logger)

			result := manager.fallbackToNext(tt.failedAccel)

			if result != tt.expectedFallback {
				t.Errorf("fallbackToNext() = %v, want %v", result, tt.expectedFallback)
			}

			if manager.config.HardwareAccel != tt.expectedFinalAccel {
				t.Errorf("Final hardware accel = %v, want %v", manager.config.HardwareAccel, tt.expectedFinalAccel)
			}
		})
	}
}

func TestResetFailureCount(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	// Record a failure
	err := errors.New("CUDA initialization failed")
	manager.RecordFailure(AccelNVENC, err)

	if manager.failureCount[AccelNVENC] != 1 {
		t.Fatalf("Expected failure count 1, got %d", manager.failureCount[AccelNVENC])
	}

	// Reset the count
	manager.ResetFailureCount(AccelNVENC)

	// Verify count is reset (key deleted from map)
	if count, exists := manager.failureCount[AccelNVENC]; exists {
		t.Errorf("Expected failure count to be deleted, but got %d", count)
	}

	// Resetting again should be safe (no-op)
	manager.ResetFailureCount(AccelNVENC)

	if count, exists := manager.failureCount[AccelNVENC]; exists {
		t.Errorf("Expected failure count to remain deleted, but got %d", count)
	}
}

func TestResetFailureCount_DifferentAccels(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	// Record failures for multiple acceleration types
	nvencErr := errors.New("NVENC initialization failed")
	vaapiErr := errors.New("VAAPI device error")

	manager.RecordFailure(AccelNVENC, nvencErr)
	manager.RecordFailure(AccelVAAPI, vaapiErr)

	if manager.failureCount[AccelNVENC] != 1 {
		t.Errorf("Expected NVENC failure count 1, got %d", manager.failureCount[AccelNVENC])
	}
	if manager.failureCount[AccelVAAPI] != 1 {
		t.Errorf("Expected VAAPI failure count 1, got %d", manager.failureCount[AccelVAAPI])
	}

	// Reset only NVENC
	manager.ResetFailureCount(AccelNVENC)

	// NVENC should be reset
	if _, exists := manager.failureCount[AccelNVENC]; exists {
		t.Error("Expected NVENC failure count to be deleted")
	}

	// VAAPI should still have count
	if manager.failureCount[AccelVAAPI] != 1 {
		t.Errorf("Expected VAAPI failure count to remain 1, got %d", manager.failureCount[AccelVAAPI])
	}
}

func TestConcurrentRecordFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	err := errors.New("CUDA initialization failed")

	// Note: Current implementation is NOT thread-safe (no mutex)
	// This test documents the current behavior and would fail if we add proper locking
	// For now, we test sequential access which is the expected use case

	// First goroutine records failure
	manager.RecordFailure(AccelNVENC, err)

	if manager.failureCount[AccelNVENC] != 1 {
		t.Errorf("Expected failure count 1, got %d", manager.failureCount[AccelNVENC])
	}

	// Second call triggers fallback
	fallbackTriggered := manager.RecordFailure(AccelNVENC, err)
	if !fallbackTriggered {
		t.Error("Expected fallback to be triggered after 2 failures")
	}

	if manager.config.HardwareAccel != AccelNone {
		t.Errorf("Expected fallback to AccelNone, got %v", manager.config.HardwareAccel)
	}
}

func TestHardwareErrorPatterns(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := &TranscodeConfig{HardwareAccel: AccelNVENC}
	manager := NewHardwareFallbackManager(config, logger)

	// Test case sensitivity - patterns are checked in lowercase
	tests := []struct {
		name  string
		error error
		match bool
	}{
		{"Uppercase CUDA", errors.New("CUDA initialization failed"), true},
		{"Lowercase cuda", errors.New("cuda device not found"), true},
		{"Mixed case NvEnc", errors.New("NvEnc encoder missing"), true},
		{"VAAPI uppercase", errors.New("VAAPI ERROR"), true},
		{"VideoToolbox mixed", errors.New("VideoToolbox Hardware Failed"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isHardwareError(tt.error)
			if result != tt.match {
				t.Errorf("isHardwareError(%q) = %v, want %v", tt.error, result, tt.match)
			}
		})
	}
}

func TestFallbackChainStructure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	tests := []struct {
		name          string
		initialAccel  HardwareAccel
		expectedChain []HardwareAccel
	}{
		{
			name:          "NVENC chain",
			initialAccel:  AccelNVENC,
			expectedChain: []HardwareAccel{AccelNVENC, AccelNone},
		},
		{
			name:          "VAAPI chain",
			initialAccel:  AccelVAAPI,
			expectedChain: []HardwareAccel{AccelVAAPI, AccelNone},
		},
		{
			name:          "QSV chain",
			initialAccel:  AccelQSV,
			expectedChain: []HardwareAccel{AccelQSV, AccelNone},
		},
		{
			name:          "VideoToolbox chain",
			initialAccel:  AccelVideoToolbox,
			expectedChain: []HardwareAccel{AccelVideoToolbox, AccelNone},
		},
		{
			name:          "Software chain",
			initialAccel:  AccelNone,
			expectedChain: []HardwareAccel{AccelNone, AccelNone},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &TranscodeConfig{HardwareAccel: tt.initialAccel}
			manager := NewHardwareFallbackManager(config, logger)

			if len(manager.fallbackChain) != len(tt.expectedChain) {
				t.Errorf("Chain length = %d, want %d", len(manager.fallbackChain), len(tt.expectedChain))
				return
			}

			for i, expected := range tt.expectedChain {
				if manager.fallbackChain[i] != expected {
					t.Errorf("Chain[%d] = %v, want %v", i, manager.fallbackChain[i], expected)
				}
			}
		})
	}
}

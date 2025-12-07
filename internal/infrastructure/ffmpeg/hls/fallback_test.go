package hls

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

func TestNewHardwareFallbackManager(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	if mgr == nil {
		t.Fatal("NewHardwareFallbackManager returned nil")
	}
	if mgr.config != config {
		t.Error("config not set correctly")
	}
	if mgr.logger != logger {
		t.Error("logger not set correctly")
	}
	if mgr.maxRetries != 2 {
		t.Errorf("maxRetries = %d, want 2", mgr.maxRetries)
	}
	if len(mgr.fallbackChain) != 2 {
		t.Errorf("fallbackChain length = %d, want 2", len(mgr.fallbackChain))
	}
	if mgr.fallbackChain[0] != AccelNVENC {
		t.Errorf("fallbackChain[0] = %s, want %s", mgr.fallbackChain[0], AccelNVENC)
	}
	if mgr.fallbackChain[1] != AccelNone {
		t.Errorf("fallbackChain[1] = %s, want %s", mgr.fallbackChain[1], AccelNone)
	}
}

func TestGetCurrentAccel(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelVAAPI,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)
	got := mgr.GetCurrentAccel()

	if got != AccelVAAPI {
		t.Errorf("GetCurrentAccel() = %s, want %s", got, AccelVAAPI)
	}
}

func TestRecordFailureNonHardwareError(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Non-hardware error should not trigger fallback
	err := errors.New("invalid argument")
	fallback := mgr.RecordFailure(AccelNVENC, err)

	if fallback {
		t.Error("Non-hardware error should not trigger fallback")
	}
	if mgr.failureCount[AccelNVENC] != 0 {
		t.Error("Non-hardware error should not increment failure count")
	}
}

func TestRecordFailureHardwareError(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Hardware error should increment failure count
	err := errors.New("cuda error: initialization failed")
	fallback := mgr.RecordFailure(AccelNVENC, err)

	if fallback {
		t.Error("First failure should not trigger fallback")
	}
	if mgr.failureCount[AccelNVENC] != 1 {
		t.Errorf("Failure count = %d, want 1", mgr.failureCount[AccelNVENC])
	}
}

func TestRecordFailureTriggersFallback(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Record two failures to trigger fallback
	err := errors.New("nvenc initialization failed")
	mgr.RecordFailure(AccelNVENC, err)
	fallback := mgr.RecordFailure(AccelNVENC, err)

	if !fallback {
		t.Error("Second failure should trigger fallback")
	}
	if mgr.config.HardwareAccel != AccelNone {
		t.Errorf("Should fallback to software, got %s", mgr.config.HardwareAccel)
	}
}

func TestIsHardwareError(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"cuda error", errors.New("cuda device error"), true},
		{"nvenc error", errors.New("nvenc encoder failed"), true},
		{"qsv error", errors.New("qsv initialization failed"), true},
		{"vaapi error", errors.New("vaapi device not found"), true},
		{"videotoolbox error", errors.New("videotoolbox cannot load"), true},
		{"hardware keyword", errors.New("hardware acceleration failed"), true},
		{"hwaccel keyword", errors.New("hwaccel not supported"), true},
		{"gpu keyword", errors.New("gpu driver error"), true},
		{"device keyword", errors.New("device cannot open"), true},
		{"driver keyword", errors.New("driver initialization failed"), true},
		{"non-hardware error", errors.New("invalid argument"), false},
		{"generic error", errors.New("invalid argument"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.isHardwareError(tt.err)
			if got != tt.want {
				t.Errorf("isHardwareError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestFallbackToNext(t *testing.T) {
	tests := []struct {
		name           string
		initialAccel   HardwareAccel
		failedAccel    HardwareAccel
		wantFallback   bool
		wantFinalAccel HardwareAccel
	}{
		{
			name:           "fallback from nvenc to software",
			initialAccel:   AccelNVENC,
			failedAccel:    AccelNVENC,
			wantFallback:   true,
			wantFinalAccel: AccelNone,
		},
		{
			name:           "software stays at end of chain",
			initialAccel:   AccelNone,
			failedAccel:    AccelNone,
			wantFallback:   true, // Returns true because it moves from position 0 to 1 in chain
			wantFinalAccel: AccelNone, // But still ends up at AccelNone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				HardwareAccel: tt.initialAccel,
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

			mgr := NewHardwareFallbackManager(config, logger)
			fallback := mgr.fallbackToNext(tt.failedAccel)

			if fallback != tt.wantFallback {
				t.Errorf("fallbackToNext() = %v, want %v", fallback, tt.wantFallback)
			}
			if mgr.config.HardwareAccel != tt.wantFinalAccel {
				t.Errorf("Final accel = %s, want %s", mgr.config.HardwareAccel, tt.wantFinalAccel)
			}
		})
	}
}

func TestGetEncoderForAccel(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	tests := []struct {
		hwAccel     HardwareAccel
		wantEncoder string
	}{
		{AccelNone, "libx264"},
		{AccelNVENC, "h264_nvenc"},
		{AccelVAAPI, "h264_vaapi"},
		{AccelQSV, "h264_qsv"},
		{AccelVideoToolbox, "h264_videotoolbox"},
	}

	for _, tt := range tests {
		t.Run(string(tt.hwAccel), func(t *testing.T) {
			encoder, _ := mgr.getEncoderForAccel(tt.hwAccel)
			if encoder != tt.wantEncoder {
				t.Errorf("getEncoderForAccel(%s) = %s, want %s", tt.hwAccel, encoder, tt.wantEncoder)
			}
		})
	}
}

func TestResetFailureCount(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Record a failure
	err := errors.New("nvenc error")
	mgr.RecordFailure(AccelNVENC, err)

	if mgr.failureCount[AccelNVENC] != 1 {
		t.Fatal("Failure count should be 1")
	}

	// Reset failure count
	mgr.ResetFailureCount(AccelNVENC)

	if mgr.failureCount[AccelNVENC] != 0 {
		t.Errorf("Failure count after reset = %d, want 0", mgr.failureCount[AccelNVENC])
	}
}

func TestVerifyHardwareAvailabilitySoftware(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNone,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Software encoding should always be available
	err := mgr.VerifyHardwareAvailability(AccelNone)
	if err != nil {
		t.Errorf("VerifyHardwareAvailability(AccelNone) should return nil, got %v", err)
	}
}

func TestVerifyHardwareAvailabilityNonExistentEncoder(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNone,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Test with a hardware type that will fail encoder check
	// NVENC will fail if ffmpeg doesn't have h264_nvenc
	err := mgr.VerifyHardwareAvailability(AccelNVENC)
	// This may pass or fail depending on system ffmpeg, so we just check it doesn't panic
	_ = err
}

func TestMultipleHardwareFailures(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Record multiple failures for NVENC
	err := errors.New("nvenc hardware error")
	for i := 0; i < 3; i++ {
		mgr.RecordFailure(AccelNVENC, err)
	}

	// Should have fallen back to software
	if mgr.config.HardwareAccel != AccelNone {
		t.Errorf("After multiple failures, should fallback to software, got %s", mgr.config.HardwareAccel)
	}
}

func TestFailureCountIsolated(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Record failures for different accelerators
	nvencErr := errors.New("nvenc error")
	vaapiErr := errors.New("vaapi error")

	mgr.RecordFailure(AccelNVENC, nvencErr)
	mgr.RecordFailure(AccelVAAPI, vaapiErr)

	// Failure counts should be isolated
	if mgr.failureCount[AccelNVENC] != 1 {
		t.Errorf("NVENC failure count = %d, want 1", mgr.failureCount[AccelNVENC])
	}
	if mgr.failureCount[AccelVAAPI] != 1 {
		t.Errorf("VAAPI failure count = %d, want 1", mgr.failureCount[AccelVAAPI])
	}
}

func TestHardwareErrorPatternMatching(t *testing.T) {
	config := &Config{
		HardwareAccel: AccelNVENC,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mgr := NewHardwareFallbackManager(config, logger)

	// Test case sensitivity (should be case-insensitive)
	err := errors.New("CUDA ERROR: INITIALIZATION FAILED")
	if !mgr.isHardwareError(err) {
		t.Error("Hardware error detection should be case-insensitive")
	}

	// Test multiple pattern matches
	err = errors.New("GPU device driver cannot load")
	if !mgr.isHardwareError(err) {
		t.Error("Should detect multiple hardware-related keywords")
	}
}

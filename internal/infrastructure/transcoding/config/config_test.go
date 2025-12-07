package config

import (
	"os"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Verify FFmpegPaths is set
	if cfg.FFmpegPaths == nil {
		t.Error("FFmpegPaths should not be nil")
	}

	// Verify hardware acceleration is set
	if cfg.HardwareAccel == "" {
		t.Error("HardwareAccel should not be empty")
	}

	// Verify reasonable defaults
	if cfg.MinFreeDiskGB != 10 {
		t.Errorf("MinFreeDiskGB = %d, want 10", cfg.MinFreeDiskGB)
	}

	if cfg.MaxCPUPercent != 0 {
		t.Errorf("MaxCPUPercent = %d, want 0 (unlimited)", cfg.MaxCPUPercent)
	}

	if !cfg.ProcessGroupKill {
		t.Error("ProcessGroupKill should be true by default")
	}

	if !cfg.ToneMappingEnabled {
		t.Error("ToneMappingEnabled should be true by default")
	}

	if cfg.ToneMappingAlgorithm != "bt.2390" {
		t.Errorf("ToneMappingAlgorithm = %s, want bt.2390", cfg.ToneMappingAlgorithm)
	}

	if cfg.ToneMappingBackend != "auto" {
		t.Errorf("ToneMappingBackend = %s, want auto", cfg.ToneMappingBackend)
	}

	if !cfg.LibPlaceboPeakDetect {
		t.Error("LibPlaceboPeakDetect should be true by default")
	}

	if cfg.LibPlaceboContrastRecovery != 0.3 {
		t.Errorf("LibPlaceboContrastRecovery = %f, want 0.3", cfg.LibPlaceboContrastRecovery)
	}

	if !cfg.FFmpegLogEnabled {
		t.Error("FFmpegLogEnabled should be true by default")
	}

	if cfg.FFmpegLogRetentionHours != 48 {
		t.Errorf("FFmpegLogRetentionHours = %d, want 48", cfg.FFmpegLogRetentionHours)
	}
}

func TestDefaultFromProfile(t *testing.T) {
	// Mock encoder checker that always returns false
	mockChecker := func(name string) bool {
		return false
	}

	tests := []struct {
		name          string
		hwAccelType   string
		expectedAccel HardwareAccel
	}{
		{
			name:          "Explicit NVENC",
			hwAccelType:   "nvenc",
			expectedAccel: AccelNVENC,
		},
		{
			name:          "Explicit VAAPI",
			hwAccelType:   "vaapi",
			expectedAccel: AccelVAAPI,
		},
		{
			name:          "Explicit QSV",
			hwAccelType:   "qsv",
			expectedAccel: AccelQSV,
		},
		{
			name:          "Explicit VideoToolbox",
			hwAccelType:   "videotoolbox",
			expectedAccel: AccelVideoToolbox,
		},
		{
			name:          "Explicit None",
			hwAccelType:   "none",
			expectedAccel: AccelNone,
		},
		{
			name:          "Empty falls back to detection",
			hwAccelType:   "",
			expectedAccel: AccelNone, // mockChecker returns false, so falls to none
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultFromProfile(tt.hwAccelType, mockChecker)

			if cfg.HardwareAccel != tt.expectedAccel {
				t.Errorf("HardwareAccel = %v, want %v", cfg.HardwareAccel, tt.expectedAccel)
			}
		})
	}
}

func TestDetectHardwareAccel(t *testing.T) {
	// This test verifies the function runs without error
	// The actual detected value depends on the test environment
	accel := DetectHardwareAccel(nil)

	// Verify it's a valid acceleration type
	validAccels := []HardwareAccel{AccelNone, AccelVAAPI, AccelQSV, AccelNVENC, AccelVideoToolbox}
	valid := false
	for _, v := range validAccels {
		if accel == v {
			valid = true
			break
		}
	}

	if !valid {
		t.Errorf("DetectHardwareAccel() = %v, want one of %v", accel, validAccels)
	}
}

func TestHardwareAccelEnvironmentOverride(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectedAccel HardwareAccel
	}{
		{
			name:          "NVENC via env",
			envValue:      "nvenc",
			expectedAccel: AccelNVENC,
		},
		{
			name:          "VAAPI via env",
			envValue:      "vaapi",
			expectedAccel: AccelVAAPI,
		},
		{
			name:          "QSV via env",
			envValue:      "qsv",
			expectedAccel: AccelQSV,
		},
		{
			name:          "VideoToolbox via env",
			envValue:      "videotoolbox",
			expectedAccel: AccelVideoToolbox,
		},
		{
			name:          "None via env",
			envValue:      "none",
			expectedAccel: AccelNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original and restore after test
			originalVal := os.Getenv("HARDWARE_ACCEL")
			defer func() {
				if originalVal == "" {
					os.Unsetenv("HARDWARE_ACCEL")
				} else {
					os.Setenv("HARDWARE_ACCEL", originalVal)
				}
			}()

			// Set test value
			os.Setenv("HARDWARE_ACCEL", tt.envValue)

			accel := DetectHardwareAccel(nil)
			if accel != tt.expectedAccel {
				t.Errorf("DetectHardwareAccel() = %v, want %v", accel, tt.expectedAccel)
			}
		})
	}
}

func TestHardwareDeviceEnvironmentOverride(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedDevice string
	}{
		{
			name:           "Custom device path",
			envValue:       "/dev/dri/renderD129",
			expectedDevice: "/dev/dri/renderD129",
		},
		{
			name:           "Empty uses default",
			envValue:       "",
			expectedDevice: "/dev/dri/renderD128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original and restore after test
			originalVal := os.Getenv("HARDWARE_DEVICE")
			defer func() {
				if originalVal == "" {
					os.Unsetenv("HARDWARE_DEVICE")
				} else {
					os.Setenv("HARDWARE_DEVICE", originalVal)
				}
			}()

			// Set test value
			if tt.envValue != "" {
				os.Setenv("HARDWARE_DEVICE", tt.envValue)
			} else {
				os.Unsetenv("HARDWARE_DEVICE")
			}

			cfg := Default()
			if cfg.HardwareDevice != tt.expectedDevice {
				t.Errorf("HardwareDevice = %s, want %s", cfg.HardwareDevice, tt.expectedDevice)
			}
		})
	}
}

func TestOutputDirEnvironmentOverride(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectedDir string
	}{
		{
			name:        "Custom output dir",
			envValue:    "/custom/dash",
			expectedDir: "/custom/dash",
		},
		{
			name:        "Empty uses default",
			envValue:    "",
			expectedDir: "./data/dash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original and restore after test
			originalVal := os.Getenv("TRANSCODE_OUTPUT_DIR")
			defer func() {
				if originalVal == "" {
					os.Unsetenv("TRANSCODE_OUTPUT_DIR")
				} else {
					os.Setenv("TRANSCODE_OUTPUT_DIR", originalVal)
				}
			}()

			// Set test value
			if tt.envValue != "" {
				os.Setenv("TRANSCODE_OUTPUT_DIR", tt.envValue)
			} else {
				os.Unsetenv("TRANSCODE_OUTPUT_DIR")
			}

			dir := GetDefaultOutputDir()
			if dir != tt.expectedDir {
				t.Errorf("GetDefaultOutputDir() = %s, want %s", dir, tt.expectedDir)
			}
		})
	}
}

func TestToneMappingEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name               string
		enabledValue       string
		algorithmValue     string
		backendValue       string
		peakDetectValue    string
		contrastRecovValue string
		expectedEnabled    bool
		expectedAlgorithm  string
		expectedBackend    string
		expectedPeakDetect bool
		expectedContrast   float64
	}{
		{
			name:               "All defaults",
			enabledValue:       "",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "",
			expectedEnabled:    true,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
		{
			name:               "Disable tone mapping",
			enabledValue:       "false",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "",
			expectedEnabled:    false,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
		{
			name:               "Enable with 0",
			enabledValue:       "0",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "",
			expectedEnabled:    false,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
		{
			name:               "Enable with 1",
			enabledValue:       "1",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "",
			expectedEnabled:    true,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
		{
			name:               "Custom algorithm",
			enabledValue:       "",
			algorithmValue:     "hable",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "",
			expectedEnabled:    true,
			expectedAlgorithm:  "hable",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
		{
			name:               "Custom backend",
			enabledValue:       "",
			algorithmValue:     "",
			backendValue:       "libplacebo",
			peakDetectValue:    "",
			contrastRecovValue: "",
			expectedEnabled:    true,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "libplacebo",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
		{
			name:               "Disable peak detect",
			enabledValue:       "",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "false",
			contrastRecovValue: "",
			expectedEnabled:    true,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: false,
			expectedContrast:   0.3,
		},
		{
			name:               "Custom contrast recovery",
			enabledValue:       "",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "1.5",
			expectedEnabled:    true,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   1.5,
		},
		{
			name:               "Invalid contrast keeps default",
			enabledValue:       "",
			algorithmValue:     "",
			backendValue:       "",
			peakDetectValue:    "",
			contrastRecovValue: "invalid",
			expectedEnabled:    true,
			expectedAlgorithm:  "bt.2390",
			expectedBackend:    "auto",
			expectedPeakDetect: true,
			expectedContrast:   0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			envVars := map[string]string{
				"TONE_MAPPING_ENABLED":         tt.enabledValue,
				"TONE_MAPPING_ALGORITHM":       tt.algorithmValue,
				"TONE_MAPPING_BACKEND":         tt.backendValue,
				"LIBPLACEBO_PEAK_DETECT":       tt.peakDetectValue,
				"LIBPLACEBO_CONTRAST_RECOVERY": tt.contrastRecovValue,
			}

			originals := make(map[string]string)
			for key, val := range envVars {
				originals[key] = os.Getenv(key)
				if val != "" {
					os.Setenv(key, val)
				} else {
					os.Unsetenv(key)
				}
			}

			defer func() {
				for key, orig := range originals {
					if orig == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, orig)
					}
				}
			}()

			cfg := Default()

			if cfg.ToneMappingEnabled != tt.expectedEnabled {
				t.Errorf("ToneMappingEnabled = %v, want %v", cfg.ToneMappingEnabled, tt.expectedEnabled)
			}

			if cfg.ToneMappingAlgorithm != tt.expectedAlgorithm {
				t.Errorf("ToneMappingAlgorithm = %s, want %s", cfg.ToneMappingAlgorithm, tt.expectedAlgorithm)
			}

			if cfg.ToneMappingBackend != tt.expectedBackend {
				t.Errorf("ToneMappingBackend = %s, want %s", cfg.ToneMappingBackend, tt.expectedBackend)
			}

			if cfg.LibPlaceboPeakDetect != tt.expectedPeakDetect {
				t.Errorf("LibPlaceboPeakDetect = %v, want %v", cfg.LibPlaceboPeakDetect, tt.expectedPeakDetect)
			}

			if cfg.LibPlaceboContrastRecovery != tt.expectedContrast {
				t.Errorf("LibPlaceboContrastRecovery = %f, want %f", cfg.LibPlaceboContrastRecovery, tt.expectedContrast)
			}
		})
	}
}

func TestFFmpegLogEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name              string
		enabledValue      string
		retentionValue    string
		expectedEnabled   bool
		expectedRetention int
	}{
		{
			name:              "Defaults",
			enabledValue:      "",
			retentionValue:    "",
			expectedEnabled:   true,
			expectedRetention: 48,
		},
		{
			name:              "Disable logging",
			enabledValue:      "false",
			retentionValue:    "",
			expectedEnabled:   false,
			expectedRetention: 48,
		},
		{
			name:              "Custom retention",
			enabledValue:      "",
			retentionValue:    "72",
			expectedEnabled:   true,
			expectedRetention: 72,
		},
		{
			name:              "Invalid retention keeps default",
			enabledValue:      "",
			retentionValue:    "invalid",
			expectedEnabled:   true,
			expectedRetention: 48,
		},
		{
			name:              "Zero retention keeps default",
			enabledValue:      "",
			retentionValue:    "0",
			expectedEnabled:   true,
			expectedRetention: 48,
		},
		{
			name:              "Negative retention keeps default",
			enabledValue:      "",
			retentionValue:    "-10",
			expectedEnabled:   true,
			expectedRetention: 48,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore environment
			origEnabled := os.Getenv("FFMPEG_LOG_ENABLED")
			origRetention := os.Getenv("FFMPEG_LOG_RETENTION_HOURS")

			defer func() {
				if origEnabled == "" {
					os.Unsetenv("FFMPEG_LOG_ENABLED")
				} else {
					os.Setenv("FFMPEG_LOG_ENABLED", origEnabled)
				}
				if origRetention == "" {
					os.Unsetenv("FFMPEG_LOG_RETENTION_HOURS")
				} else {
					os.Setenv("FFMPEG_LOG_RETENTION_HOURS", origRetention)
				}
			}()

			if tt.enabledValue != "" {
				os.Setenv("FFMPEG_LOG_ENABLED", tt.enabledValue)
			} else {
				os.Unsetenv("FFMPEG_LOG_ENABLED")
			}

			if tt.retentionValue != "" {
				os.Setenv("FFMPEG_LOG_RETENTION_HOURS", tt.retentionValue)
			} else {
				os.Unsetenv("FFMPEG_LOG_RETENTION_HOURS")
			}

			cfg := Default()

			if cfg.FFmpegLogEnabled != tt.expectedEnabled {
				t.Errorf("FFmpegLogEnabled = %v, want %v", cfg.FFmpegLogEnabled, tt.expectedEnabled)
			}

			if cfg.FFmpegLogRetentionHours != tt.expectedRetention {
				t.Errorf("FFmpegLogRetentionHours = %d, want %d", cfg.FFmpegLogRetentionHours, tt.expectedRetention)
			}
		})
	}
}

func TestHardwareAccelConstants(t *testing.T) {
	tests := []struct {
		accel    HardwareAccel
		expected string
	}{
		{AccelNone, "none"},
		{AccelVAAPI, "vaapi"},
		{AccelNVENC, "nvenc"},
		{AccelQSV, "qsv"},
		{AccelVideoToolbox, "videotoolbox"},
	}

	for _, tt := range tests {
		t.Run(string(tt.accel), func(t *testing.T) {
			if string(tt.accel) != tt.expected {
				t.Errorf("HardwareAccel constant = %s, want %s", tt.accel, tt.expected)
			}
		})
	}
}

func TestGetDefaultMaxMemoryMB(t *testing.T) {
	// This function depends on system RAM, so we test the constraints
	maxMem := getDefaultMaxMemoryMB()

	// Should be between 1GB and 4GB
	if maxMem < 1024 {
		t.Errorf("getDefaultMaxMemoryMB() = %d MB, want >= 1024 MB", maxMem)
	}

	if maxMem > 4096 {
		t.Errorf("getDefaultMaxMemoryMB() = %d MB, want <= 4096 MB", maxMem)
	}
}

func TestFFmpegPathsIntegration(t *testing.T) {
	cfg := Default()

	// FFmpegPaths should always be set
	if cfg.FFmpegPaths == nil {
		t.Fatal("FFmpegPaths should not be nil")
	}

	// Should have at least placeholder paths
	if cfg.FFmpegPaths.FFmpeg == "" {
		t.Error("FFmpegPaths.FFmpeg should not be empty")
	}

	if cfg.FFmpegPaths.FFprobe == "" {
		t.Error("FFmpegPaths.FFprobe should not be empty")
	}
}

func TestEncoderChecker(t *testing.T) {
	// Test with a custom encoder checker
	called := false
	customChecker := func(encoderName string) bool {
		called = true
		return encoderName == "h264_nvenc"
	}

	// Reset environment to ensure detection runs
	origAccel := os.Getenv("HARDWARE_ACCEL")
	os.Unsetenv("HARDWARE_ACCEL")
	defer func() {
		if origAccel != "" {
			os.Setenv("HARDWARE_ACCEL", origAccel)
		}
	}()

	// The checker will be called during detection
	_ = DefaultFromProfile("", customChecker)

	// Note: Whether the checker is called depends on system state
	// (e.g., whether nvidia-smi is present, whether /dev/dri/renderD128 exists)
	// We just verify it doesn't panic
	_ = called
}

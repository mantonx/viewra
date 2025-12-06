package transcoding

import (
	"os"
	"testing"

	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg"
)

func TestDefaultTranscodeConfig(t *testing.T) {
	config := DefaultTranscodeConfig()

	if config == nil {
		t.Fatal("DefaultTranscodeConfig() returned nil")
	}

	// Verify FFmpegPaths is set
	if config.FFmpegPaths == nil {
		t.Error("FFmpegPaths should not be nil")
	}

	// Verify hardware acceleration is set
	if config.HardwareAccel == "" {
		t.Error("HardwareAccel should not be empty")
	}

	// Verify reasonable defaults
	if config.MinFreeDiskGB != 10 {
		t.Errorf("MinFreeDiskGB = %d, want 10", config.MinFreeDiskGB)
	}

	if config.MaxCPUPercent != 0 {
		t.Errorf("MaxCPUPercent = %d, want 0 (unlimited)", config.MaxCPUPercent)
	}

	if !config.ProcessGroupKill {
		t.Error("ProcessGroupKill should be true by default")
	}

	if !config.ToneMappingEnabled {
		t.Error("ToneMappingEnabled should be true by default")
	}

	if config.ToneMappingAlgorithm != "bt.2390" {
		t.Errorf("ToneMappingAlgorithm = %s, want bt.2390", config.ToneMappingAlgorithm)
	}

	if config.ToneMappingBackend != "auto" {
		t.Errorf("ToneMappingBackend = %s, want auto", config.ToneMappingBackend)
	}

	if !config.LibPlaceboPeakDetect {
		t.Error("LibPlaceboPeakDetect should be true by default")
	}

	if config.LibPlaceboContrastRecovery != 0.3 {
		t.Errorf("LibPlaceboContrastRecovery = %f, want 0.3", config.LibPlaceboContrastRecovery)
	}

	if !config.FFmpegLogEnabled {
		t.Error("FFmpegLogEnabled should be true by default")
	}

	if config.FFmpegLogRetentionHours != 48 {
		t.Errorf("FFmpegLogRetentionHours = %d, want 48", config.FFmpegLogRetentionHours)
	}
}

func TestDefaultTranscodeConfigFromProfile(t *testing.T) {
	tests := []struct {
		name           string
		hwAccelType    string
		expectedAccel  HardwareAccel
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
			name:          "Empty uses detection",
			hwAccelType:   "",
			expectedAccel: detectHardwareAccel(), // Should match auto-detection
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultTranscodeConfigFromProfile(tt.hwAccelType)

			if config.HardwareAccel != tt.expectedAccel {
				t.Errorf("HardwareAccel = %v, want %v", config.HardwareAccel, tt.expectedAccel)
			}
		})
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
		{
			name:          "Invalid value ignored",
			envValue:      "invalid",
			expectedAccel: detectHardwareAccel(), // Falls back to detection
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

			accel := detectHardwareAccel()
			if accel != tt.expectedAccel {
				t.Errorf("detectHardwareAccel() = %v, want %v", accel, tt.expectedAccel)
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

			config := DefaultTranscodeConfig()
			if config.HardwareDevice != tt.expectedDevice {
				t.Errorf("HardwareDevice = %s, want %s", config.HardwareDevice, tt.expectedDevice)
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
				"TONE_MAPPING_ENABLED":          tt.enabledValue,
				"TONE_MAPPING_ALGORITHM":        tt.algorithmValue,
				"TONE_MAPPING_BACKEND":          tt.backendValue,
				"LIBPLACEBO_PEAK_DETECT":        tt.peakDetectValue,
				"LIBPLACEBO_CONTRAST_RECOVERY":  tt.contrastRecovValue,
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

			config := DefaultTranscodeConfig()

			if config.ToneMappingEnabled != tt.expectedEnabled {
				t.Errorf("ToneMappingEnabled = %v, want %v", config.ToneMappingEnabled, tt.expectedEnabled)
			}

			if config.ToneMappingAlgorithm != tt.expectedAlgorithm {
				t.Errorf("ToneMappingAlgorithm = %s, want %s", config.ToneMappingAlgorithm, tt.expectedAlgorithm)
			}

			if config.ToneMappingBackend != tt.expectedBackend {
				t.Errorf("ToneMappingBackend = %s, want %s", config.ToneMappingBackend, tt.expectedBackend)
			}

			if config.LibPlaceboPeakDetect != tt.expectedPeakDetect {
				t.Errorf("LibPlaceboPeakDetect = %v, want %v", config.LibPlaceboPeakDetect, tt.expectedPeakDetect)
			}

			if config.LibPlaceboContrastRecovery != tt.expectedContrast {
				t.Errorf("LibPlaceboContrastRecovery = %f, want %f", config.LibPlaceboContrastRecovery, tt.expectedContrast)
			}
		})
	}
}

func TestFFmpegLogEnvironmentOverrides(t *testing.T) {
	tests := []struct {
		name                string
		enabledValue        string
		retentionValue      string
		expectedEnabled     bool
		expectedRetention   int
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

			config := DefaultTranscodeConfig()

			if config.FFmpegLogEnabled != tt.expectedEnabled {
				t.Errorf("FFmpegLogEnabled = %v, want %v", config.FFmpegLogEnabled, tt.expectedEnabled)
			}

			if config.FFmpegLogRetentionHours != tt.expectedRetention {
				t.Errorf("FFmpegLogRetentionHours = %d, want %d", config.FFmpegLogRetentionHours, tt.expectedRetention)
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

func TestFFmpegPathsIntegration(t *testing.T) {
	config := DefaultTranscodeConfig()

	// FFmpegPaths should always be set
	if config.FFmpegPaths == nil {
		t.Fatal("FFmpegPaths should not be nil")
	}

	// Should have at least placeholder paths
	if config.FFmpegPaths.FFmpeg == "" {
		t.Error("FFmpegPaths.FFmpeg should not be empty")
	}

	if config.FFmpegPaths.FFprobe == "" {
		t.Error("FFmpegPaths.FFprobe should not be empty")
	}

	// LibPath can be empty for system installs
	// No assertion needed - it's optional
}

func TestFFmpegPathsEnvironmentOverride(t *testing.T) {
	// Save and restore environment
	origFFmpeg := os.Getenv("VIEWRA_FFMPEG_PATH")
	origFFprobe := os.Getenv("VIEWRA_FFPROBE_PATH")
	origLibPath := os.Getenv("VIEWRA_FFMPEG_LIB_PATH")

	defer func() {
		if origFFmpeg == "" {
			os.Unsetenv("VIEWRA_FFMPEG_PATH")
		} else {
			os.Setenv("VIEWRA_FFMPEG_PATH", origFFmpeg)
		}
		if origFFprobe == "" {
			os.Unsetenv("VIEWRA_FFPROBE_PATH")
		} else {
			os.Setenv("VIEWRA_FFPROBE_PATH", origFFprobe)
		}
		if origLibPath == "" {
			os.Unsetenv("VIEWRA_FFMPEG_LIB_PATH")
		} else {
			os.Setenv("VIEWRA_FFMPEG_LIB_PATH", origLibPath)
		}
	}()

	// Set custom paths
	os.Setenv("VIEWRA_FFMPEG_PATH", "/custom/ffmpeg")
	os.Setenv("VIEWRA_FFPROBE_PATH", "/custom/ffprobe")
	os.Setenv("VIEWRA_FFMPEG_LIB_PATH", "/custom/lib")

	paths, err := ffmpeg.NewPaths()
	if err != nil {
		t.Fatalf("NewPaths() failed: %v", err)
	}

	if paths.FFmpeg != "/custom/ffmpeg" {
		t.Errorf("FFmpeg path = %s, want /custom/ffmpeg", paths.FFmpeg)
	}

	if paths.FFprobe != "/custom/ffprobe" {
		t.Errorf("FFprobe path = %s, want /custom/ffprobe", paths.FFprobe)
	}

	if paths.LibPath != "/custom/lib" {
		t.Errorf("LibPath = %s, want /custom/lib", paths.LibPath)
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

func TestCheckFFmpegEncoder(t *testing.T) {
	// Test with software encoder (should always exist if FFmpeg is installed)
	hasLibx264 := CheckFFmpegEncoder("libx264")

	// We don't assert true here because FFmpeg might not be installed in test env
	// We just verify the function doesn't panic
	_ = hasLibx264

	// Test with non-existent encoder
	hasInvalid := CheckFFmpegEncoder("nonexistent_encoder_xyz")
	if hasInvalid {
		t.Error("CheckFFmpegEncoder() should return false for non-existent encoder")
	}
}

func TestDetectHardwareAccel(t *testing.T) {
	// This test verifies the function runs without error
	// The actual detected value depends on the test environment
	accel := detectHardwareAccel()

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
		t.Errorf("detectHardwareAccel() = %v, want one of %v", accel, validAccels)
	}
}

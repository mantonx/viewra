package ffmpeg

import (
	"strings"
	"testing"
)

// TestAddCodecProfile_H264 tests H.264 profile selection via addCodecProfile
func TestAddCodecProfile_H264(t *testing.T) {
	tests := []struct {
		name     string
		codec    VideoCodec
		hwAccel  HardwareAccel
		width    int
		height   int
		wantArgs map[string]string // key-value pairs we expect
	}{
		{
			name:    "H.264 software 1080p",
			codec:   CodecH264,
			hwAccel: AccelNone,
			width:   1920,
			height:  1080,
			wantArgs: map[string]string{
				"-profile:v": "high",
				"-level":     "4.1",
				"-pix_fmt":   "yuv420p",
			},
		},
		{
			name:    "H.264 software 4K",
			codec:   CodecH264,
			hwAccel: AccelNone,
			width:   3840,
			height:  2160,
			wantArgs: map[string]string{
				"-profile:v": "high",
				"-level":     "5.1",
				"-pix_fmt":   "yuv420p",
			},
		},
		{
			name:    "H.264 NVENC 1080p",
			codec:   CodecH264,
			hwAccel: AccelNVENC,
			width:   1920,
			height:  1080,
			wantArgs: map[string]string{
				"-profile:v": "high",
				"-level":     "4.1",
				"-pix_fmt":   "yuv420p",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := createTestProfile()
			profile.Width = tt.width
			profile.Height = tt.height

			opts := TranscodeOptions{
				Profile:    profile,
				VideoCodec: tt.codec,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addCodecProfile(tt.codec, tt.hwAccel)
			args := builder.Build()

			for flag, expectedValue := range tt.wantArgs {
				if !argsContain(args, flag, expectedValue) {
					t.Errorf("Expected args to contain %s=%s, got args: %v", flag, expectedValue, args)
				}
			}
		})
	}
}

// TestAddH265Profile tests H.265/HEVC profile and level configuration
func TestAddH265Profile(t *testing.T) {
	tests := []struct {
		name            string
		hwAccel         HardwareAccel
		wantProfile     string
		wantLevel       string
		wantPixFmt      string
		wantX265Params  bool
		x265ParamsValue string
	}{
		{
			name:            "H.265 software encoding",
			hwAccel:         AccelNone,
			wantProfile:     "main",
			wantLevel:       "5.1",
			wantPixFmt:      "yuv420p",
			wantX265Params:  true,
			x265ParamsValue: "log-level=error",
		},
		{
			name:           "H.265 NVENC hardware",
			hwAccel:        AccelNVENC,
			wantProfile:    "main",
			wantLevel:      "5.1",
			wantPixFmt:     "yuv420p",
			wantX265Params: false,
		},
		{
			name:           "H.265 VAAPI hardware",
			hwAccel:        AccelVAAPI,
			wantProfile:    "main",
			wantLevel:      "5.1",
			wantPixFmt:     "yuv420p",
			wantX265Params: false,
		},
		{
			name:           "H.265 QSV hardware",
			hwAccel:        AccelQSV,
			wantProfile:    "main",
			wantLevel:      "5.1",
			wantPixFmt:     "yuv420p",
			wantX265Params: false,
		},
		{
			name:           "H.265 VideoToolbox",
			hwAccel:        AccelVideoToolbox,
			wantProfile:    "main",
			wantLevel:      "5.1",
			wantPixFmt:     "yuv420p",
			wantX265Params: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: CodecH265,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addH265Profile(tt.hwAccel)
			args := builder.Build()

			// Verify profile
			if !argsContain(args, "-profile:v", tt.wantProfile) {
				t.Errorf("Expected -profile:v=%s, got args: %v", tt.wantProfile, args)
			}

			// Verify level
			if !argsContain(args, "-level", tt.wantLevel) {
				t.Errorf("Expected -level=%s, got args: %v", tt.wantLevel, args)
			}

			// Verify pixel format
			if !argsContain(args, "-pix_fmt", tt.wantPixFmt) {
				t.Errorf("Expected -pix_fmt=%s, got args: %v", tt.wantPixFmt, args)
			}

			// Verify x265-params only for software encoding
			hasX265Params := argsContain(args, "-x265-params", tt.x265ParamsValue)
			if tt.wantX265Params && !hasX265Params {
				t.Errorf("Expected -x265-params=%s for software encoding, got args: %v", tt.x265ParamsValue, args)
			}
			if !tt.wantX265Params && hasX265Params {
				t.Errorf("Did not expect -x265-params for hardware encoding, got args: %v", args)
			}
		})
	}
}

// TestAddVP9Profile tests VP9 profile configuration
func TestAddVP9Profile(t *testing.T) {
	tests := []struct {
		name        string
		hwAccel     HardwareAccel
		wantPixFmt  string
		wantQuality string
		wantSpeed   string
		wantRowMT   bool
		rowMTValue  string
	}{
		{
			name:        "VP9 software encoding",
			hwAccel:     AccelNone,
			wantPixFmt:  "yuv420p",
			wantQuality: "good",
			wantSpeed:   "2",
			wantRowMT:   true,
			rowMTValue:  "1",
		},
		{
			name:        "VP9 VAAPI hardware",
			hwAccel:     AccelVAAPI,
			wantPixFmt:  "yuv420p",
			wantQuality: "good",
			wantSpeed:   "2",
			wantRowMT:   false,
		},
		{
			name:        "VP9 QSV hardware",
			hwAccel:     AccelQSV,
			wantPixFmt:  "yuv420p",
			wantQuality: "good",
			wantSpeed:   "2",
			wantRowMT:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: CodecVP9,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addVP9Profile(tt.hwAccel)
			args := builder.Build()

			// Verify pixel format
			if !argsContain(args, "-pix_fmt", tt.wantPixFmt) {
				t.Errorf("Expected -pix_fmt=%s, got args: %v", tt.wantPixFmt, args)
			}

			// Verify quality setting
			if !argsContain(args, "-quality", tt.wantQuality) {
				t.Errorf("Expected -quality=%s, got args: %v", tt.wantQuality, args)
			}

			// Verify speed setting
			if !argsContain(args, "-speed", tt.wantSpeed) {
				t.Errorf("Expected -speed=%s, got args: %v", tt.wantSpeed, args)
			}

			// Verify row-mt only for software encoding
			hasRowMT := argsContain(args, "-row-mt", tt.rowMTValue)
			if tt.wantRowMT && !hasRowMT {
				t.Errorf("Expected -row-mt=%s for software encoding, got args: %v", tt.rowMTValue, args)
			}
			if !tt.wantRowMT && hasRowMT {
				t.Errorf("Did not expect -row-mt for hardware encoding, got args: %v", args)
			}
		})
	}
}

// TestAddVP9Profile_QualitySettings tests VP9 with different quality configurations
func TestAddVP9Profile_QualitySettings(t *testing.T) {
	opts := TranscodeOptions{
		Profile:    createTestProfile(),
		VideoCodec: CodecVP9,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.addVP9Profile(AccelNone)
	args := builder.Build()

	// VP9 quality should always be "good" (balanced)
	if !argsContain(args, "-quality", "good") {
		t.Errorf("VP9 should use 'good' quality setting for balanced speed/quality")
	}

	// VP9 speed should be 2 (0-5 range, lower = slower/better)
	if !argsContain(args, "-speed", "2") {
		t.Errorf("VP9 should use speed=2 for balanced encoding")
	}
}

// TestAddAV1Profile tests AV1 profile configuration
func TestAddAV1Profile(t *testing.T) {
	tests := []struct {
		name        string
		hwAccel     HardwareAccel
		wantPixFmt  string
		wantPreset  bool
		presetValue string
	}{
		{
			name:        "AV1 software encoding (SVT-AV1)",
			hwAccel:     AccelNone,
			wantPixFmt:  "yuv420p",
			wantPreset:  true,
			presetValue: "6",
		},
		{
			name:       "AV1 QSV hardware",
			hwAccel:    AccelQSV,
			wantPixFmt: "yuv420p",
			wantPreset: false,
		},
		{
			name:       "AV1 VAAPI hardware",
			hwAccel:    AccelVAAPI,
			wantPixFmt: "yuv420p",
			wantPreset: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: CodecAV1,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addAV1Profile(tt.hwAccel)
			args := builder.Build()

			// Verify pixel format
			if !argsContain(args, "-pix_fmt", tt.wantPixFmt) {
				t.Errorf("Expected -pix_fmt=%s, got args: %v", tt.wantPixFmt, args)
			}

			// Verify preset only for software encoding
			hasPreset := argsContain(args, "-preset", tt.presetValue)
			if tt.wantPreset && !hasPreset {
				t.Errorf("Expected -preset=%s for SVT-AV1 software encoding, got args: %v", tt.presetValue, args)
			}
			if !tt.wantPreset && hasPreset {
				t.Errorf("Did not expect -preset for hardware encoding, got args: %v", args)
			}
		})
	}
}

// TestAddAV1Profile_PresetRange tests AV1 preset is within valid range
func TestAddAV1Profile_PresetRange(t *testing.T) {
	opts := TranscodeOptions{
		Profile:    createTestProfile(),
		VideoCodec: CodecAV1,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.addAV1Profile(AccelNone)
	args := builder.Build()

	// AV1 preset should be "6" (SVT-AV1 range is 0-13, 6 is balanced)
	if !argsContain(args, "-preset", "6") {
		t.Errorf("AV1 should use preset=6 for balanced speed/quality (SVT-AV1 range: 0-13)")
	}
}

// TestAddCodecProfile_AllCodecs tests addCodecProfile routing to correct codec-specific functions
func TestAddCodecProfile_AllCodecs(t *testing.T) {
	tests := []struct {
		name          string
		codec         VideoCodec
		hwAccel       HardwareAccel
		expectedArg   string
		expectedValue string
		unexpectedArg string
	}{
		{
			name:          "Routes to H.264 profile",
			codec:         CodecH264,
			hwAccel:       AccelNone,
			expectedArg:   "-profile:v",
			expectedValue: "high",
			unexpectedArg: "-x265-params",
		},
		{
			name:          "Routes to H.265 profile",
			codec:         CodecH265,
			hwAccel:       AccelNone,
			expectedArg:   "-profile:v",
			expectedValue: "main",
			unexpectedArg: "-quality", // VP9 specific
		},
		{
			name:          "Routes to VP9 profile",
			codec:         CodecVP9,
			hwAccel:       AccelNone,
			expectedArg:   "-quality",
			expectedValue: "good",
			unexpectedArg: "-x265-params", // H.265 specific
		},
		{
			name:          "Routes to AV1 profile",
			codec:         CodecAV1,
			hwAccel:       AccelNone,
			expectedArg:   "-preset",
			expectedValue: "6",
			unexpectedArg: "-quality", // VP9 specific
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: tt.codec,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addCodecProfile(tt.codec, tt.hwAccel)
			args := builder.Build()

			// Verify expected codec-specific argument
			if !argsContain(args, tt.expectedArg, tt.expectedValue) {
				t.Errorf("Expected %s=%s for %s codec, got args: %v", tt.expectedArg, tt.expectedValue, tt.codec, args)
			}

			// Verify codec-specific args from other codecs are not present
			if argsContainFlag(args, tt.unexpectedArg) {
				t.Errorf("Did not expect %s for %s codec, got args: %v", tt.unexpectedArg, tt.codec, args)
			}
		})
	}
}

// TestAddCodecProfile_ResolutionDependentLevels tests H.264 level selection based on resolution
func TestAddCodecProfile_ResolutionDependentLevels(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		wantLevel   string
		description string
	}{
		{
			name:        "720p uses level 4.0",
			width:       1280,
			height:      720,
			wantLevel:   "4.0",
			description: "HD ready",
		},
		{
			name:        "1080p uses level 4.1",
			width:       1920,
			height:      1080,
			wantLevel:   "4.1",
			description: "Full HD",
		},
		{
			name:        "1440p uses level 5.1",
			width:       2560,
			height:      1440,
			wantLevel:   "5.1",
			description: "2K/QHD",
		},
		{
			name:        "4K uses level 5.1",
			width:       3840,
			height:      2160,
			wantLevel:   "5.1",
			description: "4K/UHD",
		},
		{
			name:        "480p uses level 4.0",
			width:       854,
			height:      480,
			wantLevel:   "4.0",
			description: "SD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := createTestProfile()
			profile.Width = tt.width
			profile.Height = tt.height

			opts := TranscodeOptions{
				Profile:    profile,
				VideoCodec: CodecH264,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addCodecProfile(CodecH264, AccelNone)
			args := builder.Build()

			if !argsContain(args, "-level", tt.wantLevel) {
				t.Errorf("%s (%dx%d): Expected -level=%s, got args: %v", tt.description, tt.width, tt.height, tt.wantLevel, args)
			}
		})
	}
}

// TestAddVideoEncoding_Integration tests the full AddVideoEncoding pipeline
func TestAddVideoEncoding_Integration(t *testing.T) {
	tests := []struct {
		name        string
		codec       VideoCodec
		width       int
		height      int
		wantCodec   string
		wantProfile string
	}{
		{
			name:        "H.264 1080p full encoding pipeline",
			codec:       CodecH264,
			width:       1920,
			height:      1080,
			wantCodec:   "high",
			wantProfile: "high",
		},
		{
			name:        "H.265 4K full encoding pipeline",
			codec:       CodecH265,
			width:       3840,
			height:      2160,
			wantCodec:   "main",
			wantProfile: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := createTestProfile()
			profile.Width = tt.width
			profile.Height = tt.height

			opts := TranscodeOptions{
				Profile:    profile,
				VideoCodec: tt.codec,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.AddVideoEncoding()
			args := builder.Build()

			// Verify codec profile is set
			if !argsContain(args, "-profile:v", tt.wantProfile) {
				t.Errorf("Expected -profile:v=%s, got args: %v", tt.wantProfile, args)
			}

			// Verify bitrate args are present
			if !argsContain(args, "-b:v", "8000k") {
				t.Error("Expected bitrate settings from AddVideoEncoding")
			}

			// Verify GOP args are present (test profile has GOPSize: 48)
			if !argsContain(args, "-g", "48") {
				t.Error("Expected GOP settings from AddVideoEncoding")
			}

			// Verify filter chain is present (-vf flag)
			if !argsContainFlag(args, "-vf") {
				t.Error("Expected video filter chain from AddVideoEncoding")
			}
		})
	}
}

// TestCodecProfile_PixelFormat tests all codecs use yuv420p for compatibility
func TestCodecProfile_PixelFormat(t *testing.T) {
	codecs := []VideoCodec{CodecH264, CodecH265, CodecVP9, CodecAV1}

	for _, codec := range codecs {
		t.Run(string(codec), func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: codec,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addCodecProfile(codec, AccelNone)
			args := builder.Build()

			// All codecs should use yuv420p for maximum compatibility
			if !argsContain(args, "-pix_fmt", "yuv420p") {
				t.Errorf("Codec %s should use yuv420p pixel format for compatibility, got args: %v", codec, args)
			}
		})
	}
}

// TestH265Profile_Level tests H.265 always uses level 5.1
func TestH265Profile_Level(t *testing.T) {
	// H.265 level 5.1 supports up to 4K@60fps
	// We always use 5.1 for broad compatibility regardless of resolution
	resolutions := []struct {
		width  int
		height int
		name   string
	}{
		{1280, 720, "720p"},
		{1920, 1080, "1080p"},
		{2560, 1440, "1440p"},
		{3840, 2160, "4K"},
	}

	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			profile := createTestProfile()
			profile.Width = res.width
			profile.Height = res.height

			opts := TranscodeOptions{
				Profile:    profile,
				VideoCodec: CodecH265,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addH265Profile(AccelNone)
			args := builder.Build()

			if !argsContain(args, "-level", "5.1") {
				t.Errorf("H.265 should always use level 5.1 (supports up to 4K@60fps), got args: %v", args)
			}
		})
	}
}

// TestVP9Profile_MultiThreading tests VP9 row-mt for better CPU utilization
func TestVP9Profile_MultiThreading(t *testing.T) {
	opts := TranscodeOptions{
		Profile:    createTestProfile(),
		VideoCodec: CodecVP9,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.addVP9Profile(AccelNone)
	args := builder.Build()

	// Software VP9 should enable row-based multi-threading for better CPU utilization
	if !argsContain(args, "-row-mt", "1") {
		t.Error("Software VP9 should enable row-mt for better multi-threading performance")
	}
}

// TestH265Profile_X265LogLevel tests x265 log suppression for cleaner output
func TestH265Profile_X265LogLevel(t *testing.T) {
	opts := TranscodeOptions{
		Profile:    createTestProfile(),
		VideoCodec: CodecH265,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.addH265Profile(AccelNone)
	args := builder.Build()

	// Software H.265 (libx265) should suppress verbose logs
	if !argsContain(args, "-x265-params", "log-level=error") {
		t.Error("Software H.265 should use x265-params to suppress verbose logging")
	}
}

// Helper to find if a string slice contains a substring in any element
func argsContainSubstring(args []string, substring string) bool {
	for _, arg := range args {
		if strings.Contains(arg, substring) {
			return true
		}
	}
	return false
}

// TestCodecProfile_NoHardwareSpecificArgsInSoftware ensures software encoding doesn't get hardware args
func TestCodecProfile_NoHardwareSpecificArgsInSoftware(t *testing.T) {
	codecs := []VideoCodec{CodecH264, CodecH265, CodecVP9, CodecAV1}

	for _, codec := range codecs {
		t.Run(string(codec), func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: codec,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.addCodecProfile(codec, AccelNone)
			args := builder.Build()

			// Software encoding should not contain hardware-specific flags
			hardwareFlags := []string{"_nvenc", "_qsv", "_vaapi", "_videotoolbox", "cuda", "opencl"}
			argsStr := strings.Join(args, " ")
			for _, hwFlag := range hardwareFlags {
				if strings.Contains(argsStr, hwFlag) {
					t.Errorf("Software encoding for %s should not contain hardware flag '%s', got args: %v", codec, hwFlag, args)
				}
			}
		})
	}
}

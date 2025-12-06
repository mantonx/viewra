package transcoding

import (
	"strings"
	"testing"
)

// Helper function to create a basic test profile
func createTestProfile() *AdaptiveProfile {
	return &AdaptiveProfile{
		ID:              "1080p-8m",
		Width:           1920,
		Height:          1080,
		VideoBitrate:    8_000_000,
		VideoMaxRate:    8_800_000,
		VideoBufSize:    16_000_000,
		AudioBitrate:    192_000,
		AudioChannels:   2,
		AudioSampleRate: 48000,
		GOPSize:         60,
		SegmentDuration: 4,
	}
}

// Helper function to create test VideoInfo
func createTestVideoInfo(codec string, width, height int, isHDR bool) *VideoInfo {
	return &VideoInfo{
		Codec:         codec,
		Width:         width,
		Height:        height,
		AudioCodec:    "aac",
		AudioChannels: 2,
		IsHDR:         isHDR,
	}
}

// Helper function to check if args contain a specific flag with value
func argsContain(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

// Helper function to check if args contain a flag
func argsContainFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func TestNewFFmpegArgsBuilder(t *testing.T) {
	profile := createTestProfile()
	opts := TranscodeOptions{
		InputPath: "/path/to/video.mp4",
		OutputDir: "/tmp/output",
		Profile:   profile,
	}

	builder := NewFFmpegArgsBuilder(opts)

	if builder == nil {
		t.Fatal("NewFFmpegArgsBuilder() returned nil")
	}

	if builder.opts.InputPath != opts.InputPath {
		t.Error("Builder opts not set correctly")
	}

	if len(builder.args) != 0 {
		t.Error("Builder args should start empty")
	}
}

func TestBuild(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/video.mp4",
		Profile:   createTestProfile(),
	}

	builder := NewFFmpegArgsBuilder(opts)
	builder.AddLogLevel("error")
	builder.AddInput()

	args := builder.Build()

	if len(args) != 4 {
		t.Errorf("Build() returned %d args, want 4", len(args))
	}

	expected := []string{"-loglevel", "error", "-i", "/path/to/video.mp4"}
	for i, arg := range expected {
		if args[i] != arg {
			t.Errorf("args[%d] = %q, want %q", i, args[i], arg)
		}
	}
}

func TestFormatBitrate(t *testing.T) {
	tests := []struct {
		name     string
		bps      int
		expected string
	}{
		{"5 Mbps", 5_000_000, "5000k"},
		{"8 Mbps", 8_000_000, "8000k"},
		{"192 kbps", 192_000, "192k"},
		{"1 kbps", 1_000, "1k"},
		{"40 Mbps", 40_000_000, "40000k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBitrate(tt.bps)
			if result != tt.expected {
				t.Errorf("formatBitrate(%d) = %q, want %q", tt.bps, result, tt.expected)
			}
		})
	}
}

func TestGetH264Level(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		expected string
	}{
		{"720p", 1280, 720, "4.0"},
		{"1080p", 1920, 1080, "4.1"},
		{"1440p", 2560, 1440, "5.1"},
		{"4K", 3840, 2160, "5.1"},
		{"480p", 854, 480, "4.0"},
		{"SD", 640, 360, "4.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getH264Level(tt.width, tt.height)
			if result != tt.expected {
				t.Errorf("getH264Level(%d, %d) = %q, want %q", tt.width, tt.height, result, tt.expected)
			}
		})
	}
}

func TestAddLogLevel(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddLogLevel("error")
	args := builder.Build()

	if !argsContain(args, "-loglevel", "error") {
		t.Error("Args should contain -loglevel error")
	}
}

func TestAddInput(t *testing.T) {
	opts := TranscodeOptions{
		InputPath: "/path/to/video.mkv",
		Profile:   createTestProfile(),
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddInput()
	args := builder.Build()

	if !argsContain(args, "-i", "/path/to/video.mkv") {
		t.Error("Args should contain -i with input path")
	}
}

func TestAddSeekPosition(t *testing.T) {
	tests := []struct {
		name               string
		useStartPosition   bool
		startPosition      int
		shouldContainSeek  bool
	}{
		{"No seek", false, 0, false},
		{"Seek disabled", false, 60, false},
		{"Seek at 0", true, 0, false},
		{"Seek at 60s", true, 60, true},
		{"Seek at 120s", true, 120, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:          createTestProfile(),
				UseStartPosition: tt.useStartPosition,
				StartPosition:    tt.startPosition,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.AddSeekPosition()
			args := builder.Build()

			hasSeek := argsContainFlag(args, "-ss")

			if hasSeek != tt.shouldContainSeek {
				t.Errorf("Seek flag presence = %v, want %v", hasSeek, tt.shouldContainSeek)
			}
		})
	}
}

func TestAddStreamMapping(t *testing.T) {
	tests := []struct {
		name                  string
		useSpecificAudioTrack bool
		audioTrackIndex       int
		expectedAudioMap      string
	}{
		{"Default audio", false, 0, "0:a:0"},
		{"Specific track 1", true, 1, "0:1"},
		{"Specific track 2", true, 2, "0:2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:               createTestProfile(),
				UseSpecificAudioTrack: tt.useSpecificAudioTrack,
				AudioTrackIndex:       tt.audioTrackIndex,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.AddStreamMapping()
			args := builder.Build()

			// Should always have video mapping
			if !argsContain(args, "-map", "0:v:0") {
				t.Error("Args should contain video mapping 0:v:0")
			}

			// Check audio mapping
			if !argsContain(args, "-map", tt.expectedAudioMap) {
				t.Errorf("Args should contain audio mapping %s", tt.expectedAudioMap)
			}
		})
	}
}

func TestAddVideoCodec(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddVideoCodec("libx264", "medium")
	args := builder.Build()

	if !argsContain(args, "-c:v", "libx264") {
		t.Error("Args should contain -c:v libx264")
	}

	if !argsContain(args, "-preset", "medium") {
		t.Error("Args should contain -preset medium")
	}
}

func TestAddVideoCodec_NoPreset(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddVideoCodec("libx264", "")
	args := builder.Build()

	if !argsContain(args, "-c:v", "libx264") {
		t.Error("Args should contain -c:v libx264")
	}

	if argsContainFlag(args, "-preset") {
		t.Error("Args should not contain -preset when empty")
	}
}

func TestAddH264Copy(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddH264Copy()
	args := builder.Build()

	if !argsContain(args, "-c:v", "copy") {
		t.Error("Args should contain -c:v copy")
	}

	if !argsContain(args, "-bsf:v", "h264_mp4toannexb") {
		t.Error("Args should contain H.264 bitstream filter")
	}
}

func TestAddHEVCCopy(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddHEVCCopy()
	args := builder.Build()

	if !argsContain(args, "-c:v", "copy") {
		t.Error("Args should contain -c:v copy")
	}

	if !argsContain(args, "-bsf:v", "hevc_mp4toannexb") {
		t.Error("Args should contain HEVC bitstream filter")
	}
}

func TestAddAudioCodec(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddAudioCodec("aac")
	args := builder.Build()

	if !argsContain(args, "-c:a", "aac") {
		t.Error("Args should contain -c:a aac")
	}
}

func TestAddAudioEncoding(t *testing.T) {
	profile := createTestProfile()
	opts := TranscodeOptions{
		Profile: profile,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddAudioEncoding()
	args := builder.Build()

	if !argsContain(args, "-c:a", "aac") {
		t.Error("Args should contain -c:a aac")
	}

	if !argsContain(args, "-b:a", "192k") {
		t.Error("Args should contain audio bitrate")
	}

	if !argsContain(args, "-ac", "2") {
		t.Error("Args should contain audio channels")
	}

	if !argsContain(args, "-ar", "48000") {
		t.Error("Args should contain audio sample rate")
	}
}

func TestAddAudioEncoding_ChannelDownmix(t *testing.T) {
	profile := createTestProfile()
	profile.AudioChannels = 6 // Request 5.1

	videoInfo := createTestVideoInfo("h264", 1920, 1080, false)
	videoInfo.AudioChannels = 2 // Source is stereo

	opts := TranscodeOptions{
		Profile:   profile,
		VideoInfo: videoInfo,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddAudioEncoding()
	args := builder.Build()

	// Should use source channels (2) not profile target (6)
	if !argsContain(args, "-ac", "2") {
		t.Error("Args should limit channels to source channels (2), not expand")
	}
}

func TestAddAudioDownmix(t *testing.T) {
	opts := TranscodeOptions{
		Profile: createTestProfile(),
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddAudioDownmix()
	args := builder.Build()

	if !argsContain(args, "-c:a", "aac") {
		t.Error("Args should contain -c:a aac")
	}

	if !argsContain(args, "-ac", "2") {
		t.Error("Args should force stereo (2 channels)")
	}
}

func TestAddVideoEncoding(t *testing.T) {
	profile := createTestProfile()
	opts := TranscodeOptions{
		Profile: profile,
	}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddVideoEncoding()
	args := builder.Build()

	// Should contain bitrate settings
	if !argsContain(args, "-b:v", "8000k") {
		t.Error("Args should contain video bitrate")
	}

	if !argsContain(args, "-maxrate", "8800k") {
		t.Error("Args should contain max bitrate")
	}

	if !argsContain(args, "-bufsize", "16000k") {
		t.Error("Args should contain buffer size")
	}

	// Should contain GOP settings
	if !argsContain(args, "-g", "60") {
		t.Error("Args should contain GOP size")
	}

	// Should contain H.264 profile settings
	if !argsContain(args, "-profile:v", "high") {
		t.Error("Args should contain H.264 profile")
	}
}

func TestAddHardwareAccel(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	hwArgs := []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}
	builder.AddHardwareAccel(hwArgs)
	args := builder.Build()

	if len(args) < 4 {
		t.Fatal("Args too short")
	}

	// Check all hardware accel args are present
	for _, expected := range hwArgs {
		if !argsContainFlag(args, expected) {
			t.Errorf("Args should contain %s", expected)
		}
	}
}

func TestAddOpenCLDevice(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddOpenCLDevice()
	args := builder.Build()

	if !argsContain(args, "-init_hw_device", "opencl=ocl:0.0") {
		t.Error("Args should initialize OpenCL device")
	}
}

func TestAddOpenCLFilterDevice(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddOpenCLFilterDevice()
	args := builder.Build()

	if !argsContain(args, "-filter_hw_device", "ocl") {
		t.Error("Args should set OpenCL filter device")
	}
}

func TestAddProgressReporting(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddProgressReporting()
	args := builder.Build()

	if !argsContain(args, "-progress", "pipe:2") {
		t.Error("Args should contain progress reporting")
	}

	if !argsContainFlag(args, "-stats") {
		t.Error("Args should contain -stats flag")
	}
}

func TestAddOverwriteOutput(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddOverwriteOutput()
	args := builder.Build()

	if !argsContainFlag(args, "-y") {
		t.Error("Args should contain -y flag")
	}
}

func TestAddCopyTimestamps(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddCopyTimestamps()
	args := builder.Build()

	if !argsContainFlag(args, "-copyts") {
		t.Error("Args should contain -copyts flag")
	}
}

func TestAddCopyTimestampsWithReset(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddCopyTimestampsWithReset()
	args := builder.Build()

	if !argsContainFlag(args, "-copyts") {
		t.Error("Args should contain -copyts flag")
	}

	if !argsContainFlag(args, "-start_at_zero") {
		t.Error("Args should contain -start_at_zero flag")
	}
}

func TestAddMemorySafetyOptions(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddMemorySafetyOptions(2048) // 2GB
	args := builder.Build()

	expectedMaxAlloc := "2147483648" // 2GB in bytes
	if !argsContain(args, "-max_alloc", expectedMaxAlloc) {
		t.Errorf("Args should contain -max_alloc %s", expectedMaxAlloc)
	}

	if !argsContain(args, "-thread_queue_size", "512") {
		t.Error("Args should contain thread queue size limit")
	}
}

func TestAddMemorySafetyOptions_DefaultValue(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddMemorySafetyOptions(0) // Use default
	args := builder.Build()

	// Should use default 2GB
	expectedMaxAlloc := "2147483648"
	if !argsContain(args, "-max_alloc", expectedMaxAlloc) {
		t.Errorf("Args should contain default -max_alloc %s", expectedMaxAlloc)
	}
}

func TestBuildToneMappingFilter_NoHDR(t *testing.T) {
	videoInfo := createTestVideoInfo("h264", 1920, 1080, false) // SDR content

	opts := TranscodeOptions{
		Profile:            createTestProfile(),
		VideoInfo:          videoInfo,
		ToneMappingEnabled: true,
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildToneMappingFilter(AccelNone)

	if filter != "" {
		t.Error("buildToneMappingFilter() should return empty string for SDR content")
	}
}

func TestBuildToneMappingFilter_Disabled(t *testing.T) {
	videoInfo := createTestVideoInfo("hevc", 3840, 2160, true) // HDR content

	opts := TranscodeOptions{
		Profile:            createTestProfile(),
		VideoInfo:          videoInfo,
		ToneMappingEnabled: false, // Disabled
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildToneMappingFilter(AccelNone)

	if filter != "" {
		t.Error("buildToneMappingFilter() should return empty string when disabled")
	}
}

func TestBuildToneMappingFilter_NoVideoInfo(t *testing.T) {
	opts := TranscodeOptions{
		Profile:            createTestProfile(),
		VideoInfo:          nil,
		ToneMappingEnabled: true,
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildToneMappingFilter(AccelNone)

	if filter != "" {
		t.Error("buildToneMappingFilter() should return empty string without VideoInfo")
	}
}

func TestBuildToneMappingFilter_SoftwareEncoding(t *testing.T) {
	videoInfo := createTestVideoInfo("hevc", 3840, 2160, true) // HDR content

	opts := TranscodeOptions{
		Profile:              createTestProfile(),
		VideoInfo:            videoInfo,
		ToneMappingEnabled:   true,
		ToneMappingAlgorithm: "hable",
		ToneMappingBackend:   "cpu", // Force CPU path to avoid libplacebo
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildToneMappingFilter(AccelNone)

	if filter == "" {
		t.Fatal("buildToneMappingFilter() should return filter chain for HDR content")
	}

	// Should contain CPU-based tone mapping components
	if !strings.Contains(filter, "zscale") {
		t.Error("Software tone mapping should use zscale")
	}

	if !strings.Contains(filter, "tonemap=hable") {
		t.Error("Software tone mapping should use specified algorithm")
	}
}

func TestBuildToneMappingFilter_NVENC(t *testing.T) {
	videoInfo := createTestVideoInfo("hevc", 3840, 2160, true)

	opts := TranscodeOptions{
		Profile:              createTestProfile(),
		VideoInfo:            videoInfo,
		ToneMappingEnabled:   true,
		ToneMappingAlgorithm: "mobius",
		ToneMappingBackend:   "opencl", // Force OpenCL path to avoid libplacebo
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildToneMappingFilter(AccelNVENC)

	if filter == "" {
		t.Fatal("buildToneMappingFilter() should return filter chain for NVENC HDR")
	}

	// NVENC should use OpenCL tone mapping with hwupload/hwdownload
	if !strings.Contains(filter, "tonemap_opencl") {
		t.Error("NVENC tone mapping should use tonemap_opencl")
	}

	if !strings.Contains(filter, "mobius") {
		t.Error("NVENC tone mapping should use specified algorithm")
	}
}

func TestBuildScalingFilter_SkipWhenNotNeeded(t *testing.T) {
	videoInfo := createTestVideoInfo("h264", 1920, 1080, false)

	profile := createTestProfile() // 1920x1080

	opts := TranscodeOptions{
		Profile:   profile,
		VideoInfo: videoInfo,
	}
	builder := NewFFmpegArgsBuilder(opts)

	// Source matches target - should skip for NVENC without HDR
	filter := builder.buildScalingFilter(AccelNVENC, true)

	// Should still return something for NVENC (needs format conversion)
	if filter == "" {
		t.Error("NVENC scaling filter should not be empty even when resolution matches")
	}
}

func TestBuildScalingFilter_Software(t *testing.T) {
	profile := createTestProfile()

	opts := TranscodeOptions{
		Profile: profile,
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildScalingFilter(AccelNone, false)

	if filter == "" {
		t.Fatal("buildScalingFilter() should return filter chain")
	}

	// Should contain CPU scaling
	if !strings.Contains(filter, "scale=") {
		t.Error("Software scaling should use scale filter")
	}

	if !strings.Contains(filter, "1920:1080") {
		t.Error("Scaling should use profile dimensions")
	}

	if !strings.Contains(filter, "pad=") {
		t.Error("Software scaling should include padding")
	}
}

func TestBuildScalingFilter_NVENC(t *testing.T) {
	profile := createTestProfile()

	opts := TranscodeOptions{
		Profile: profile,
	}
	builder := NewFFmpegArgsBuilder(opts)

	filter := builder.buildScalingFilter(AccelNVENC, false)

	if filter == "" {
		t.Fatal("buildScalingFilter() should return filter chain for NVENC")
	}

	// Should use CUDA-accelerated scaling
	if !strings.Contains(filter, "scale_cuda") {
		t.Error("NVENC scaling should use scale_cuda")
	}

	if !strings.Contains(filter, "pad_cuda") {
		t.Error("NVENC scaling should use pad_cuda")
	}
}

func TestGetToneMappingAlgorithm(t *testing.T) {
	tests := []struct {
		name       string
		algorithm  string
		expected   string
	}{
		{"Default empty", "", "hable"},
		{"Hable", "hable", "hable"},
		{"Reinhard", "reinhard", "reinhard"},
		{"Mobius", "mobius", "mobius"},
		{"BT.2390 - map to mobius", "bt.2390", "mobius"},
		{"BT.2446a - map to mobius", "bt.2446a", "mobius"},
		{"Unknown - default to hable", "unknown", "hable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:              createTestProfile(),
				ToneMappingAlgorithm: tt.algorithm,
			}
			builder := NewFFmpegArgsBuilder(opts)

			result := builder.getToneMappingAlgorithm()
			if result != tt.expected {
				t.Errorf("getToneMappingAlgorithm() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetToneMappingLibPlaceboAlgorithm(t *testing.T) {
	tests := []struct {
		name       string
		algorithm  string
		expected   string
	}{
		{"Default empty", "", "bt.2390"},
		{"BT.2390", "bt.2390", "bt.2390"},
		{"BT.2446a", "bt.2446a", "bt.2446a"},
		{"ST2094-40", "st2094-40", "st2094-40"},
		{"Spline", "spline", "spline"},
		{"Hable", "hable", "hable"},
		{"Mobius", "mobius", "mobius"},
		{"Reinhard", "reinhard", "reinhard"},
		{"Clip", "clip", "clip"},
		{"Unknown - default", "unknown", "bt.2390"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:              createTestProfile(),
				ToneMappingAlgorithm: tt.algorithm,
			}
			builder := NewFFmpegArgsBuilder(opts)

			result := builder.getToneMappingLibPlaceboAlgorithm()
			if result != tt.expected {
				t.Errorf("getToneMappingLibPlaceboAlgorithm() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNeedsHDRToneMapping(t *testing.T) {
	tests := []struct {
		name               string
		toneMappingEnabled bool
		videoInfo          *VideoInfo
		expected           bool
	}{
		{
			name:               "Disabled",
			toneMappingEnabled: false,
			videoInfo:          createTestVideoInfo("hevc", 3840, 2160, true),
			expected:           false,
		},
		{
			name:               "No VideoInfo",
			toneMappingEnabled: true,
			videoInfo:          nil,
			expected:           false,
		},
		{
			name:               "SDR content",
			toneMappingEnabled: true,
			videoInfo:          createTestVideoInfo("h264", 1920, 1080, false),
			expected:           false,
		},
		{
			name:               "HDR content",
			toneMappingEnabled: true,
			videoInfo:          createTestVideoInfo("hevc", 3840, 2160, true),
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:            createTestProfile(),
				ToneMappingEnabled: tt.toneMappingEnabled,
				VideoInfo:          tt.videoInfo,
			}
			builder := NewFFmpegArgsBuilder(opts)

			result := builder.needsHDRToneMapping()
			if result != tt.expected {
				t.Errorf("needsHDRToneMapping() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetVideoCodec_Default(t *testing.T) {
	opts := TranscodeOptions{
		Profile:    createTestProfile(),
		VideoCodec: "", // Not specified
	}
	builder := NewFFmpegArgsBuilder(opts)

	codec := builder.getVideoCodec()
	if codec != CodecH264 {
		t.Errorf("getVideoCodec() = %v, want %v (default)", codec, CodecH264)
	}
}

func TestGetVideoCodec_Specified(t *testing.T) {
	tests := []struct {
		name  string
		codec VideoCodec
	}{
		{"H.264", CodecH264},
		{"H.265", CodecH265},
		{"VP9", CodecVP9},
		{"AV1", CodecAV1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:    createTestProfile(),
				VideoCodec: tt.codec,
			}
			builder := NewFFmpegArgsBuilder(opts)

			result := builder.getVideoCodec()
			if result != tt.codec {
				t.Errorf("getVideoCodec() = %v, want %v", result, tt.codec)
			}
		})
	}
}

func TestAddFastInputOptions(t *testing.T) {
	opts := TranscodeOptions{Profile: createTestProfile()}
	builder := NewFFmpegArgsBuilder(opts)

	builder.AddFastInputOptions()
	args := builder.Build()

	if !argsContain(args, "-analyzeduration", "5000000") {
		t.Error("Args should contain analyzeduration limit")
	}

	if !argsContain(args, "-probesize", "5000000") {
		t.Error("Args should contain probesize limit")
	}

	if !argsContain(args, "-fflags", "+genpts+discardcorrupt") {
		t.Error("Args should contain fflags for stability")
	}
}

func TestAddNoAccurateSeek(t *testing.T) {
	tests := []struct {
		name             string
		useStartPosition bool
		startPosition    int
		shouldHaveFlag   bool
	}{
		{"No seek", false, 0, false},
		{"Seek at 0", true, 0, false},
		{"Seek at 60s", true, 60, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:          createTestProfile(),
				UseStartPosition: tt.useStartPosition,
				StartPosition:    tt.startPosition,
			}
			builder := NewFFmpegArgsBuilder(opts)

			builder.AddNoAccurateSeek()
			args := builder.Build()

			hasFlag := argsContainFlag(args, "-noaccurate_seek")
			if hasFlag != tt.shouldHaveFlag {
				t.Errorf("noaccurate_seek flag presence = %v, want %v", hasFlag, tt.shouldHaveFlag)
			}
		})
	}
}

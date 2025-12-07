package hls

import (
	"strings"
	"testing"
)

func TestFormatBitrate(t *testing.T) {
	tests := []struct {
		name string
		bps  int
		want string
	}{
		{"5Mbps", 5000000, "5000k"},
		{"15Mbps", 15000000, "15000k"},
		{"192kbps", 192000, "192k"},
		{"zero", 0, "0k"},
		{"1kbps", 1000, "1k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBitrate(tt.bps)
			if got != tt.want {
				t.Errorf("formatBitrate(%d) = %s, want %s", tt.bps, got, tt.want)
			}
		})
	}
}

func TestGetVideoCodec(t *testing.T) {
	tests := []struct {
		name       string
		videoCodec VideoCodec
		want       VideoCodec
	}{
		{"h264 explicit", CodecH264, CodecH264},
		{"h265 explicit", CodecH265, CodecH265},
		{"vp9 explicit", CodecVP9, CodecVP9},
		{"av1 explicit", CodecAV1, CodecAV1},
		{"empty defaults to h264", "", CodecH264},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				VideoCodec: tt.videoCodec,
			}
			builder := NewBuilder(opts)
			got := builder.getVideoCodec()
			if got != tt.want {
				t.Errorf("getVideoCodec() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestAddVideoEncoding(t *testing.T) {
	opts := Options{
		InputPath: "/test.mkv",
		OutputDir: "/out",
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
		VideoCodec: CodecH264,
	}

	args := NewBuilder(opts).AddVideoEncoding().Build()

	// Should contain profile, level, pixel format
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-profile:v", "high") {
		t.Error("Missing H.264 profile:v high")
	}
	if !containsArg(args, "-pix_fmt", "yuv420p") {
		t.Error("Missing pix_fmt yuv420p")
	}
	if !containsArg(args, "-b:v", "5000k") {
		t.Error("Missing video bitrate")
	}
	if !containsArg(args, "-maxrate", "7500k") {
		t.Error("Missing maxrate")
	}
	if !containsArg(args, "-bufsize", "10000k") {
		t.Error("Missing bufsize")
	}
	if !containsArg(args, "-g", "72") {
		t.Error("Missing GOP size")
	}
	if !containsArg(args, "-sc_threshold", "0") {
		t.Error("Missing scene threshold")
	}
}

func TestAddH264Profile(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		wantLevel string
	}{
		{"720p", 1280, 720, "4.0"},
		{"1080p", 1920, 1080, "4.1"},
		{"4K", 3840, 2160, "5.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{
					Width:  tt.width,
					Height: tt.height,
				},
			}
			builder := NewBuilder(opts)
			builder.addH264Profile(AccelNone)
			args := builder.Build()

			// Check for profile and level
			foundLevel := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-level" && args[i+1] == tt.wantLevel {
					foundLevel = true
					break
				}
			}
			if !foundLevel {
				t.Errorf("Expected level %s, args: %v", tt.wantLevel, args)
			}
		})
	}
}

func TestAddH265Profile(t *testing.T) {
	tests := []struct {
		name    string
		hwAccel HardwareAccel
		wantX265Params bool
	}{
		{"software", AccelNone, true},
		{"nvenc", AccelNVENC, false},
		{"vaapi", AccelVAAPI, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{Width: 1920, Height: 1080},
			}
			builder := NewBuilder(opts)
			builder.addH265Profile(tt.hwAccel)
			args := builder.Build()

			hasX265Params := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-x265-params" {
					hasX265Params = true
					break
				}
			}

			if hasX265Params != tt.wantX265Params {
				t.Errorf("x265-params present = %v, want %v", hasX265Params, tt.wantX265Params)
			}
		})
	}
}

func TestAddVP9Profile(t *testing.T) {
	tests := []struct {
		name    string
		hwAccel HardwareAccel
		wantRowMT bool
	}{
		{"software", AccelNone, true},
		{"vaapi", AccelVAAPI, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{Width: 1920, Height: 1080},
			}
			builder := NewBuilder(opts)
			builder.addVP9Profile(tt.hwAccel)
			args := builder.Build()

			hasRowMT := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-row-mt" {
					hasRowMT = true
					break
				}
			}

			if hasRowMT != tt.wantRowMT {
				t.Errorf("row-mt present = %v, want %v", hasRowMT, tt.wantRowMT)
			}
		})
	}
}

func TestAddAV1Profile(t *testing.T) {
	tests := []struct {
		name    string
		hwAccel HardwareAccel
		wantPreset bool
	}{
		{"software", AccelNone, true},
		{"nvenc", AccelNVENC, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{Width: 1920, Height: 1080},
			}
			builder := NewBuilder(opts)
			builder.addAV1Profile(tt.hwAccel)
			args := builder.Build()

			hasPreset := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-preset" && args[i+1] == "6" {
					hasPreset = true
					break
				}
			}

			if hasPreset != tt.wantPreset {
				t.Errorf("preset 6 present = %v, want %v", hasPreset, tt.wantPreset)
			}
		})
	}
}

func TestAddAudioEncoding(t *testing.T) {
	tests := []struct {
		name           string
		profileChannels int
		sourceChannels int
		wantChannels   string
	}{
		{"stereo to stereo", 2, 2, "2"},
		{"5.1 to stereo downmix", 2, 6, "2"},
		{"stereo source, 5.1 profile", 6, 2, "2"}, // Should use source channels
		{"no video info", 2, 0, "2"}, // Should use profile channels
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{
					AudioBitrate:    192000,
					AudioChannels:   tt.profileChannels,
					AudioSampleRate: 48000,
				},
			}
			if tt.sourceChannels > 0 {
				opts.VideoInfo = &VideoInfo{
					AudioChannels: tt.sourceChannels,
				}
			}

			args := NewBuilder(opts).AddAudioEncoding().Build()

			foundChannels := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-ac" && args[i+1] == tt.wantChannels {
					foundChannels = true
					break
				}
			}

			if !foundChannels {
				t.Errorf("Expected -ac %s, got args: %v", tt.wantChannels, args)
			}
		})
	}
}

func TestAddAudioDownmix(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			AudioBitrate:    192000,
			AudioSampleRate: 48000,
		},
	}

	args := NewBuilder(opts).AddAudioDownmix().Build()

	// Should force 2 channels
	foundStereo := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-ac" && args[i+1] == "2" {
			foundStereo = true
			break
		}
	}

	if !foundStereo {
		t.Error("AddAudioDownmix should force -ac 2")
	}
}

func TestAddVideoCodec(t *testing.T) {
	tests := []struct {
		name   string
		codec  string
		preset string
		wantPreset bool
	}{
		{"with preset", "libx264", "medium", true},
		{"without preset", "h264_nvenc", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{}
			args := NewBuilder(opts).AddVideoCodec(tt.codec, tt.preset).Build()

			hasCodec := false
			hasPreset := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-c:v" && args[i+1] == tt.codec {
					hasCodec = true
				}
				if args[i] == "-preset" {
					hasPreset = true
				}
			}

			if !hasCodec {
				t.Errorf("Missing codec %s", tt.codec)
			}
			if hasPreset != tt.wantPreset {
				t.Errorf("preset present = %v, want %v", hasPreset, tt.wantPreset)
			}
		})
	}
}

func TestAddAudioCodec(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddAudioCodec("aac").Build()

	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-c:a" && args[i+1] == "aac" {
			found = true
			break
		}
	}

	if !found {
		t.Error("AddAudioCodec should add -c:a aac")
	}
}

func TestAddCopyTimestamps(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddCopyTimestamps().Build()

	found := false
	for _, arg := range args {
		if arg == "-copyts" {
			found = true
			break
		}
	}

	if !found {
		t.Error("AddCopyTimestamps should add -copyts")
	}
}

func TestAddCopyTimestampsWithReset(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddCopyTimestampsWithReset().Build()

	hasCopyts := false
	hasStartAtZero := false
	for _, arg := range args {
		if arg == "-copyts" {
			hasCopyts = true
		}
		if arg == "-start_at_zero" {
			hasStartAtZero = true
		}
	}

	if !hasCopyts {
		t.Error("AddCopyTimestampsWithReset should add -copyts")
	}
	if !hasStartAtZero {
		t.Error("AddCopyTimestampsWithReset should add -start_at_zero")
	}
}

func TestAddBitrateArgs(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
		},
	}

	builder := NewBuilder(opts)
	builder.addBitrateArgs()
	args := builder.Build()

	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-b:v", "5000k") {
		t.Error("Missing -b:v 5000k")
	}
	if !containsArg(args, "-maxrate", "7500k") {
		t.Error("Missing -maxrate 7500k")
	}
	if !containsArg(args, "-bufsize", "10000k") {
		t.Error("Missing -bufsize 10000k")
	}
}

func TestAddCodecProfileArgs(t *testing.T) {
	tests := []struct {
		name    string
		codec   VideoCodec
		hwAccel HardwareAccel
		wantProfileOrTier bool
	}{
		{"h264", CodecH264, AccelNVENC, true},
		{"h265", CodecH265, AccelNVENC, true},
		{"vp9", CodecVP9, AccelVAAPI, false}, // VP9 doesn't use profile/level
		{"av1", CodecAV1, AccelNVENC, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{
					Width:  1920,
					Height: 1080,
				},
			}
			builder := NewBuilder(opts)
			builder.addCodecProfileArgs(tt.codec, tt.hwAccel)
			args := builder.Build()

			hasProfileOrTier := false
			for i := 0; i < len(args); i++ {
				if args[i] == "-profile:v" || args[i] == "-tier" || args[i] == "-level" {
					hasProfileOrTier = true
					break
				}
			}

			if hasProfileOrTier != tt.wantProfileOrTier {
				t.Errorf("profile/tier/level present = %v, want %v, args: %v", hasProfileOrTier, tt.wantProfileOrTier, args)
			}
		})
	}
}

func TestAddGOPArgs(t *testing.T) {
	tests := []struct {
		name                  string
		disableSceneDetection bool
		wantScThreshold       bool
	}{
		{"with scene detection disabled", true, true},
		{"with scene detection enabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{
					GOPSize: 72,
				},
			}
			builder := NewBuilder(opts)
			builder.addGOPArgs(tt.disableSceneDetection)
			args := builder.Build()

			hasScThreshold := false
			for i := 0; i < len(args)-1; i++ {
				if args[i] == "-sc_threshold" {
					hasScThreshold = true
					break
				}
			}

			if hasScThreshold != tt.wantScThreshold {
				t.Errorf("sc_threshold present = %v, want %v", hasScThreshold, tt.wantScThreshold)
			}
		})
	}
}

func TestAddOpenCLDevice(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddOpenCLDevice().Build()

	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-init_hw_device" && strings.Contains(args[i+1], "opencl") {
			found = true
			break
		}
	}

	if !found {
		t.Error("AddOpenCLDevice should add -init_hw_device opencl")
	}
}

func TestAddOpenCLFilterDevice(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddOpenCLFilterDevice().Build()

	found := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-filter_hw_device" && args[i+1] == "ocl" {
			found = true
			break
		}
	}

	if !found {
		t.Error("AddOpenCLFilterDevice should add -filter_hw_device ocl")
	}
}

func TestAddFastInputOptions(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddFastInputOptions().Build()

	hasAnalyzeduration := false
	hasProbesize := false
	hasFflags := false

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-analyzeduration" {
			hasAnalyzeduration = true
		}
		if args[i] == "-probesize" {
			hasProbesize = true
		}
		if args[i] == "-fflags" {
			hasFflags = true
		}
	}

	if !hasAnalyzeduration {
		t.Error("AddFastInputOptions should add -analyzeduration")
	}
	if !hasProbesize {
		t.Error("AddFastInputOptions should add -probesize")
	}
	if !hasFflags {
		t.Error("AddFastInputOptions should add -fflags")
	}
}

func TestAddVideoFilterChain(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:  1920,
			Height: 1080,
		},
		VideoInfo: &VideoInfo{
			Width:  3840,
			Height: 2160,
		},
	}

	builder := NewBuilder(opts)
	builder.addVideoFilterChain(AccelNone, false)
	args := builder.Build()

	// Should have -vf flag
	hasVF := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-vf" {
			hasVF = true
			break
		}
	}

	if !hasVF {
		t.Error("addVideoFilterChain should add -vf")
	}
}

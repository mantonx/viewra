package hls

import (
	"strings"
	"testing"
)

func TestAddHardwareVideoEncoding(t *testing.T) {
	tests := []struct {
		name        string
		hwAccel     HardwareAccel
		videoCodec  VideoCodec
		wantContains string
	}{
		{"nvenc h264", AccelNVENC, CodecH264, "-preset"},
		{"qsv h264", AccelQSV, CodecH264, "-global_quality"},
		{"vaapi h264", AccelVAAPI, CodecH264, "-quality"},
		{"videotoolbox h264", AccelVideoToolbox, CodecH264, "-b:v"},
		{"software fallback", AccelNone, CodecH264, "-profile:v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{
					Width:        1920,
					Height:       1080,
					VideoBitrate: 5000000,
					VideoMaxRate: 7500000,
					VideoBufSize: 10000000,
					GOPSize:      72,
				},
				VideoCodec: tt.videoCodec,
			}

			args := NewBuilder(opts).AddHardwareVideoEncoding(tt.hwAccel).Build()

			found := false
			for _, arg := range args {
				if strings.Contains(arg, tt.wantContains) || arg == tt.wantContains {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("AddHardwareVideoEncoding(%s) should contain %s, got: %v", tt.hwAccel, tt.wantContains, args)
			}
		})
	}
}

func TestAddNVENCEncoding(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
	}

	builder := NewBuilder(opts)
	builder.addNVENCEncoding(opts.Profile, CodecH264)
	args := builder.Build()

	// Check for NVENC-specific parameters
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-preset", "p4") {
		t.Error("Missing NVENC preset p4")
	}
	if !containsArg(args, "-tune", "hq") {
		t.Error("Missing NVENC tune hq")
	}
	if !containsArg(args, "-rc", "vbr") {
		t.Error("Missing NVENC rate control vbr")
	}
	if !containsArg(args, "-cq", "21") {
		t.Error("Missing NVENC quality level")
	}
	if !containsArg(args, "-spatial-aq", "1") {
		t.Error("Missing NVENC spatial-aq")
	}
	if !containsArg(args, "-temporal-aq", "1") {
		t.Error("Missing NVENC temporal-aq")
	}
	if !containsArg(args, "-aq-strength", "8") {
		t.Error("Missing NVENC aq-strength")
	}
	if !containsArg(args, "-rc-lookahead", "32") {
		t.Error("Missing NVENC rc-lookahead")
	}
	if !containsArg(args, "-b_ref_mode", "middle") {
		t.Error("Missing NVENC b_ref_mode")
	}
	if !containsArg(args, "-bf", "3") {
		t.Error("Missing NVENC b-frames")
	}
}

func TestAddNVENCEncodingH265(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
	}

	builder := NewBuilder(opts)
	builder.addNVENCEncoding(opts.Profile, CodecH265)
	args := builder.Build()

	// Should have H.265 profile and level
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-profile:v", "main") {
		t.Error("Missing H.265 profile main")
	}
	if !containsArg(args, "-level", "5.1") {
		t.Error("Missing H.265 level 5.1")
	}
}

func TestAddQSVEncoding(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
	}

	builder := NewBuilder(opts)
	builder.addQSVEncoding(opts.Profile, CodecH264)
	args := builder.Build()

	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-preset", "medium") {
		t.Error("Missing QSV preset medium")
	}
	if !containsArg(args, "-global_quality", "23") {
		t.Error("Missing QSV global_quality")
	}
	if !containsArg(args, "-b:v", "5000k") {
		t.Error("Missing QSV bitrate")
	}
}

func TestAddVAAPIEncoding(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
	}

	builder := NewBuilder(opts)
	builder.addVAAPIEncoding(opts.Profile, CodecH264)
	args := builder.Build()

	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-quality", "4") {
		t.Error("Missing VAAPI quality")
	}
	if !containsArg(args, "-b:v", "5000k") {
		t.Error("Missing VAAPI bitrate")
	}
}

func TestAddVideoToolboxEncoding(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
	}

	builder := NewBuilder(opts)
	builder.addVideoToolboxEncoding(opts.Profile, CodecH264)
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
		t.Error("Missing VideoToolbox bitrate")
	}
	// VideoToolbox uses CPU filters for scaling
	hasVF := false
	for _, arg := range args {
		if arg == "-vf" {
			hasVF = true
			break
		}
	}
	if !hasVF {
		t.Error("VideoToolbox should have video filter chain")
	}
}

func TestHardwareEncodingWithDifferentCodecs(t *testing.T) {
	tests := []struct {
		name       string
		hwAccel    HardwareAccel
		videoCodec VideoCodec
	}{
		{"nvenc h265", AccelNVENC, CodecH265},
		{"nvenc vp9", AccelNVENC, CodecVP9},
		{"nvenc av1", AccelNVENC, CodecAV1},
		{"vaapi h265", AccelVAAPI, CodecH265},
		{"vaapi vp9", AccelVAAPI, CodecVP9},
		{"qsv h265", AccelQSV, CodecH265},
		{"qsv vp9", AccelQSV, CodecVP9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Profile: &Profile{
					Width:        1920,
					Height:       1080,
					VideoBitrate: 5000000,
					VideoMaxRate: 7500000,
					VideoBufSize: 10000000,
					GOPSize:      72,
				},
				VideoCodec: tt.videoCodec,
			}

			// Should not panic
			args := NewBuilder(opts).AddHardwareVideoEncoding(tt.hwAccel).Build()

			// Should produce some args
			if len(args) == 0 {
				t.Errorf("AddHardwareVideoEncoding(%s, %s) produced no args", tt.hwAccel, tt.videoCodec)
			}
		})
	}
}

func TestHardwareEncodingWithHDR(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
		VideoInfo: &VideoInfo{
			Width:  3840,
			Height: 2160,
			IsHDR:  true,
		},
		VideoCodec:         CodecH264,
		ToneMappingEnabled: true,
		ToneMappingBackend: "opencl",
	}

	tests := []struct {
		name    string
		hwAccel HardwareAccel
	}{
		{"nvenc with hdr", AccelNVENC},
		{"vaapi with hdr", AccelVAAPI},
		{"qsv with hdr", AccelQSV},
		{"software with hdr", AccelNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic with HDR content
			args := NewBuilder(opts).AddHardwareVideoEncoding(tt.hwAccel).Build()

			// Should have video filter
			hasVF := false
			for _, arg := range args {
				if arg == "-vf" {
					hasVF = true
					break
				}
			}

			if !hasVF {
				t.Errorf("Hardware encoding with HDR should have -vf filter chain")
			}
		})
	}
}

func TestNVENCEncodingBitrateControl(t *testing.T) {
	// Test that NVENC properly sets bitrate control parameters
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 8000000,
			VideoMaxRate: 12000000,
			VideoBufSize: 16000000,
			GOPSize:      72,
		},
	}

	builder := NewBuilder(opts)
	builder.addNVENCEncoding(opts.Profile, CodecH264)
	args := builder.Build()

	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	// Check bitrate args are properly formatted
	if !containsArg(args, "-b:v", "8000k") {
		t.Error("Missing NVENC bitrate 8000k")
	}
	if !containsArg(args, "-maxrate", "12000k") {
		t.Error("Missing NVENC maxrate 12000k")
	}
	if !containsArg(args, "-bufsize", "16000k") {
		t.Error("Missing NVENC bufsize 16000k")
	}
}

func TestHardwareEncodingGOPStructure(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      120,
		},
	}

	tests := []struct {
		name    string
		hwAccel HardwareAccel
	}{
		{"nvenc", AccelNVENC},
		{"qsv", AccelQSV},
		{"vaapi", AccelVAAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(opts)
			builder.AddHardwareVideoEncoding(tt.hwAccel)
			args := builder.Build()

			containsArg := func(args []string, key, value string) bool {
				for i := 0; i < len(args)-1; i++ {
					if args[i] == key && args[i+1] == value {
						return true
					}
				}
				return false
			}

			if !containsArg(args, "-g", "120") {
				t.Errorf("%s should set GOP size to 120", tt.hwAccel)
			}
			if !containsArg(args, "-keyint_min", "120") {
				t.Errorf("%s should set keyint_min to 120", tt.hwAccel)
			}
		})
	}
}

func TestVAAPIEncodingVP9(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
		VideoCodec: CodecVP9,
	}

	builder := NewBuilder(opts)
	builder.addVAAPIEncoding(opts.Profile, CodecVP9)
	args := builder.Build()

	// VP9 should not have profile/level args
	hasProfile := false
	for i := 0; i < len(args); i++ {
		if args[i] == "-profile:v" {
			hasProfile = true
			break
		}
	}

	if hasProfile {
		t.Error("VP9 with VAAPI should not have -profile:v")
	}
}

func TestQSVEncodingAV1(t *testing.T) {
	opts := Options{
		Profile: &Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: 5000000,
			VideoMaxRate: 7500000,
			VideoBufSize: 10000000,
			GOPSize:      72,
		},
		VideoCodec: CodecAV1,
	}

	builder := NewBuilder(opts)
	builder.addQSVEncoding(opts.Profile, CodecAV1)
	args := builder.Build()

	// AV1 should have tier
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-tier", "main") {
		t.Error("AV1 should have -tier main")
	}
}

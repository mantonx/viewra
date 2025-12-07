package hls

import "testing"

func TestGetVideoCodecAndPreset(t *testing.T) {
	tests := []struct {
		hwAccel     HardwareAccel
		wantCodec   string
		wantPreset  string
	}{
		{AccelNone, "libx264", "medium"},
		{AccelNVENC, "h264_nvenc", ""},
		{AccelVAAPI, "h264_vaapi", ""},
		{AccelQSV, "h264_qsv", ""},
		{AccelVideoToolbox, "h264_videotoolbox", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.hwAccel), func(t *testing.T) {
			codec, preset := GetVideoCodecAndPreset(tt.hwAccel)
			if codec != tt.wantCodec {
				t.Errorf("GetVideoCodecAndPreset(%s) codec = %s, want %s", tt.hwAccel, codec, tt.wantCodec)
			}
			if preset != tt.wantPreset {
				t.Errorf("GetVideoCodecAndPreset(%s) preset = %s, want %s", tt.hwAccel, preset, tt.wantPreset)
			}
		})
	}
}

func TestGetVideoCodecAndPresetForCodec(t *testing.T) {
	tests := []struct {
		name       string
		hwAccel    HardwareAccel
		videoCodec VideoCodec
		wantCodec  string
	}{
		// H.264 encoders
		{"h264_software", AccelNone, CodecH264, "libx264"},
		{"h264_nvenc", AccelNVENC, CodecH264, "h264_nvenc"},
		{"h264_vaapi", AccelVAAPI, CodecH264, "h264_vaapi"},
		{"h264_qsv", AccelQSV, CodecH264, "h264_qsv"},
		{"h264_vtb", AccelVideoToolbox, CodecH264, "h264_videotoolbox"},

		// H.265/HEVC encoders
		{"h265_software", AccelNone, CodecH265, "libx265"},
		{"h265_nvenc", AccelNVENC, CodecH265, "hevc_nvenc"},
		{"h265_vaapi", AccelVAAPI, CodecH265, "hevc_vaapi"},
		{"h265_qsv", AccelQSV, CodecH265, "hevc_qsv"},
		{"h265_vtb", AccelVideoToolbox, CodecH265, "hevc_videotoolbox"},

		// VP9 encoders
		{"vp9_software", AccelNone, CodecVP9, "libvpx-vp9"},
		{"vp9_vaapi", AccelVAAPI, CodecVP9, "vp9_vaapi"},
		{"vp9_qsv", AccelQSV, CodecVP9, "vp9_qsv"},
		// NVENC and VideoToolbox don't support VP9, fall back to software
		{"vp9_nvenc_fallback", AccelNVENC, CodecVP9, "libvpx-vp9"},
		{"vp9_vtb_fallback", AccelVideoToolbox, CodecVP9, "libvpx-vp9"},

		// AV1 encoders
		{"av1_software", AccelNone, CodecAV1, "libsvtav1"},
		{"av1_nvenc", AccelNVENC, CodecAV1, "av1_nvenc"},
		{"av1_vaapi", AccelVAAPI, CodecAV1, "av1_vaapi"},
		{"av1_qsv", AccelQSV, CodecAV1, "av1_qsv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codec, _ := GetVideoCodecAndPresetForCodec(tt.hwAccel, tt.videoCodec)
			if codec != tt.wantCodec {
				t.Errorf("GetVideoCodecAndPresetForCodec(%s, %s) = %s, want %s",
					tt.hwAccel, tt.videoCodec, codec, tt.wantCodec)
			}
		})
	}
}

func TestIsCodecSupported(t *testing.T) {
	tests := []struct {
		hwAccel    HardwareAccel
		videoCodec VideoCodec
		want       bool
	}{
		// H.264 is universally supported
		{AccelNone, CodecH264, true},
		{AccelNVENC, CodecH264, true},
		{AccelVAAPI, CodecH264, true},

		// H.265 is widely supported
		{AccelNone, CodecH265, true},
		{AccelNVENC, CodecH265, true},

		// VP9 has limited hardware support
		{AccelNone, CodecVP9, true},
		{AccelVAAPI, CodecVP9, true},
		{AccelQSV, CodecVP9, true},
		{AccelNVENC, CodecVP9, false},       // Not supported
		{AccelVideoToolbox, CodecVP9, false}, // Not supported

		// AV1 is supported on all (though may require newer hardware)
		{AccelNone, CodecAV1, true},
		{AccelNVENC, CodecAV1, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.hwAccel)+"_"+string(tt.videoCodec), func(t *testing.T) {
			got := IsCodecSupported(tt.hwAccel, tt.videoCodec)
			if got != tt.want {
				t.Errorf("IsCodecSupported(%s, %s) = %v, want %v",
					tt.hwAccel, tt.videoCodec, got, tt.want)
			}
		})
	}
}

func TestGetHardwareAccelArgs(t *testing.T) {
	tests := []struct {
		hwAccel    HardwareAccel
		wantLen    int
		wantFirst  string
	}{
		{AccelNone, 0, ""},
		{AccelNVENC, 4, "-hwaccel"},
		{AccelVAAPI, 6, "-hwaccel"},
		{AccelQSV, 4, "-hwaccel"},
		{AccelVideoToolbox, 2, "-hwaccel"},
	}

	for _, tt := range tests {
		t.Run(string(tt.hwAccel), func(t *testing.T) {
			args := GetHardwareAccelArgs(tt.hwAccel)
			if len(args) != tt.wantLen {
				t.Errorf("GetHardwareAccelArgs(%s) len = %d, want %d", tt.hwAccel, len(args), tt.wantLen)
			}
			if tt.wantLen > 0 && args[0] != tt.wantFirst {
				t.Errorf("GetHardwareAccelArgs(%s)[0] = %s, want %s", tt.hwAccel, args[0], tt.wantFirst)
			}
		})
	}
}

func TestGetBestCodecForHardware(t *testing.T) {
	tests := []struct {
		name         string
		hwAccel      HardwareAccel
		clientCodecs []string
		want         VideoCodec
	}{
		{"prefer av1", AccelNVENC, []string{"av1", "h265", "h264"}, CodecAV1},
		{"prefer h265 when no av1", AccelNVENC, []string{"h265", "h264"}, CodecH265},
		{"fallback to h264", AccelNVENC, []string{"h264"}, CodecH264},
		{"vp9 on vaapi", AccelVAAPI, []string{"vp9", "h264"}, CodecVP9},
		{"vp9 not on nvenc", AccelNVENC, []string{"vp9", "h264"}, CodecH264}, // VP9 not supported on NVENC
		{"empty client codecs", AccelNone, []string{}, CodecH264},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBestCodecForHardware(tt.hwAccel, tt.clientCodecs)
			if got != tt.want {
				t.Errorf("GetBestCodecForHardware(%s, %v) = %s, want %s",
					tt.hwAccel, tt.clientCodecs, got, tt.want)
			}
		})
	}
}

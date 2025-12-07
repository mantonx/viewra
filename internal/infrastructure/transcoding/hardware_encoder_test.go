package transcoding

import (
	"testing"
)

func TestIsCodecSupported(t *testing.T) {
	tests := []struct {
		name       string
		hwAccel    HardwareAccel
		videoCodec VideoCodec
		expected   bool
	}{
		{"H.264 with VAAPI", AccelVAAPI, CodecH264, true},
		{"H.264 with NVENC", AccelNVENC, CodecH264, true},
		{"H.264 with QSV", AccelQSV, CodecH264, true},
		{"H.264 with VideoToolbox", AccelVideoToolbox, CodecH264, true},
		{"H.264 with software", AccelNone, CodecH264, true},
		{"H.265 with VAAPI", AccelVAAPI, CodecH265, true},
		{"H.265 with NVENC", AccelNVENC, CodecH265, true},
		{"H.265 with QSV", AccelQSV, CodecH265, true},
		{"H.265 with VideoToolbox", AccelVideoToolbox, CodecH265, true},
		{"H.265 with software", AccelNone, CodecH265, true},
		{"VP9 with VAAPI", AccelVAAPI, CodecVP9, true},
		{"VP9 with QSV", AccelQSV, CodecVP9, true},
		{"VP9 with software", AccelNone, CodecVP9, true},
		{"VP9 with NVENC", AccelNVENC, CodecVP9, false},
		{"VP9 with VideoToolbox", AccelVideoToolbox, CodecVP9, false},
		{"AV1 with VAAPI", AccelVAAPI, CodecAV1, true},
		{"AV1 with NVENC", AccelNVENC, CodecAV1, true},
		{"AV1 with QSV", AccelQSV, CodecAV1, true},
		{"AV1 with VideoToolbox", AccelVideoToolbox, CodecAV1, true},
		{"AV1 with software", AccelNone, CodecAV1, true},
		{"Invalid codec", AccelVAAPI, "invalid", false},
		{"Empty codec", AccelNVENC, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCodecSupported(tt.hwAccel, tt.videoCodec)
			if result != tt.expected {
				t.Errorf("IsCodecSupported(%v, %v) = %v, want %v",
					tt.hwAccel, tt.videoCodec, result, tt.expected)
			}
		})
	}
}

func TestGetBestCodecForHardware(t *testing.T) {
	tests := []struct {
		name          string
		hwAccel       HardwareAccel
		clientCodecs  []string
		expectedCodec VideoCodec
	}{
		{"All codecs - prefer AV1", AccelVAAPI, []string{"h264", "h265", "vp9", "av1"}, CodecAV1},
		{"AV1 only", AccelNVENC, []string{"av1"}, CodecAV1},
		{"H.265, VP9, H.264 - prefer H.265", AccelVAAPI, []string{"h264", "h265", "vp9"}, CodecH265},
		{"H.265 and H.264 - prefer H.265", AccelQSV, []string{"h264", "h265"}, CodecH265},
		{"VP9 and H.264 with VAAPI", AccelVAAPI, []string{"h264", "vp9"}, CodecVP9},
		{"VP9 and H.264 with QSV", AccelQSV, []string{"h264", "vp9"}, CodecVP9},
		{"VP9 and H.264 with NVENC - fallback to H.264", AccelNVENC, []string{"h264", "vp9"}, CodecH264},
		{"VP9 and H.264 with VideoToolbox - fallback to H.264", AccelVideoToolbox, []string{"h264", "vp9"}, CodecH264},
		{"H.264 only", AccelVAAPI, []string{"h264"}, CodecH264},
		{"Empty client codecs", AccelNVENC, []string{}, CodecH264},
		{"No matching codecs", AccelQSV, []string{"unknown", "invalid"}, CodecH264},
		{"Nil client codecs", AccelVideoToolbox, nil, CodecH264},
		{"Software - all codecs", AccelNone, []string{"h264", "h265", "vp9", "av1"}, CodecAV1},
		{"Software - VP9", AccelNone, []string{"h264", "vp9"}, CodecVP9},
		{"Case sensitive - no match", AccelVAAPI, []string{"H264", "H265"}, CodecH264},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBestCodecForHardware(tt.hwAccel, tt.clientCodecs)
			if result != tt.expectedCodec {
				t.Errorf("GetBestCodecForHardware(%v, %v) = %v, want %v",
					tt.hwAccel, tt.clientCodecs, result, tt.expectedCodec)
			}
		})
	}
}

func TestGetHardwareAccelArgs(t *testing.T) {
	tests := []struct {
		name         string
		hwAccel      HardwareAccel
		expectedArgs []string
	}{
		{"VAAPI", AccelVAAPI, []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128", "-hwaccel_output_format", "vaapi"}},
		{"NVENC", AccelNVENC, []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}},
		{"QSV", AccelQSV, []string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}},
		{"VideoToolbox", AccelVideoToolbox, []string{"-hwaccel", "videotoolbox"}},
		{"None", AccelNone, []string{}},
		{"Invalid", "invalid", []string{}},
		{"Empty", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHardwareAccelArgs(tt.hwAccel)
			if len(result) != len(tt.expectedArgs) {
				t.Errorf("GetHardwareAccelArgs(%v) returned %d args, want %d",
					tt.hwAccel, len(result), len(tt.expectedArgs))
				return
			}
			for i, arg := range result {
				if arg != tt.expectedArgs[i] {
					t.Errorf("GetHardwareAccelArgs(%v)[%d] = %s, want %s",
						tt.hwAccel, i, arg, tt.expectedArgs[i])
				}
			}
		})
	}
}

func TestGetHardwareAccelArgsWithDevice(t *testing.T) {
	tests := []struct {
		name         string
		hwAccel      HardwareAccel
		device       string
		expectedArgs []string
	}{
		{"VAAPI default", AccelVAAPI, "/dev/dri/renderD128", []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD128", "-hwaccel_output_format", "vaapi"}},
		{"VAAPI custom", AccelVAAPI, "/dev/dri/renderD129", []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/renderD129", "-hwaccel_output_format", "vaapi"}},
		{"VAAPI card0", AccelVAAPI, "/dev/dri/card0", []string{"-hwaccel", "vaapi", "-hwaccel_device", "/dev/dri/card0", "-hwaccel_output_format", "vaapi"}},
		{"VAAPI empty device", AccelVAAPI, "", []string{"-hwaccel", "vaapi", "-hwaccel_device", "", "-hwaccel_output_format", "vaapi"}},
		{"QSV ignores device", AccelQSV, "/dev/dri/renderD129", []string{"-hwaccel", "qsv", "-hwaccel_output_format", "qsv"}},
		{"NVENC ignores device", AccelNVENC, "/dev/dri/renderD128", []string{"-hwaccel", "cuda", "-hwaccel_output_format", "cuda"}},
		{"VideoToolbox ignores device", AccelVideoToolbox, "/dev/dri/renderD128", []string{"-hwaccel", "videotoolbox"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHardwareAccelArgsWithDevice(tt.hwAccel, tt.device)
			if len(result) != len(tt.expectedArgs) {
				t.Errorf("GetHardwareAccelArgsWithDevice(%v, %s) returned %d args, want %d",
					tt.hwAccel, tt.device, len(result), len(tt.expectedArgs))
				return
			}
			for i, arg := range result {
				if arg != tt.expectedArgs[i] {
					t.Errorf("GetHardwareAccelArgsWithDevice(%v, %s)[%d] = %s, want %s",
						tt.hwAccel, tt.device, i, arg, tt.expectedArgs[i])
				}
			}
		})
	}
}

func TestGetVideoCodecAndPresetForCodec(t *testing.T) {
	tests := []struct {
		name           string
		hwAccel        HardwareAccel
		videoCodec     VideoCodec
		expectedCodec  string
		expectedPreset string
	}{
		{"H.264 VAAPI", AccelVAAPI, CodecH264, "h264_vaapi", ""},
		{"H.264 NVENC", AccelNVENC, CodecH264, "h264_nvenc", ""},
		{"H.264 QSV", AccelQSV, CodecH264, "h264_qsv", ""},
		{"H.264 VideoToolbox", AccelVideoToolbox, CodecH264, "h264_videotoolbox", ""},
		{"H.264 software", AccelNone, CodecH264, "libx264", "medium"},
		{"H.265 VAAPI", AccelVAAPI, CodecH265, "hevc_vaapi", ""},
		{"H.265 NVENC", AccelNVENC, CodecH265, "hevc_nvenc", ""},
		{"H.265 QSV", AccelQSV, CodecH265, "hevc_qsv", ""},
		{"H.265 VideoToolbox", AccelVideoToolbox, CodecH265, "hevc_videotoolbox", ""},
		{"H.265 software", AccelNone, CodecH265, "libx265", "medium"},
		{"VP9 VAAPI", AccelVAAPI, CodecVP9, "vp9_vaapi", ""},
		{"VP9 QSV", AccelQSV, CodecVP9, "vp9_qsv", ""},
		{"VP9 NVENC fallback", AccelNVENC, CodecVP9, "libvpx-vp9", ""},
		{"VP9 VideoToolbox fallback", AccelVideoToolbox, CodecVP9, "libvpx-vp9", ""},
		{"VP9 software", AccelNone, CodecVP9, "libvpx-vp9", ""},
		{"AV1 VAAPI", AccelVAAPI, CodecAV1, "av1_vaapi", ""},
		{"AV1 NVENC", AccelNVENC, CodecAV1, "av1_nvenc", ""},
		{"AV1 QSV", AccelQSV, CodecAV1, "av1_qsv", ""},
		{"AV1 VideoToolbox fallback", AccelVideoToolbox, CodecAV1, "libsvtav1", ""},
		{"AV1 software", AccelNone, CodecAV1, "libsvtav1", ""},
		{"Unknown codec VAAPI", AccelVAAPI, "unknown", "h264_vaapi", ""},
		{"Empty codec NVENC", AccelNVENC, "", "h264_nvenc", ""},
		{"Invalid codec software", AccelNone, "invalid", "libx264", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codec, preset := GetVideoCodecAndPresetForCodec(tt.hwAccel, tt.videoCodec)
			if codec != tt.expectedCodec {
				t.Errorf("GetVideoCodecAndPresetForCodec(%v, %v) codec = %s, want %s",
					tt.hwAccel, tt.videoCodec, codec, tt.expectedCodec)
			}
			if preset != tt.expectedPreset {
				t.Errorf("GetVideoCodecAndPresetForCodec(%v, %v) preset = %s, want %s",
					tt.hwAccel, tt.videoCodec, preset, tt.expectedPreset)
			}
		})
	}
}

func TestGetVideoCodecAndPreset(t *testing.T) {
	tests := []struct {
		name           string
		hwAccel        HardwareAccel
		expectedCodec  string
		expectedPreset string
	}{
		{"VAAPI", AccelVAAPI, "h264_vaapi", ""},
		{"NVENC", AccelNVENC, "h264_nvenc", ""},
		{"QSV", AccelQSV, "h264_qsv", ""},
		{"VideoToolbox", AccelVideoToolbox, "h264_videotoolbox", ""},
		{"Software", AccelNone, "libx264", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codec, preset := GetVideoCodecAndPreset(tt.hwAccel)
			if codec != tt.expectedCodec {
				t.Errorf("GetVideoCodecAndPreset(%v) codec = %s, want %s",
					tt.hwAccel, codec, tt.expectedCodec)
			}
			if preset != tt.expectedPreset {
				t.Errorf("GetVideoCodecAndPreset(%v) preset = %s, want %s",
					tt.hwAccel, preset, tt.expectedPreset)
			}
		})
	}
}

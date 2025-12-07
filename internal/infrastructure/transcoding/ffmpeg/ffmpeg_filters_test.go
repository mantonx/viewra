package ffmpeg

import (
	"strings"
	"testing"
)

func TestBuildToneMappingFilter(t *testing.T) {
	videoInfo := createTestVideoInfo("hevc", 3840, 2160, true)

	tests := []struct {
		name           string
		hwAccel        HardwareAccel
		backend        string
		algorithm      string
		peakDetect     bool
		contrast       float64
		mustContain    []string
		mustNotContain []string
	}{
		{
			name:      "NVENC libplacebo",
			hwAccel:   AccelNVENC,
			backend:   "libplacebo",
			algorithm: "bt.2390",
			peakDetect: true,
			contrast:  0.5,
			mustContain: []string{
				"hwdownload", "format=p010le", "libplacebo", "w=1920:h=1080",
				"tonemapping=bt.2390", "peak_detect=true", "contrast_recovery=0.50",
				"format=yuv420p", "hwupload_cuda",
			},
		},
		{
			name:       "NVENC libplacebo peak detect disabled",
			hwAccel:    AccelNVENC,
			backend:    "libplacebo",
			algorithm:  "hable",
			peakDetect: false,
			contrast:   0.3,
			mustContain: []string{"peak_detect=false", "tonemapping=hable", "contrast_recovery=0.30"},
		},
		{
			name:      "Software libplacebo",
			hwAccel:   AccelNone,
			backend:   "libplacebo",
			algorithm: "spline",
			peakDetect: true,
			contrast:  0.7,
			mustContain: []string{
				"libplacebo", "tonemapping=spline", "peak_detect=true", "contrast_recovery=0.70",
			},
			mustNotContain: []string{"hwdownload", "hwupload"},
		},
		{
			name:      "QSV OpenCL",
			hwAccel:   AccelQSV,
			backend:   "opencl",
			algorithm: "reinhard",
			mustContain: []string{
				"hwdownload", "format=p010le", "hwupload", "tonemap_opencl",
				"tonemap=reinhard", "format=nv12", "extra_hw_frames=64",
			},
		},
		{
			name:           "VAAPI native",
			hwAccel:        AccelVAAPI,
			backend:        "auto",
			algorithm:      "mobius",
			mustContain:    []string{"tonemap_vaapi", "mobius"},
			mustNotContain: []string{"hwdownload"},
		},
		{
			name:      "VideoToolbox CPU",
			hwAccel:   AccelVideoToolbox,
			backend:   "cpu",
			algorithm: "hable",
			mustContain: []string{
				"zscale", "tonemap=hable", "t=linear", "format=gbrpf32le",
				"p=bt709", "m=bt709", "r=tv",
			},
		},
		{
			name:      "NVENC OpenCL",
			hwAccel:   AccelNVENC,
			backend:   "opencl",
			algorithm: "hable",
			mustContain: []string{"tonemap_opencl", "hwdownload", "hwupload"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := TranscodeOptions{
				Profile:                    createTestProfile(),
				VideoInfo:                  videoInfo,
				ToneMappingEnabled:         true,
				ToneMappingAlgorithm:       tt.algorithm,
				ToneMappingBackend:         tt.backend,
				LibPlaceboPeakDetect:       tt.peakDetect,
				LibPlaceboContrastRecovery: tt.contrast,
			}
			builder := NewFFmpegArgsBuilder(opts)
			filter := builder.buildToneMappingFilter(tt.hwAccel)

			if filter == "" {
				t.Fatal("expected non-empty filter")
			}

			for _, mustHave := range tt.mustContain {
				if !strings.Contains(filter, mustHave) {
					t.Errorf("filter missing %q, got: %s", mustHave, filter)
				}
			}

			for _, mustNotHave := range tt.mustNotContain {
				if strings.Contains(filter, mustNotHave) {
					t.Errorf("filter should not contain %q, got: %s", mustNotHave, filter)
				}
			}
		})
	}
}

func TestBuildScalingFilter(t *testing.T) {
	tests := []struct {
		name        string
		hwAccel     HardwareAccel
		width       int
		height      int
		isHDR       bool
		backend     string
		skipLibP    bool
		mustContain []string
		wantEmpty   bool
	}{
		{
			name:    "NVENC with HDR",
			hwAccel: AccelNVENC,
			width:   3840,
			height:  2160,
			isHDR:   true,
			backend: "opencl",
			mustContain: []string{
				"hwupload_cuda", "scale_cuda=1920:1080", "pad_cuda=1920:1080",
				"force_original_aspect_ratio=decrease",
			},
		},
		{
			name:      "NVENC matching resolution",
			hwAccel:   AccelNVENC,
			width:     1920,
			height:    1080,
			isHDR:     true,
			backend:   "opencl",
			mustContain: []string{"hwupload_cuda"},
		},
		{
			name:    "NVENC without HDR",
			hwAccel: AccelNVENC,
			width:   3840,
			height:  2160,
			mustContain: []string{"scale_cuda", "format=nv12", "pad_cuda"},
		},
		{
			name:    "QSV",
			hwAccel: AccelQSV,
			width:   3840,
			height:  2160,
			mustContain: []string{"scale_qsv", "w=1920:h=1080", "format=nv12"},
		},
		{
			name:    "VAAPI",
			hwAccel: AccelVAAPI,
			width:   3840,
			height:  2160,
			mustContain: []string{
				"scale_vaapi", "w=1920:h=1080", "format=nv12",
				"pad_vaapi", "width=1920:height=1080", "x=(ow-iw)/2:y=(oh-ih)/2",
			},
		},
		{
			name:    "VideoToolbox",
			hwAccel: AccelVideoToolbox,
			width:   3840,
			height:  2160,
			mustContain: []string{
				"scale=1920:1080", "pad=1920:1080", "format=yuv420p",
				"force_original_aspect_ratio=decrease",
			},
		},
		{
			name:    "Software",
			hwAccel: AccelNone,
			width:   3840,
			height:  2160,
			mustContain: []string{"scale=1920:1080", "pad=1920:1080", "format=yuv420p"},
		},
		{
			name:      "Skip when libplacebo handled",
			hwAccel:   AccelNone,
			width:     3840,
			height:    2160,
			isHDR:     true,
			backend:   "libplacebo",
			skipLibP:  true,
			wantEmpty: true,
		},
		{
			name:     "No skip when libplacebo not used",
			hwAccel:  AccelNone,
			width:    3840,
			height:   2160,
			isHDR:    true,
			backend:  "cpu",
			skipLibP: true,
			mustContain: []string{"scale="},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var videoInfo *VideoInfo
			if tt.isHDR {
				videoInfo = createTestVideoInfo("hevc", tt.width, tt.height, true)
			}

			opts := TranscodeOptions{
				Profile:              createTestProfile(),
				VideoInfo:            videoInfo,
				ToneMappingEnabled:   tt.isHDR,
				ToneMappingAlgorithm: "hable",
				ToneMappingBackend:   tt.backend,
			}
			builder := NewFFmpegArgsBuilder(opts)
			filter := builder.buildScalingFilter(tt.hwAccel, tt.skipLibP)

			if tt.wantEmpty {
				if filter != "" {
					t.Errorf("expected empty filter, got: %s", filter)
				}
				return
			}

			if filter == "" {
				t.Fatal("expected non-empty filter")
			}

			for _, mustHave := range tt.mustContain {
				if !strings.Contains(filter, mustHave) {
					t.Errorf("filter missing %q, got: %s", mustHave, filter)
				}
			}
		})
	}
}

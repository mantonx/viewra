package hls

import (
	"strings"
	"testing"
)

func TestNeedsHDRToneMapping(t *testing.T) {
	tests := []struct {
		name               string
		toneMappingEnabled bool
		videoInfo          *VideoInfo
		want               bool
	}{
		{
			name:               "disabled tone mapping",
			toneMappingEnabled: false,
			videoInfo:          &VideoInfo{IsHDR: true},
			want:               false,
		},
		{
			name:               "no video info",
			toneMappingEnabled: true,
			videoInfo:          nil,
			want:               false,
		},
		{
			name:               "SDR content",
			toneMappingEnabled: true,
			videoInfo:          &VideoInfo{IsHDR: false},
			want:               false,
		},
		{
			name:               "HDR content with tone mapping enabled",
			toneMappingEnabled: true,
			videoInfo:          &VideoInfo{IsHDR: true},
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingEnabled: tt.toneMappingEnabled,
				VideoInfo:          tt.videoInfo,
			}
			builder := NewBuilder(opts)
			got := builder.needsHDRToneMapping()
			if got != tt.want {
				t.Errorf("needsHDRToneMapping() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetToneMappingAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		want      string
	}{
		{"default", "", "hable"},
		{"hable", "hable", "hable"},
		{"reinhard", "reinhard", "reinhard"},
		{"mobius", "mobius", "mobius"},
		{"linear", "linear", "linear"},
		{"gamma", "gamma", "gamma"},
		{"clip", "clip", "clip"},
		{"none", "none", "none"},
		{"bt.2390 maps to mobius", "bt.2390", "mobius"},
		{"bt2390 maps to mobius", "bt2390", "mobius"},
		{"bt.2446a maps to mobius", "bt.2446a", "mobius"},
		{"spline maps to mobius", "spline", "mobius"},
		{"unknown maps to hable", "unknown", "hable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingAlgorithm: tt.algorithm,
			}
			builder := NewBuilder(opts)
			got := builder.getToneMappingAlgorithm()
			if got != tt.want {
				t.Errorf("getToneMappingAlgorithm() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetToneMappingLibPlaceboAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		want      string
	}{
		{"default", "", "bt.2390"},
		{"bt.2390", "bt.2390", "bt.2390"},
		{"bt2390", "bt2390", "bt.2390"},
		{"bt.2446a", "bt.2446a", "bt.2446a"},
		{"bt2446a alt", "bt2446a", "bt.2446a"},
		{"st2094-40", "st2094-40", "st2094-40"},
		{"st2094_40", "st2094_40", "st2094-40"},
		{"st2094-10", "st2094-10", "st2094-10"},
		{"spline", "spline", "spline"},
		{"reinhard", "reinhard", "reinhard"},
		{"mobius", "mobius", "mobius"},
		{"hable", "hable", "hable"},
		{"gamma", "gamma", "gamma"},
		{"linear", "linear", "linear"},
		{"clip", "clip", "clip"},
		{"none", "none", "clip"},
		{"unknown", "unknown", "bt.2390"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingAlgorithm: tt.algorithm,
			}
			builder := NewBuilder(opts)
			got := builder.getToneMappingLibPlaceboAlgorithm()
			if got != tt.want {
				t.Errorf("getToneMappingLibPlaceboAlgorithm() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestGetToneMappingCudaAlgorithm(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		want      string
	}{
		{"default", "", "bt2390"},
		{"bt.2390", "bt.2390", "bt2390"},
		{"bt2390", "bt2390", "bt2390"},
		{"reinhard", "reinhard", "reinhard"},
		{"hable", "hable", "hable"},
		{"mobius", "mobius", "mobius"},
		{"linear", "linear", "linear"},
		{"gamma", "gamma", "gamma"},
		{"clip", "clip", "clip"},
		{"none", "none", "clip"},
		{"unknown", "unknown", "bt2390"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingAlgorithm: tt.algorithm,
			}
			builder := NewBuilder(opts)
			got := builder.getToneMappingCudaAlgorithm()
			if got != tt.want {
				t.Errorf("getToneMappingCudaAlgorithm() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestShouldUseToneCuda(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		want    bool // Will depend on whether tonemap_cuda filter is available
	}{
		{"backend cuda", "cuda", false}, // False because CheckFFmpegFilter will fail in tests
		{"backend auto", "auto", false}, // False because CheckFFmpegFilter will fail in tests
		{"backend libplacebo", "libplacebo", false},
		{"backend opencl", "opencl", false},
		{"backend cpu", "cpu", false},
		{"backend empty defaults to auto", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingBackend: tt.backend,
			}
			builder := NewBuilder(opts)
			got := builder.shouldUseToneCuda()
			// In test environment, tonemap_cuda won't be available, so expect false
			if got != tt.want {
				t.Errorf("shouldUseToneCuda() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldUseLibPlacebo(t *testing.T) {
	// Check if libplacebo is available in this environment
	hasLibPlacebo := CheckFFmpegFilter("libplacebo")

	tests := []struct {
		name    string
		backend string
		hwAccel HardwareAccel
		want    bool
	}{
		{"backend libplacebo", "libplacebo", AccelNone, hasLibPlacebo},
		{"backend auto with software", "auto", AccelNone, hasLibPlacebo},
		{"backend auto with nvenc", "auto", AccelNVENC, hasLibPlacebo},
		{"backend auto with vaapi", "auto", AccelVAAPI, false}, // VAAPI uses native
		{"backend auto with qsv", "auto", AccelQSV, false},      // QSV uses native
		{"backend opencl", "opencl", AccelNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingBackend: tt.backend,
			}
			builder := NewBuilder(opts)
			got := builder.shouldUseLibPlacebo(tt.hwAccel)
			if got != tt.want {
				t.Errorf("shouldUseLibPlacebo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildToneMappingFilter(t *testing.T) {
	// Check available filters
	hasLibPlacebo := CheckFFmpegFilter("libplacebo")

	tests := []struct {
		name     string
		opts     Options
		hwAccel  HardwareAccel
		wantContains string
		alternateContains string // Alternative match if libplacebo is available
	}{
		{
			name: "no tone mapping when disabled",
			opts: Options{
				ToneMappingEnabled: false,
				VideoInfo:          &VideoInfo{IsHDR: true},
				Profile:            &Profile{Width: 1920, Height: 1080},
			},
			hwAccel:      AccelNone,
			wantContains: "",
		},
		{
			name: "no tone mapping for SDR",
			opts: Options{
				ToneMappingEnabled: true,
				VideoInfo:          &VideoInfo{IsHDR: false},
				Profile:            &Profile{Width: 1920, Height: 1080},
			},
			hwAccel:      AccelNone,
			wantContains: "",
		},
		{
			name: "software tone mapping",
			opts: Options{
				ToneMappingEnabled:   true,
				ToneMappingAlgorithm: "hable",
				VideoInfo:            &VideoInfo{IsHDR: true},
				Profile:              &Profile{Width: 1920, Height: 1080},
			},
			hwAccel:      AccelNone,
			wantContains: "zscale",
			alternateContains: "libplacebo", // If libplacebo is available, it will be used instead
		},
		{
			name: "vaapi tone mapping",
			opts: Options{
				ToneMappingEnabled:   true,
				ToneMappingAlgorithm: "hable",
				VideoInfo:            &VideoInfo{IsHDR: true},
				Profile:              &Profile{Width: 1920, Height: 1080},
			},
			hwAccel:      AccelVAAPI,
			wantContains: "tonemap_vaapi",
		},
		{
			name: "nvenc with opencl tone mapping",
			opts: Options{
				ToneMappingEnabled:   true,
				ToneMappingAlgorithm: "hable",
				ToneMappingBackend:   "opencl",
				VideoInfo:            &VideoInfo{IsHDR: true},
				Profile:              &Profile{Width: 1920, Height: 1080},
			},
			hwAccel:      AccelNVENC,
			wantContains: "tonemap_opencl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(tt.opts)
			got := builder.buildToneMappingFilter(tt.hwAccel)

			if tt.wantContains == "" {
				if got != "" {
					t.Errorf("buildToneMappingFilter() = %s, want empty", got)
				}
			} else {
				// Check for primary match or alternate match
				matched := strings.Contains(got, tt.wantContains)
				if !matched && tt.alternateContains != "" && hasLibPlacebo {
					matched = strings.Contains(got, tt.alternateContains)
				}
				if !matched {
					t.Errorf("buildToneMappingFilter() = %s, want to contain %s (or %s if libplacebo available)", got, tt.wantContains, tt.alternateContains)
				}
			}
		})
	}
}

func TestBuildScalingFilter(t *testing.T) {
	tests := []struct {
		name             string
		opts             Options
		hwAccel          HardwareAccel
		skipIfLibPlacebo bool
		wantContains     string
	}{
		{
			name: "software scaling",
			opts: Options{
				Profile: &Profile{Width: 1920, Height: 1080},
				VideoInfo: &VideoInfo{Width: 3840, Height: 2160},
			},
			hwAccel:          AccelNone,
			skipIfLibPlacebo: false,
			wantContains:     "scale=",
		},
		{
			name: "nvenc scaling",
			opts: Options{
				Profile: &Profile{Width: 1920, Height: 1080},
				VideoInfo: &VideoInfo{Width: 3840, Height: 2160},
			},
			hwAccel:          AccelNVENC,
			skipIfLibPlacebo: false,
			wantContains:     "scale_cuda",
		},
		{
			name: "vaapi scaling",
			opts: Options{
				Profile: &Profile{Width: 1920, Height: 1080},
				VideoInfo: &VideoInfo{Width: 3840, Height: 2160},
			},
			hwAccel:          AccelVAAPI,
			skipIfLibPlacebo: false,
			wantContains:     "scale_vaapi",
		},
		{
			name: "qsv scaling",
			opts: Options{
				Profile: &Profile{Width: 1920, Height: 1080},
				VideoInfo: &VideoInfo{Width: 3840, Height: 2160},
			},
			hwAccel:          AccelQSV,
			skipIfLibPlacebo: false,
			wantContains:     "scale_qsv",
		},
		{
			name: "same resolution still scales for format conversion",
			opts: Options{
				Profile: &Profile{Width: 1920, Height: 1080},
				VideoInfo: &VideoInfo{Width: 1920, Height: 1080},
			},
			hwAccel:          AccelNVENC,
			skipIfLibPlacebo: false,
			wantContains:     "scale_cuda", // NVENC still needs scale_cuda for format conversion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewBuilder(tt.opts)
			got := builder.buildScalingFilter(tt.hwAccel, tt.skipIfLibPlacebo)

			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("buildScalingFilter() = %s, want to contain %s", got, tt.wantContains)
			}
		})
	}
}

func TestBuildScalingFilterNoScalingNeeded(t *testing.T) {
	// Test case where source resolution matches target (no scaling needed)
	opts := Options{
		Profile: &Profile{Width: 1920, Height: 1080},
		VideoInfo: &VideoInfo{Width: 1920, Height: 1080},
	}

	builder := NewBuilder(opts)
	// For non-HDR NVENC, should still have scaling filters (for format conversion)
	got := builder.buildScalingFilter(AccelNVENC, false)

	// NVENC always adds scale_cuda for format conversion even if resolution matches
	if !strings.Contains(got, "scale_cuda") {
		t.Errorf("buildScalingFilter() should include scale_cuda for NVENC, got: %s", got)
	}
}

func TestBuildScalingFilterWithHDRToneMapping(t *testing.T) {
	// When HDR tone mapping is active, scaling behavior changes
	opts := Options{
		Profile: &Profile{Width: 1920, Height: 1080},
		VideoInfo: &VideoInfo{
			Width:  3840,
			Height: 2160,
			IsHDR:  true,
		},
		ToneMappingEnabled: true,
		ToneMappingBackend: "opencl",
	}

	builder := NewBuilder(opts)
	got := builder.buildScalingFilter(AccelNVENC, false)

	// With HDR tone mapping on NVENC, should have hwupload_cuda and scaling
	if !strings.Contains(got, "scale_cuda") {
		t.Errorf("buildScalingFilter() should include scale_cuda after HDR tone mapping, got: %s", got)
	}
}

func TestToneMappingFilterIntegration(t *testing.T) {
	// Test the full tone mapping + scaling pipeline
	opts := Options{
		Profile: &Profile{Width: 1920, Height: 1080},
		VideoInfo: &VideoInfo{
			Width:  3840,
			Height: 2160,
			IsHDR:  true,
		},
		ToneMappingEnabled:         true,
		ToneMappingAlgorithm:       "bt.2390",
		ToneMappingBackend:         "auto",
		LibPlaceboPeakDetect:       true,
		LibPlaceboContrastRecovery: 0.3,
	}

	builder := NewBuilder(opts)

	// Test software encoding path
	toneFilter := builder.buildToneMappingFilter(AccelNone)
	_ = builder.buildScalingFilter(AccelNone, true)

	// Software should use zscale or libplacebo for tone mapping
	hasLibPlacebo := CheckFFmpegFilter("libplacebo")
	if hasLibPlacebo {
		if !strings.Contains(toneFilter, "libplacebo") {
			t.Errorf("Software tone mapping with libplacebo should use libplacebo, got: %s", toneFilter)
		}
	} else {
		if !strings.Contains(toneFilter, "zscale") {
			t.Errorf("Software tone mapping should use zscale, got: %s", toneFilter)
		}
	}
}

func TestShouldUseOpenCL(t *testing.T) {
	// Check if tonemap_opencl is available in this environment
	hasOpenCLFilter := CheckFFmpegFilter("tonemap_opencl")

	tests := []struct {
		name    string
		backend string
		want    bool
	}{
		{"backend opencl explicit", "opencl", hasOpenCLFilter}, // Returns true if filter is available
		{"backend auto", "auto", hasOpenCLFilter},              // Auto mode uses opencl if available
		{"backend cuda", "cuda", false},                        // Explicitly set to cuda, should not use opencl
		{"backend libplacebo", "libplacebo", false},            // Explicitly set to libplacebo, should not use opencl
		{"backend cpu", "cpu", false},                          // Explicitly set to cpu, should not use opencl
		{"backend empty defaults to auto", "", hasOpenCLFilter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				ToneMappingBackend: tt.backend,
			}
			builder := NewBuilder(opts)
			got := builder.shouldUseOpenCL()
			if got != tt.want {
				t.Errorf("shouldUseOpenCL() = %v, want %v (hasOpenCLFilter=%v)", got, tt.want, hasOpenCLFilter)
			}
		})
	}
}

func TestShouldUseOpenCL_ExplicitNonOpenCL(t *testing.T) {
	// Test that explicitly setting backends other than "auto" and "opencl"
	// returns false without checking for the filter
	backends := []string{"cuda", "libplacebo", "cpu", "native"}

	for _, backend := range backends {
		t.Run(backend, func(t *testing.T) {
			opts := Options{
				ToneMappingBackend: backend,
			}
			builder := NewBuilder(opts)
			got := builder.shouldUseOpenCL()
			if got {
				t.Errorf("shouldUseOpenCL() = true for backend %s, want false", backend)
			}
		})
	}
}

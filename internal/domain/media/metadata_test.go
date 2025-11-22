package media

import "testing"

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		filename   string
		wantSource string
	}{
		{"Movie.2020.1080p.BluRay.x264.mkv", "BluRay"},
		{"Show.S01E01.1080p.WEB-DL.DD5.1.H264.mkv", "WEB-DL"},
		{"Show.S01E01.1080p.WEBRip.x264.mkv", "WEBRip"},
		{"Show.S01E01.HDTV.x264.mkv", "HDTV"},
		{"Movie.2020.DVDRip.XviD.avi", "DVDRip"},
		{"Movie.2020.REMUX.mkv", "Remux"},
		{"Movie.2020.1080p.mkv", ""}, // No source indicator
		{"Movie.2020.BluRay.Remux.mkv", "BluRay"}, // BluRay has priority
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := DetectSourceType(tt.filename)
			if got != tt.wantSource {
				t.Errorf("DetectSourceType(%q) = %q, want %q", tt.filename, got, tt.wantSource)
			}
		})
	}
}

func TestDetect3D(t *testing.T) {
	tests := []struct {
		filename       string
		want3D         bool
		wantStereoMode string
	}{
		{"Movie.2020.3D.mkv", true, ""},
		{"Movie.2020.3D.HSBS.mkv", true, "half-sbs"},
		{"Movie.2020.3D.SBS.mkv", true, "sbs"},
		{"Movie.2020.3D.HTAB.mkv", true, "half-tab"},
		{"Movie.2020.3D.TAB.mkv", true, "tab"},
		{"Movie.2020.3D.MVC.mkv", true, "mvc"},
		{"Movie.2020.1080p.mkv", false, ""},
		{"Movie.2020.HALF.SBS.mkv", true, "half-sbs"},
		{"Movie.2020.Side.By.Side.mkv", true, "sbs"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got3D, gotStereoMode := Detect3D(tt.filename)
			if got3D != tt.want3D {
				t.Errorf("Detect3D(%q) is3D = %v, want %v", tt.filename, got3D, tt.want3D)
			}
			if gotStereoMode != tt.wantStereoMode {
				t.Errorf("Detect3D(%q) stereoMode = %q, want %q", tt.filename, gotStereoMode, tt.wantStereoMode)
			}
		})
	}
}

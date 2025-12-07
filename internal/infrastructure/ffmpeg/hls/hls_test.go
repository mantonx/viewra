package hls

import (
	"strings"
	"testing"
)

func TestNewBuilder(t *testing.T) {
	opts := Options{
		InputPath: "/path/to/video.mkv",
		OutputDir: "/cache/transcodes/123/1080p",
		Profile: &Profile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    5000000,
			VideoMaxRate:    7500000,
			VideoBufSize:    10000000,
			AudioBitrate:    192000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			SegmentDuration: 6,
			GOPSize:         72,
		},
	}

	builder := NewBuilder(opts)
	if builder == nil {
		t.Fatal("NewBuilder returned nil")
	}

	// Initial args should be empty
	args := builder.Build()
	if len(args) != 0 {
		t.Errorf("Expected empty args, got %v", args)
	}
}

func TestBuilderAddInput(t *testing.T) {
	opts := Options{
		InputPath: "/path/to/video.mkv",
		OutputDir: "/cache/transcodes/123/1080p",
	}

	args := NewBuilder(opts).AddInput().Build()

	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}
	if args[0] != "-i" {
		t.Errorf("Expected -i, got %s", args[0])
	}
	if args[1] != "/path/to/video.mkv" {
		t.Errorf("Expected input path, got %s", args[1])
	}
}

func TestBuilderAddSeekPosition(t *testing.T) {
	tests := []struct {
		name           string
		seconds        int
		expectedArgs   int
		expectedSeekAt int // index where -ss should be, -1 if not expected
	}{
		{"seek at 0", 0, 0, -1},       // 0 seconds = no seek
		{"seek at 100", 100, 2, 0},    // 100 seconds = add -ss
		{"seek at 3600", 3600, 2, 0},  // 1 hour = add -ss
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				InputPath: "/path/to/video.mkv",
				OutputDir: "/cache/transcodes/123/1080p",
			}

			args := NewBuilder(opts).AddSeekPosition(tt.seconds).Build()

			if len(args) != tt.expectedArgs {
				t.Errorf("Expected %d args, got %d: %v", tt.expectedArgs, len(args), args)
			}

			if tt.expectedSeekAt >= 0 && len(args) > tt.expectedSeekAt {
				if args[tt.expectedSeekAt] != "-ss" {
					t.Errorf("Expected -ss at index %d, got %s", tt.expectedSeekAt, args[tt.expectedSeekAt])
				}
			}
		})
	}
}

func TestBuilderAddStreamMapping(t *testing.T) {
	tests := []struct {
		name              string
		audioTrackIndex   int
		useSpecificTrack  bool
		expectedAudioMap  string
	}{
		{"default audio", 0, false, "0:a:0"},
		{"specific track 0", 0, true, "0:a:0"},
		{"specific track 2", 2, true, "0:a:2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				InputPath:             "/path/to/video.mkv",
				OutputDir:             "/cache/transcodes/123/1080p",
				AudioTrackIndex:       tt.audioTrackIndex,
				UseSpecificAudioTrack: tt.useSpecificTrack,
			}

			args := NewBuilder(opts).AddStreamMapping().Build()

			// Should have video map and audio map
			if len(args) != 4 {
				t.Fatalf("Expected 4 args, got %d: %v", len(args), args)
			}

			// Check video mapping
			if args[0] != "-map" || args[1] != "0:v:0" {
				t.Errorf("Expected -map 0:v:0, got %s %s", args[0], args[1])
			}

			// Check audio mapping
			if args[2] != "-map" || args[3] != tt.expectedAudioMap {
				t.Errorf("Expected audio map %s, got %s", tt.expectedAudioMap, args[3])
			}
		})
	}
}

func TestBuilderAddHLSOutput(t *testing.T) {
	opts := Options{
		InputPath: "/path/to/video.mkv",
		OutputDir: "/cache/transcodes/123/1080p",
		Profile: &Profile{
			SegmentDuration: 6,
		},
	}

	args := NewBuilder(opts).AddHLSOutput().Build()

	// Check for key HLS arguments
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-f", "hls") {
		t.Error("Missing -f hls")
	}
	if !containsArg(args, "-hls_time", "6") {
		t.Error("Missing -hls_time 6")
	}
	if !containsArg(args, "-hls_segment_type", "mpegts") {
		t.Error("Missing -hls_segment_type mpegts")
	}
}

func TestBuilderAddH264Copy(t *testing.T) {
	opts := Options{
		InputPath: "/path/to/video.mkv",
		OutputDir: "/cache/transcodes/123/1080p",
	}

	args := NewBuilder(opts).AddH264Copy().Build()

	// Should have codec copy and bitstream filter
	if len(args) != 4 {
		t.Fatalf("Expected 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "-c:v" || args[1] != "copy" {
		t.Errorf("Expected -c:v copy, got %s %s", args[0], args[1])
	}
	if args[2] != "-bsf:v" || args[3] != "h264_mp4toannexb" {
		t.Errorf("Expected h264_mp4toannexb, got %s %s", args[2], args[3])
	}
}

func TestBuilderAddHEVCCopy(t *testing.T) {
	opts := Options{
		InputPath: "/path/to/video.mkv",
		OutputDir: "/cache/transcodes/123/1080p",
	}

	args := NewBuilder(opts).AddHEVCCopy().Build()

	if len(args) != 4 {
		t.Fatalf("Expected 4 args, got %d: %v", len(args), args)
	}
	if args[2] != "-bsf:v" || args[3] != "hevc_mp4toannexb" {
		t.Errorf("Expected hevc_mp4toannexb, got %s %s", args[2], args[3])
	}
}

func TestParsePlaylistMetadata(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantPTS   int64
		wantOff   float64
		wantHas   bool
	}{
		{
			name: "with metadata",
			content: `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-START-PTS:3527500
#EXT-X-START-OFFSET:-2.5
#EXTINF:6.0,
seg_000588.ts`,
			wantPTS: 3527500,
			wantOff: -2.5,
			wantHas: true,
		},
		{
			name: "without metadata",
			content: `#EXTM3U
#EXT-X-VERSION:3
#EXTINF:6.0,
seg_000000.ts`,
			wantPTS: 0,
			wantOff: 0,
			wantHas: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := ParsePlaylistMetadata(tt.content)

			if meta.StartPTSMs != tt.wantPTS {
				t.Errorf("StartPTSMs = %d, want %d", meta.StartPTSMs, tt.wantPTS)
			}
			if meta.StartOffsetSec != tt.wantOff {
				t.Errorf("StartOffsetSec = %f, want %f", meta.StartOffsetSec, tt.wantOff)
			}
			if meta.HasStartInfo != tt.wantHas {
				t.Errorf("HasStartInfo = %v, want %v", meta.HasStartInfo, tt.wantHas)
			}
		})
	}
}

func TestBuilderFluentChain(t *testing.T) {
	opts := Options{
		InputPath: "/path/to/video.mkv",
		OutputDir: "/cache/transcodes/123/1080p",
		Profile: &Profile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    5000000,
			VideoMaxRate:    7500000,
			VideoBufSize:    10000000,
			AudioBitrate:    192000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			SegmentDuration: 6,
			GOPSize:         72,
		},
	}

	// Test fluent chain doesn't panic
	args := NewBuilder(opts).
		AddHideBanner().
		AddLogLevel("error").
		AddNostdin().
		AddInput().
		AddStreamMapping().
		AddH264Copy().
		AddAudioCodec("copy").
		AddHLSOutput().
		AddOverwriteOutput().
		AddOutputFile().
		Build()

	// Should have produced some args
	if len(args) == 0 {
		t.Error("Fluent chain produced no args")
	}

	// Should contain -i (input)
	containsInput := false
	for _, arg := range args {
		if arg == "-i" {
			containsInput = true
			break
		}
	}
	if !containsInput {
		t.Error("Missing -i in fluent chain")
	}
}

func TestGetH264Level(t *testing.T) {
	tests := []struct {
		width  int
		height int
		want   string
	}{
		{1280, 720, "4.0"},   // 720p
		{1920, 1080, "4.1"},  // 1080p
		{2560, 1440, "5.1"},  // 1440p
		{3840, 2160, "5.1"},  // 4K
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(strings.ToLower(tt.want), ".", "_"), func(t *testing.T) {
			got := getH264Level(tt.width, tt.height)
			if got != tt.want {
				t.Errorf("getH264Level(%d, %d) = %s, want %s", tt.width, tt.height, got, tt.want)
			}
		})
	}
}

func TestAddLogLevel(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddLogLevel("error").Build()

	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}
	if args[0] != "-loglevel" || args[1] != "error" {
		t.Errorf("Expected -loglevel error, got %s %s", args[0], args[1])
	}
}

func TestAddHideBanner(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddHideBanner().Build()

	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}
	if args[0] != "-hide_banner" {
		t.Errorf("Expected -hide_banner, got %s", args[0])
	}
}

func TestAddNostdin(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddNostdin().Build()

	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}
	if args[0] != "-nostdin" {
		t.Errorf("Expected -nostdin, got %s", args[0])
	}
}

func TestAddTimestampHandling(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddTimestampHandling().Build()

	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}
	if args[0] != "-copyts" || args[1] != "-start_at_zero" {
		t.Errorf("Expected -copyts -start_at_zero, got %v", args)
	}
}

func TestAddNoAccurateSeek(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddNoAccurateSeek().Build()

	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}
	if args[0] != "-noaccurate_seek" {
		t.Errorf("Expected -noaccurate_seek, got %s", args[0])
	}
}

func TestAddHardwareAccel(t *testing.T) {
	opts := Options{}
	hwArgs := []string{"-hwaccel", "cuda"}
	args := NewBuilder(opts).AddHardwareAccel(hwArgs).Build()

	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}
	if args[0] != "-hwaccel" || args[1] != "cuda" {
		t.Errorf("Expected -hwaccel cuda, got %v", args)
	}
}

func TestAddProgressReporting(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddProgressReporting().Build()

	if len(args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(args))
	}
	if args[0] != "-progress" || args[1] != "pipe:2" {
		t.Errorf("Expected -progress pipe:2, got %v", args)
	}
}

func TestAddMemorySafetyOptions(t *testing.T) {
	tests := []struct {
		name        string
		maxAllocMB  int
		expectArgs  bool
		expectValue string
	}{
		{"with limit", 512, true, "536870912"},
		{"zero limit", 0, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{}
			args := NewBuilder(opts).AddMemorySafetyOptions(tt.maxAllocMB).Build()

			if tt.expectArgs {
				if len(args) != 2 {
					t.Fatalf("Expected 2 args, got %d", len(args))
				}
				if args[0] != "-max_alloc" {
					t.Errorf("Expected -max_alloc, got %s", args[0])
				}
				if args[1] != tt.expectValue {
					t.Errorf("Expected %s, got %s", tt.expectValue, args[1])
				}
			} else {
				if len(args) != 0 {
					t.Errorf("Expected no args, got %v", args)
				}
			}
		})
	}
}

func TestAddSegmentMuxerOutput(t *testing.T) {
	opts := Options{
		OutputDir: "/cache/transcodes/123/1080p",
		Profile: &Profile{
			SegmentDuration: 6,
		},
	}

	args := NewBuilder(opts).AddSegmentMuxerOutput().Build()

	// Should contain segment muxer args
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-f", "segment") {
		t.Error("Missing -f segment")
	}
	if !containsArg(args, "-segment_format", "mpegts") {
		t.Error("Missing -segment_format mpegts")
	}
	if !containsArg(args, "-segment_time", "6") {
		t.Error("Missing -segment_time 6")
	}
}

func TestAddSegmentMuxerOutputWithSeek(t *testing.T) {
	opts := Options{
		OutputDir:        "/cache/transcodes/123/1080p",
		Profile:          &Profile{SegmentDuration: 6},
		UseStartPosition: true,
		StartPosition:    3600, // 1 hour
	}

	args := NewBuilder(opts).AddSegmentMuxerOutput().Build()

	// Should have segment start PTS reporting
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	if !containsArg(args, "-segment_start_pts_report", "1") {
		t.Error("Missing -segment_start_pts_report")
	}
	if !containsArg(args, "-segment_seek_time", "3600000000") {
		t.Error("Missing -segment_seek_time with correct microseconds")
	}
}

func TestAddSegmentMuxerOutputFile(t *testing.T) {
	opts := Options{
		OutputDir: "/cache/transcodes/123/1080p",
	}

	args := NewBuilder(opts).AddSegmentMuxerOutputFile().Build()

	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}
	if !strings.Contains(args[0], "seg_%06d.ts") {
		t.Errorf("Expected segment filename pattern, got %s", args[0])
	}
}

func TestAddOutputFile(t *testing.T) {
	opts := Options{
		OutputDir: "/cache/transcodes/123/1080p",
	}

	args := NewBuilder(opts).AddOutputFile().Build()

	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}
	if !strings.Contains(args[0], "playlist.m3u8") {
		t.Errorf("Expected playlist.m3u8, got %s", args[0])
	}
}

func TestAddOverwriteOutput(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).AddOverwriteOutput().Build()

	if len(args) != 1 {
		t.Fatalf("Expected 1 arg, got %d", len(args))
	}
	if args[0] != "-y" {
		t.Errorf("Expected -y, got %s", args[0])
	}
}

func TestAdd(t *testing.T) {
	opts := Options{}
	args := NewBuilder(opts).Add("-custom", "arg1", "-another", "arg2").Build()

	if len(args) != 4 {
		t.Fatalf("Expected 4 args, got %d", len(args))
	}
	expected := []string{"-custom", "arg1", "-another", "arg2"}
	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %s, want %s", i, args[i], want)
		}
	}
}

func TestHLSOutputWithSeekPosition(t *testing.T) {
	opts := Options{
		OutputDir: "/cache/transcodes/123/1080p",
		Profile: &Profile{
			SegmentDuration: 6,
		},
		UseStartPosition: true,
		StartPosition:    600, // 10 minutes = 100 segments
	}

	args := NewBuilder(opts).AddHLSOutput().Build()

	// Should have start_number set based on seek position
	containsArg := func(args []string, key, value string) bool {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == key && args[i+1] == value {
				return true
			}
		}
		return false
	}

	// 600 seconds / 6 second segments = segment 100
	if !containsArg(args, "-start_number", "100") {
		t.Error("Missing -start_number 100 for seek position 600")
	}
}

func TestSegmentFilenameFormat(t *testing.T) {
	if SegmentFilenameFormat != "seg_%06d.ts" {
		t.Errorf("SegmentFilenameFormat = %s, want seg_%%06d.ts", SegmentFilenameFormat)
	}
}

func TestBuilderChaining(t *testing.T) {
	// Test that all builder methods return *Builder for chaining
	opts := Options{
		InputPath: "/test.mkv",
		OutputDir: "/out",
		Profile: &Profile{
			Width:           1920,
			Height:          1080,
			VideoBitrate:    5000000,
			VideoMaxRate:    7500000,
			VideoBufSize:    10000000,
			AudioBitrate:    192000,
			AudioChannels:   2,
			AudioSampleRate: 48000,
			SegmentDuration: 6,
			GOPSize:         72,
		},
	}

	// This should compile and not panic
	_ = NewBuilder(opts).
		AddLogLevel("error").
		AddHideBanner().
		AddNostdin().
		AddTimestampHandling().
		AddSeekPosition(100).
		AddNoAccurateSeek().
		AddInput().
		AddStreamMapping().
		AddProgressReporting().
		AddMemorySafetyOptions(512).
		AddHLSOutput().
		AddOutputFile().
		Build()
}

func TestParsePlaylistMetadataPartialInfo(t *testing.T) {
	// Test with only PTS, no offset
	content := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-START-PTS:1000000
#EXTINF:6.0,
seg_000000.ts`

	meta := ParsePlaylistMetadata(content)

	if meta.StartPTSMs != 1000000 {
		t.Errorf("StartPTSMs = %d, want 1000000", meta.StartPTSMs)
	}
	if meta.StartOffsetSec != 0 {
		t.Errorf("StartOffsetSec = %f, want 0", meta.StartOffsetSec)
	}
	if !meta.HasStartInfo {
		t.Error("HasStartInfo should be true when PTS is present")
	}
}

func TestParsePlaylistMetadataInvalidValues(t *testing.T) {
	// Test with invalid values
	content := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-START-PTS:invalid
#EXT-X-START-OFFSET:notanumber
#EXTINF:6.0,
seg_000000.ts`

	meta := ParsePlaylistMetadata(content)

	// Should not parse invalid values
	if meta.HasStartInfo {
		t.Error("HasStartInfo should be false with invalid PTS")
	}
	if meta.StartPTSMs != 0 {
		t.Errorf("StartPTSMs = %d, want 0 for invalid value", meta.StartPTSMs)
	}
}

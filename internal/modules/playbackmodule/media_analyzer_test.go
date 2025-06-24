package playbackmodule

import (
	"testing"
)

func TestFFProbeMediaAnalyzer(t *testing.T) {
	analyzer := NewFFProbeMediaAnalyzer()
	
	// Test with our test video
	testVideoPath := "/tmp/test_video.mp4"
	
	info, err := analyzer.AnalyzeMedia(testVideoPath)
	if err != nil {
		t.Fatalf("Failed to analyze test video: %v", err)
	}
	
	// Verify the extracted information
	if info.Container != "mp4" {
		t.Errorf("Expected container 'mp4', got '%s'", info.Container)
	}
	
	if info.VideoCodec != "h264" {
		t.Errorf("Expected video codec 'h264', got '%s'", info.VideoCodec)
	}
	
	if info.AudioCodec != "aac" {
		t.Errorf("Expected audio codec 'aac', got '%s'", info.AudioCodec)
	}
	
	if info.Resolution != "1080p" {
		t.Errorf("Expected resolution '1080p', got '%s'", info.Resolution)
	}
	
	if info.Duration != 10 {
		t.Errorf("Expected duration 10 seconds, got %d", info.Duration)
	}
	
	if info.HasHDR {
		t.Errorf("Expected HasHDR to be false, got true")
	}
	
	t.Logf("✅ FFprobe analysis successful:")
	t.Logf("   Container: %s", info.Container)
	t.Logf("   Video: %s", info.VideoCodec)
	t.Logf("   Audio: %s", info.AudioCodec)
	t.Logf("   Resolution: %s", info.Resolution)
	t.Logf("   Bitrate: %d", info.Bitrate)
	t.Logf("   Duration: %d seconds", info.Duration)
}

func TestSimpleMediaAnalyzer(t *testing.T) {
	analyzer := NewSimpleMediaAnalyzer()
	
	// Test with our test video
	testVideoPath := "/tmp/test_video.mp4"
	
	info, err := analyzer.AnalyzeMedia(testVideoPath)
	if err != nil {
		t.Fatalf("Failed to analyze test video: %v", err)
	}
	
	// Simple analyzer uses defaults
	if info.VideoCodec != "h264" {
		t.Errorf("Expected default video codec 'h264', got '%s'", info.VideoCodec)
	}
	
	if info.Resolution != "1080p" {
		t.Errorf("Expected default resolution '1080p', got '%s'", info.Resolution)
	}
	
	t.Logf("✅ Simple analysis (defaults):")
	t.Logf("   Container: %s", info.Container)
	t.Logf("   Video: %s", info.VideoCodec)
	t.Logf("   Audio: %s", info.AudioCodec)
	t.Logf("   Resolution: %s", info.Resolution)
	t.Logf("   Duration: %d seconds", info.Duration)
}
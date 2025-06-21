package ffmpeg

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	plugins "github.com/mantonx/viewra/sdk"
)

// MockCommandRunner implements CommandRunner interface for testing
type MockCommandRunner struct {
	commands  []string
	outputs   [][]byte
	errors    []error
	callIndex int
}

func (m *MockCommandRunner) Run(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	fullCmd := fmt.Sprintf("%s %s", cmd, strings.Join(args, " "))
	m.commands = append(m.commands, fullCmd)

	if m.callIndex < len(m.outputs) {
		output := m.outputs[m.callIndex]
		var err error
		if m.callIndex < len(m.errors) {
			err = m.errors[m.callIndex]
		}
		m.callIndex++
		return output, err
	}

	return []byte("success"), nil
}

// MockLogger implements plugins.Logger for testing
type MockLogger struct {
	logs []map[string]interface{}
}

func (m *MockLogger) Info(msg string, keysAndValues ...interface{}) {
	m.logs = append(m.logs, map[string]interface{}{"level": "info", "msg": msg, "args": keysAndValues})
}

func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) {
	m.logs = append(m.logs, map[string]interface{}{"level": "error", "msg": msg, "args": keysAndValues})
}

func (m *MockLogger) Warn(msg string, keysAndValues ...interface{}) {
	m.logs = append(m.logs, map[string]interface{}{"level": "warn", "msg": msg, "args": keysAndValues})
}

func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) {
	m.logs = append(m.logs, map[string]interface{}{"level": "debug", "msg": msg, "args": keysAndValues})
}

func (m *MockLogger) With(keysAndValues ...interface{}) hclog.Logger {
	// Return a no-op hclog logger for testing
	return hclog.NewNullLogger()
}

func TestBuildFFmpegArgs(t *testing.T) {
	runner := NewRunnerWithExecutor(nil, &MockCommandRunner{})

	tests := []struct {
		name     string
		request  plugins.TranscodeRequest
		expected []string
	}{
		{
			name: "Basic H.264 transcoding",
			request: plugins.TranscodeRequest{
				InputPath:  "/input/video.mp4",
				OutputPath: "/output/video.mp4",
				CodecOpts: &plugins.CodecOptions{
					Video:     "h264",
					Audio:     "aac",
					Container: "mp4",
					Bitrate:   "1000k",
				},
			},
			expected: []string{
				"-i", "/input/video.mp4",
				"-c:v", "libx264",
				"-b:v", "1000k",
				"-map", "0:v:0?",
				"-c:a", "aac",
				"-b:a", "128k",
				"-map", "0:a:0?",
				"-y", "/output/video.mp4",
			},
		},
		{
			name: "DASH transcoding with seek",
			request: plugins.TranscodeRequest{
				InputPath:  "/input/video.mkv",
				OutputPath: "/output/manifest.mpd",
				Seek:       30 * time.Second,
				CodecOpts: &plugins.CodecOptions{
					Video:     "h265",
					Audio:     "aac",
					Container: "dash",
				},
			},
			expected: []string{
				"-ss", "30s",
				"-i", "/input/video.mkv",
				"-c:v", "libx265",
				"-map", "0:v:0?",
				"-c:a", "aac",
				"-b:a", "128k",
				"-map", "0:a:0?",
				"-f", "dash",
				"-y", "/output/manifest.mpd",
			},
		},
		{
			name: "HLS transcoding",
			request: plugins.TranscodeRequest{
				InputPath:  "/input/video.avi",
				OutputPath: "/output/playlist.m3u8",
				CodecOpts: &plugins.CodecOptions{
					Video:     "vp9",
					Audio:     "aac",
					Container: "hls",
				},
			},
			expected: []string{
				"-i", "/input/video.avi",
				"-c:v", "libvpx-vp9",
				"-map", "0:v:0?",
				"-c:a", "aac",
				"-b:a", "128k",
				"-map", "0:a:0?",
				"-f", "hls",
				"-y", "/output/playlist.m3u8",
			},
		},
		{
			name: "NVENC hardware transcoding",
			request: plugins.TranscodeRequest{
				InputPath:  "/input/video.mp4",
				OutputPath: "/output/video.mp4",
				CodecOpts: &plugins.CodecOptions{
					Video: "h264_nvenc",
					Audio: "aac",
					Extra: []string{"-hwaccel", "cuda"},
				},
			},
			expected: []string{
				"-i", "/input/video.mp4",
				"-c:v", "h264_nvenc",
				"-hwaccel", "cuda",
				"-map", "0:v:0?",
				"-c:a", "aac",
				"-b:a", "128k",
				"-map", "0:a:0?",
				"-y", "/output/video.mp4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runner.BuildFFmpegArgs(tt.request)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d arguments, got %d", len(tt.expected), len(result))
				t.Errorf("Expected: %v", tt.expected)
				t.Errorf("Got: %v", result)
				return
			}

			for i, arg := range result {
				if i < len(tt.expected) && arg != tt.expected[i] {
					t.Errorf("Argument %d: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

func TestMapVideoCodec(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"h264", "libx264"},
		{"h265", "libx265"},
		{"hevc", "libx265"},
		{"vp8", "libvpx"},
		{"vp9", "libvpx-vp9"},
		{"av1", "librav1e"},
		{"libx264", "libx264"},       // Should pass through specific encoders
		{"h264_nvenc", "h264_nvenc"}, // Should pass through hardware encoders
		{"unknown", "unknown"},       // Should pass through unknown codecs
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getVideoEncoder(tt.input)
			if result != tt.expected {
				t.Errorf("getVideoEncoder(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapAudioCodec(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"aac", "aac"},
		{"mp3", "libmp3lame"},
		{"opus", "libopus"},
		{"vorbis", "libvorbis"},
		{"ac3", "ac3"},
		{"unknown", "unknown"}, // Should pass through unknown codecs
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getAudioEncoder(tt.input)
			if result != tt.expected {
				t.Errorf("getAudioEncoder(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildSeekArgs(t *testing.T) {
	runner := NewRunnerWithExecutor(nil, &MockCommandRunner{})

	tests := []struct {
		name     string
		seek     time.Duration
		expected []string
	}{
		{"No seek", 0, []string{}},
		{"30 second seek", 30 * time.Second, []string{"-ss", "30s"}},
		{"5 minute seek", 5 * time.Minute, []string{"-ss", "5m0s"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugins.TranscodeRequest{Seek: tt.seek}
			result := runner.buildSeekArgs(req)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d arguments, got %d", len(tt.expected), len(result))
				return
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("Argument %d: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

func TestBuildAudioArgs(t *testing.T) {
	runner := NewRunnerWithExecutor(nil, &MockCommandRunner{})

	tests := []struct {
		name     string
		opts     *plugins.CodecOptions
		expected []string
	}{
		{
			name: "AAC audio",
			opts: &plugins.CodecOptions{Audio: "aac"},
			expected: []string{
				"-c:a", "aac",
				"-b:a", "128k",
				"-map", "0:a:0?",
			},
		},
		{
			name: "MP3 audio",
			opts: &plugins.CodecOptions{Audio: "mp3"},
			expected: []string{
				"-c:a", "libmp3lame",
				"-b:a", "128k",
				"-map", "0:a:0?",
			},
		},
		{
			name:     "No audio codec",
			opts:     &plugins.CodecOptions{},
			expected: []string{"-map", "0:a:0?"},
		},
		{
			name:     "Nil codec options",
			opts:     nil,
			expected: []string{"-map", "0:a:0?"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugins.TranscodeRequest{CodecOpts: tt.opts}
			result := runner.buildAudioArgs(req)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d arguments, got %d", len(tt.expected), len(result))
				t.Errorf("Expected: %v", tt.expected)
				t.Errorf("Got: %v", result)
				return
			}

			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("Argument %d: expected %q, got %q", i, tt.expected[i], arg)
				}
			}
		})
	}
}

func TestRunFFmpegTimeout(t *testing.T) {
	mockExec := &MockCommandRunner{
		errors: []error{context.DeadlineExceeded},
	}
	runner := NewRunnerWithExecutor(&MockLogger{}, mockExec)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := runner.RunFFmpeg(ctx, []string{"test", "args"})

	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestRunFFmpegSuccess(t *testing.T) {
	mockExec := &MockCommandRunner{
		outputs: [][]byte{[]byte("FFmpeg success")},
	}
	runner := NewRunnerWithExecutor(&MockLogger{}, mockExec)

	ctx := context.Background()
	err := runner.RunFFmpeg(ctx, []string{"test", "args"})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(mockExec.commands) != 1 {
		t.Errorf("Expected 1 command to be executed, got %d", len(mockExec.commands))
	}
}

func TestCommandRunnerInterface(t *testing.T) {
	// Verify that MockCommandRunner implements CommandRunner
	var _ CommandRunner = &MockCommandRunner{}
	var _ CommandRunner = &DefaultCommandRunner{}
}

func TestStopSession(t *testing.T) {
	runner := NewRunnerWithExecutor(&MockLogger{}, &MockCommandRunner{})

	// Test stopping non-existent session
	err := runner.StopSession("nonexistent")
	if err == nil {
		t.Error("Expected error when stopping non-existent session")
	}

	expectedError := "session nonexistent not found"
	if err.Error() != expectedError {
		t.Errorf("Expected error %q, got %q", expectedError, err.Error())
	}
}

func TestSessionManagement(t *testing.T) {
	runner := NewRunnerWithExecutor(&MockLogger{}, &MockCommandRunner{})

	// Initially no sessions
	sessions := runner.ListActiveSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 active sessions, got %d", len(sessions))
	}

	// Test getting non-existent session
	_, exists := runner.GetSession("nonexistent")
	if exists {
		t.Error("Expected session to not exist")
	}
}

func TestFFmpegWorkflow(t *testing.T) {
	// Test complete workflow with mock executor
	mockExec := &MockCommandRunner{
		outputs: [][]byte{[]byte("FFmpeg completed successfully")},
	}
	mockLogger := &MockLogger{}
	runner := NewRunnerWithExecutor(mockLogger, mockExec)

	req := plugins.TranscodeRequest{
		InputPath:  "/input/test.mp4",
		OutputPath: "/output/test.mp4",
		CodecOpts: &plugins.CodecOptions{
			Video: "h264",
			Audio: "aac",
		},
	}

	// Build arguments
	args := runner.BuildFFmpegArgs(req)

	// Execute
	ctx := context.Background()
	err := runner.RunFFmpeg(ctx, args)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify command was executed
	if len(mockExec.commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(mockExec.commands))
	}

	// Verify logging occurred
	if len(mockLogger.logs) < 1 {
		t.Error("Expected logging to occur")
	}
}

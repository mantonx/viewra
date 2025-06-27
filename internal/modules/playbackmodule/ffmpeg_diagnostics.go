package playbackmodule

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// FFmpegDiagnostics provides diagnostic information about FFmpeg
type FFmpegDiagnostics struct {
	Version        string            `json:"version"`
	Codecs         []string          `json:"codecs"`
	Formats        []string          `json:"formats"`
	ProcessLimits  map[string]uint64 `json:"process_limits"`
	SystemInfo     SystemInfo        `json:"system_info"`
	TestResults    []TestResult      `json:"test_results"`
	RecentFailures []FailureInfo     `json:"recent_failures"`
}

// SystemInfo contains system resource information
type SystemInfo struct {
	CPUs           int    `json:"cpus"`
	MemoryMB       uint64 `json:"memory_mb"`
	GoRoutines     int    `json:"go_routines"`
	FFmpegPath     string `json:"ffmpeg_path"`
	PluginPath     string `json:"plugin_path"`
	TranscodingDir string `json:"transcoding_dir"`
}

// TestResult contains results from diagnostic tests
type TestResult struct {
	Test    string `json:"test"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

// FailureInfo contains information about recent failures
type FailureInfo struct {
	SessionID string    `json:"session_id"`
	Time      time.Time `json:"time"`
	Error     string    `json:"error"`
	Signal    string    `json:"signal,omitempty"`
}

// HandleFFmpegDiagnostics returns detailed FFmpeg diagnostics
func (h *APIHandler) HandleFFmpegDiagnostics(c *gin.Context) {
	diag := &FFmpegDiagnostics{
		ProcessLimits: make(map[string]uint64),
		TestResults:   []TestResult{},
	}

	// Get FFmpeg version
	if version, err := getFFmpegVersion(); err == nil {
		diag.Version = version
	} else {
		diag.Version = fmt.Sprintf("error: %v", err)
	}

	// Get system info
	diag.SystemInfo = getSystemInfo()

	// Get process limits
	diag.ProcessLimits = getProcessLimits()

	// Run diagnostic tests
	diag.TestResults = runDiagnosticTests()

	// Get recent failures from database
	diag.RecentFailures = h.getRecentFailures()

	c.JSON(200, diag)
}

// HandleFFmpegTest runs a test transcode with detailed logging
func (h *APIHandler) HandleFFmpegTest(c *gin.Context) {
	var request struct {
		InputFile string `json:"input_file"`
		Duration  int    `json:"duration"` // seconds
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Default to 5 seconds
	if request.Duration <= 0 {
		request.Duration = 5
	}

	// Get a test file if not provided
	if request.InputFile == "" {
		// Get first media file from database
		var mediaFile struct {
			Path string
		}
		if err := h.manager.db.Table("media_files").Select("path").First(&mediaFile).Error; err != nil {
			c.JSON(400, gin.H{"error": "no media files found for testing"})
			return
		}
		request.InputFile = mediaFile.Path
	}

	// Run test with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(request.Duration+10)*time.Second)
	defer cancel()

	// Create test output directory
	testDir := fmt.Sprintf("/app/viewra-data/transcoding/test_%s", time.Now().Format("20060102_150405"))
	os.MkdirAll(testDir, 0755)

	// Build FFmpeg command for testing
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", request.InputFile,
		"-t", fmt.Sprintf("%d", request.Duration),
		"-c:v", "h264",
		"-c:a", "aac",
		"-f", "null",
		"-", // Output to null
	)

	// Capture output
	output, err := cmd.CombinedOutput()

	result := gin.H{
		"test_file":     request.InputFile,
		"duration":      request.Duration,
		"output_dir":    testDir,
		"ffmpeg_output": string(output),
	}

	if err != nil {
		result["error"] = err.Error()

		// Check if it was killed by signal
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					result["signal"] = status.Signal().String()
				}
				result["exit_code"] = status.ExitStatus()
			}
		}

		c.JSON(500, result)
		return
	}

	result["success"] = true
	c.JSON(200, result)
}

// Helper functions

func getFFmpegVersion() (string, error) {
	cmd := exec.Command("ffmpeg", "-version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return lines[0], nil
	}
	return "unknown", nil
}

func getSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemInfo{
		CPUs:           runtime.NumCPU(),
		MemoryMB:       m.Sys / 1024 / 1024,
		GoRoutines:     runtime.NumGoroutine(),
		FFmpegPath:     findExecutable("ffmpeg"),
		PluginPath:     "/app/data/plugins",
		TranscodingDir: "/app/viewra-data/transcoding",
	}
}

func getProcessLimits() map[string]uint64 {
	limits := make(map[string]uint64)

	var rlimit syscall.Rlimit

	// Memory limit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &rlimit); err == nil {
		limits["memory_limit"] = rlimit.Cur
	}

	// CPU limit
	if err := syscall.Getrlimit(syscall.RLIMIT_CPU, &rlimit); err == nil {
		limits["cpu_limit"] = rlimit.Cur
	}

	// File descriptors
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err == nil {
		limits["file_descriptors"] = rlimit.Cur
	}

	// Stack size limit
	if err := syscall.Getrlimit(syscall.RLIMIT_STACK, &rlimit); err == nil {
		limits["stack_size"] = rlimit.Cur
	}

	return limits
}

func runDiagnosticTests() []TestResult {
	tests := []TestResult{}

	// Test 1: FFmpeg executable
	ffmpegPath := findExecutable("ffmpeg")
	tests = append(tests, TestResult{
		Test:    "FFmpeg executable",
		Success: ffmpegPath != "",
		Message: ffmpegPath,
		Time:    time.Now().Format(time.RFC3339),
	})

	// Test 2: Write permissions
	testFile := "/app/viewra-data/transcoding/test_write.txt"
	err := os.WriteFile(testFile, []byte("test"), 0644)
	tests = append(tests, TestResult{
		Test:    "Transcoding directory write",
		Success: err == nil,
		Message: fmt.Sprintf("%v", err),
		Time:    time.Now().Format(time.RFC3339),
	})
	os.Remove(testFile)

	// Test 3: Plugin directory
	if _, err := os.Stat("/app/data/plugins"); err == nil {
		tests = append(tests, TestResult{
			Test:    "Plugin directory exists",
			Success: true,
			Message: "OK",
			Time:    time.Now().Format(time.RFC3339),
		})
	} else {
		tests = append(tests, TestResult{
			Test:    "Plugin directory exists",
			Success: false,
			Message: err.Error(),
			Time:    time.Now().Format(time.RFC3339),
		})
	}

	// Test 4: Memory available
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		freeMB := si.Freeram / 1024 / 1024
		tests = append(tests, TestResult{
			Test:    "Free memory check",
			Success: freeMB > 100, // At least 100MB free
			Message: fmt.Sprintf("%d MB free", freeMB),
			Time:    time.Now().Format(time.RFC3339),
		})
	}

	return tests
}

func (h *APIHandler) getRecentFailures() []FailureInfo {
	// This would query the database for recent failed sessions
	// For now, return empty
	return []FailureInfo{}
}

func findExecutable(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// RegisterDiagnosticRoutes registers diagnostic endpoints
func RegisterDiagnosticRoutes(api *gin.RouterGroup, handler *APIHandler) {
	diag := api.Group("/diagnostics")
	{
		diag.GET("/ffmpeg", handler.HandleFFmpegDiagnostics)
		diag.POST("/ffmpeg/test", handler.HandleFFmpegTest)
	}
}

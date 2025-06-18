package services

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/pkg/plugins"
)

// SessionStatusCallback is called when a session status changes
type SessionStatusCallback func(sessionID string, status string, errorMsg string)

// FFmpegService handles FFmpeg operations and process management
type FFmpegService struct {
	logger         plugins.Logger
	configService  *config.FFmpegConfigurationService
	activeJobs     map[string]*FFmpegJob
	jobsMutex      sync.RWMutex
	statusCallback SessionStatusCallback
}

// FileBufferedStreamReader buffers data from an input stream to a disk file to prevent blocking
type FileBufferedStreamReader struct {
	input      io.ReadCloser
	tempFile   *os.File
	filePath   string
	mutex      sync.RWMutex
	eof        bool
	err        error
	done       chan struct{}
	fileSize   int64
	readOffset int64
}

// NewFileBufferedStreamReader creates a new file-based buffered stream reader
func NewFileBufferedStreamReader(input io.ReadCloser, sessionID string) (*FileBufferedStreamReader, error) {
	// Create temporary file in transcoding directory
	tempDir := getTranscodingDir()
	tempFile, err := os.CreateTemp(tempDir, fmt.Sprintf("stream_%s_*.mp4", sessionID))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	reader := &FileBufferedStreamReader{
		input:    input,
		tempFile: tempFile,
		filePath: tempFile.Name(),
		done:     make(chan struct{}),
	}

	// Start background goroutine to consume input immediately and write to file
	go reader.consumeInputToFile()

	return reader, nil
}

// consumeInputToFile reads from input stream in background and writes to disk file
func (f *FileBufferedStreamReader) consumeInputToFile() {
	defer close(f.done)
	defer f.input.Close()
	defer f.tempFile.Close()

	buffer := make([]byte, 64*1024) // Read in 64KB chunks for better disk I/O
	for {
		n, err := f.input.Read(buffer)
		if n > 0 {
			// Write immediately to disk file
			if _, writeErr := f.tempFile.Write(buffer[:n]); writeErr != nil {
				f.mutex.Lock()
				f.err = writeErr
				f.mutex.Unlock()
				return
			}

			// Force sync to ensure data is written to disk
			if syncErr := f.tempFile.Sync(); syncErr != nil {
				f.mutex.Lock()
				f.err = syncErr
				f.mutex.Unlock()
				return
			}

			// Update file size
			f.mutex.Lock()
			f.fileSize += int64(n)
			f.mutex.Unlock()
		}

		if err != nil {
			f.mutex.Lock()
			f.eof = true
			if err != io.EOF {
				f.err = err
			}
			f.mutex.Unlock()
			return
		}
	}
}

// Read implements io.Reader interface by reading from the temporary file
func (f *FileBufferedStreamReader) Read(p []byte) (n int, err error) {
	for {
		f.mutex.RLock()

		// Check for error
		if f.err != nil {
			f.mutex.RUnlock()
			return 0, f.err
		}

		// Check if we have data to read from file
		available := f.fileSize - f.readOffset
		if available > 0 {
			f.mutex.RUnlock()

			// Open a separate file descriptor for reading to avoid conflicts
			readFile, openErr := os.Open(f.filePath)
			if openErr != nil {
				return 0, openErr
			}
			defer readFile.Close()

			// Seek to current read position
			if _, seekErr := readFile.Seek(f.readOffset, 0); seekErr != nil {
				return 0, seekErr
			}

			// Read data
			bytesToRead := int64(len(p))
			if bytesToRead > available {
				bytesToRead = available
			}

			n, readErr := readFile.Read(p[:bytesToRead])
			if n > 0 {
				f.mutex.Lock()
				f.readOffset += int64(n)
				f.mutex.Unlock()
			}

			return n, readErr
		}

		// No data available, check if EOF
		if f.eof {
			f.mutex.RUnlock()
			return 0, io.EOF
		}

		f.mutex.RUnlock()

		// No data yet, wait a bit before trying again (blocking behavior)
		time.Sleep(10 * time.Millisecond)
	}
}

// Close implements io.Closer interface
func (f *FileBufferedStreamReader) Close() error {
	select {
	case <-f.done:
		// Already closed
	default:
		// Force close input if still running
		if f.input != nil {
			f.input.Close()
		}
		<-f.done // Wait for background goroutine to finish
	}

	// Clean up temporary file
	if f.filePath != "" {
		os.Remove(f.filePath)
	}

	return nil
}

// FileStreamReader reads from a file that's being written to by FFmpeg
type FileStreamReader struct {
	filePath     string
	sessionID    string
	service      *FFmpegService
	mutex        sync.RWMutex
	readOffset   int64
	done         chan struct{}
	lastReadTime time.Time
}

// Read implements io.Reader interface by reading from the file
func (f *FileStreamReader) Read(p []byte) (n int, err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// Update last read time to detect client disconnections
	f.lastReadTime = time.Now()

	select {
	case <-f.done:
		return 0, io.EOF
	default:
	}

	// Check if file exists
	fileInfo, err := os.Stat(f.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, wait a bit and try again
			f.mutex.Unlock()
			time.Sleep(100 * time.Millisecond)
			f.mutex.Lock()

			fileInfo, err = os.Stat(f.filePath)
			if err != nil {
				if os.IsNotExist(err) {
					// Still doesn't exist, check if we should timeout
					if time.Since(f.lastReadTime) > 5*time.Second {
						f.service.logger.Debug("File still doesn't exist after timeout, returning EOF",
							"session_id", f.sessionID, "file", f.filePath)
						return 0, io.EOF
					}
					// Return 0 bytes but no error to indicate "try again later"
					return 0, nil
				}
				return 0, fmt.Errorf("error checking file: %w", err)
			}
		} else {
			return 0, fmt.Errorf("error checking file: %w", err)
		}
	}

	// Open file for reading
	file, err := os.Open(f.filePath)
	if err != nil {
		return 0, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Seek to current read position
	if _, err := file.Seek(f.readOffset, 0); err != nil {
		return 0, fmt.Errorf("error seeking file: %w", err)
	}

	// Try to read data
	n, err = file.Read(p)
	if n > 0 {
		f.readOffset += int64(n)
		f.service.logger.Debug("FileStreamReader read data",
			"session_id", f.sessionID,
			"bytes_read", n,
			"total_read", f.readOffset,
			"file_size", fileInfo.Size())
	}

	// Handle different error conditions
	if err != nil {
		if err == io.EOF {
			// Check if this is a temporary EOF (file is still being written)
			// by comparing file size with our read position
			if f.readOffset < fileInfo.Size() {
				// There's more data in the file, this shouldn't be EOF
				f.service.logger.Debug("EOF but file has more data, continuing",
					"session_id", f.sessionID,
					"read_offset", f.readOffset,
					"file_size", fileInfo.Size())
				// Return what we read (if any) without the EOF error
				if n > 0 {
					return n, nil
				}
				// No data read but file has more, wait a bit
				return 0, nil
			}

			// We've read all available data. Check if FFmpeg is still running.
			// If FFmpeg is done, this is a real EOF. Otherwise, wait for more data.
			if job, exists := f.service.GetJob(f.sessionID); exists {
				// Check if the process is still running
				if job.Process != nil && job.Process.ProcessState == nil {
					// Process is still running, this is likely a temporary EOF
					f.service.logger.Debug("Temporary EOF, FFmpeg still running",
						"session_id", f.sessionID,
						"read_offset", f.readOffset,
						"file_size", fileInfo.Size())
					return n, nil // Return data without EOF if we have any
				} else {
					// Process is done, this is a real EOF
					f.service.logger.Debug("Real EOF, FFmpeg process completed",
						"session_id", f.sessionID,
						"read_offset", f.readOffset,
						"file_size", fileInfo.Size())
					return n, io.EOF
				}
			} else {
				// Job doesn't exist anymore, treat as EOF
				f.service.logger.Debug("Job not found, treating as EOF",
					"session_id", f.sessionID)
				return n, io.EOF
			}
		} else {
			// Other read error
			return n, fmt.Errorf("error reading file: %w", err)
		}
	}

	return n, nil
}

// Close implements io.Closer interface
func (f *FileStreamReader) Close() error {
	select {
	case <-f.done:
		// Already closed
	default:
		close(f.done)
	}

	// Clean up the temporary file
	if f.filePath != "" {
		os.Remove(f.filePath)
	}

	return nil
}

// BufferedStreamReader buffers data from an input stream to prevent blocking
type BufferedStreamReader struct {
	input    io.ReadCloser
	buffer   []byte
	mutex    sync.RWMutex
	eof      bool
	err      error
	position int
	done     chan struct{}
}

// NewBufferedStreamReader creates a new buffered stream reader
func NewBufferedStreamReader(input io.ReadCloser) *BufferedStreamReader {
	reader := &BufferedStreamReader{
		input:  input,
		buffer: make([]byte, 0, 1024*1024), // Start with 1MB capacity
		done:   make(chan struct{}),
	}

	// Start background goroutine to consume input immediately
	go reader.consumeInput()

	return reader
}

// consumeInput reads from input stream in background to prevent blocking
func (b *BufferedStreamReader) consumeInput() {
	defer close(b.done)
	defer b.input.Close()

	buffer := make([]byte, 32*1024) // Read in 32KB chunks
	for {
		n, err := b.input.Read(buffer)
		if n > 0 {
			b.mutex.Lock()
			b.buffer = append(b.buffer, buffer[:n]...)
			b.mutex.Unlock()
		}

		if err != nil {
			b.mutex.Lock()
			b.eof = true
			if err != io.EOF {
				b.err = err
			}
			b.mutex.Unlock()
			return
		}
	}
}

// Read implements io.Reader interface
func (b *BufferedStreamReader) Read(p []byte) (n int, err error) {
	for {
		b.mutex.RLock()

		// Check for error
		if b.err != nil {
			b.mutex.RUnlock()
			return 0, b.err
		}

		// Check if we have data to read
		available := len(b.buffer) - b.position
		if available > 0 {
			n = copy(p, b.buffer[b.position:])
			b.position += n
			b.mutex.RUnlock()
			return n, nil
		}

		// No data available, check if EOF
		if b.eof {
			b.mutex.RUnlock()
			return 0, io.EOF
		}

		b.mutex.RUnlock()

		// No data yet, wait a bit before trying again (blocking behavior)
		time.Sleep(10 * time.Millisecond)
	}
}

// Close implements io.Closer interface
func (b *BufferedStreamReader) Close() error {
	select {
	case <-b.done:
		// Already closed
	default:
		// Force close input if still running
		if b.input != nil {
			b.input.Close()
		}
		<-b.done // Wait for background goroutine to finish
	}
	return nil
}

// FFmpegJob represents an active FFmpeg transcoding job
type FFmpegJob struct {
	SessionID    string
	Process      *exec.Cmd
	OutputStream io.ReadCloser
	ErrorStream  io.ReadCloser
	Cancel       context.CancelFunc
	StartTime    time.Time
	Stats        *FFmpegStats
	StatsMutex   sync.RWMutex

	// Output reader to prevent broken pipes (can be FileStreamReader or FileBufferedStreamReader)
	OutputReader io.ReadCloser
}

// FFmpegStats contains real-time transcoding statistics
type FFmpegStats struct {
	Frame      int64
	FPS        float64
	Bitrate    string
	TotalSize  int64
	OutTimeUs  int64
	DupFrames  int64
	DropFrames int64
	Speed      float64
	Progress   float64
	LastUpdate time.Time
}

// getTranscodingDir returns the transcoding directory from environment variable
func getTranscodingDir() string {
	// Use the dedicated transcoding directory environment variable
	transcodingDir := os.Getenv("VIEWRA_TRANSCODING_DIR")
	if transcodingDir != "" {
		return transcodingDir
	}

	// Fallback to legacy behavior for backward compatibility
	dataDir := os.Getenv("VIEWRA_DATA_DIR")
	if dataDir == "" {
		dataDir = "/app/viewra-data" // Default fallback
	}
	return filepath.Join(dataDir, "transcoding")
}

// NewFFmpegService creates a new FFmpeg service
func NewFFmpegService(logger plugins.Logger, configService *config.FFmpegConfigurationService, statusCallback SessionStatusCallback) (*FFmpegService, error) {
	service := &FFmpegService{
		logger:         logger,
		configService:  configService,
		activeJobs:     make(map[string]*FFmpegJob),
		statusCallback: statusCallback,
	}

	// Ensure transcoding directory exists
	transcodingDir := getTranscodingDir()
	if err := os.MkdirAll(transcodingDir, 0755); err != nil {
		logger.Warn("failed to create transcoding directory", "dir", transcodingDir, "error", err)
	}

	// Verify FFmpeg is available
	if err := service.CheckAvailability(); err != nil {
		return nil, fmt.Errorf("FFmpeg not available: %w", err)
	}

	// Start background cleanup routine
	go service.backgroundCleanup()

	return service, nil
}

// CheckAvailability verifies that FFmpeg is available and functional
func (s *FFmpegService) CheckAvailability() error {
	ffmpegPath := s.configService.GetFFmpegConfig().FFmpegPath

	// Check if FFmpeg executable exists and is accessible
	cmd := exec.Command(ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("FFmpeg not found at %s: %w", ffmpegPath, err)
	}

	// Parse version information
	versionStr := string(output)
	if !strings.Contains(versionStr, "ffmpeg version") {
		return fmt.Errorf("invalid FFmpeg executable at %s", ffmpegPath)
	}

	s.logger.Info("FFmpeg availability verified", "path", ffmpegPath)
	return nil
}

// StartTranscode starts a new FFmpeg transcoding process
func (s *FFmpegService) StartTranscode(ctx context.Context, sessionID string, req *plugins.TranscodeRequest) (*FFmpegJob, error) {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	// Check if session already exists
	if _, exists := s.activeJobs[sessionID]; exists {
		return nil, fmt.Errorf("session %s already exists", sessionID)
	}

	// Create temporary output file to eliminate SIGPIPE issues
	tempDir := getTranscodingDir()
	s.logger.Info("Creating temporary output file", "session_id", sessionID, "temp_dir", tempDir)

	// Ensure temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		s.logger.Error("failed to create temp directory", "session_id", sessionID, "temp_dir", tempDir, "error", err)
		return nil, fmt.Errorf("failed to create temp directory %s: %w", tempDir, err)
	}

	// Test directory permissions before creating temp file
	testFile := filepath.Join(tempDir, fmt.Sprintf("test_write_%s.tmp", sessionID))
	if file, err := os.Create(testFile); err != nil {
		s.logger.Error("temp directory not writable", "session_id", sessionID, "temp_dir", tempDir, "error", err)
		return nil, fmt.Errorf("temp directory not writable %s: %w", tempDir, err)
	} else {
		file.Close()
		os.Remove(testFile)
		s.logger.Info("temp directory write test successful", "session_id", sessionID, "temp_dir", tempDir)
	}

	var tempFilePath string

	// Handle DASH/HLS output paths differently
	switch req.TargetContainer {
	case "dash":
		// Create session directory for DASH segments
		sessionDir := filepath.Join(tempDir, fmt.Sprintf("dash_%s", sessionID))
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create DASH session directory: %w", err)
		}
		tempFilePath = filepath.Join(sessionDir, "manifest.mpd")

	case "hls":
		// Create session directory for HLS segments
		sessionDir := filepath.Join(tempDir, fmt.Sprintf("hls_%s", sessionID))
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create HLS session directory: %w", err)
		}
		tempFilePath = filepath.Join(sessionDir, "playlist.m3u8")

	default:
		// Standard progressive output
		tempFile, err := os.CreateTemp(tempDir, fmt.Sprintf("stream_%s_*.mp4", sessionID))
		if err != nil {
			s.logger.Error("failed to create temp file", "session_id", sessionID, "temp_dir", tempDir, "error", err)
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tempFilePath = tempFile.Name()

		// Check file permissions immediately
		if info, err := tempFile.Stat(); err != nil {
			s.logger.Error("failed to stat temp file", "session_id", sessionID, "temp_file", tempFilePath, "error", err)
		} else {
			s.logger.Info("temp file created with permissions", "session_id", sessionID, "temp_file", tempFilePath, "mode", info.Mode(), "size", info.Size())
		}

		tempFile.Close() // Close it so FFmpeg can write to it

		// Remove the file so FFmpeg can create it fresh without any locking issues
		// We just needed the unique filename
		if err := os.Remove(tempFilePath); err != nil {
			s.logger.Warn("failed to remove temp file for fresh creation", "session_id", sessionID, "temp_file", tempFilePath, "error", err)
		} else {
			s.logger.Info("temp file removed for fresh FFmpeg creation", "session_id", sessionID, "temp_file", tempFilePath)
		}
	}

	s.logger.Info("Temporary file path prepared for FFmpeg",
		"session_id", sessionID,
		"temp_file", tempFilePath)

	// Validate input file exists and is accessible
	if _, err := os.Stat(req.InputPath); err != nil {
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("input file not accessible: %w", err)
	}
	s.logger.Info("Input file validated", "session_id", sessionID, "input_path", req.InputPath)

	// Build FFmpeg command arguments to write to file
	args, err := s.buildFFmpegArgs(req)
	if err != nil {
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("failed to build FFmpeg arguments: %w", err)
	}

	// Add output file to args instead of stdout
	args = append(args, tempFilePath)

	// Create cancellable context
	// Create independent context for FFmpeg that's not tied to the request context
	// This prevents FFmpeg from being killed when the HTTP request context is cancelled
	jobCtx, cancel := context.WithCancel(context.Background())

	// Create FFmpeg command
	ffmpegPath := s.configService.GetFFmpegConfig().FFmpegPath
	s.logger.Info("Creating FFmpeg command",
		"session_id", sessionID,
		"ffmpeg_path", ffmpegPath,
		"input_exists", fileExists(req.InputPath),
		"temp_dir_writable", isDirWritable(tempDir))

	cmd := exec.CommandContext(jobCtx, ffmpegPath, args...)

	// Set working directory to a known location
	cmd.Dir = "/app"

	// Capture environment debugging info
	wd, _ := os.Getwd()
	s.logger.Info("Process execution environment",
		"session_id", sessionID,
		"working_directory", wd,
		"cmd_dir", cmd.Dir,
		"path_env", os.Getenv("PATH"),
		"user", os.Getenv("USER"),
		"home", os.Getenv("HOME"))

	// Note: Removed Setpgid isolation to allow proper process termination

	// Log the full FFmpeg command for debugging
	s.logger.Info("Starting FFmpeg process with file output",
		"session_id", sessionID,
		"command", ffmpegPath,
		"args", strings.Join(args, " "),
		"input", req.InputPath,
		"output_file", tempFilePath,
		"target_codec", req.TargetCodec,
		"resolution", req.Resolution)

	// Set up stderr pipe for progress monitoring
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Note: Not capturing stdout to avoid interfering with FFmpeg's normal operation

	// Create appropriate reader based on container type
	var outputReader io.ReadCloser

	switch req.TargetContainer {
	case "dash", "hls":
		// Use DASH/HLS reader for manifest-based streaming
		sessionDir := filepath.Dir(tempFilePath)
		outputReader = NewDashHlsStreamReader(tempFilePath, sessionDir, sessionID, s)
	default:
		// Use file reader for progressive streaming
		fileReader := &FileStreamReader{
			filePath:     tempFilePath,
			sessionID:    sessionID,
			service:      s,
			done:         make(chan struct{}),
			lastReadTime: time.Now(),
		}
		outputReader = fileReader

		// Start idle connection monitor for this session (only for progressive streams)
		go s.monitorIdleConnection(sessionID, fileReader)
	}

	// Create job
	job := &FFmpegJob{
		SessionID:    sessionID,
		Process:      cmd,
		OutputStream: nil, // No stdout capture to avoid interference
		ErrorStream:  stderr,
		Cancel:       cancel,
		StartTime:    time.Now(),
		Stats:        &FFmpegStats{LastUpdate: time.Now()},
		OutputReader: outputReader,
	}

	s.logger.Info("Starting FFmpeg process", "session_id", sessionID)

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		stderr.Close()
		os.Remove(tempFilePath)
		return nil, fmt.Errorf("failed to start FFmpeg process: %w", err)
	}

	s.logger.Info("FFmpeg process started successfully",
		"session_id", sessionID,
		"pid", cmd.Process.Pid)

	// Store job
	s.activeJobs[sessionID] = job

	// Start monitoring goroutines
	go s.monitorProgress(job)
	go s.monitorProcess(job)

	s.logger.Info("FFmpeg transcoding started with file output",
		"session_id", sessionID,
		"pid", cmd.Process.Pid,
		"input", req.InputPath,
		"output_file", tempFilePath,
		"target_codec", req.TargetCodec,
		"resolution", req.Resolution)

	return job, nil
}

// Helper functions for debugging
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDirWritable(path string) bool {
	testFile := filepath.Join(path, ".write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile)
	return true
}

// StopTranscode stops an active transcoding job
func (s *FFmpegService) StopTranscode(sessionID string) error {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	job, exists := s.activeJobs[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	s.logger.Info("Stopping FFmpeg transcoding", "session_id", sessionID, "pid", job.Process.Process.Pid)

	// Cancel the context first
	job.Cancel()

	// Force kill the process if it's still running
	if job.Process != nil && job.Process.Process != nil {
		process := job.Process.Process

		// Check if process is still running
		if process.Pid > 0 {
			s.logger.Info("Force killing FFmpeg process", "session_id", sessionID, "pid", process.Pid)

			// Kill the process
			if err := process.Kill(); err != nil {
				s.logger.Warn("Failed to kill FFmpeg process", "session_id", sessionID, "pid", process.Pid, "error", err)
			} else {
				s.logger.Info("FFmpeg process killed successfully", "session_id", sessionID, "pid", process.Pid)
			}
		}
	}

	// Clean up temp file if it exists
	if fileReader, ok := job.OutputReader.(*FileStreamReader); ok {
		if fileReader.filePath != "" {
			if err := os.Remove(fileReader.filePath); err != nil {
				s.logger.Warn("Failed to clean up temp file", "session_id", sessionID, "file", fileReader.filePath, "error", err)
			} else {
				s.logger.Debug("Cleaned up temp file", "session_id", sessionID, "file", fileReader.filePath)
			}
		}
	}

	// Remove from active jobs
	delete(s.activeJobs, sessionID)

	s.logger.Info("FFmpeg transcoding stopped and cleaned up", "session_id", sessionID)
	return nil
}

// GetJob retrieves an active job
func (s *FFmpegService) GetJob(sessionID string) (*FFmpegJob, bool) {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	job, exists := s.activeJobs[sessionID]
	return job, exists
}

// ListActiveJobs returns all active jobs
func (s *FFmpegService) ListActiveJobs() []*FFmpegJob {
	s.jobsMutex.RLock()
	defer s.jobsMutex.RUnlock()

	jobs := make([]*FFmpegJob, 0, len(s.activeJobs))
	for _, job := range s.activeJobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetJobStats returns current statistics for a job
func (s *FFmpegService) GetJobStats(sessionID string) (*FFmpegStats, error) {
	job, exists := s.GetJob(sessionID)
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	job.StatsMutex.RLock()
	defer job.StatsMutex.RUnlock()

	// Create a copy of the stats
	stats := *job.Stats
	return &stats, nil
}

// buildFFmpegArgs constructs the FFmpeg command arguments
func (s *FFmpegService) buildFFmpegArgs(req *plugins.TranscodeRequest) ([]string, error) {
	cfg := s.configService.GetFFmpegConfig()

	// Base arguments with additional safety measures for HEVC input
	args := []string{}

	// Add seek time if specified (for seek-ahead functionality)
	if req.StartTime > 0 {
		startTimeStr := fmt.Sprintf("%d", req.StartTime)
		args = append(args, "-ss", startTimeStr) // Seek to start time before input
		s.logger.Info("FFmpeg seek-ahead configured", "start_time", req.StartTime, "start_time_str", startTimeStr)
	}

	// Hardware acceleration must come before input
	args = append(args,
		"-hwaccel", "auto", // Use hardware acceleration if available (safer than forcing)
		"-i", req.InputPath,
		"-progress", "pipe:2", // Send progress to stderr
		"-nostats",                  // Disable default stats output
		"-fflags", "+flush_packets", // Flush packets immediately
		"-max_muxing_queue_size", "1024", // Increase muxing queue size to prevent blocking
		"-avoid_negative_ts", "make_zero", // Fix timestamp issues early
		"-analyzeduration", "20000000", // 20 seconds analysis for complex files
		"-probesize", "20000000", // 20MB probe size for complex files
		"-strict", "experimental", // Allow experimental features if needed
		"-thread_queue_size", "512", // Increase thread queue size for HEVC
		"-vsync", "0", // Disable video sync to prevent timing issues
		"-copytb", "1", // Copy input stream time base to output
	)

	// Limit threads for stability with HEVC input
	threadLimit := cfg.Threads
	if threadLimit <= 0 || threadLimit > 4 {
		threadLimit = 4 // Limit to 4 threads for HEVC stability
	}
	args = append(args, "-threads", strconv.Itoa(threadLimit))

	// Video codec settings
	args = append(args, s.buildVideoCodecArgs(req, cfg)...)

	// Audio codec settings
	args = append(args, s.buildAudioCodecArgs(req, cfg)...)

	// Resolution/scaling settings
	if req.Resolution != "" {
		scaleFilter := s.buildScaleFilter(req.Resolution)
		if scaleFilter != "" {
			args = append(args, "-vf", scaleFilter)
		}
	}

	// Subtitle settings
	if req.Subtitles != nil && req.Subtitles.Enabled {
		args = append(args, s.buildSubtitleArgs(req)...)
	}

	// Container-specific settings
	switch req.TargetContainer {
	case "dash":
		args = append(args,
			"-f", "dash",
			"-seg_duration", "4", // 4-second segments
			"-use_template", "1", // Use template-based naming
			"-use_timeline", "1", // Use timeline for better seeking
			"-adaptation_sets", "id=0,streams=v id=1,streams=a", // Separate video/audio adaptation sets
			"-init_seg_name", "init-$RepresentationID$.m4s", // Initialization segment naming
			"-media_seg_name", "chunk-$RepresentationID$-$Number$.m4s", // Media segment naming
		)
	case "hls":
		args = append(args,
			"-f", "hls",
			"-hls_time", "4", // 4-second segments
			"-hls_playlist_type", "vod", // Video-on-demand playlist
			"-hls_segment_type", "mpegts", // MPEG-TS segments for compatibility
			"-hls_segment_filename", "segment_%03d.ts", // Segment naming pattern
		)
	default:
		// Standard progressive containers (mp4, webm, mkv)
		args = append(args,
			"-f", req.TargetContainer,
			"-movflags", "frag_keyframe+empty_moov", // Enable streaming for mp4
		)
	}

	// Additional options
	for key, value := range req.Options {
		args = append(args, "-"+key, value)
	}

	// Note: Output file path will be added by StartTranscode method

	return args, nil
}

// buildVideoCodecArgs builds video codec specific arguments
func (s *FFmpegService) buildVideoCodecArgs(req *plugins.TranscodeRequest, cfg *config.FFmpegConfig) []string {
	args := []string{}

	switch req.TargetCodec {
	case "h264":
		args = append(args, "-c:v", "libx264")
		// Use more conservative preset for HEVC input stability
		preset := "medium"
		if cfg.Preset == "ultrafast" || cfg.Preset == "superfast" || cfg.Preset == "veryfast" {
			preset = "fast" // Safer for HEVC input
		} else if cfg.Preset != "" {
			preset = cfg.Preset
		}
		args = append(args, "-preset", preset)

		// Use conservative profile and level for better compatibility
		args = append(args, "-profile:v", "high")
		args = append(args, "-level", "4.1")
		args = append(args, "-pix_fmt", "yuv420p") // Force standard pixel format

		if req.Quality > 0 {
			args = append(args, "-crf", strconv.Itoa(req.Quality))
		} else {
			args = append(args, "-crf", strconv.FormatFloat(cfg.CRFH264, 'f', 1, 64))
		}

	case "hevc":
		args = append(args, "-c:v", "libx265")
		args = append(args, "-preset", cfg.Preset)
		if req.Quality > 0 {
			args = append(args, "-crf", strconv.Itoa(req.Quality))
		} else {
			args = append(args, "-crf", strconv.FormatFloat(cfg.CRFHEVC, 'f', 1, 64))
		}

	case "vp8":
		args = append(args, "-c:v", "libvpx")
		args = append(args, "-quality", "good")
		args = append(args, "-cpu-used", "0")

	case "vp9":
		args = append(args, "-c:v", "libvpx-vp9")
		args = append(args, "-quality", "good")
		args = append(args, "-cpu-used", "0")

	case "av1":
		args = append(args, "-c:v", "libaom-av1")
		args = append(args, "-cpu-used", "4")

	default:
		// Default to H.264 with conservative settings
		args = append(args, "-c:v", "libx264")
		args = append(args, "-preset", "fast") // Conservative default
		args = append(args, "-profile:v", "high")
		args = append(args, "-level", "4.1")
		args = append(args, "-pix_fmt", "yuv420p")
		args = append(args, "-crf", strconv.FormatFloat(cfg.CRFH264, 'f', 1, 64))
	}

	// Bitrate settings
	if req.Bitrate > 0 {
		bitrateStr := strconv.Itoa(req.Bitrate) + "k"
		args = append(args, "-b:v", bitrateStr)

		// Set max bitrate and buffer size
		maxBitrate := strconv.Itoa(int(float64(req.Bitrate)*cfg.MaxBitrateMultiplier)) + "k"
		bufSize := strconv.Itoa(int(float64(req.Bitrate)*cfg.BufferSizeMultiplier)) + "k"
		args = append(args, "-maxrate", maxBitrate)
		args = append(args, "-bufsize", bufSize)
	}

	// Video stream mapping - always map first video stream
	args = append(args, "-map", "0:v:0?") // Use ? to make it optional in case no video stream exists

	return args
}

// buildAudioCodecArgs builds audio codec specific arguments
func (s *FFmpegService) buildAudioCodecArgs(req *plugins.TranscodeRequest, cfg *config.FFmpegConfig) []string {
	args := []string{}

	audioCodec := req.AudioCodec
	if audioCodec == "" {
		audioCodec = cfg.AudioCodec
	}

	audioBitrate := req.AudioBitrate
	if audioBitrate == 0 {
		audioBitrate = cfg.AudioBitrate
	}

	// DEBUG: Log what we're building
	s.logger.Info("buildAudioCodecArgs called",
		"audioCodec", audioCodec,
		"audioBitrate", audioBitrate,
		"config_sample_rate", cfg.AudioSampleRate)

	// Basic audio codec and bitrate
	args = append(args, "-c:a", audioCodec)
	args = append(args, "-b:a", strconv.Itoa(audioBitrate)+"k")

	// Audio stream mapping - always map first audio stream
	args = append(args, "-map", "0:a:0?") // Use ? to make it optional in case no audio stream exists

	// Simplified audio settings to prevent segfaults
	if audioCodec == "aac" {
		// AAC Low Complexity profile for better compatibility
		args = append(args, "-profile:a", "aac_low")

		// Use simple audio normalization instead of complex filter chain
		// This prevents segfaults with HEVC/EAC3 input files
		args = append(args, "-af", "volume=0.8")
	}

	// Sample rate - use 44.1kHz for better compatibility
	args = append(args, "-ar", strconv.Itoa(cfg.AudioSampleRate))

	// Channel configuration - force stereo for web compatibility with proper downmixing
	args = append(args, "-ac", "2")

	// Audio stream selection - always map first audio stream
	args = append(args, "-map", "0:a:0?") // Use ? to make it optional in case no audio stream exists

	// DEBUG: Log what we built
	s.logger.Info("buildAudioCodecArgs result", "args", args)

	return args
}

// buildSubtitleArgs builds subtitle handling arguments
func (s *FFmpegService) buildSubtitleArgs(req *plugins.TranscodeRequest) []string {
	args := []string{}

	if req.Subtitles.BurnIn {
		// Burn subtitles into video
		subtitleFilter := fmt.Sprintf("subtitles=%s", req.InputPath)
		if req.Subtitles.StreamIdx >= 0 {
			subtitleFilter += fmt.Sprintf(":si=%d", req.Subtitles.StreamIdx)
		}
		if req.Subtitles.FontSize > 0 {
			subtitleFilter += fmt.Sprintf(":force_style='FontSize=%d'", req.Subtitles.FontSize)
		}
		if req.Subtitles.FontColor != "" {
			subtitleFilter += fmt.Sprintf(":force_style='PrimaryColour=%s'", req.Subtitles.FontColor)
		}

		// This would need to be combined with any existing video filters
		args = append(args, "-vf", subtitleFilter)
	} else {
		// Include subtitle stream
		cfg := s.configService.GetFFmpegConfig()
		args = append(args, "-c:s", cfg.SoftCodec)
		if req.Subtitles.StreamIdx >= 0 {
			args = append(args, "-map", fmt.Sprintf("0:s:%d", req.Subtitles.StreamIdx))
		}
	}

	return args
}

// buildScaleFilter creates a scale filter for resolution changes
func (s *FFmpegService) buildScaleFilter(resolution string) string {
	switch strings.ToLower(resolution) {
	case "480p":
		// Use safer scaling with format specification for HEVC compatibility
		return "scale=-2:480:flags=lanczos,format=yuv420p"
	case "720p":
		return "scale=-2:720:flags=lanczos,format=yuv420p"
	case "1080p":
		return "scale=-2:1080:flags=lanczos,format=yuv420p"
	case "1440p":
		return "scale=-2:1440:flags=lanczos,format=yuv420p"
	case "2160p", "4k":
		return "scale=-2:2160:flags=lanczos,format=yuv420p"
	default:
		return ""
	}
}

// monitorProgress monitors FFmpeg progress output
func (s *FFmpegService) monitorProgress(job *FFmpegJob) {
	defer job.ErrorStream.Close()

	scanner := bufio.NewScanner(job.ErrorStream)
	progressRegex := regexp.MustCompile(`(\w+)=\s*([^\s]+)`)
	lineCount := 0

	s.logger.Info("Starting progress monitoring", "session_id", job.SessionID)

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Log first few lines and any error-looking lines for debugging
		if lineCount <= 10 || strings.Contains(strings.ToLower(line), "error") || strings.Contains(strings.ToLower(line), "fail") {
			s.logger.Info("FFmpeg stderr", "session_id", job.SessionID, "line", line)
		}

		// Parse progress information
		matches := progressRegex.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		job.StatsMutex.Lock()
		for _, match := range matches {
			if len(match) != 3 {
				continue
			}

			key := match[1]
			value := match[2]

			switch key {
			case "frame":
				if frame, err := strconv.ParseInt(value, 10, 64); err == nil {
					job.Stats.Frame = frame
				}
			case "fps":
				if fps, err := strconv.ParseFloat(value, 64); err == nil {
					job.Stats.FPS = fps
				}
			case "bitrate":
				job.Stats.Bitrate = value
			case "total_size":
				if size, err := strconv.ParseInt(value, 10, 64); err == nil {
					job.Stats.TotalSize = size
				}
			case "out_time_us":
				if timeUs, err := strconv.ParseInt(value, 10, 64); err == nil {
					job.Stats.OutTimeUs = timeUs
				}
			case "dup_frames":
				if dup, err := strconv.ParseInt(value, 10, 64); err == nil {
					job.Stats.DupFrames = dup
				}
			case "drop_frames":
				if drop, err := strconv.ParseInt(value, 10, 64); err == nil {
					job.Stats.DropFrames = drop
				}
			case "speed":
				if speed, err := strconv.ParseFloat(strings.TrimSuffix(value, "x"), 64); err == nil {
					job.Stats.Speed = speed
				}
			case "progress":
				if value == "end" {
					job.Stats.Progress = 1.0
				}
			}
		}
		job.Stats.LastUpdate = time.Now()
		job.StatsMutex.Unlock()
	}

	s.logger.Info("Progress monitoring completed",
		"session_id", job.SessionID,
		"total_lines", lineCount,
		"scanner_error", scanner.Err())

	if err := scanner.Err(); err != nil {
		s.logger.Warn("error reading FFmpeg progress", "session_id", job.SessionID, "error", err)
	}
}

// monitorStdout monitors FFmpeg stdout for debugging
func (s *FFmpegService) monitorStdout(job *FFmpegJob) {
	if job.OutputStream == nil {
		return
	}
	defer job.OutputStream.Close()

	buf := make([]byte, 1024)
	for {
		n, err := job.OutputStream.Read(buf)
		if n > 0 {
			output := string(buf[:n])
			s.logger.Debug("FFmpeg stdout", "session_id", job.SessionID, "output", output)
		}
		if err != nil {
			if err != io.EOF {
				s.logger.Warn("Error reading FFmpeg stdout", "session_id", job.SessionID, "error", err)
			}
			break
		}
	}
}

// monitorProcess monitors the FFmpeg process lifecycle
func (s *FFmpegService) monitorProcess(job *FFmpegJob) {
	// Log process start details
	s.logger.Info("FFmpeg process monitor started",
		"session_id", job.SessionID,
		"pid", job.Process.Process.Pid,
		"start_time", job.StartTime)

	// Wait for process to complete
	err := job.Process.Wait()

	duration := time.Since(job.StartTime)

	// Log detailed completion information
	s.logger.Info("FFmpeg process monitor completed",
		"session_id", job.SessionID,
		"duration", duration,
		"error", err)

	if err != nil {
		// Log more detailed error information
		processState := job.Process.ProcessState
		exitCode := -1
		if processState != nil && processState.Exited() {
			exitCode = processState.ExitCode()
		}

		s.logger.Error("FFmpeg process ended with error",
			"session_id", job.SessionID,
			"error", err,
			"exit_code", exitCode,
			"duration", duration,
			"pid", job.Process.Process.Pid)

		// Mark the job as failed immediately
		job.StatsMutex.Lock()
		job.Stats.Progress = -1 // Use -1 to indicate failure
		job.StatsMutex.Unlock()

		// Notify session manager that the job failed
		if s.statusCallback != nil {
			s.statusCallback(job.SessionID, string(plugins.TranscodeStatusFailed), fmt.Sprintf("FFmpeg process failed with exit code %d: %v", exitCode, err))
		}

		// If the process crashed very quickly (< 1 second), don't remove it immediately
		// to allow GetOutputStream to return a proper error
		if duration < time.Second {
			s.logger.Warn("FFmpeg process crashed very quickly, keeping job for error reporting",
				"session_id", job.SessionID,
				"duration", duration)

			// Remove it after a short delay to allow error reporting
			go func() {
				time.Sleep(5 * time.Second)
				s.jobsMutex.Lock()
				if existingJob, exists := s.activeJobs[job.SessionID]; exists && existingJob == job {
					delete(s.activeJobs, job.SessionID)
					s.logger.Debug("Delayed cleanup of failed job", "session_id", job.SessionID)

					// Clean up the temporary output file if it's a FileStreamReader
					if fileReader, ok := existingJob.OutputReader.(*FileStreamReader); ok {
						if fileReader.filePath != "" {
							if err := os.Remove(fileReader.filePath); err != nil {
								s.logger.Warn("Failed to clean up temporary file",
									"session_id", job.SessionID, "file", fileReader.filePath, "error", err)
							} else {
								s.logger.Debug("Cleaned up temporary file",
									"session_id", job.SessionID, "file", fileReader.filePath)
							}
						}
					}
				}
				s.jobsMutex.Unlock()
			}()
		} else {
			// Normal cleanup for jobs that ran longer
			s.jobsMutex.Lock()
			delete(s.activeJobs, job.SessionID)
			s.jobsMutex.Unlock()

			// Clean up the temporary output file if it's a FileStreamReader
			if fileReader, ok := job.OutputReader.(*FileStreamReader); ok {
				if fileReader.filePath != "" {
					if err := os.Remove(fileReader.filePath); err != nil {
						s.logger.Warn("Failed to clean up temporary file",
							"session_id", job.SessionID, "file", fileReader.filePath, "error", err)
					} else {
						s.logger.Debug("Cleaned up temporary file",
							"session_id", job.SessionID, "file", fileReader.filePath)
					}
				}
			}
		}
	} else {
		s.logger.Info("FFmpeg process completed successfully",
			"session_id", job.SessionID,
			"duration", duration)

		// Mark job as completed but keep it available for streaming
		job.StatsMutex.Lock()
		job.Stats.Progress = 1.0 // Mark as completed
		job.StatsMutex.Unlock()

		// Notify session manager that the job completed
		if s.statusCallback != nil {
			s.statusCallback(job.SessionID, string(plugins.TranscodeStatusCompleted), "")
		}

		// Keep completed jobs available for streaming for 30 seconds
		// This allows clients to connect and stream the completed transcoding
		go func() {
			time.Sleep(30 * time.Second)
			s.jobsMutex.Lock()
			if existingJob, exists := s.activeJobs[job.SessionID]; exists && existingJob == job {
				delete(s.activeJobs, job.SessionID)
				s.logger.Debug("Delayed cleanup of completed job", "session_id", job.SessionID)

				// Clean up the temporary output file if it's a FileStreamReader
				if fileReader, ok := existingJob.OutputReader.(*FileStreamReader); ok {
					if fileReader.filePath != "" {
						if err := os.Remove(fileReader.filePath); err != nil {
							s.logger.Warn("Failed to clean up temporary file",
								"session_id", job.SessionID, "file", fileReader.filePath, "error", err)
						} else {
							s.logger.Debug("Cleaned up temporary file",
								"session_id", job.SessionID, "file", fileReader.filePath)
						}
					}
				}
			}
			s.jobsMutex.Unlock()
		}()
	}

	// Close streams
	if job.OutputReader != nil {
		job.OutputReader.Close()
	}
	if job.OutputStream != nil {
		job.OutputStream.Close()
	}
	if job.ErrorStream != nil {
		job.ErrorStream.Close()
	}
}

// GetOutputStream returns the output stream for a job
func (s *FFmpegService) GetOutputStream(sessionID string) (io.ReadCloser, error) {
	job, exists := s.GetJob(sessionID)
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Check if job has failed
	job.StatsMutex.RLock()
	progress := job.Stats.Progress
	job.StatsMutex.RUnlock()

	if progress == -1 {
		return nil, fmt.Errorf("session %s failed: FFmpeg process crashed", sessionID)
	}

	// Return the buffered reader instead of raw stdout
	return job.OutputReader, nil
}

// Cleanup stops all active jobs (called during shutdown)
func (s *FFmpegService) Cleanup() {
	s.jobsMutex.Lock()
	defer s.jobsMutex.Unlock()

	s.logger.Info("cleaning up FFmpeg service", "active_jobs", len(s.activeJobs))

	for sessionID, job := range s.activeJobs {
		s.logger.Info("stopping job during cleanup", "session_id", sessionID)
		job.Cancel()
	}

	// Clear the map
	s.activeJobs = make(map[string]*FFmpegJob)
}

// backgroundCleanup is a background routine that monitors for abandoned sessions and stops FFmpeg processes that have been running too long without client connections
func (s *FFmpegService) backgroundCleanup() {
	s.logger.Info("Starting background cleanup routine")

	for {
		time.Sleep(1 * time.Minute) // Check every 1 minute

		s.jobsMutex.Lock()

		// Log current session status for debugging
		activeCount := len(s.activeJobs)
		s.logger.Info("Background cleanup check", "active_sessions", activeCount)

		// Debug log all active sessions
		for sessionID, job := range s.activeJobs {
			duration := time.Since(job.StartTime)
			s.logger.Info("Active session",
				"session_id", sessionID,
				"duration", duration,
				"start_time", job.StartTime)
		}

		// Check session limits
		if activeCount > 10 { // Maximum 10 concurrent sessions
			s.logger.Warn("Too many active sessions, cleaning up oldest", "active_count", activeCount)

			// Find the oldest session and stop it
			var oldestSession string
			var oldestTime time.Time
			for sessionID, job := range s.activeJobs {
				if oldestSession == "" || job.StartTime.Before(oldestTime) {
					oldestSession = sessionID
					oldestTime = job.StartTime
				}
			}

			if oldestSession != "" {
				s.logger.Info("Stopping oldest session due to session limit", "session_id", oldestSession, "age", time.Since(oldestTime))
				if job, exists := s.activeJobs[oldestSession]; exists {
					job.Cancel()
					delete(s.activeJobs, oldestSession)
				}
			}
		}

		// Check for long-running sessions
		for sessionID, job := range s.activeJobs {
			duration := time.Since(job.StartTime)

			// Emergency: Stop sessions running longer than 15 minutes (most videos should stream faster than realtime)
			if duration > 15*time.Minute {
				s.logger.Warn("Job has been running too long (>15min), emergency stop",
					"session_id", sessionID,
					"duration", duration)

				// Force kill the process
				if job.Process != nil && job.Process.Process != nil {
					if killErr := job.Process.Process.Kill(); killErr != nil {
						s.logger.Warn("Failed to emergency kill process", "session_id", sessionID, "error", killErr)
					} else {
						s.logger.Info("Emergency killed long-running process", "session_id", sessionID)
					}
				}

				job.Cancel()
				delete(s.activeJobs, sessionID)
				continue
			}

			// Stop sessions running longer than 2 hours
			if duration > 2*time.Hour {
				s.logger.Info("Job has been running too long, stopping",
					"session_id", sessionID,
					"duration", duration)
				job.Cancel()
				delete(s.activeJobs, sessionID)
				continue
			}

			// Warn about sessions running longer than 1 hour
			if duration > 1*time.Hour {
				s.logger.Warn("Job has been running for a long time",
					"session_id", sessionID,
					"duration", duration)
			}

			// Debug: Warn about sessions running longer than 5 minutes for immediate debugging
			if duration > 5*time.Minute {
				s.logger.Warn("Job has been running for more than 5 minutes",
					"session_id", sessionID,
					"duration", duration)
			}
		}
		s.jobsMutex.Unlock()

		// Check for orphaned FFmpeg processes (running but not in activeJobs)
		s.cleanupOrphanedProcesses()
	}
}

// cleanupOrphanedProcesses finds and kills FFmpeg processes that are no longer tracked
func (s *FFmpegService) cleanupOrphanedProcesses() {
	// Get list of current activeJobs PIDs for comparison
	s.jobsMutex.RLock()
	activePIDs := make(map[int]string)
	for sessionID, job := range s.activeJobs {
		if job.Process != nil && job.Process.Process != nil {
			activePIDs[job.Process.Process.Pid] = sessionID
		}
	}
	s.jobsMutex.RUnlock()

	// Find all ffmpeg processes with our output pattern
	cmd := exec.Command("sh", "-c", "ps aux | grep 'ffmpeg.*stream_.*\\.mp4' | grep -v grep | awk '{print $1}' || true")
	output, err := cmd.Output()
	if err != nil {
		s.logger.Warn("Failed to check for orphaned processes", "error", err)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	orphanedCount := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		pidStr := strings.TrimSpace(line)
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			s.logger.Warn("Failed to parse PID", "pid_string", pidStr, "error", err)
			continue
		}

		// Check if this PID is tracked in activeJobs
		if sessionID, exists := activePIDs[pid]; exists {
			s.logger.Debug("Found tracked FFmpeg process", "pid", pid, "session_id", sessionID)
			continue
		}

		// This is an orphaned process - kill it
		s.logger.Warn("Found orphaned FFmpeg process, killing it", "pid", pid)

		killCmd := exec.Command("kill", "-9", pidStr)
		if killErr := killCmd.Run(); killErr != nil {
			s.logger.Warn("Failed to kill orphaned process", "pid", pid, "error", killErr)
		} else {
			s.logger.Info("Successfully killed orphaned FFmpeg process", "pid", pid)
			orphanedCount++
		}
	}

	if orphanedCount > 0 {
		s.logger.Info("Cleaned up orphaned processes", "count", orphanedCount)
	}
}

// monitorIdleConnection monitors idle connections for a session
func (s *FFmpegService) monitorIdleConnection(sessionID string, fileReader *FileStreamReader) {
	s.logger.Info("Starting idle connection monitor", "session_id", sessionID)

	for {
		time.Sleep(2 * time.Minute) // Check every 2 minutes

		// Check if the session still exists
		if _, exists := s.GetJob(sessionID); !exists {
			s.logger.Debug("Session no longer exists, stopping idle monitor", "session_id", sessionID)
			break
		}

		// Check if the connection is idle (safely)
		fileReader.mutex.RLock()
		lastRead := fileReader.lastReadTime
		fileReader.mutex.RUnlock()

		duration := time.Since(lastRead)
		if duration > 10*time.Minute { // Stop if idle for more than 10 minutes
			s.logger.Info("Session is idle for too long, stopping FFmpeg process",
				"session_id", sessionID,
				"idle_duration", duration)

			// Stop the transcoding session
			if err := s.StopTranscode(sessionID); err != nil {
				s.logger.Warn("Failed to stop idle session", "session_id", sessionID, "error", err)
			}
			break
		}
	}
}

// DashHlsStreamReader handles DASH/HLS manifest and segment streaming
type DashHlsStreamReader struct {
	manifestPath string
	sessionDir   string
	sessionID    string
	service      *FFmpegService
	mutex        sync.RWMutex
	done         chan struct{}
	lastReadTime time.Time
	isCompleted  bool
	manifestData []byte
}

// NewDashHlsStreamReader creates a new DASH/HLS stream reader
func NewDashHlsStreamReader(manifestPath, sessionDir, sessionID string, service *FFmpegService) *DashHlsStreamReader {
	return &DashHlsStreamReader{
		manifestPath: manifestPath,
		sessionDir:   sessionDir,
		sessionID:    sessionID,
		service:      service,
		done:         make(chan struct{}),
		lastReadTime: time.Now(),
		isCompleted:  false,
	}
}

func (d *DashHlsStreamReader) Read(p []byte) (n int, err error) {
	d.mutex.Lock()
	d.lastReadTime = time.Now()
	d.mutex.Unlock()

	// Check if reader is closed
	select {
	case <-d.done:
		return 0, io.EOF
	default:
	}

	// For DASH/HLS, we serve the current manifest file
	// The manifest is updated by FFmpeg as transcoding progresses
	file, err := os.Open(d.manifestPath)
	if err != nil {
		// Wait for manifest to be created by FFmpeg
		maxRetries := 50 // Wait up to 5 seconds
		for i := 0; i < maxRetries; i++ {
			time.Sleep(100 * time.Millisecond)
			file, err = os.Open(d.manifestPath)
			if err == nil {
				break
			}
			if i == maxRetries-1 {
				d.service.logger.Error("manifest file not found after retries",
					"path", d.manifestPath, "session_id", d.sessionID)
				return 0, fmt.Errorf("manifest file not available: %w", err)
			}
		}
	}
	defer file.Close()

	// Read the entire manifest content
	content, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("failed to read manifest: %w", err)
	}

	if len(content) == 0 {
		// Manifest not ready yet or empty
		d.service.logger.Debug("manifest file empty, waiting for FFmpeg",
			"path", d.manifestPath, "session_id", d.sessionID)
		return 0, nil // Return 0 bytes but no error, client should retry
	}

	// Copy content to provided buffer
	n = copy(p, content)

	// For live streaming, we want to continuously serve the manifest
	// For VOD, we serve it once completely
	if n == len(content) {
		// We've copied the complete manifest
		d.manifestData = content
		d.isCompleted = true
		return n, io.EOF // Signal end of current manifest read
	}

	return n, nil // Partial read, more data available
}

func (d *DashHlsStreamReader) Close() error {
	select {
	case <-d.done:
		// Already closed
		return nil
	default:
		close(d.done)

		// Clean up session directory after a delay to allow ongoing segment requests
		if d.sessionDir != "" {
			go func() {
				time.Sleep(2 * time.Minute) // Give time for any ongoing segment requests
				d.service.logger.Info("cleaning up session directory",
					"session_id", d.sessionID, "dir", d.sessionDir)
				if err := os.RemoveAll(d.sessionDir); err != nil {
					d.service.logger.Warn("failed to clean up session directory",
						"session_id", d.sessionID, "dir", d.sessionDir, "error", err)
				}
			}()
		}

		return nil
	}
}

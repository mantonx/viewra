package transcoding

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/executor"
	pkgLogger "github.com/mantonx/viewra/internal/pkg/logger"
)

// Service provides video transcoding functionality for HLS streaming.
type Service interface {
	// TranscodeToHLS transcodes a video file to HLS format at the specified quality level.
	// It updates the transcode job in the repository throughout the process.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - job: The transcode job to execute (must be in queued status)
	//   - inputPath: Absolute path to the input video file
	//   - outputBaseDir: Base directory where HLS output will be stored
	//
	// The output structure will be:
	//   outputBaseDir/
	//     hls/
	//       <mediaID>/
	//         <quality>/
	//           playlist.m3u8
	//           segment_*.ts
	//
	// Returns an error if transcoding fails at any stage.
	TranscodeToHLS(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error

	// RemuxToHLS remuxes a video to HLS format by copying streams without re-encoding (2-5 min).
	// Used when video is already H.264 and audio is stereo, but container is incompatible.
	RemuxToHLS(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error

	// RemuxWithAudioDownmixHLS remuxes video to HLS while copying video and downmixing multi-channel audio (5-10 min).
	// Used when video is H.264 compatible but audio has too many channels for browser playback.
	RemuxWithAudioDownmixHLS(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error
}

// service implements the Service interface.
type service struct {
	jobExecutor *executor.JobExecutor
}

// NewService creates a new transcoding service.
// It verifies that FFmpeg is available in the system PATH.
func NewService(repo transcode.Repository, logger *slog.Logger) (Service, error) {
	ffmpegExec, err := executor.NewFFmpegExecutor()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ffmpeg executor: %w", err)
	}

	logger = pkgLogger.DefaultIfNil(logger)

	jobExec := executor.NewJobExecutor(repo, ffmpegExec, logger)

	return &service{
		jobExecutor: jobExec,
	}, nil
}

// TranscodeToHLS implements the Service interface.
func (s *service) TranscodeToHLS(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error {
	ffmpegExec := s.jobExecutor.FFmpegExecutorRef()
	return s.jobExecutor.Execute(ctx, job, inputPath, outputBaseDir, "transcode", ffmpegExec.TranscodeToHLS)
}

// RemuxToHLS implements the Service interface for remuxing operations.
func (s *service) RemuxToHLS(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error {
	ffmpegExec := s.jobExecutor.FFmpegExecutorRef()
	return s.jobExecutor.Execute(ctx, job, inputPath, outputBaseDir, "remux", ffmpegExec.RemuxToHLS)
}

// RemuxWithAudioDownmixHLS implements the Service interface for remux with audio downmix operations.
func (s *service) RemuxWithAudioDownmixHLS(ctx context.Context, job *transcode.TranscodeJob, inputPath, outputBaseDir string) error {
	ffmpegExec := s.jobExecutor.FFmpegExecutorRef()
	return s.jobExecutor.Execute(ctx, job, inputPath, outputBaseDir, "remux with audio downmix", ffmpegExec.RemuxWithAudioDownmixHLS)
}

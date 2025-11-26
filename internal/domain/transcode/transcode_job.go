package transcode

import (
	"errors"
	"time"
)

// Quality levels for transcoding
const (
	Quality360p  = "360p"
	Quality720p  = "720p"
	Quality1080p = "1080p"
	Quality4K    = "4k"
)

// Job types for different streaming strategies
const (
	TypeRemux      = "remux"       // Copy streams to DASH container (2-5 min)
	TypeRemuxAudio = "remux_audio" // Copy video, downmix audio to stereo (5-10 min)
	TypeTranscode  = "transcode"   // Full re-encode (20-60 min)
)

// Job status values
const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Common errors
var (
	ErrJobNotFound      = errors.New("transcode job not found")
	ErrJobAlreadyExists = errors.New("transcode job already exists for this media and quality")
	ErrInvalidQuality   = errors.New("invalid quality level")
	ErrInvalidStatus    = errors.New("invalid status")
	ErrInvalidMediaID   = errors.New("invalid media ID")
	ErrInvalidProgress  = errors.New("progress must be between 0 and 100")
	ErrInvalidType      = errors.New("invalid job type")
)

// TranscodeJob represents a transcoding job in the system.
type TranscodeJob struct {
	ID             int64
	MediaID        int64
	Quality        string
	Codec          string // Video codec: h264, h265, vp9, av1 (defaults to h264)
	Type           string // Job type: remux, remux_audio, or transcode
	Status         string
	Progress       int    // 0-100
	Error          string // Error message if failed
	StartedAt      time.Time
	CompletedAt    time.Time
	CreatedAt      time.Time
	FilePath       string    // Path to transcode output directory
	FileSizeBytes  int64     // Total size of transcode output
	LastAccessedAt time.Time // Last time this transcode was served
	AccessCount    int       // Number of times this transcode was accessed
	StartPosition  int       // Start position in seconds (for seek-based transcoding, 0 = from beginning)
}

// NewTranscodeJob creates a new transcode job for a media item.
func NewTranscodeJob(mediaID int64, quality string, jobType string) (*TranscodeJob, error) {
	job := &TranscodeJob{
		MediaID:   mediaID,
		Quality:   quality,
		Type:      jobType,
		Status:    StatusQueued,
		Progress:  0,
		CreatedAt: time.Now(),
	}

	if err := job.IsValid(); err != nil {
		return nil, err
	}

	return job, nil
}

// IsValid validates the transcode job.
func (j *TranscodeJob) IsValid() error {
	if j.MediaID <= 0 {
		return ErrInvalidMediaID
	}

	if !isValidQuality(j.Quality) {
		return ErrInvalidQuality
	}

	if !isValidType(j.Type) {
		return ErrInvalidType
	}

	if !isValidStatus(j.Status) {
		return ErrInvalidStatus
	}

	if j.Progress < 0 || j.Progress > 100 {
		return ErrInvalidProgress
	}

	return nil
}

// MarkAsProcessing marks the job as currently processing.
func (j *TranscodeJob) MarkAsProcessing() {
	j.Status = StatusProcessing
	if j.StartedAt.IsZero() {
		j.StartedAt = time.Now()
	}
}

// MarkAsCompleted marks the job as successfully completed.
func (j *TranscodeJob) MarkAsCompleted() {
	j.Status = StatusCompleted
	j.Progress = 100
	j.CompletedAt = time.Now()
}

// MarkAsFailed marks the job as failed with an error message.
func (j *TranscodeJob) MarkAsFailed(errMsg string) {
	j.Status = StatusFailed
	j.Error = errMsg
	j.CompletedAt = time.Now()
}

// UpdateProgress updates the job's progress percentage.
func (j *TranscodeJob) UpdateProgress(progress int) error {
	if progress < 0 || progress > 100 {
		return ErrInvalidProgress
	}
	j.Progress = progress
	return nil
}

// IsQueued returns true if the job is queued.
func (j *TranscodeJob) IsQueued() bool {
	return j.Status == StatusQueued
}

// IsProcessing returns true if the job is currently processing.
func (j *TranscodeJob) IsProcessing() bool {
	return j.Status == StatusProcessing
}

// IsCompleted returns true if the job completed successfully.
func (j *TranscodeJob) IsCompleted() bool {
	return j.Status == StatusCompleted
}

// IsFailed returns true if the job failed.
func (j *TranscodeJob) IsFailed() bool {
	return j.Status == StatusFailed
}

// IsFinished returns true if the job is completed or failed.
func (j *TranscodeJob) IsFinished() bool {
	return j.IsCompleted() || j.IsFailed()
}

// isValidQuality checks if the quality level is valid.
func isValidQuality(quality string) bool {
	switch quality {
	case Quality360p, Quality720p, Quality1080p, Quality4K:
		return true
	default:
		return false
	}
}

// isValidType checks if the job type is valid.
func isValidType(jobType string) bool {
	switch jobType {
	case TypeRemux, TypeRemuxAudio, TypeTranscode:
		return true
	default:
		return false
	}
}

// isValidStatus checks if the status is valid.
func isValidStatus(status string) bool {
	switch status {
	case StatusQueued, StatusProcessing, StatusCompleted, StatusFailed:
		return true
	default:
		return false
	}
}

// GetAllQualities returns all available quality levels.
func GetAllQualities() []string {
	return []string{Quality360p, Quality720p, Quality1080p, Quality4K}
}

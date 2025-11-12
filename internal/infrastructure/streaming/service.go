package streaming

import (
	"fmt"
	"io"
	"os"
)

// ByteRange represents a requested byte range
type ByteRange struct {
	Start int64
	End   int64
}

// StreamRequest contains parameters for streaming a file
type StreamRequest struct {
	FilePath    string
	RangeHeader string // Optional: e.g., "bytes=0-1023"
}

// StreamResponse contains the file and metadata for streaming
type StreamResponse struct {
	File        *os.File
	FileSize    int64
	FileName    string
	ContentType string
	Range       *ByteRange // nil if no range requested
	IsPartial   bool       // true if range request
}

// Service handles media file streaming operations
type Service struct{}

// NewService creates a new streaming service
func NewService() *Service {
	return &Service{}
}

// PrepareStream opens a file and prepares it for streaming
func (s *Service) PrepareStream(req StreamRequest) (*StreamResponse, error) {
	// Open file
	file, err := os.Open(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	fileSize := stat.Size()
	fileName := stat.Name()
	contentType := DetectContentType(fileName)

	resp := &StreamResponse{
		File:        file,
		FileSize:    fileSize,
		FileName:    fileName,
		ContentType: contentType,
		IsPartial:   false,
	}

	// Parse range header if present
	if req.RangeHeader != "" {
		ranges, err := ParseRangeHeader(req.RangeHeader, fileSize)
		if err != nil {
			file.Close()
			return nil, err
		}

		// Only support single range for now
		if len(ranges) != 1 {
			file.Close()
			return nil, fmt.Errorf("multiple ranges not supported")
		}

		resp.Range = &ranges[0]
		resp.IsPartial = true

		// Seek to start position
		if _, err := file.Seek(ranges[0].Start, io.SeekStart); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to seek file: %w", err)
		}
	}

	return resp, nil
}

// Close closes the stream response file
func (r *StreamResponse) Close() error {
	if r.File != nil {
		return r.File.Close()
	}
	return nil
}

// ContentLength returns the content length for the response
func (r *StreamResponse) ContentLength() int64 {
	if r.Range != nil {
		return r.Range.End - r.Range.Start + 1
	}
	return r.FileSize
}

// Reader returns an io.Reader for the content
func (r *StreamResponse) Reader() io.Reader {
	if r.Range != nil {
		return io.LimitReader(r.File, r.ContentLength())
	}
	return r.File
}

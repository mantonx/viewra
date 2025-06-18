package main

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/mantonx/viewra/pkg/plugins"
)

// LazyTranscodingService is a wrapper that implements plugins.TranscodingService
// but defers actual work to the real service once the plugin is initialized
type LazyTranscodingService struct {
	mu      sync.RWMutex
	plugin  *FFmpegTranscoderPlugin
	realSvc plugins.TranscodingService
}

// NewLazyTranscodingService creates a new lazy transcoding service
func NewLazyTranscodingService(plugin *FFmpegTranscoderPlugin) *LazyTranscodingService {
	return &LazyTranscodingService{
		plugin: plugin,
	}
}

// NotifyReady should be called when the real service is available
func (l *LazyTranscodingService) NotifyReady() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.realSvc = l.plugin.transcodingService
}

// ensureInitialized ensures the real service is available
func (l *LazyTranscodingService) ensureInitialized() (plugins.TranscodingService, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if l.realSvc != nil {
		return l.realSvc, nil
	}

	return nil, fmt.Errorf("transcoding service not yet initialized")
}

// GetCapabilities returns transcoding capabilities
func (l *LazyTranscodingService) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	fmt.Printf("DEBUG: LazyTranscodingService.GetCapabilities called\n")
	svc, err := l.ensureInitialized()
	if err != nil {
		fmt.Printf("DEBUG: ensureInitialized failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("DEBUG: Calling real service GetCapabilities\n")
	result, err := svc.GetCapabilities(ctx)
	if err != nil {
		fmt.Printf("DEBUG: Real service GetCapabilities failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("DEBUG: Real service GetCapabilities succeeded\n")
	return result, nil
}

// StartTranscode initiates a transcoding session
func (l *LazyTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	fmt.Printf("DEBUG: LazyTranscodingService.StartTranscode called\n")
	svc, err := l.ensureInitialized()
	if err != nil {
		fmt.Printf("DEBUG: ensureInitialized failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("DEBUG: Calling real service StartTranscode\n")
	result, err := svc.StartTranscode(ctx, req)
	if err != nil {
		fmt.Printf("DEBUG: Real service StartTranscode failed: %v\n", err)
		return nil, err
	}
	fmt.Printf("DEBUG: Real service StartTranscode succeeded\n")
	return result, nil
}

// GetTranscodeSession retrieves information about an active session
func (l *LazyTranscodingService) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	svc, err := l.ensureInitialized()
	if err != nil {
		return nil, err
	}
	return svc.GetTranscodeSession(ctx, sessionID)
}

// StopTranscode terminates a transcoding session
func (l *LazyTranscodingService) StopTranscode(ctx context.Context, sessionID string) error {
	svc, err := l.ensureInitialized()
	if err != nil {
		return err
	}
	return svc.StopTranscode(ctx, sessionID)
}

// ListActiveSessions returns all currently active transcoding sessions
func (l *LazyTranscodingService) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	svc, err := l.ensureInitialized()
	if err != nil {
		return nil, err
	}
	return svc.ListActiveSessions(ctx)
}

// GetTranscodeStream returns the output stream for a transcoding session
func (l *LazyTranscodingService) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	svc, err := l.ensureInitialized()
	if err != nil {
		return nil, err
	}
	return svc.GetTranscodeStream(ctx, sessionID)
}

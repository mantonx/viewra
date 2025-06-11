package transcode

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/modules/playbackmodule"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
)

// Register transcode core plugin with the plugin registry
func init() {
	pluginmodule.RegisterCorePluginFactory("transcode", func() pluginmodule.CorePlugin {
		return NewTranscodeCorePlugin()
	})
}

// TranscodeCorePlugin implements both CorePlugin and TranscodeManager interfaces
type TranscodeCorePlugin struct {
	name        string
	enabled     bool
	initialized bool

	// TranscodeManager implementation
	transcoders map[string]playbackmodule.Transcoder
	sessions    map[string]*playbackmodule.TranscodeSession
	mutex       sync.RWMutex

	// Playback planner
	planner playbackmodule.PlaybackPlanner
}

// NewTranscodeCorePlugin creates a new transcode core plugin instance
func NewTranscodeCorePlugin() pluginmodule.CorePlugin {
	return &TranscodeCorePlugin{
		name:        "transcode_core_plugin",
		enabled:     true,
		transcoders: make(map[string]playbackmodule.Transcoder),
		sessions:    make(map[string]*playbackmodule.TranscodeSession),
		planner:     playbackmodule.NewPlaybackPlanner(),
	}
}

// CorePlugin interface implementation

func (p *TranscodeCorePlugin) GetName() string {
	return p.name
}

func (p *TranscodeCorePlugin) GetDisplayName() string {
	return "Transcode Core Plugin"
}

func (p *TranscodeCorePlugin) GetPluginType() string {
	return "transcode"
}

func (p *TranscodeCorePlugin) GetType() string {
	return "transcode"
}

func (p *TranscodeCorePlugin) GetSupportedExtensions() []string {
	// This plugin works with any video file that needs transcoding
	return []string{".mkv", ".avi", ".mov", ".wmv", ".flv", ".m4v", ".3gp", ".ogv", ".mpg", ".mpeg", ".ts", ".mts", ".m2ts"}
}

func (p *TranscodeCorePlugin) IsEnabled() bool {
	return p.enabled
}

func (p *TranscodeCorePlugin) Enable() error {
	p.enabled = true
	return nil
}

func (p *TranscodeCorePlugin) Disable() error {
	p.enabled = false
	return nil
}

func (p *TranscodeCorePlugin) Initialize() error {
	if p.initialized {
		return nil
	}
	
	// Initialize any builtin transcoders here
	// For now, we'll register them when external plugins connect
	
	p.initialized = true
	return nil
}

func (p *TranscodeCorePlugin) Shutdown() error {
	// Stop all active sessions
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for _, session := range p.sessions {
		if session.Stream != nil {
			session.Stream.Close()
		}
	}
	
	p.initialized = false
	return nil
}

func (p *TranscodeCorePlugin) Match(path string, info fs.FileInfo) bool {
	// This plugin doesn't directly handle files - it handles transcode requests
	return false
}

func (p *TranscodeCorePlugin) HandleFile(path string, ctx *pluginmodule.MetadataContext) error {
	// This plugin doesn't directly handle files
	return nil
}

// TranscodeManager interface implementation

func (p *TranscodeCorePlugin) RegisterTranscoder(transcoder playbackmodule.Transcoder) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.transcoders[transcoder.Name()] = transcoder
}

func (p *TranscodeCorePlugin) StartTranscode(ctx context.Context, req playbackmodule.TranscodeRequest) (*playbackmodule.TranscodeSession, error) {
	// Select best transcoder for this request
	transcoder, err := p.selectBestTranscoder(req)
	if err != nil {
		return nil, fmt.Errorf("no suitable transcoder found: %w", err)
	}

	// Generate session ID
	sessionID := uuid.New().String()

	// Create session
	session := &playbackmodule.TranscodeSession{
		ID:        sessionID,
		Request:   req,
		Status:    playbackmodule.StatusPending,
		StartTime: time.Now(),
		Backend:   transcoder.Name(),
	}

	// Store session
	p.mutex.Lock()
	p.sessions[sessionID] = session
	p.mutex.Unlock()

	// Start transcoding in background
	go p.runTranscoding(ctx, session, transcoder)

	return session, nil
}

func (p *TranscodeCorePlugin) StopTranscode(sessionID string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	session, exists := p.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	// Close stream if it exists
	if session.Stream != nil {
		session.Stream.Close()
	}

	// Update status
	session.Status = playbackmodule.StatusFailed
	return nil
}

func (p *TranscodeCorePlugin) GetSession(sessionID string) (*playbackmodule.TranscodeSession, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	session, exists := p.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

func (p *TranscodeCorePlugin) ListActiveSessions() []*playbackmodule.TranscodeSession {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var active []*playbackmodule.TranscodeSession
	for _, session := range p.sessions {
		if session.Status == playbackmodule.StatusRunning || session.Status == playbackmodule.StatusPending {
			active = append(active, session)
		}
	}

	return active
}

// Playback planning methods

func (p *TranscodeCorePlugin) DecidePlayback(ctx context.Context, mediaPath string, profile playbackmodule.DeviceProfile) (*playbackmodule.PlaybackDecision, error) {
	return p.planner.DecidePlayback(ctx, mediaPath, profile)
}

// Helper methods

func (p *TranscodeCorePlugin) selectBestTranscoder(req playbackmodule.TranscodeRequest) (playbackmodule.Transcoder, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.transcoders) == 0 {
		return nil, fmt.Errorf("no transcoders registered")
	}

	// Convert target codec string to Codec enum
	var targetCodec playbackmodule.Codec
	switch req.TargetCodec {
	case "h264":
		targetCodec = playbackmodule.CodecH264
	case "hevc":
		targetCodec = playbackmodule.CodecHEVC
	case "vp8":
		targetCodec = playbackmodule.CodecVP8
	case "vp9":
		targetCodec = playbackmodule.CodecVP9
	case "av1":
		targetCodec = playbackmodule.CodecAV1
	default:
		targetCodec = playbackmodule.CodecH264 // Default fallback
	}

	// Convert resolution string to Resolution enum
	var targetRes playbackmodule.Resolution
	switch req.Resolution {
	case "480p":
		targetRes = playbackmodule.Res480p
	case "720p":
		targetRes = playbackmodule.Res720p
	case "1080p":
		targetRes = playbackmodule.Res1080p
	case "1440p":
		targetRes = playbackmodule.Res1440p
	case "2160p":
		targetRes = playbackmodule.Res2160p
	default:
		targetRes = playbackmodule.Res1080p // Default
	}

	// Find transcoders that support the required codec and resolution
	var candidates []playbackmodule.Transcoder
	for _, transcoder := range p.transcoders {
		if transcoder.Supports(targetCodec, targetRes) {
			candidates = append(candidates, transcoder)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no transcoder supports codec %s at resolution %s", targetCodec, targetRes)
	}

	// Select highest priority transcoder
	bestTranscoder := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Priority() > bestTranscoder.Priority() {
			bestTranscoder = candidate
		}
	}

	return bestTranscoder, nil
}

func (p *TranscodeCorePlugin) runTranscoding(ctx context.Context, session *playbackmodule.TranscodeSession, transcoder playbackmodule.Transcoder) {
	// Update status to running
	p.mutex.Lock()
	session.Status = playbackmodule.StatusRunning
	p.mutex.Unlock()

	// Start transcoding
	stream, err := transcoder.Transcode(ctx, session.Request)
	if err != nil {
		p.mutex.Lock()
		session.Status = playbackmodule.StatusFailed
		p.mutex.Unlock()
		return
	}

	// Store stream in session
	p.mutex.Lock()
	session.Stream = stream
	session.Status = playbackmodule.StatusRunning
	p.mutex.Unlock()

	// Monitor stream completion
	go p.monitorStream(session, stream)
}

func (p *TranscodeCorePlugin) monitorStream(session *playbackmodule.TranscodeSession, stream io.ReadCloser) {
	// Create a small buffer to detect stream closure
	buffer := make([]byte, 1)
	
	for {
		// Try to read a byte to detect if stream is closed
		_, err := stream.Read(buffer)
		if err != nil {
			// Stream ended or errored
			p.mutex.Lock()
			if session.Status == playbackmodule.StatusRunning {
				session.Status = playbackmodule.StatusComplete
			}
			p.mutex.Unlock()
			break
		}
		
		// Sleep before next check
		time.Sleep(5 * time.Second)
	}
}

// Public helper methods for external access

// GetTranscodeManager returns this plugin as a TranscodeManager interface
// This allows other components to use it without knowing the implementation
func (p *TranscodeCorePlugin) GetTranscodeManager() playbackmodule.TranscodeManager {
	return p
}

// GetPlaybackPlanner returns the embedded planner interface
func (p *TranscodeCorePlugin) GetPlaybackPlanner() playbackmodule.PlaybackPlanner {
	return p.planner
}

// CleanupSessions removes completed or failed sessions older than specified duration
func (p *TranscodeCorePlugin) CleanupSessions(maxAge time.Duration) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	
	for sessionID, session := range p.sessions {
		if (session.Status == playbackmodule.StatusComplete || session.Status == playbackmodule.StatusFailed) &&
			session.StartTime.Before(cutoff) {
			
			// Close stream if still open
			if session.Stream != nil {
				session.Stream.Close()
			}
			
			delete(p.sessions, sessionID)
		}
	}
}

// GetStats returns statistics about the transcode manager
func (p *TranscodeCorePlugin) GetStats() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_sessions":   len(p.sessions),
		"active_sessions":  0,
		"pending_sessions": 0,
		"failed_sessions":  0,
		"transcoders":      len(p.transcoders),
	}

	for _, session := range p.sessions {
		switch session.Status {
		case playbackmodule.StatusRunning:
			stats["active_sessions"] = stats["active_sessions"].(int) + 1
		case playbackmodule.StatusPending:
			stats["pending_sessions"] = stats["pending_sessions"].(int) + 1
		case playbackmodule.StatusFailed:
			stats["failed_sessions"] = stats["failed_sessions"].(int) + 1
		}
	}

	return stats
} 
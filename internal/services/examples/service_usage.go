// Package examples demonstrates how to use the service registry pattern
// in Viewra modules. This file shows real-world usage patterns.
package examples

import (
	"context"
	"fmt"
	"log"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// ExamplePlaybackToTranscoding demonstrates how the MediaModule might
// use both PlaybackService and TranscodingService together
func ExamplePlaybackToTranscoding() {
	ctx := context.Background()

	// Get the playback service to analyze media
	playbackService, err := services.GetService[services.PlaybackService]("playback")
	if err != nil {
		log.Printf("Failed to get playback service: %v", err)
		return
	}

	// Get the transcoding service for actual transcoding
	transcodingService, err := services.GetService[services.TranscodingService]("transcoding")
	if err != nil {
		log.Printf("Failed to get transcoding service: %v", err)
		return
	}

	// Example media file and device profile
	mediaPath := "/media/movies/example.mkv"
	deviceProfile := &types.DeviceProfile{
		UserAgent:       "Mozilla/5.0 (Smart TV)",
		SupportedCodecs: []string{"h264", "aac"},
		MaxResolution:   "1080p",
		MaxBitrate:      8000000,
		SupportsHEVC:    false,
		SupportsAV1:     false,
	}

	// Step 1: Decide if transcoding is needed
	decision, err := playbackService.DecidePlayback(mediaPath, deviceProfile)
	if err != nil {
		log.Printf("Failed to decide playback: %v", err)
		return
	}

	// Step 2: If transcoding is needed, start it
	if decision.ShouldTranscode {
		log.Printf("Transcoding required: %s", decision.Reason)

		// Get recommended transcode parameters
		transcodeReq, err := playbackService.GetRecommendedTranscodeParams(mediaPath, deviceProfile)
		if err != nil {
			log.Printf("Failed to get transcode params: %v", err)
			return
		}

		// Start the transcoding session
		session, err := transcodingService.StartTranscode(ctx, transcodeReq)
		if err != nil {
			log.Printf("Failed to start transcode: %v", err)
			return
		}

		log.Printf("Transcoding started with session ID: %s", session.SessionID)

		// Monitor progress (in a real implementation, this would be async)
		progress, err := transcodingService.GetProgress(session.SessionID)
		if err != nil {
			log.Printf("Failed to get progress: %v", err)
			return
		}

		log.Printf("Transcoding progress: %.2f%%", progress.Progress)
	} else {
		// Direct play is possible
		log.Printf("Direct play available at: %s", decision.DirectPlayURL)
	}
}

// ExampleTranscodingStats demonstrates how to get transcoding statistics
func ExampleTranscodingStats() {
	// Get the transcoding service
	transcodingService, err := services.GetService[services.TranscodingService]("transcoding")
	if err != nil {
		log.Printf("Failed to get transcoding service: %v", err)
		return
	}

	// Get current statistics
	stats, err := transcodingService.GetStats()
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
		return
	}

	fmt.Printf("Transcoding Statistics:\n")
	fmt.Printf("  Active Sessions: %d\n", stats.ActiveSessions)
	fmt.Printf("  Total Sessions: %d\n", stats.TotalSessions)
	fmt.Printf("  Completed: %d\n", stats.CompletedSessions)
	fmt.Printf("  Failed: %d\n", stats.FailedSessions)
	fmt.Printf("  Average Speed: %.2fx\n", stats.AverageSpeed)

	// Show provider information
	fmt.Printf("\nAvailable Providers:\n")
	providers := transcodingService.GetProviders()
	for _, provider := range providers {
		fmt.Printf("  - %s (v%s): %s\n", provider.Name, provider.Version, provider.Description)
	}
}

// ExampleServiceInitialization shows how a module initializes and registers its service
func ExampleServiceInitialization() {
	// This would be in your module's Init() method

	// Create your service implementation
	// manager := NewManager(...) // Your module's manager
	// service := NewMyServiceImpl(manager)

	// Register the service
	// services.RegisterService("myservice", service)

	// The service is now available to other modules
}

// ExampleMustGetService demonstrates the MustGetService pattern for critical dependencies
func ExampleMustGetService() {
	// This pattern is useful during initialization when a service MUST exist
	// It will panic if the service is not found, which is appropriate for
	// initialization failures

	// Only use this pattern during initialization or when failure is unrecoverable
	transcodingService := services.MustGetService[services.TranscodingService]("transcoding")

	// If we get here, the service exists and is the correct type
	providers := transcodingService.GetProviders()
	log.Printf("Found %d transcoding providers", len(providers))
}

// ExampleErrorHandling demonstrates proper error handling
func ExampleErrorHandling() {
	// Always handle errors when using GetService in normal operation
	service, err := services.GetService[services.PlaybackService]("playback")
	if err != nil {
		// Service might not be initialized yet, or might not exist
		// Handle gracefully based on your use case
		log.Printf("Playback service not available: %v", err)

		// You might:
		// 1. Return an error to the caller
		// 2. Use a fallback implementation
		// 3. Queue the operation for later
		// 4. Return a "service unavailable" response
		return
	}

	// Use the service
	_ = service
}

// ExampleMockingForTests shows how to use the service registry in tests
func ExampleMockingForTests() {
	// In your test setup, you can register mock implementations

	// Create a mock service that implements the interface
	mockService := &MockTranscodingService{
		sessions: make(map[string]*database.TranscodeSession),
	}

	// Register the mock
	services.RegisterService("transcoding", mockService)

	// Now code that uses GetService will get your mock
	// This makes testing module interactions much easier
}

// MockTranscodingService is an example mock implementation for testing
type MockTranscodingService struct {
	sessions map[string]*database.TranscodeSession
}

func (m *MockTranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	// Mock implementation
	session := &database.TranscodeSession{
		SessionID: "mock-session-123",
		Status:    "processing",
	}
	m.sessions[session.SessionID] = session
	return session, nil
}

func (m *MockTranscodingService) GetSession(sessionID string) (*database.TranscodeSession, error) {
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return session, nil
}

func (m *MockTranscodingService) StopSession(sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockTranscodingService) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	return &plugins.TranscodingProgress{
		Progress: 50.0,
		Speed:    1.5,
	}, nil
}

func (m *MockTranscodingService) GetStats() (*types.TranscodingStats, error) {
	return &types.TranscodingStats{
		ActiveSessions: len(m.sessions),
	}, nil
}

func (m *MockTranscodingService) GetProviders() []plugins.ProviderInfo {
	return []plugins.ProviderInfo{
		{
			Name:        "mock-provider",
			Version:     "1.0.0",
			Description: "Mock provider for testing",
		},
	}
}

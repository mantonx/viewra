package grpc

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

// mockEnricherClient implements pluginv1.EnricherClient for testing
type mockEnricherClient struct {
	capabilities *pluginv1.EnricherCapabilities
	capsErr      error
	enrichResp   *pluginv1.EnrichResponse
	enrichErr    error
}

func (m *mockEnricherClient) GetCapabilities(ctx context.Context, in *pluginv1.Empty, opts ...grpc.CallOption) (*pluginv1.EnricherCapabilities, error) {
	if m.capsErr != nil {
		return nil, m.capsErr
	}
	return m.capabilities, nil
}

func (m *mockEnricherClient) Enrich(ctx context.Context, in *pluginv1.EnrichRequest, opts ...grpc.CallOption) (*pluginv1.EnrichResponse, error) {
	if m.enrichErr != nil {
		return nil, m.enrichErr
	}
	return m.enrichResp, nil
}

func TestNewEnricher_Success(t *testing.T) {
	logger := slog.Default()
	client := &mockEnricherClient{
		capabilities: &pluginv1.EnricherCapabilities{
			MediaTypes: []string{"movie", "tv"},
			Priority:   100,
		},
	}

	enricher, err := NewEnricher("test-plugin", client, logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if enricher.Stage() != "test-plugin" {
		t.Errorf("expected stage 'test-plugin', got %s", enricher.Stage())
	}

	caps := enricher.Capabilities()
	if !caps.SupportsMediaType("movie") {
		t.Error("expected capabilities to support 'movie'")
	}
	if !caps.SupportsMediaType("tv") {
		t.Error("expected capabilities to support 'tv'")
	}
	if caps.SupportsMediaType("music") {
		t.Error("expected capabilities to not support 'music'")
	}
}

func TestNewEnricher_GetCapabilitiesError(t *testing.T) {
	logger := slog.Default()
	client := &mockEnricherClient{
		capsErr: errors.New("connection failed"),
	}

	_, err := NewEnricher("test-plugin", client, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection failed" {
		t.Errorf("expected error 'connection failed', got %v", err)
	}
}

func TestEnricher_Enrich(t *testing.T) {
	logger := slog.Default()
	title := "Test Movie"
	client := &mockEnricherClient{
		capabilities: &pluginv1.EnricherCapabilities{
			MediaTypes: []string{"movie"},
		},
		enrichResp: &pluginv1.EnrichResponse{
			Matched: true,
			Metadata: &pluginv1.EnrichedMetadata{
				Title: &title,
			},
		},
	}

	enricher, err := NewEnricher("test-plugin", client, logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &pluginv1.EnrichRequest{
		MediaType: "movie",
		MediaId:   123,
	}

	resp, err := enricher.Enrich(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.Matched {
		t.Error("expected matched to be true")
	}
	if resp.Metadata == nil || resp.Metadata.Title == nil {
		t.Fatal("expected metadata with title")
	}
	if *resp.Metadata.Title != "Test Movie" {
		t.Errorf("expected title 'Test Movie', got %s", *resp.Metadata.Title)
	}
}

func TestEnricher_EnrichError(t *testing.T) {
	logger := slog.Default()
	client := &mockEnricherClient{
		capabilities: &pluginv1.EnricherCapabilities{},
		enrichErr:    errors.New("enrichment failed"),
	}

	enricher, err := NewEnricher("test-plugin", client, logger)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &pluginv1.EnrichRequest{
		MediaType: "movie",
		MediaId:   123,
	}

	_, err = enricher.Enrich(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestEnricher_Stage(t *testing.T) {
	logger := slog.Default()
	client := &mockEnricherClient{
		capabilities: &pluginv1.EnricherCapabilities{},
	}

	enricher, _ := NewEnricher("my-enricher-id", client, logger)

	if enricher.Stage() != "my-enricher-id" {
		t.Errorf("expected stage 'my-enricher-id', got %s", enricher.Stage())
	}
}

package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

func TestNewWorkerPool(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	enricher := newMockEnricher("test")
	config := StageWorkerConfig{
		Stage:       "test",
		Concurrency: 2,
		RateLimit:   0,
		Timeout:     30,
		BatchSize:   5,
	}

	pipelineCache := NewPipelineCache(deps.PipelineRepo, 5*time.Minute)
	entityCache := NewEntityCache(1000)
	pool := NewWorkerPool(deps, enricher, nil, pipelineCache, entityCache, nil, config)
	if pool == nil {
		t.Fatal("NewWorkerPool returned nil")
	}
	if pool.limiter != nil {
		t.Error("limiter should be nil when RateLimit=0")
	}
}

func TestNewWorkerPool_WithRateLimit(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	enricher := newMockEnricher("test")
	config := StageWorkerConfig{
		Stage:       "test",
		Concurrency: 2,
		RateLimit:   5,
		Timeout:     30,
		BatchSize:   5,
	}

	pipelineCache := NewPipelineCache(deps.PipelineRepo, 5*time.Minute)
	entityCache := NewEntityCache(1000)
	pool := NewWorkerPool(deps, enricher, nil, pipelineCache, entityCache, nil, config)
	if pool == nil {
		t.Fatal("NewWorkerPool returned nil")
	}
	if pool.limiter == nil {
		t.Error("limiter should not be nil when RateLimit > 0")
	}
}

func TestWorkerPool_Run_ContextCancellation(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	enricher := newMockEnricher("test")
	config := DefaultLocalStageConfig("test")
	pipelineCache := NewPipelineCache(deps.PipelineRepo, 5*time.Minute)
	entityCache := NewEntityCache(1000)
	pool := NewWorkerPool(deps, enricher, nil, pipelineCache, entityCache, nil, config)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		pool.Run(ctx)
		close(done)
	}()

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for workers to stop
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("workers did not stop after context cancellation")
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected enrichment.ErrorCategory
	}{
		{"nil error", nil, ""},
		{"timeout", errors.New("connection timeout"), enrichment.ErrorCategoryNetwork},
		{"connection refused", errors.New("connection refused"), enrichment.ErrorCategoryNetwork},
		{"no such host", errors.New("no such host"), enrichment.ErrorCategoryNetwork},
		{"network error", errors.New("network unreachable"), enrichment.ErrorCategoryNetwork},
		{"rate limit", errors.New("rate limit exceeded"), enrichment.ErrorCategoryRateLimit},
		{"429", errors.New("HTTP 429"), enrichment.ErrorCategoryRateLimit},
		{"too many requests", errors.New("too many requests"), enrichment.ErrorCategoryRateLimit},
		{"not found", errors.New("item not found"), enrichment.ErrorCategoryNotFound},
		{"404", errors.New("HTTP 404"), enrichment.ErrorCategoryNotFound},
		{"other error", errors.New("some plugin error"), enrichment.ErrorCategoryPlugin},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeError(tt.err)
			if got != tt.expected {
				t.Errorf("categorizeError() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substrings []string
		expected   bool
	}{
		{"match first", "hello world", []string{"hello", "foo"}, true},
		{"match second", "hello world", []string{"foo", "world"}, true},
		{"no match", "hello world", []string{"foo", "bar"}, false},
		{"empty string", "", []string{"foo"}, false},
		{"empty substrings", "hello", []string{}, false},
		{"empty substring in list", "hello", []string{""}, false},
		{"exact match", "hello", []string{"hello"}, true},
		{"partial match", "hello world", []string{"lo wo"}, true},
		{"case sensitive", "Hello World", []string{"hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(tt.s, tt.substrings...)
			if got != tt.expected {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrings, got, tt.expected)
			}
		})
	}
}

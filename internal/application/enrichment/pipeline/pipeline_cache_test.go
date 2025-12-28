package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/mantonx/viewra/internal/domain/enrichment"
)

func TestNewPipelineCache(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)

	if cache == nil {
		t.Fatal("NewPipelineCache returned nil")
	}
	if cache.repo != repo {
		t.Error("repo not set correctly")
	}
	if cache.ttl != 5*time.Minute {
		t.Errorf("ttl = %v, want 5m", cache.ttl)
	}
}

func TestPipelineCache_GetStageByName(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	// First call should populate cache
	stage, err := cache.GetStageByName(ctx, enrichment.MediaTypeMovie, "nfo")
	if err != nil {
		t.Fatalf("GetStageByName failed: %v", err)
	}
	if stage == nil {
		t.Fatal("expected non-nil stage")
	}
	if stage.StageName != "nfo" {
		t.Errorf("stage.StageName = %s, want nfo", stage.StageName)
	}

	// Verify cache is now valid
	cache.mu.RLock()
	valid := cache.isValid()
	cache.mu.RUnlock()
	if !valid {
		t.Error("cache should be valid after first access")
	}

	// Second call should use cache
	stage2, err := cache.GetStageByName(ctx, enrichment.MediaTypeMovie, "nfo")
	if err != nil {
		t.Fatalf("second GetStageByName failed: %v", err)
	}
	if stage2 != stage {
		t.Error("second call should return same cached pointer")
	}
}

func TestPipelineCache_GetStageByName_NotFound(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	stage, err := cache.GetStageByName(ctx, enrichment.MediaTypeMovie, "nonexistent")
	if err != nil {
		t.Fatalf("GetStageByName failed: %v", err)
	}
	if stage != nil {
		t.Error("expected nil for nonexistent stage")
	}
}

func TestPipelineCache_GetNextStage(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	// Position 0 (nfo) should return position 1 (local_images)
	stage, err := cache.GetNextStage(ctx, enrichment.MediaTypeMovie, 0)
	if err != nil {
		t.Fatalf("GetNextStage failed: %v", err)
	}
	if stage == nil {
		t.Fatal("expected non-nil stage")
	}
	if stage.StageName != "local_images" {
		t.Errorf("stage.StageName = %s, want local_images", stage.StageName)
	}
}

func TestPipelineCache_GetNextStage_NoMore(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	// Position 1 (local_images) is last for movies
	stage, err := cache.GetNextStage(ctx, enrichment.MediaTypeMovie, 1)
	if err != nil {
		t.Fatalf("GetNextStage failed: %v", err)
	}
	if stage != nil {
		t.Error("expected nil when no more stages")
	}
}

func TestPipelineCache_GetFirstStage(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	stage, err := cache.GetFirstStage(ctx, enrichment.MediaTypeMovie)
	if err != nil {
		t.Fatalf("GetFirstStage failed: %v", err)
	}
	if stage == nil {
		t.Fatal("expected non-nil stage")
	}
	if stage.StageName != "nfo" {
		t.Errorf("stage.StageName = %s, want nfo", stage.StageName)
	}
	if stage.Position != 0 {
		t.Errorf("stage.Position = %d, want 0", stage.Position)
	}
}

func TestPipelineCache_GetFirstStage_NoPipeline(t *testing.T) {
	repo := &mockPipelineRepo{stages: nil}
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	stage, err := cache.GetFirstStage(ctx, enrichment.MediaTypeMusic)
	if err != nil {
		t.Fatalf("GetFirstStage failed: %v", err)
	}
	if stage != nil {
		t.Error("expected nil for media type with no pipeline")
	}
}

func TestPipelineCache_Invalidate(t *testing.T) {
	repo := newMockPipelineRepo()
	cache := NewPipelineCache(repo, 5*time.Minute)
	ctx := context.Background()

	// Populate cache
	_, _ = cache.GetStageByName(ctx, enrichment.MediaTypeMovie, "nfo")

	cache.mu.RLock()
	valid := cache.isValid()
	cache.mu.RUnlock()
	if !valid {
		t.Fatal("cache should be valid")
	}

	// Invalidate
	cache.Invalidate()

	cache.mu.RLock()
	valid = cache.isValid()
	cache.mu.RUnlock()
	if valid {
		t.Error("cache should be invalid after Invalidate()")
	}
}

func TestPipelineCache_TTLExpiry(t *testing.T) {
	repo := newMockPipelineRepo()
	// Use very short TTL for testing
	cache := NewPipelineCache(repo, 10*time.Millisecond)
	ctx := context.Background()

	// Populate cache
	_, _ = cache.GetStageByName(ctx, enrichment.MediaTypeMovie, "nfo")

	cache.mu.RLock()
	valid := cache.isValid()
	cache.mu.RUnlock()
	if !valid {
		t.Fatal("cache should be valid immediately after population")
	}

	// Wait for TTL to expire
	time.Sleep(20 * time.Millisecond)

	cache.mu.RLock()
	valid = cache.isValid()
	cache.mu.RUnlock()
	if valid {
		t.Error("cache should be invalid after TTL expiry")
	}
}

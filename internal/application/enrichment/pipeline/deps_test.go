package pipeline

import (
	"testing"
)

func TestDefaultLocalStageConfig(t *testing.T) {
	config := DefaultLocalStageConfig("nfo")

	if config.Stage != "nfo" {
		t.Errorf("Stage = %s, want nfo", config.Stage)
	}
	if config.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", config.Concurrency)
	}
	if config.RateLimit != 0 {
		t.Errorf("RateLimit = %f, want 0 (unlimited)", config.RateLimit)
	}
	if config.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", config.Timeout)
	}
	if config.BatchSize != 10 {
		t.Errorf("BatchSize = %d, want 10", config.BatchSize)
	}
}

func TestDefaultRemoteStageConfig(t *testing.T) {
	config := DefaultRemoteStageConfig("tmdb")

	if config.Stage != "tmdb" {
		t.Errorf("Stage = %s, want tmdb", config.Stage)
	}
	if config.Concurrency != 2 {
		t.Errorf("Concurrency = %d, want 2", config.Concurrency)
	}
	if config.RateLimit != 5 {
		t.Errorf("RateLimit = %f, want 5", config.RateLimit)
	}
	if config.Timeout != 30 {
		t.Errorf("Timeout = %d, want 30", config.Timeout)
	}
	if config.BatchSize != 5 {
		t.Errorf("BatchSize = %d, want 5", config.BatchSize)
	}
}

func TestStageWorkerConfig_Fields(t *testing.T) {
	config := StageWorkerConfig{
		Stage:       "custom",
		Concurrency: 8,
		RateLimit:   10.5,
		Timeout:     120,
		BatchSize:   20,
	}

	if config.Stage != "custom" {
		t.Errorf("Stage = %s, want custom", config.Stage)
	}
	if config.Concurrency != 8 {
		t.Errorf("Concurrency = %d, want 8", config.Concurrency)
	}
	if config.RateLimit != 10.5 {
		t.Errorf("RateLimit = %f, want 10.5", config.RateLimit)
	}
	if config.Timeout != 120 {
		t.Errorf("Timeout = %d, want 120", config.Timeout)
	}
	if config.BatchSize != 20 {
		t.Errorf("BatchSize = %d, want 20", config.BatchSize)
	}
}

package types

import (
	"testing"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

func TestInstance_UpdateHealth(t *testing.T) {
	instance := &Instance{
		ID: "test-plugin",
	}

	// Initial state
	if instance.Health.Status != pluginv1.HealthStatus_UNKNOWN {
		t.Errorf("expected initial status UNKNOWN, got %v", instance.Health.Status)
	}

	// Update to healthy
	instance.UpdateHealth(pluginv1.HealthStatus_HEALTHY, "all good")

	if instance.Health.Status != pluginv1.HealthStatus_HEALTHY {
		t.Errorf("expected status HEALTHY, got %v", instance.Health.Status)
	}
	if instance.Health.Message != "all good" {
		t.Errorf("expected message 'all good', got %q", instance.Health.Message)
	}
	if instance.Health.LastHeartbeat.IsZero() {
		t.Error("expected LastHeartbeat to be set")
	}

	// Update to unhealthy
	instance.UpdateHealth(pluginv1.HealthStatus_UNHEALTHY, "connection lost")

	if instance.Health.Status != pluginv1.HealthStatus_UNHEALTHY {
		t.Errorf("expected status UNHEALTHY, got %v", instance.Health.Status)
	}
}

func TestInstance_IsHealthy(t *testing.T) {
	tests := []struct {
		name     string
		status   pluginv1.HealthStatus_Status
		expected bool
	}{
		{"healthy", pluginv1.HealthStatus_HEALTHY, true},
		{"unhealthy", pluginv1.HealthStatus_UNHEALTHY, false},
		{"unknown", pluginv1.HealthStatus_UNKNOWN, false},
		{"degraded", pluginv1.HealthStatus_DEGRADED, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := &Instance{
				ID:     "test",
				Health: Health{Status: tt.status},
			}
			if got := instance.IsHealthy(); got != tt.expected {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInstance_HasCategory(t *testing.T) {
	instance := &Instance{
		ID:         "test",
		Categories: []Category{CategoryEnricher, CategoryAI},
	}

	if !instance.HasCategory(CategoryEnricher) {
		t.Error("expected HasCategory(CategoryEnricher) to be true")
	}
	if !instance.HasCategory(CategoryAI) {
		t.Error("expected HasCategory(CategoryAI) to be true")
	}
	if instance.HasCategory(CategoryProvider) {
		t.Error("expected HasCategory(CategoryProvider) to be false")
	}
}

func TestParseCategories(t *testing.T) {
	input := []string{"enricher", "provider", "ai"}
	result := ParseCategories(input)

	if len(result) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(result))
	}
	if result[0] != CategoryEnricher {
		t.Errorf("expected first category to be enricher, got %v", result[0])
	}
	if result[1] != CategoryProvider {
		t.Errorf("expected second category to be provider, got %v", result[1])
	}
	if result[2] != CategoryAI {
		t.Errorf("expected third category to be ai, got %v", result[2])
	}
}

func TestHasCategoryIn(t *testing.T) {
	categories := []Category{CategoryEnricher, CategoryAI}

	if !HasCategoryIn(categories, CategoryEnricher) {
		t.Error("expected HasCategoryIn to find enricher")
	}
	if HasCategoryIn(categories, CategoryProvider) {
		t.Error("expected HasCategoryIn to not find provider")
	}
	if HasCategoryIn(nil, CategoryEnricher) {
		t.Error("expected HasCategoryIn to return false for nil slice")
	}
}

func TestInstance_ConcurrentHealthUpdates(t *testing.T) {
	instance := &Instance{ID: "test"}

	// Run concurrent updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				if n%2 == 0 {
					instance.UpdateHealth(pluginv1.HealthStatus_HEALTHY, "healthy")
				} else {
					instance.UpdateHealth(pluginv1.HealthStatus_UNHEALTHY, "unhealthy")
				}
				_ = instance.IsHealthy()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for goroutines")
		}
	}
}

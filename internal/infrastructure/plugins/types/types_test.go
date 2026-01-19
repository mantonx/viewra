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
		Categories: []Category{CategoryEnricher, CategoryProvider},
	}

	if !instance.HasCategory(CategoryEnricher) {
		t.Error("expected HasCategory(CategoryEnricher) to be true")
	}
	if !instance.HasCategory(CategoryProvider) {
		t.Error("expected HasCategory(CategoryProvider) to be true")
	}
	if instance.HasCategory(CategoryNotificationSink) {
		t.Error("expected HasCategory(CategoryNotificationSink) to be false")
	}
}

func TestInferCategoriesFromCapabilities(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []string
		wantCats     []Category
	}{
		{
			name:         "provider capabilities",
			capabilities: []string{"provider", "embedding"},
			wantCats:     []Category{CategoryProvider},
		},
		{
			name:         "dynamic provider capability",
			capabilities: []string{"provider:ollama", "embedding"},
			wantCats:     []Category{CategoryProvider},
		},
		{
			name:         "enricher capabilities",
			capabilities: []string{"metadata", "artwork"},
			wantCats:     []Category{CategoryEnricher},
		},
		{
			name:         "trending capability",
			capabilities: []string{"trending"},
			wantCats:     []Category{CategoryTrending},
		},
		{
			name:         "notification sink",
			capabilities: []string{"notification_sink"},
			wantCats:     []Category{CategoryNotificationSink},
		},
		{
			name:         "mixed capabilities",
			capabilities: []string{"provider", "metadata", "trending"},
			wantCats:     []Category{CategoryProvider, CategoryEnricher, CategoryTrending},
		},
		{
			name:         "search capabilities - no category mapping",
			capabilities: []string{"search", "vector_search"},
			wantCats:     []Category{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferCategoriesFromCapabilities(tt.capabilities)

			// Check that all expected categories are present
			for _, wantCat := range tt.wantCats {
				found := false
				for _, gotCat := range got {
					if gotCat == wantCat {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected category %v not found in result %v", wantCat, got)
				}
			}

			// Check no extra categories
			if len(got) != len(tt.wantCats) {
				t.Errorf("expected %d categories, got %d: %v", len(tt.wantCats), len(got), got)
			}
		})
	}
}

func TestHasCategoryIn(t *testing.T) {
	categories := []Category{CategoryEnricher, CategoryProvider}

	if !HasCategoryIn(categories, CategoryEnricher) {
		t.Error("expected HasCategoryIn to find enricher")
	}
	if HasCategoryIn(categories, CategoryNotificationSink) {
		t.Error("expected HasCategoryIn to not find notification_sink")
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

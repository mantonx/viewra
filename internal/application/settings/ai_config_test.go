package settings

import (
	"context"
	"testing"

	"github.com/mantonx/viewra/internal/domain/ai"
	settingsDomain "github.com/mantonx/viewra/internal/domain/settings"
)

// mockSystemRepo is a simple in-memory mock for testing
type mockSystemRepo struct {
	settings map[string]*settingsDomain.SystemSetting
}

func newMockSystemRepo() *mockSystemRepo {
	return &mockSystemRepo{
		settings: make(map[string]*settingsDomain.SystemSetting),
	}
}

func (m *mockSystemRepo) Get(_ context.Context, key string) (*settingsDomain.SystemSetting, error) {
	if setting, ok := m.settings[key]; ok {
		return setting, nil
	}
	return nil, settingsDomain.ErrSettingNotFound
}

func (m *mockSystemRepo) Set(_ context.Context, setting *settingsDomain.SystemSetting) error {
	m.settings[setting.Key] = setting
	return nil
}

func (m *mockSystemRepo) GetAll(_ context.Context) ([]*settingsDomain.SystemSetting, error) {
	result := make([]*settingsDomain.SystemSetting, 0, len(m.settings))
	for _, s := range m.settings {
		result = append(result, s)
	}
	return result, nil
}

func (m *mockSystemRepo) GetByCategory(_ context.Context, category settingsDomain.Category) ([]*settingsDomain.SystemSetting, error) {
	result := make([]*settingsDomain.SystemSetting, 0)
	for _, s := range m.settings {
		if s.Category == category {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSystemRepo) Delete(_ context.Context, key string) error {
	delete(m.settings, key)
	return nil
}

// mockUserRepo is a simple mock for user settings (not used in AI config tests)
type mockUserRepo struct{}

func (m *mockUserRepo) Get(_ context.Context, _, _ string) (*settingsDomain.UserSetting, error) {
	return nil, settingsDomain.ErrSettingNotFound
}

func (m *mockUserRepo) Set(_ context.Context, _ *settingsDomain.UserSetting) error {
	return nil
}

func (m *mockUserRepo) GetAll(_ context.Context, _ string) ([]*settingsDomain.UserSetting, error) {
	return nil, nil
}

func (m *mockUserRepo) Delete(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockUserRepo) DeleteAll(_ context.Context, _ string) error {
	return nil
}

func TestAIConfigReader_Defaults(t *testing.T) {
	svc := NewService(newMockSystemRepo(), &mockUserRepo{}, nil)
	reader := NewAIConfigReader(svc)
	ctx := context.Background()

	// Test default values when no settings exist
	if reader.IsEnabled(ctx) {
		t.Error("Expected AI to be disabled by default")
	}

	if provider := reader.GetEmbeddingProvider(ctx); provider != ai.ProviderOllama {
		t.Errorf("Expected default embedding provider to be Ollama, got %v", provider)
	}

	if provider := reader.GetChatProvider(ctx); provider != ai.ProviderOllama {
		t.Errorf("Expected default chat provider to be Ollama, got %v", provider)
	}

	if results := reader.GetMaxResults(ctx); results != 20 {
		t.Errorf("Expected default max results 20, got %d", results)
	}

	if threshold := reader.GetSimilarityThreshold(ctx); threshold != 0.5 {
		t.Errorf("Expected default similarity threshold 0.5, got %f", threshold)
	}
}

func TestAIConfigReader_CustomValues(t *testing.T) {
	repo := newMockSystemRepo()
	svc := NewService(repo, &mockUserRepo{}, nil)
	reader := NewAIConfigReader(svc)
	ctx := context.Background()

	// Set custom values
	if err := svc.SetSystem(ctx, "ai.enabled", true, "test"); err != nil {
		t.Fatalf("Failed to set ai.enabled: %v", err)
	}
	if err := svc.SetSystem(ctx, "ai.embedding_provider", "openai", "test"); err != nil {
		t.Fatalf("Failed to set ai.embedding_provider: %v", err)
	}
	if err := svc.SetSystem(ctx, "ai.chat_provider", "anthropic", "test"); err != nil {
		t.Fatalf("Failed to set ai.chat_provider: %v", err)
	}
	if err := svc.SetSystem(ctx, "ai.max_results", 50, "test"); err != nil {
		t.Fatalf("Failed to set ai.max_results: %v", err)
	}
	if err := svc.SetSystem(ctx, "ai.similarity_threshold", "0.75", "test"); err != nil {
		t.Fatalf("Failed to set ai.similarity_threshold: %v", err)
	}

	// Verify custom values are read correctly
	if !reader.IsEnabled(ctx) {
		t.Error("Expected AI to be enabled")
	}

	if provider := reader.GetEmbeddingProvider(ctx); provider != ai.ProviderOpenAI {
		t.Errorf("Expected embedding provider OpenAI, got %v", provider)
	}

	if provider := reader.GetChatProvider(ctx); provider != ai.ProviderAnthropic {
		t.Errorf("Expected chat provider Anthropic, got %v", provider)
	}

	if results := reader.GetMaxResults(ctx); results != 50 {
		t.Errorf("Expected max results 50, got %d", results)
	}

	if threshold := reader.GetSimilarityThreshold(ctx); threshold != 0.75 {
		t.Errorf("Expected similarity threshold 0.75, got %f", threshold)
	}
}

func TestAIConfigReader_ProviderSelection(t *testing.T) {
	repo := newMockSystemRepo()
	svc := NewService(repo, &mockUserRepo{}, nil)
	reader := NewAIConfigReader(svc)
	ctx := context.Background()

	// Test various provider selections
	testCases := []struct {
		setting  string
		value    string
		expected ai.ProviderType
	}{
		{"ai.embedding_provider", "ollama", ai.ProviderOllama},
		{"ai.embedding_provider", "openai", ai.ProviderOpenAI},
		{"ai.embedding_provider", "voyage", ai.ProviderVoyage},
		{"ai.chat_provider", "ollama", ai.ProviderOllama},
		{"ai.chat_provider", "openai", ai.ProviderOpenAI},
		{"ai.chat_provider", "anthropic", ai.ProviderAnthropic},
		{"ai.chat_provider", "openrouter", ai.ProviderOpenRouter},
	}

	for _, tc := range testCases {
		if err := svc.SetSystem(ctx, tc.setting, tc.value, "test"); err != nil {
			t.Fatalf("Failed to set %s: %v", tc.setting, err)
		}

		var got ai.ProviderType
		if tc.setting == "ai.embedding_provider" {
			got = reader.GetEmbeddingProvider(ctx)
		} else {
			got = reader.GetChatProvider(ctx)
		}

		if got != tc.expected {
			t.Errorf("For %s=%s, expected %v, got %v", tc.setting, tc.value, tc.expected, got)
		}
	}
}

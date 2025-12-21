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

	if provider := reader.GetProvider(ctx); provider != ai.ProviderOllama {
		t.Errorf("Expected default provider to be Ollama, got %v", provider)
	}

	baseURL, model := reader.GetOllamaConfig(ctx)
	if baseURL != "http://localhost:11434" {
		t.Errorf("Expected default Ollama URL, got %v", baseURL)
	}
	if model != "nomic-embed-text" {
		t.Errorf("Expected default Ollama model, got %v", model)
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
	if err := svc.SetSystem(ctx, "ai.provider", "openai", "test"); err != nil {
		t.Fatalf("Failed to set ai.provider: %v", err)
	}
	if err := svc.SetSystem(ctx, "ai.ollama_url", "http://custom:11434", "test"); err != nil {
		t.Fatalf("Failed to set ai.ollama_url: %v", err)
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

	if provider := reader.GetProvider(ctx); provider != ai.ProviderOpenAI {
		t.Errorf("Expected provider OpenAI, got %v", provider)
	}

	baseURL, _ := reader.GetOllamaConfig(ctx)
	if baseURL != "http://custom:11434" {
		t.Errorf("Expected custom Ollama URL, got %v", baseURL)
	}

	if results := reader.GetMaxResults(ctx); results != 50 {
		t.Errorf("Expected max results 50, got %d", results)
	}

	if threshold := reader.GetSimilarityThreshold(ctx); threshold != 0.75 {
		t.Errorf("Expected similarity threshold 0.75, got %f", threshold)
	}
}

func TestAIConfigReader_GetEmbeddingConfig(t *testing.T) {
	repo := newMockSystemRepo()
	svc := NewService(repo, &mockUserRepo{}, nil)
	reader := NewAIConfigReader(svc)
	ctx := context.Background()

	// Test default Ollama config
	config := reader.GetEmbeddingConfig(ctx)
	if config.Provider != ai.ProviderOllama {
		t.Errorf("Expected Ollama provider, got %v", config.Provider)
	}

	// Set to OpenAI and verify
	svc.SetSystem(ctx, "ai.provider", "openai", "test")
	svc.SetSystem(ctx, "ai.openai_api_key", "sk-test123", "test")
	svc.SetSystem(ctx, "ai.openai_model", "text-embedding-3-large", "test")

	config = reader.GetEmbeddingConfig(ctx)
	if config.Provider != ai.ProviderOpenAI {
		t.Errorf("Expected OpenAI provider, got %v", config.Provider)
	}
	if config.Model != "text-embedding-3-large" {
		t.Errorf("Expected model text-embedding-3-large, got %v", config.Model)
	}
	if config.APIKey != "sk-test123" {
		t.Errorf("Expected API key sk-test123, got %v", config.APIKey)
	}
}

func TestAIConfigReader_GetLLMConfig(t *testing.T) {
	repo := newMockSystemRepo()
	svc := NewService(repo, &mockUserRepo{}, nil)
	reader := NewAIConfigReader(svc)
	ctx := context.Background()

	// Test default Ollama config
	config := reader.GetLLMConfig(ctx)
	if config.Type != ai.ProviderOllama {
		t.Errorf("Expected Ollama provider, got %v", config.Type)
	}
	if config.BaseURL != "http://localhost:11434" {
		t.Errorf("Expected default Ollama URL, got %v", config.BaseURL)
	}

	// Set custom Ollama URL
	svc.SetSystem(ctx, "ai.ollama_url", "http://gpu-server:11434", "test")
	svc.SetSystem(ctx, "ai.ollama_model", "llama3.2:8b", "test")

	config = reader.GetLLMConfig(ctx)
	if config.BaseURL != "http://gpu-server:11434" {
		t.Errorf("Expected custom URL, got %v", config.BaseURL)
	}
	if config.Model != "llama3.2:8b" {
		t.Errorf("Expected model llama3.2:8b, got %v", config.Model)
	}
}

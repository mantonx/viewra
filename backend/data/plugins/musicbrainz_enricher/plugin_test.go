package musicbrainz_enricher

import (
	"context"
	"testing"

	"musicbrainz_enricher/config"
)

func TestPluginInitialization(t *testing.T) {
	// Test configuration loading
	cfg := config.Default()
	if cfg == nil {
		t.Fatal("Failed to load default configuration")
	}

	// Test plugin creation (without database for now)
	plugin := NewPlugin(nil, cfg)
	if plugin == nil {
		t.Fatal("Failed to create plugin instance")
	}

	// Test plugin info
	info := plugin.Info()
	if info.ID != PluginID {
		t.Errorf("Expected plugin ID %s, got %s", PluginID, info.ID)
	}
	if info.Name != PluginName {
		t.Errorf("Expected plugin name %s, got %s", PluginName, info.Name)
	}
	if info.Version != PluginVersion {
		t.Errorf("Expected plugin version %s, got %s", PluginVersion, info.Version)
	}
}

func TestPluginInterfaces(t *testing.T) {
	cfg := config.Default()
	// Disable plugin for first test
	cfg.Enabled = false
	plugin := NewPlugin(nil, cfg)

	// Test ScannerHookPlugin interface
	err := plugin.OnScanStarted(1, 1, "/test/path")
	if err != nil {
		t.Errorf("OnScanStarted failed: %v", err)
	}

	err = plugin.OnScanCompleted(1, 1, map[string]interface{}{"files": 10})
	if err != nil {
		t.Errorf("OnScanCompleted failed: %v", err)
	}

	// Test MetadataScraperPlugin interface
	supported := plugin.SupportedTypes()
	if len(supported) == 0 {
		t.Error("Plugin should support at least one file type")
	}

	canHandle := plugin.CanHandle("/test/file.mp3", "audio/mpeg")
	if canHandle {
		t.Error("Plugin should not handle files when not enabled")
	}

	// Enable plugin and test again
	cfg.Enabled = true
	plugin = NewPlugin(nil, cfg)
	canHandle = plugin.CanHandle("/test/file.mp3", "audio/mpeg")
	if !canHandle {
		t.Error("Plugin should handle MP3 files when enabled")
	}
}

func TestIntegration(t *testing.T) {
	// Test integration creation
	configData := []byte(`
config:
  enabled: true
  api_rate_limit: 0.8
  user_agent: "Viewra/1.0.0"
  enable_artwork: true
  artwork_max_size: 1200
  artwork_quality: "front"
  match_threshold: 0.85
  auto_enrich: false
  overwrite_existing: false
  cache_duration_hours: 168
`)

	integration, err := NewIntegration(nil, configData)
	if err != nil {
		t.Fatalf("Failed to create integration: %v", err)
	}

	if integration == nil {
		t.Fatal("Integration should not be nil")
	}

	plugin := integration.GetPlugin()
	if plugin == nil {
		t.Fatal("Plugin should not be nil")
	}

	// Test start/stop without database (should not fail)
	ctx := context.Background()
	err = integration.Start(ctx)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	err = integration.Stop(ctx)
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
} 
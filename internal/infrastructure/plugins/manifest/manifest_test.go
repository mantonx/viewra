package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, m *Manifest)
	}{
		{
			name: "minimal required fields",
			yaml: `
id: test-plugin
name: Test Plugin
version: 1.0.0
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.ID != "test-plugin" {
					t.Errorf("expected ID 'test-plugin', got %q", m.ID)
				}
				if m.Name != "Test Plugin" {
					t.Errorf("expected Name 'Test Plugin', got %q", m.Name)
				}
				if m.Version != "1.0.0" {
					t.Errorf("expected Version '1.0.0', got %q", m.Version)
				}
			},
		},
		{
			name: "full manifest with all fields",
			yaml: `
id: full-plugin
name: Full Plugin
version: 2.0.0
description: A full-featured plugin
author: Test Author
license: MIT
homepage: https://example.com
min_host_version: 1.0.0
categories:
  - enricher
  - ai
capabilities:
  media_types:
    - movie
    - tv_show
  provides:
    - metadata
    - images
  is_local: true
  rate_limit: 100
service_capabilities:
  - semantic_search
  - similar_items
dependencies:
  - capability: provider
    required: true
  - capability: ai:search
    required: false
provides:
  - search
  - recommendations
requires:
  - embedding
type: enricher
media_types:
  - movie
permissions:
  - network
  - storage
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.ID != "full-plugin" {
					t.Errorf("expected ID 'full-plugin', got %q", m.ID)
				}
				if m.Description != "A full-featured plugin" {
					t.Errorf("expected Description 'A full-featured plugin', got %q", m.Description)
				}
				if m.Author != "Test Author" {
					t.Errorf("expected Author 'Test Author', got %q", m.Author)
				}
				if m.License != "MIT" {
					t.Errorf("expected License 'MIT', got %q", m.License)
				}
				if m.Homepage != "https://example.com" {
					t.Errorf("expected Homepage 'https://example.com', got %q", m.Homepage)
				}
				if m.MinHostVersion != "1.0.0" {
					t.Errorf("expected MinHostVersion '1.0.0', got %q", m.MinHostVersion)
				}
				if len(m.Categories) != 2 {
					t.Errorf("expected 2 categories, got %d", len(m.Categories))
				}
				if m.Capabilities == nil {
					t.Fatal("expected Capabilities to be set")
				}
				if len(m.Capabilities.MediaTypes) != 2 {
					t.Errorf("expected 2 media types in capabilities, got %d", len(m.Capabilities.MediaTypes))
				}
				if !m.Capabilities.IsLocal {
					t.Error("expected IsLocal to be true")
				}
				if m.Capabilities.RateLimit != 100 {
					t.Errorf("expected RateLimit 100, got %d", m.Capabilities.RateLimit)
				}
				if len(m.ServiceCapabilities) != 2 {
					t.Errorf("expected 2 service capabilities, got %d", len(m.ServiceCapabilities))
				}
				if len(m.Dependencies) != 2 {
					t.Errorf("expected 2 dependencies, got %d", len(m.Dependencies))
				}
				if m.Dependencies[0].Capability != "provider" || !m.Dependencies[0].Required {
					t.Error("first dependency should be provider (required)")
				}
				if m.Dependencies[1].Capability != "ai:search" || m.Dependencies[1].Required {
					t.Error("second dependency should be ai:search (optional)")
				}
				if len(m.Provides) != 2 {
					t.Errorf("expected 2 provides, got %d", len(m.Provides))
				}
				if len(m.Requires) != 1 || m.Requires[0] != "embedding" {
					t.Errorf("expected requires ['embedding'], got %v", m.Requires)
				}
				if m.Type != "enricher" {
					t.Errorf("expected Type 'enricher', got %q", m.Type)
				}
				if len(m.MediaTypes) != 1 || m.MediaTypes[0] != "movie" {
					t.Errorf("expected MediaTypes ['movie'], got %v", m.MediaTypes)
				}
				if len(m.Permissions) != 2 {
					t.Errorf("expected 2 permissions, got %d", len(m.Permissions))
				}
			},
		},
		{
			name: "empty optional fields",
			yaml: `
id: minimal
name: Minimal
version: 0.1.0
categories: []
permissions: []
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.Capabilities != nil {
					t.Error("expected Capabilities to be nil")
				}
				if len(m.Categories) != 0 {
					t.Errorf("expected empty categories, got %d", len(m.Categories))
				}
				if len(m.Permissions) != 0 {
					t.Errorf("expected empty permissions, got %d", len(m.Permissions))
				}
			},
		},
		{
			name: "semver with prerelease",
			yaml: `
id: prerelease
name: Prerelease Plugin
version: 1.0.0-beta.1
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.Version != "1.0.0-beta.1" {
					t.Errorf("expected Version '1.0.0-beta.1', got %q", m.Version)
				}
			},
		},
		{
			name: "capabilities without provides",
			yaml: `
id: cap-test
name: Capabilities Test
version: 1.0.0
capabilities:
  media_types:
    - music
  is_local: false
  rate_limit: 50
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.Capabilities == nil {
					t.Fatal("expected Capabilities to be set")
				}
				if len(m.Capabilities.Provides) != 0 {
					t.Errorf("expected empty provides in capabilities, got %d", len(m.Capabilities.Provides))
				}
				if m.Capabilities.RateLimit != 50 {
					t.Errorf("expected RateLimit 50, got %d", m.Capabilities.RateLimit)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.yml"), []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			m, err := Load(dir)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			tt.validate(t, m)
		})
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		wantErrText string
	}{
		{
			name:        "missing id",
			yaml:        "name: Test\nversion: 1.0.0",
			wantErrText: "id is required",
		},
		{
			name:        "empty id",
			yaml:        "id: \"\"\nname: Test\nversion: 1.0.0",
			wantErrText: "id is required",
		},
		{
			name:        "missing name",
			yaml:        "id: test\nversion: 1.0.0",
			wantErrText: "name is required",
		},
		{
			name:        "empty name",
			yaml:        "id: test\nname: \"\"\nversion: 1.0.0",
			wantErrText: "name is required",
		},
		{
			name:        "missing version",
			yaml:        "id: test\nname: Test",
			wantErrText: "version is required",
		},
		{
			name:        "empty version",
			yaml:        "id: test\nname: Test\nversion: \"\"",
			wantErrText: "version is required",
		},
		{
			name:        "all required fields missing",
			yaml:        "description: Some description",
			wantErrText: "id is required",
		},
		{
			name:        "whitespace only id",
			yaml:        "id: \"   \"\nname: Test\nversion: 1.0.0",
			wantErrText: "id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.yml"), []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			_, err := Load(dir)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsStr(err.Error(), tt.wantErrText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrText)
			}
		})
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantErrText string
	}{
		{
			name:        "malformed yaml - bad indentation",
			content:     "id: test\n  name: bad indent",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "invalid yaml syntax - unclosed quote",
			content:     "id: \"unclosed",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "duplicate key",
			content:     "id: test\nid: test2\nname: Test\nversion: 1.0.0",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "wrong type for categories",
			content:     "id: test\nname: Test\nversion: 1.0.0\ncategories: not-a-list",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "wrong type for capabilities",
			content:     "id: test\nname: Test\nversion: 1.0.0\ncapabilities: not-an-object",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "wrong type for dependencies",
			content:     "id: test\nname: Test\nversion: 1.0.0\ndependencies: not-a-list",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "binary content",
			content:     "\x00\x01\x02\x03",
			wantErrText: "failed to parse plugin.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.yml"), []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			_, err := Load(dir)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsStr(err.Error(), tt.wantErrText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrText)
			}
		})
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(t *testing.T) string
		wantErrText string
	}{
		{
			name: "non-existent directory",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/to/plugin"
			},
			wantErrText: "failed to read plugin.yml",
		},
		{
			name: "empty directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErrText: "failed to read plugin.yml",
		},
		{
			name: "directory with wrong filename",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte("id: test"), 0644); err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
				return dir
			},
			wantErrText: "failed to read plugin.yml",
		},
		{
			name: "plugin.yml is a directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.Mkdir(filepath.Join(dir, "plugin.yml"), 0755); err != nil {
					t.Fatalf("failed to create directory: %v", err)
				}
				return dir
			},
			wantErrText: "failed to read plugin.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := tt.setup(t)

			_, err := Load(dir)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsStr(err.Error(), tt.wantErrText) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrText)
			}
		})
	}
}

func TestLoad_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, m *Manifest, err error)
	}{
		{
			name: "unicode in fields",
			yaml: `
id: unicode-plugin
name: Plugin 日本語 中文 العربية
version: 1.0.0
description: Émojis are fine 🎉
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if m.Name != "Plugin 日本語 中文 العربية" {
					t.Errorf("unicode name not preserved: %q", m.Name)
				}
				if m.Description != "Émojis are fine 🎉" {
					t.Errorf("unicode description not preserved: %q", m.Description)
				}
			},
		},
		{
			name: "extra unknown fields are ignored",
			yaml: `
id: extra-fields
name: Extra Fields Plugin
version: 1.0.0
unknown_field: should be ignored
another_unknown:
  nested: value
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if m.ID != "extra-fields" {
					t.Errorf("expected ID 'extra-fields', got %q", m.ID)
				}
			},
		},
		{
			name: "very long values",
			yaml: `
id: long-values
name: ` + string(make([]byte, 1000)) + `
version: 1.0.0
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				// This should fail validation since name becomes empty null bytes
				// which get trimmed or cause issues
				if err == nil && m.Name == "" {
					t.Error("expected error or non-empty name")
				}
			},
		},
		{
			name: "yaml anchors and aliases",
			yaml: `
id: anchors
name: Anchor Test
version: 1.0.0
categories: &cats
  - enricher
  - ai
media_types: *cats
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(m.Categories) != 2 {
					t.Errorf("expected 2 categories, got %d", len(m.Categories))
				}
				if len(m.MediaTypes) != 2 {
					t.Errorf("expected 2 media_types (via alias), got %d", len(m.MediaTypes))
				}
			},
		},
		{
			name: "multiline description",
			yaml: `
id: multiline
name: Multiline Test
version: 1.0.0
description: |
  This is a multiline
  description that spans
  multiple lines.
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !containsStr(m.Description, "multiline") || !containsStr(m.Description, "multiple lines") {
					t.Errorf("multiline description not preserved: %q", m.Description)
				}
			},
		},
		{
			name: "null values for optional fields",
			yaml: `
id: nulls
name: Null Test
version: 1.0.0
description: ~
capabilities: null
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if m.Description != "" {
					t.Errorf("expected empty description for null, got %q", m.Description)
				}
				if m.Capabilities != nil {
					t.Error("expected nil capabilities for null value")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.yml"), []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			m, err := Load(dir)
			tt.validate(t, m, err)
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchStr(s, substr)))
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

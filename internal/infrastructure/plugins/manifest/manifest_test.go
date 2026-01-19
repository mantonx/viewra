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
display_category: other
capabilities:
  - metadata
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
				if m.DisplayCategory != "other" {
					t.Errorf("expected DisplayCategory 'other', got %q", m.DisplayCategory)
				}
				if len(m.Capabilities) != 1 || m.Capabilities[0] != "metadata" {
					t.Errorf("expected Capabilities ['metadata'], got %v", m.Capabilities)
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
display_category: providers
capabilities:
  - provider
  - provider:ollama
  - embedding
  - chat
requires:
  - embedding
media_types:
  - movie
  - tv_show
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
				if m.DisplayCategory != "providers" {
					t.Errorf("expected DisplayCategory 'providers', got %q", m.DisplayCategory)
				}
				if len(m.Capabilities) != 4 {
					t.Errorf("expected 4 capabilities, got %d", len(m.Capabilities))
				}
				if len(m.Requires) != 1 || m.Requires[0] != "embedding" {
					t.Errorf("expected Requires ['embedding'], got %v", m.Requires)
				}
				if len(m.MediaTypes) != 2 {
					t.Errorf("expected 2 media types, got %d", len(m.MediaTypes))
				}
				if len(m.Permissions) != 2 {
					t.Errorf("expected 2 permissions, got %d", len(m.Permissions))
				}
			},
		},
		{
			name: "search plugin",
			yaml: `
id: semantic-search
name: Semantic Search
version: 1.0.0
display_category: search
capabilities:
  - search
  - vector_search
  - search_provider
requires:
  - embedding
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.DisplayCategory != "search" {
					t.Errorf("expected DisplayCategory 'search', got %q", m.DisplayCategory)
				}
				if len(m.Capabilities) != 3 {
					t.Errorf("expected 3 capabilities, got %d: %v", len(m.Capabilities), m.Capabilities)
				}
				if len(m.Requires) != 1 {
					t.Errorf("expected 1 requires, got %d", len(m.Requires))
				}
			},
		},
		{
			name: "enricher plugin",
			yaml: `
id: tmdb
name: TMDb Enricher
version: 1.0.0
display_category: enrichers
capabilities:
  - metadata
  - artwork
  - external_ids
media_types:
  - movie
  - tv_show
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.DisplayCategory != "enrichers" {
					t.Errorf("expected DisplayCategory 'enrichers', got %q", m.DisplayCategory)
				}
				if len(m.Capabilities) != 3 {
					t.Errorf("expected 3 capabilities, got %d", len(m.Capabilities))
				}
				if len(m.MediaTypes) != 2 {
					t.Errorf("expected 2 media types, got %d", len(m.MediaTypes))
				}
			},
		},
		{
			name: "semver with prerelease",
			yaml: `
id: prerelease
name: Prerelease Plugin
version: 1.0.0-beta.1
display_category: other
capabilities:
  - metadata
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.Version != "1.0.0-beta.1" {
					t.Errorf("expected Version '1.0.0-beta.1', got %q", m.Version)
				}
			},
		},
		{
			name: "local builtin plugin",
			yaml: `
id: local-scanner
name: Local Scanner
version: 1.0.0
display_category: local
capabilities:
  - metadata
`,
			validate: func(t *testing.T, m *Manifest) {
				if m.DisplayCategory != "local" {
					t.Errorf("expected DisplayCategory 'local', got %q", m.DisplayCategory)
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
			yaml:        "name: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "id is required",
		},
		{
			name:        "empty id",
			yaml:        "id: \"\"\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "id is required",
		},
		{
			name:        "missing name",
			yaml:        "id: test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "name is required",
		},
		{
			name:        "empty name",
			yaml:        "id: test\nname: \"\"\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "name is required",
		},
		{
			name:        "missing version",
			yaml:        "id: test\nname: Test\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "version is required",
		},
		{
			name:        "empty version",
			yaml:        "id: test\nname: Test\nversion: \"\"\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "version is required",
		},
		{
			name:        "missing display_category",
			yaml:        "id: test\nname: Test\nversion: 1.0.0\ncapabilities:\n  - metadata",
			wantErrText: "display_category is required",
		},
		{
			name:        "invalid display_category",
			yaml:        "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: invalid_cat\ncapabilities:\n  - metadata",
			wantErrText: "invalid display_category",
		},
		{
			name:        "missing capabilities",
			yaml:        "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: other",
			wantErrText: "capabilities is required",
		},
		{
			name:        "empty capabilities",
			yaml:        "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities: []",
			wantErrText: "capabilities is required",
		},
		{
			name:        "invalid capability",
			yaml:        "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - invalid_cap",
			wantErrText: "invalid capability",
		},
		{
			name:        "invalid required capability",
			yaml:        "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata\nrequires:\n  - invalid_req",
			wantErrText: "invalid required capability",
		},
		{
			name:        "whitespace only id",
			yaml:        "id: \"   \"\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata",
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
			content:     "id: test\nid: test2\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "wrong type for capabilities",
			content:     "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities: not-a-list",
			wantErrText: "failed to parse plugin.yml",
		},
		{
			name:        "wrong type for requires",
			content:     "id: test\nname: Test\nversion: 1.0.0\ndisplay_category: other\ncapabilities:\n  - metadata\nrequires: not-a-list",
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
display_category: other
capabilities:
  - metadata
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
display_category: other
capabilities:
  - metadata
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
			name: "multiline description",
			yaml: `
id: multiline
name: Multiline Test
version: 1.0.0
display_category: other
capabilities:
  - metadata
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
display_category: other
capabilities:
  - metadata
description: ~
requires: null
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if m.Description != "" {
					t.Errorf("expected empty description for null, got %q", m.Description)
				}
				if m.Requires != nil && len(m.Requires) > 0 {
					t.Error("expected nil/empty requires for null value")
				}
			},
		},
		{
			name: "dynamic provider capability",
			yaml: `
id: ollama-provider
name: Ollama Provider
version: 1.0.0
display_category: providers
capabilities:
  - provider
  - provider:ollama
  - embedding
`,
			validate: func(t *testing.T, m *Manifest, err error) {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(m.Capabilities) != 3 {
					t.Errorf("expected 3 capabilities, got %d", len(m.Capabilities))
				}
				found := false
				for _, cap := range m.Capabilities {
					if cap == "provider:ollama" {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected provider:ollama capability")
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

func TestIsValidDisplayCategory(t *testing.T) {
	t.Parallel()

	valid := []string{"search", "recommendations", "enrichers", "providers", "local", "other"}
	for _, cat := range valid {
		if !IsValidDisplayCategory(cat) {
			t.Errorf("expected %q to be valid", cat)
		}
	}

	invalid := []string{"", "invalid", "SEARCH", "Search", "unknown"}
	for _, cat := range invalid {
		if IsValidDisplayCategory(cat) {
			t.Errorf("expected %q to be invalid", cat)
		}
	}
}

func TestIsValidCapability(t *testing.T) {
	t.Parallel()

	valid := []string{
		"search", "search_provider", "vector_search",
		"provider", "embedding", "chat",
		"metadata", "artwork", "external_ids", "trending",
		"notification_sink", "recommendations",
		"provider:ollama", "provider:anthropic", "provider:anything",
	}
	for _, cap := range valid {
		if !IsValidCapability(cap) {
			t.Errorf("expected %q to be valid", cap)
		}
	}

	invalid := []string{"", "invalid", "SEARCH", "Search", "unknown", "provideer"}
	for _, cap := range invalid {
		if IsValidCapability(cap) {
			t.Errorf("expected %q to be invalid", cap)
		}
	}
}

func TestGetDisplayCategory(t *testing.T) {
	t.Parallel()

	// Valid category
	cat := GetDisplayCategory("search")
	if cat.ID != "search" || cat.Label != "Search" || cat.Priority != 1 {
		t.Errorf("unexpected category: %+v", cat)
	}

	// Invalid category returns "other"
	cat = GetDisplayCategory("invalid")
	if cat.ID != "other" || cat.Label != "Other" || cat.Priority != 99 {
		t.Errorf("expected 'other' category for invalid input, got %+v", cat)
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

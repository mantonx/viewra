package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// BuildConfig represents build configuration for plugins
type BuildConfig struct {
	PluginName             string   `json:"plugin_name"`
	Version                string   `json:"version"`
	GoVersion              string   `json:"go_version"`
	TargetOS               string   `json:"target_os"`
	TargetArch             string   `json:"target_arch"`
	EnableCGO              bool     `json:"enable_cgo"`
	LDFlags                []string `json:"ld_flags"`
	BuildTags              []string `json:"build_tags"`
	StaticLink             bool     `json:"static_link"`
	CompressOutput         bool     `json:"compress_output"`
	OutputDir              string   `json:"output_dir"`
	ShouldGenerateManifest bool     `json:"generate_manifest"`
}

// DefaultBuildConfig returns sensible defaults for plugin building
func DefaultBuildConfig(pluginName string) *BuildConfig {
	return &BuildConfig{
		PluginName:             pluginName,
		Version:                "1.0.0",
		GoVersion:              runtime.Version(),
		TargetOS:               runtime.GOOS,
		TargetArch:             runtime.GOARCH,
		EnableCGO:              false,                // Disable CGO for better portability
		LDFlags:                []string{"-s", "-w"}, // Strip debug info
		BuildTags:              []string{},
		StaticLink:             true,
		CompressOutput:         false,
		OutputDir:              "./dist",
		ShouldGenerateManifest: true,
	}
}

// GenerateGoMod creates a standardized go.mod file for the plugin
func (bc *BuildConfig) GenerateGoMod(modulePath string) error {
	goModTemplate := `module {{.ModulePath}}

go {{.GoVersion}}

require (
	github.com/mantonx/viewra/pkg/plugins {{.PluginsVersion}}
	github.com/hashicorp/go-hclog v1.5.0
	github.com/hashicorp/go-plugin v1.4.10
	google.golang.org/grpc v1.58.3
	google.golang.org/protobuf v1.31.0
)

replace github.com/mantonx/viewra/pkg/plugins => ../../pkg/plugins
`

	data := struct {
		ModulePath     string
		GoVersion      string
		PluginsVersion string
	}{
		ModulePath:     modulePath,
		GoVersion:      strings.TrimPrefix(bc.GoVersion, "go"),
		PluginsVersion: "v0.1.0", // This should be dynamic
	}

	tmpl, err := template.New("go.mod").Parse(goModTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod template: %w", err)
	}

	file, err := os.Create("go.mod")
	if err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// GenerateManifest creates a plugin manifest file
func (bc *BuildConfig) GenerateManifest() error {
	if !bc.ShouldGenerateManifest {
		return nil
	}

	manifestTemplate := `{
    "id": "{{.PluginName}}",
    "name": "{{.DisplayName}}",
    "version": "{{.Version}}",
    "description": "{{.Description}}",
    "author": "{{.Author}}",
    "type": "{{.Type}}",
    "enabled_by_default": {{.EnabledByDefault}},
    "capabilities": {
        "metadata_scraper": {{.HasMetadataScraper}},
        "scanner_hook": {{.HasScannerHook}},
        "asset_service": {{.HasAssetService}},
        "admin_page": {{.HasAdminPage}}
    },
    "entry_points": {
        "main": "./{{.PluginName}}"
    },
    "permissions": {{.Permissions}},
    "build_info": {
        "go_version": "{{.GoVersion}}",
        "built_at": "{{.BuildTime}}",
        "target_os": "{{.TargetOS}}",
        "target_arch": "{{.TargetArch}}",
        "static_link": {{.StaticLink}}
    }
}`

	// This would need actual plugin metadata - showing structure
	data := struct {
		PluginName         string
		DisplayName        string
		Version            string
		Description        string
		Author             string
		Type               string
		EnabledByDefault   bool
		HasMetadataScraper bool
		HasScannerHook     bool
		HasAssetService    bool
		HasAdminPage       bool
		Permissions        string
		GoVersion          string
		BuildTime          string
		TargetOS           string
		TargetArch         string
		StaticLink         bool
	}{
		PluginName:  bc.PluginName,
		DisplayName: strings.Title(strings.ReplaceAll(bc.PluginName, "_", " ")),
		Version:     bc.Version,
		Description: fmt.Sprintf("Auto-generated plugin: %s", bc.PluginName),
		Author:      "Plugin Developer",
		Type:        "metadata_scraper",
		GoVersion:   bc.GoVersion,
		TargetOS:    bc.TargetOS,
		TargetArch:  bc.TargetArch,
		StaticLink:  bc.StaticLink,
		Permissions: `["network", "filesystem"]`,
	}

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse manifest template: %w", err)
	}

	file, err := os.Create("plugin.json")
	if err != nil {
		return fmt.Errorf("failed to create plugin.json: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// GenerateMakefile creates a standardized Makefile for plugin building
func (bc *BuildConfig) GenerateMakefile() error {
	makefileTemplate := `# Auto-generated Makefile for {{.PluginName}}
PLUGIN_NAME={{.PluginName}}
VERSION={{.Version}}
BUILD_DIR={{.OutputDir}}
GO_VERSION={{.GoVersion}}

# Build flags
LDFLAGS={{.LDFlags}}
BUILD_TAGS={{.BuildTags}}
CGO_ENABLED={{.CGOEnabled}}

.PHONY: build clean test install deps

# Default target
build: clean deps
	@echo "Building $(PLUGIN_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build \
		-tags "$(BUILD_TAGS)" \
		-ldflags "$(LDFLAGS) -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(PLUGIN_NAME) .
	@echo "✅ Build complete: $(BUILD_DIR)/$(PLUGIN_NAME)"

# Cross-compilation targets
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "$(BUILD_TAGS)" \
		-ldflags "$(LDFLAGS) -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(PLUGIN_NAME)-linux-amd64 .

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "$(BUILD_TAGS)" \
		-ldflags "$(LDFLAGS) -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(PLUGIN_NAME)-windows-amd64.exe .

build-darwin:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build \
		-tags "$(BUILD_TAGS)" \
		-ldflags "$(LDFLAGS) -X main.version=$(VERSION)" \
		-o $(BUILD_DIR)/$(PLUGIN_NAME)-darwin-amd64 .

# Build for all platforms
build-all: build-linux build-windows build-darwin

# Development targets
dev: deps
	go run . --help

test:
	go test -v ./...

deps:
	go mod download
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)
	go clean

# Installation helpers
install: build
	@echo "Installing $(PLUGIN_NAME) to viewra plugins directory..."
	# This would copy to the appropriate plugin directory

# Linting and formatting
lint:
	golangci-lint run

fmt:
	go fmt ./...

# Version information
version:
	@echo "Plugin: $(PLUGIN_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(GO_VERSION)"

help:
	@echo "Available targets:"
	@echo "  build       - Build the plugin for current platform"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  deps        - Download dependencies"
	@echo "  lint        - Run linter"
	@echo "  fmt         - Format code"
	@echo "  dev         - Run plugin in development mode"
	@echo "  version     - Show version information"
`

	data := struct {
		PluginName string
		Version    string
		OutputDir  string
		GoVersion  string
		LDFlags    string
		BuildTags  string
		CGOEnabled string
	}{
		PluginName: bc.PluginName,
		Version:    bc.Version,
		OutputDir:  bc.OutputDir,
		GoVersion:  bc.GoVersion,
		LDFlags:    strings.Join(bc.LDFlags, " "),
		BuildTags:  strings.Join(bc.BuildTags, " "),
		CGOEnabled: map[bool]string{true: "1", false: "0"}[bc.EnableCGO],
	}

	tmpl, err := template.New("makefile").Parse(makefileTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Makefile template: %w", err)
	}

	file, err := os.Create("Makefile")
	if err != nil {
		return fmt.Errorf("failed to create Makefile: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// GenerateDockerfile creates a standardized Dockerfile for plugin building
func (bc *BuildConfig) GenerateDockerfile() error {
	dockerfileTemplate := `# Auto-generated Dockerfile for {{.PluginName}}
FROM golang:{{.GoVersion}}-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the plugin
RUN CGO_ENABLED={{.CGOEnabled}} go build \
    -tags "{{.BuildTags}}" \
    -ldflags "{{.LDFlags}}" \
    -o {{.PluginName}} .

# Final stage - minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create plugin user
RUN addgroup -g 1000 plugin && \
    adduser -D -s /bin/sh -u 1000 -G plugin plugin

# Copy built plugin
COPY --from=builder /app/{{.PluginName}} /usr/local/bin/{{.PluginName}}

# Set ownership and permissions
RUN chown plugin:plugin /usr/local/bin/{{.PluginName}} && \
    chmod +x /usr/local/bin/{{.PluginName}}

# Switch to plugin user
USER plugin

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD /usr/local/bin/{{.PluginName}} --health || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/{{.PluginName}}"]
CMD ["--help"]
`

	data := struct {
		PluginName string
		GoVersion  string
		CGOEnabled string
		BuildTags  string
		LDFlags    string
	}{
		PluginName: bc.PluginName,
		GoVersion:  strings.TrimPrefix(bc.GoVersion, "go"),
		CGOEnabled: map[bool]string{true: "1", false: "0"}[bc.EnableCGO],
		BuildTags:  strings.Join(bc.BuildTags, " "),
		LDFlags:    strings.Join(bc.LDFlags, " "),
	}

	tmpl, err := template.New("dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse Dockerfile template: %w", err)
	}

	file, err := os.Create("Dockerfile")
	if err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

// SetupPluginProject initializes a complete plugin project structure
func SetupPluginProject(pluginName, modulePath string) error {
	config := DefaultBuildConfig(pluginName)

	// Create directory structure
	dirs := []string{
		"internal",
		"internal/config",
		"internal/services",
		"pkg",
		"cmd",
		"test",
		"docs",
		"scripts",
		config.OutputDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Generate standard files
	if err := config.GenerateGoMod(modulePath); err != nil {
		return fmt.Errorf("failed to generate go.mod: %w", err)
	}

	if err := config.GenerateManifest(); err != nil {
		return fmt.Errorf("failed to generate manifest: %w", err)
	}

	if err := config.GenerateMakefile(); err != nil {
		return fmt.Errorf("failed to generate Makefile: %w", err)
	}

	if err := config.GenerateDockerfile(); err != nil {
		return fmt.Errorf("failed to generate Dockerfile: %w", err)
	}

	// Generate .gitignore
	gitignoreContent := `# Build artifacts
/dist/
*.exe
*.exe~
*.dll
*.so
*.dylib

# Test binary, built with 'go test -c'
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# IDE files
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Plugin-specific
*.db
*.log
config.json
*.pid
`

	if err := os.WriteFile(".gitignore", []byte(gitignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	fmt.Printf("✅ Plugin project '%s' initialized successfully!\n", pluginName)
	fmt.Printf("📁 Project structure created in current directory\n")
	fmt.Printf("🔧 Run 'make help' to see available build targets\n")

	return nil
}

// GetBuildCommand returns the complete build command for the plugin
func (bc *BuildConfig) GetBuildCommand() string {
	cmd := "go build"

	if len(bc.BuildTags) > 0 {
		cmd += fmt.Sprintf(" -tags \"%s\"", strings.Join(bc.BuildTags, " "))
	}

	if len(bc.LDFlags) > 0 {
		cmd += fmt.Sprintf(" -ldflags \"%s\"", strings.Join(bc.LDFlags, " "))
	}

	outputPath := filepath.Join(bc.OutputDir, bc.PluginName)
	if bc.TargetOS == "windows" {
		outputPath += ".exe"
	}

	cmd += fmt.Sprintf(" -o %s", outputPath)

	return cmd
}

// Validate checks if the build configuration is valid
func (bc *BuildConfig) Validate() error {
	if bc.PluginName == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if bc.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	if bc.OutputDir == "" {
		return fmt.Errorf("output directory cannot be empty")
	}

	// Validate target OS/arch combinations
	validCombinations := map[string][]string{
		"linux":   {"amd64", "arm64", "386", "arm"},
		"windows": {"amd64", "386"},
		"darwin":  {"amd64", "arm64"},
	}

	if validArches, ok := validCombinations[bc.TargetOS]; ok {
		valid := false
		for _, arch := range validArches {
			if bc.TargetArch == arch {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid target combination: %s/%s", bc.TargetOS, bc.TargetArch)
		}
	} else {
		return fmt.Errorf("unsupported target OS: %s", bc.TargetOS)
	}

	return nil
}

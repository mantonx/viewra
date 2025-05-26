package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mantonx/viewra/internal/plugins"
)

// Simple logger that implements the PluginLogger interface
type testLogger struct{}

func (l *testLogger) Debug(msg string, args ...interface{}) {
log.Printf("[DEBUG] %s %v\n", msg, args)
}

func (l *testLogger) Info(msg string, args ...interface{}) {
log.Printf("[INFO] %s %v\n", msg, args)
}

func (l *testLogger) Warn(msg string, args ...interface{}) {
log.Printf("[WARN] %s %v\n", msg, args)
}

func (l *testLogger) Error(msg string, args ...interface{}) {
log.Printf("[ERROR] %s %v\n", msg, args)
}

func main() {
// Test manifest reading directly without using the Manager
fmt.Println("Testing YAML Plugin Discovery")
fmt.Println("============================")

// Get the current directory
cwd, err := os.Getwd()
if err != nil {
log.Fatal("Failed to get current directory:", err)
}

// Check for all plugin manifests
files := []string{
filepath.Join(cwd, "../..", "data/plugins/example_yaml_plugin/plugin.yml"),
// Add any other plugin files to test here
}

for _, file := range files {
fmt.Printf("\nTesting manifest file: %s\n", file)
manifest, err := plugins.ReadPluginManifestFile(file)

if err != nil {
fmt.Printf("Error reading manifest: %s\n", err)
continue
}

fmt.Println("Successfully parsed manifest:")
fmt.Printf("ID: %s\n", manifest.ID)
fmt.Printf("Name: %s\n", manifest.Name)
fmt.Printf("Version: %s\n", manifest.Version)
fmt.Printf("Description: %s\n", manifest.Description)
fmt.Printf("Type: %s\n", manifest.Type)

fmt.Printf("Tags: %v\n", manifest.Tags)

fmt.Println("Capabilities:")
fmt.Printf("  admin_pages: %v\n", manifest.Capabilities.AdminPages)
fmt.Printf("  api_endpoints: %v\n", manifest.Capabilities.APIEndpoints)
fmt.Printf("  ui_components: %v\n", manifest.Capabilities.UIComponents)
}
}

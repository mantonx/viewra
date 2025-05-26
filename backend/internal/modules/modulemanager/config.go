package modulemanager

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ModuleConfig represents the module configuration structure
type ModuleConfig struct {
Modules struct {
Disabled []string `yaml:"disabled"`
} `yaml:"modules"`
}

// LoadConfig loads module configuration from YAML file
func LoadConfig(configPath string) (*ModuleConfig, error) {
config := &ModuleConfig{}

// Check if config file exists
if _, err := os.Stat(configPath); os.IsNotExist(err) {
// Return default config if file doesn't exist
return config, nil
}

// Read config file
data, err := os.ReadFile(configPath)
if err != nil {
return nil, fmt.Errorf("failed to read config file: %w", err)
}

// Parse YAML
if err := yaml.Unmarshal(data, config); err != nil {
return nil, fmt.Errorf("failed to parse config file: %w", err)
}

return config, nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
// Check for config in current directory first
if _, err := os.Stat("viewra-modules.yml"); err == nil {
return "viewra-modules.yml"
}

// Check for config in data directory
dataDir := os.Getenv("DATA_DIR")
if dataDir == "" {
dataDir = "./data"
}

return filepath.Join(dataDir, "viewra-modules.yml")
}

// CreateExampleConfig creates an example configuration file
func CreateExampleConfig(configPath string) error {
config := &ModuleConfig{}

// Example: disable scanner module for development
config.Modules.Disabled = []string{
// "system.scanner",  // Uncomment to disable scanner module
}

data, err := yaml.Marshal(config)
if err != nil {
return fmt.Errorf("failed to marshal config: %w", err)
}

// Ensure directory exists
dir := filepath.Dir(configPath)
if err := os.MkdirAll(dir, 0755); err != nil {
return fmt.Errorf("failed to create config directory: %w", err)
}

// Write config file
if err := os.WriteFile(configPath, data, 0644); err != nil {
return fmt.Errorf("failed to write config file: %w", err)
}

return nil
}

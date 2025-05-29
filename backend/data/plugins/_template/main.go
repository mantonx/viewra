package main

import (
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/mantonx/viewra/internal/plugins"
	"github.com/mantonx/viewra/internal/plugins/proto"
)

// TemplatePlugin implements the plugin interfaces
type TemplatePlugin struct {
	logger   hclog.Logger
	basePath string
}

func (t *TemplatePlugin) Initialize(ctx *proto.PluginContext) error {
	t.logger = hclog.New(&hclog.LoggerOptions{
		Name:   "template-plugin",
		Output: os.Stderr,
		Level:  hclog.Info,
	})

	t.basePath = ctx.BasePath
	t.logger.Info("Template plugin initialized", "base_path", t.basePath)
	return nil
}

func (t *TemplatePlugin) Start() error {
	t.logger.Info("Template plugin started")
	return nil
}

func (t *TemplatePlugin) Stop() error {
	t.logger.Info("Template plugin stopped")
	return nil
}

func (t *TemplatePlugin) Info() (*proto.PluginInfo, error) {
	return &proto.PluginInfo{
		Name:        "Template Plugin",
		Version:     "1.0.0",
		Description: "Template plugin for development",
		Author:      "Viewra Team",
		Capabilities: []string{
			"metadata_scraper",
		},
	}, nil
}

func (t *TemplatePlugin) Health() error {
	return nil
}

// Metadata scraper implementation
func (t *TemplatePlugin) MetadataScraperService() plugins.MetadataScraperService {
	return t
}

func (t *TemplatePlugin) CanHandle(filePath, mimeType string) bool {
	ext := filepath.Ext(filePath)
	return ext == ".mp3" || ext == ".flac" || ext == ".m4a"
}

func (t *TemplatePlugin) ExtractMetadata(filePath string) (map[string]string, error) {
	// Simple metadata extraction without database
	return map[string]string{
		"title":  "Template Title",
		"artist": "Template Artist",
		"album":  "Template Album",
	}, nil
}

func (t *TemplatePlugin) GetSupportedTypes() []string {
	return []string{"audio/mpeg", "audio/flac", "audio/mp4"}
}

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "VIEWRA_PLUGIN",
	MagicCookieValue: "template",
}

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			"template": &plugins.ViewraPlugin{Implementation: &TemplatePlugin{}},
		},
		GRPCServer: plugin.DefaultGRPCServer,
	})
} 
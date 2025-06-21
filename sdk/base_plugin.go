package plugins


// BasePlugin provides default implementations for all optional plugin services
// Plugins can embed this struct and only override the methods they need
type BasePlugin struct {
	name        string
	version     string
	pluginType  string
	description string
}

// NewBasePlugin creates a new base plugin with the given metadata
func NewBasePlugin(name, version, pluginType, description string) *BasePlugin {
	return &BasePlugin{
		name:        name,
		version:     version,
		pluginType:  pluginType,
		description: description,
	}
}

// Core plugin methods
func (b *BasePlugin) Initialize(ctx *PluginContext) error {
	return nil
}

func (b *BasePlugin) Start() error {
	return nil
}

func (b *BasePlugin) Stop() error {
	return nil
}

func (b *BasePlugin) Info() (*PluginInfo, error) {
	return &PluginInfo{
		Name:        b.name,
		Version:     b.version,
		Type:        b.pluginType,
		Description: b.description,
		Author:      "Plugin Developer",
	}, nil
}

func (b *BasePlugin) Health() error {
	return nil
}

// Optional service implementations (return nil if not supported)
func (b *BasePlugin) MetadataScraperService() MetadataScraperService {
	return nil
}

func (b *BasePlugin) ScannerHookService() ScannerHookService {
	return nil
}

func (b *BasePlugin) AssetService() AssetService {
	return nil
}

func (b *BasePlugin) DatabaseService() DatabaseService {
	return nil
}

func (b *BasePlugin) AdminPageService() AdminPageService {
	return nil
}

func (b *BasePlugin) APIRegistrationService() APIRegistrationService {
	return nil
}

func (b *BasePlugin) SearchService() SearchService {
	return nil
}

// Enhanced service interfaces (return nil if not supported)
func (b *BasePlugin) HealthMonitorService() HealthMonitorService {
	return nil
}

func (b *BasePlugin) ConfigurationService() ConfigurationService {
	return nil
}

func (b *BasePlugin) PerformanceMonitorService() PerformanceMonitorService {
	return nil
}

func (b *BasePlugin) TranscodingProvider() TranscodingProvider {
	return nil
}

func (b *BasePlugin) EnhancedAdminPageService() EnhancedAdminPageService {
	return nil
}
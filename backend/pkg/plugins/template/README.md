# Template Plugin for Viewra SDK

This is a comprehensive template plugin that demonstrates how to implement all available service interfaces using the modern Viewra plugin SDK. It serves as a reference implementation for plugin developers and is part of the SDK package.

## Features Demonstrated

This template plugin showcases:

- **Plugin Lifecycle Management**: Proper initialization, startup, shutdown, and health checks
- **Configuration System**: Structured configuration with validation and defaults
- **Database Integration**: GORM models, migrations, and database operations
- **Multiple Service Interfaces**: Implementation of all major plugin service types
- **Logging**: Structured logging with different levels
- **Error Handling**: Proper error propagation and handling patterns
- **Modern SDK Usage**: Using `github.com/mantonx/viewra/pkg/plugins`

## Service Interfaces Implemented

### 1. MetadataScraperService

- Demonstrates file type detection
- Shows metadata extraction patterns
- Handles MIME type validation

### 2. ScannerHookService

- Hooks into media scanning lifecycle
- Shows database storage patterns
- Demonstrates event-based processing

### 3. SearchService

- Implements search functionality
- Shows result formatting
- Demonstrates pagination and limits

### 4. DatabaseService

- Custom database models
- Migration and rollback patterns
- Database initialization

### 5. APIRegistrationService

- Custom API endpoint registration
- RESTful API patterns
- Route documentation

## Project Structure

```
backend/pkg/plugins/template/
├── main.go         # Main plugin implementation
├── go.mod          # Go module with SDK dependencies
├── plugin.cue      # Plugin metadata and configuration schema
├── README.md       # This documentation
└── template        # Built plugin binary (after build)
```

## Building the Template

### As Reference (Recommended)

The template is primarily intended as a reference. To create a new plugin:

```bash
# Copy template to plugin directory
cp -r backend/pkg/plugins/template backend/data/plugins/my_plugin
cd backend/data/plugins/my_plugin

# Update module name in go.mod
sed -i 's|github.com/mantonx/viewra/pkg/plugins/template|github.com/mantonx/viewra/plugins/my_plugin|' go.mod

# Update replace directive for plugin location
sed -i 's|replace github.com/mantonx/viewra/pkg/plugins => ../|replace github.com/mantonx/viewra/pkg/plugins => ../../../pkg/plugins|' go.mod

# Update plugin identifiers in plugin.cue
# Update binary name in entry_points
# Build your plugin
make build-plugin p=my_plugin
```

### Building Template Directly (Testing)

```bash
cd backend/pkg/plugins/template
go mod tidy
go build -o template main.go
```

## Configuration

The plugin supports the following configuration options in `plugin.cue`:

```cue
settings: {
    enabled: bool | *true

    config: {
        api_key:      string | *""
        user_agent:   string | *"Viewra-Template/1.0"
        max_results:  int & >=1 & <=100 | *10
        cache_hours:  int & >=1 & <=168 | *24
        debug:        bool | *false
    }
}
```

## Database Models

The template defines a sample database model:

```go
type TemplateData struct {
    ID          uint      `gorm:"primaryKey"`
    MediaFileID uint      `gorm:"not null;index"`
    Title       string    `gorm:"size:255"`
    Artist      string    `gorm:"size:255"`
    ProcessedAt time.Time `gorm:"autoCreateTime"`
    Metadata    string    `gorm:"type:text"`
}
```

## API Endpoints

When enabled, the plugin registers these API endpoints:

- `GET /api/plugins/template/info` - Get plugin information
- `GET /api/plugins/template/search` - Search functionality
- `POST /api/plugins/template/process` - Process media files

## Creating Your Own Plugin

To create a new plugin based on this template:

1. **Copy the template**:

   ```bash
   cp -r backend/pkg/plugins/template backend/data/plugins/my_plugin
   cd backend/data/plugins/my_plugin
   ```

2. **Update module and paths**:

   ```bash
   # Update go.mod module name
   sed -i 's|pkg/plugins/template|plugins/my_plugin|' go.mod

   # Fix replace directive path
   sed -i 's|=> ../|=> ../../../pkg/plugins|' go.mod
   ```

3. **Update identifiers**:

   - Update plugin ID, name, and description in `plugin.cue`
   - Rename binary in entry_points
   - Update module name references

4. **Implement your logic**:

   - Modify the service interface implementations
   - Add your database models
   - Update configuration schema
   - Add your API endpoints

5. **Build and test**:
   ```bash
   go mod tidy
   make build-plugin p=my_plugin
   make test-plugin p=my_plugin
   ```

## Code Patterns

### Plugin Structure

```go
type MyPlugin struct {
    logger   plugins.Logger      // Use provided logger
    config   *Config            // Plugin configuration
    db       *gorm.DB          // Database connection
    basePath string            // Plugin base directory
}
```

### Service Interface Implementation

```go
func (p *MyPlugin) MetadataScraperService() plugins.MetadataScraperService {
    return p  // Return self if implementing
}

func (p *MyPlugin) SearchService() plugins.SearchService {
    return nil  // Return nil if not implementing
}
```

### Error Handling

```go
func (p *MyPlugin) someMethod() error {
    if !p.config.Enabled {
        return fmt.Errorf("plugin is disabled")
    }

    if err := p.doSomething(); err != nil {
        p.logger.Error("Operation failed", "error", err)
        return fmt.Errorf("failed to do something: %w", err)
    }

    return nil
}
```

### Database Operations

```go
func (p *MyPlugin) saveData(data *MyModel) error {
    if err := p.db.Create(data).Error; err != nil {
        return fmt.Errorf("failed to save data: %w", err)
    }
    return nil
}
```

### Logging

```go
p.logger.Info("Operation completed", "file", filePath, "count", count)
p.logger.Debug("Debug info", "details", debugData)
p.logger.Warn("Warning occurred", "reason", reason)
p.logger.Error("Error occurred", "error", err)
```

## Best Practices Demonstrated

1. **Configuration Validation**: Check required settings on startup
2. **Resource Cleanup**: Properly close database connections on shutdown
3. **Error Context**: Wrap errors with meaningful context
4. **Structured Logging**: Use key-value pairs for better log analysis
5. **Health Checks**: Implement meaningful health status checks
6. **Graceful Degradation**: Handle missing optional dependencies
7. **Database Migrations**: Proper schema management
8. **Service Interface Separation**: Clean separation of concerns

## Testing

The template includes patterns for:

- Unit testing individual methods
- Integration testing with database
- Mocking external dependencies
- Testing plugin lifecycle

## Advanced Features

The template demonstrates:

- **Configuration Schema Validation**: Using CueLang for type safety
- **Database Model Relationships**: GORM associations and indexes
- **API Route Registration**: Dynamic endpoint registration
- **Search Result Formatting**: Consistent result structures
- **Caching Patterns**: Database-backed caching strategies

## Contributing

When contributing improvements to this template:

1. Maintain backward compatibility
2. Add comprehensive comments
3. Include examples for new patterns
4. Update this documentation
5. Test with multiple plugin types

## See Also

- [Plugin System Documentation](../../../docs/PLUGINS.md)
- [Plugin SDK Interfaces](../) - SDK interface definitions
- [MusicBrainz Enricher](../../../data/plugins/musicbrainz_enricher/) - Production plugin example
- [AudioDB Enricher](../../../data/plugins/audiodb_enricher/) - API integration example

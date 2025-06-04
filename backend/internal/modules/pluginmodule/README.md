# Plugin Module

The Plugin Module is a comprehensive system for managing core and external plugins in the Viewra media server. It provides a clean separation between core functionality and external extensions while enforcing strict library-plugin relationships.

## Architecture

### Core Components

- **PluginModule**: Main module coordinator that manages all plugin operations
- **PluginRegistry**: Global registry system for plugin self-registration
- **LibraryPluginConfig**: Configuration system defining which plugins can be used with which library types
- **Core Plugin Managers**: Manage built-in plugins (FFmpeg, structure parsers, etc.)
- **External Plugin Managers**: Manage dynamically loaded plugins via gRPC
- **Proto Definitions**: Protocol buffer definitions for external plugin communication

### Plugin Organization

```
backend/internal/modules/pluginmodule/     # Plugin module (all infrastructure)
├── module.go          # Main plugin module coordinator
├── types.go           # Plugin interfaces and types
├── registry.go        # Global plugin registration system
├── config.go          # Library-plugin configuration
├── core_manager.go    # Core plugin management
├── external_manager.go # External plugin management
├── library_manager.go # Library-specific configuration
├── media_manager.go   # Asset handling
├── proto/            # Protocol buffer definitions for external plugins
│   ├── plugin.proto  # gRPC service definitions
│   ├── plugin.pb.go  # Generated protobuf types
│   └── plugin_grpc.pb.go # Generated gRPC services
└── README.md         # This file

backend/internal/plugins/                  # Core plugin implementations only
├── ffmpeg/           # FFmpeg technical analysis (core)
├── tvstructure/      # TV show structure parsing (core)
├── moviestructure/   # Movie structure parsing (core)
├── enrichment/       # Music metadata extraction (core)
└── README.md         # Plugin development guide
```

## Library-Plugin Mapping

The system enforces strict relationships between library types and allowed plugins:

### TV Libraries

- **Core Plugins**:
  - `tv_structure_parser_core_plugin` - TV show structure parsing
  - `ffmpeg_probe_core_plugin` - Video technical analysis
- **External Plugins**:
  - `tmdb_enricher` - TheMovieDB metadata enrichment

### Movie Libraries

- **Core Plugins**:
  - `movie_structure_parser_core_plugin` - Movie structure parsing
  - `ffmpeg_probe_core_plugin` - Video technical analysis
- **External Plugins**:
  - `tmdb_enricher` - TheMovieDB metadata enrichment

### Music Libraries

- **Core Plugins**:
  - `music_metadata_extractor_plugin` - Music metadata extraction and structure
  - `ffmpeg_probe_core_plugin` - Audio technical analysis
- **External Plugins**:
  - `musicbrainz_enricher` - MusicBrainz metadata enrichment

## Key Features

### Self-Registration

Core plugins register themselves automatically via `init()` functions:

```go
func init() {
    pluginmodule.RegisterCorePluginFactory("plugin_name", func() pluginmodule.CorePlugin {
        return NewPluginInstance()
    })
}
```

### Configuration-Driven

Plugin usage is controlled via JSON configuration files that define:

- Required core plugins per library type
- Allowed/forbidden external plugins per library type
- File extension restrictions
- External plugin rules and approval requirements

### Clean Separation

- **Core plugins** live in `internal/plugins/` and handle essential functionality
- **External plugins** communicate via gRPC using protocol definitions in `proto/`
- **Module** coordinates everything and owns all infrastructure

### No Hardcoded Dependencies

- Both core and external plugins register themselves
- No hardcoded plugin lists in the main application
- Configuration files control which plugins are active

### Zero Tech Debt

- No deprecated code or backwards compatibility layers
- Clean interfaces with modern architecture
- Self-documenting design patterns

## Usage

### Initializing the Plugin Module

```go
config := &pluginmodule.PluginModuleConfig{
    PluginDir:  "/path/to/external/plugins",
    ConfigPath: "/path/to/library-plugin-config.json",
    HostPort:   ":50051",
}

module := pluginmodule.NewPluginModule(db, config)
if err := module.Initialize(ctx); err != nil {
    log.Fatal("Failed to initialize plugin module:", err)
}
```

### Processing Files

The module automatically determines which plugins to use based on library configuration:

```go
err := module.ProcessFile(libraryID, filePath, mediaFile)
```

### External Plugin Development

External plugins use the gRPC protocol definitions in `proto/` to communicate:

```go
import "github.com/mantonx/viewra/internal/modules/pluginmodule/proto"

// Implement the plugin services defined in plugin.proto
type MyPlugin struct {
    proto.UnimplementedPluginServiceServer
    proto.UnimplementedMetadataScraperServiceServer
}
```

## External Plugin Communication

External plugins communicate with the system via gRPC, providing:

- Metadata enrichment (MusicBrainz, TMDB)
- Custom processing pipelines
- Third-party integrations

The system includes proper sandboxing and approval workflows for external plugins to ensure security and stability.

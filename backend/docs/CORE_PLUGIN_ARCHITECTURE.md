# Core Plugin Architecture

## Overview

The scanner module has been refactored to use a generic, extensible core plugin system that supports multiple media types (music, video, images, etc.) instead of hardcoded music-specific logic.

## Architecture

### Core Components

```
backend/internal/plugins/
├── registry.go           # Plugin registry + Match logic
├── types.go              # Original plugin interfaces (for external plugins)
├── media_types.go        # Core media plugin interfaces
├── core.go               # Core plugins manager
├── music/                # Music core plugin
│   ├── plugin.go
│   └── tagreader.go
├── tv/                   # Future: TV core plugin
│   ├── plugin.go
│   └── parser.go
└── movies/               # Future: Movie core plugin
    ├── plugin.go
    └── matcher.go
```

### Key Interfaces

#### `MediaHandlerPlugin`

Base interface for all media file processors:

```go
type MediaHandlerPlugin interface {
    Match(path string, info fs.FileInfo) bool
    HandleFile(path string, ctx MediaContext) error
    GetName() string
    GetSupportedExtensions() []string
    GetMediaType() string  // "music", "video", "image", etc.
}
```

#### `CoreMediaPlugin`

Interface for built-in plugins that are always available:

```go
type CoreMediaPlugin interface {
    MediaHandlerPlugin
    IsEnabled() bool
    Initialize() error
    Shutdown() error
    Enable() error
    Disable() error
}
```

### Generic Data Structures

#### `scanResult`

```go
type scanResult struct {
    mediaFile    *database.MediaFile
    metadata     interface{}  // Can be MusicMetadata, VideoMetadata, etc.
    metadataType string      // "music", "video", "image"
    path         string
    error        error
    needsPluginHooks bool
}
```

#### `MetadataItem`

```go
type MetadataItem struct {
    Data interface{}  // The actual metadata struct
    Type string      // Type identifier
    Path string      // File path for linking
}
```

## Scanner Module Changes

### Generic File Processing

- File type detection is now plugin-driven
- Extensions are dynamically collected from all registered plugins
- Metadata extraction uses plugin system instead of hardcoded logic

### Batch Processing

- `BatchProcessor` now handles multiple metadata types
- Type-safe insertion based on metadata type
- Graceful handling of unknown metadata types

### Plugin Hooks

- Interfaces updated to use `interface{}` for metadata
- Hooks work with any media type
- Future-proof for TV/Movie plugins

## Usage Example

### Registering Core Plugins

```go
// Initialize core media plugins
coreManager := plugins.NewCorePluginsManager()

// Register music plugin factory
coreManager.RegisterPluginFactory("music", func() plugins.CoreMediaPlugin {
    return music.NewMusicPlugin()
})

// Future: Register other core plugins
// coreManager.RegisterPluginFactory("tv", func() plugins.CoreMediaPlugin {
//     return tv.NewTVPlugin()
// })

// Initialize all plugins
if err := coreManager.InitializeCorePlugins(); err != nil {
    log.Fatal(err)
}
```

### Creating New Core Plugins

To add support for a new media type (e.g., TV shows):

1. Create `backend/internal/plugins/tv/` directory
2. Implement `CoreMediaPlugin` interface in `plugin.go`
3. Create metadata extraction logic in supporting files
4. Register the plugin factory in the scanner

### Metadata Types

Each plugin works with its own metadata type:

- Music: `*database.MusicMetadata`
- Video: `*database.VideoMetadata` (future)
- Images: `*database.ImageMetadata` (future)

The batch processor handles type-specific database operations based on the `metadataType` field.

## Benefits

1. **Extensible**: Easy to add new media types without modifying scanner core
2. **Type-Safe**: Each plugin handles its own metadata type properly
3. **Generic**: Scanner is completely media-type agnostic
4. **Maintainable**: Media-specific logic is contained within plugins
5. **Future-Proof**: Ready for TV shows, movies, podcasts, etc.

## Migration Notes

- All hardcoded music file extensions removed from scanner
- File discovery now uses plugin-provided extensions
- Metadata processing is completely generic
- Plugin hooks support any metadata type
- BatchProcessor handles multiple metadata types safely

## Future Additions

When adding TV/Movie plugins:

1. Create the plugin directory structure
2. Implement the `CoreMediaPlugin` interface
3. Define the appropriate metadata struct (e.g., `TVMetadata`)
4. Add the case in `BatchProcessor.flushInternal()`
5. Register the plugin factory

The scanner core requires no changes for new media types.

# Viewra Transcoding SDK

This package provides a minimal, lightweight SDK for building transcoding plugins for Viewra.

## Files

- **`types.go`** - Essential interfaces and types that plugins need to implement
- **`transcoder.go`** - Base `Transcoder` struct that plugins can embed  
- **`wrapper.go`** - `TranscoderWrapper` for more advanced plugin implementations

## Usage

### Basic Plugin

Implement the `TranscodingProvider` interface:

```go
package main

import (
    "context"
    "github.com/mantonx/viewra/sdk/transcoding"
)

type MyTranscodingPlugin struct {
    *transcoding.Transcoder
}

func (p *MyTranscodingPlugin) StartTranscode(ctx context.Context, req transcoding.TranscodeRequest) (*transcoding.TranscodeHandle, error) {
    // Your transcoding implementation
}

func main() {
    plugin := &MyTranscodingPlugin{
        Transcoder: transcoding.NewTranscoder("my-plugin", "My Transcoding Plugin", "1.0.0", "Me", 100),
    }
    // Register plugin with Viewra...
}
```

### Advanced Plugin

Use the `TranscoderWrapper` for more control:

```go
type AdvancedPlugin struct {
    *transcoding.TranscoderWrapper
    // Additional fields
}

func main() {
    plugin := &AdvancedPlugin{
        TranscoderWrapper: transcoding.NewTranscoderWrapper("advanced-plugin", "Advanced Plugin", "1.0.0", "Me", 100),
    }
    // Register plugin with Viewra...
}
```

## Architecture

The SDK is intentionally minimal - it only provides interfaces and types. The actual transcoding implementation is handled by the Viewra transcoding module, ensuring:

- **Lightweight plugins** - No heavy dependencies
- **Clean separation** - Business logic stays in Viewra core
- **Easy development** - Simple interfaces to implement
- **Performance** - Minimal overhead

## Key Interface

The main interface to implement is `TranscodingProvider`:

```go
type TranscodingProvider interface {
    GetInfo() ProviderInfo
    GetSupportedFormats() []ContainerFormat
    StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error)
    GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error)
    StopTranscode(handle *TranscodeHandle) error
    // ... other methods
}
```
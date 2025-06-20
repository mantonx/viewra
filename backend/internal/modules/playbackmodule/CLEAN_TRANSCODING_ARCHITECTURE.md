# Clean Transcoding Architecture

## Overview

This document describes the new, clean transcoding architecture that supports multiple transcoding providers without any legacy baggage or backwards compatibility concerns.

## Key Design Principles

1. **No Tech Debt**: Complete clean slate with no legacy code
2. **Provider Agnostic**: Works with any transcoding backend (FFmpeg, NVENC, VAAPI, QSV, etc.)
3. **Centralized Management**: Single place for sessions, cleanup, and configuration
4. **Environment-Driven**: All paths and settings configurable via environment variables
5. **Dashboard Integration**: Built-in support for admin panels and monitoring

## Architecture Components

### 1. Core Infrastructure (`core/`)

#### Session Store (`session_store.go`)
- Single database table for ALL transcoding sessions
- Provider-agnostic session management
- Unified progress tracking

#### File Manager (`file_manager.go`)
- Centralized file operations
- Standardized directory structure: `[container]_[provider]_[sessionid]`
- Manifest and segment management

#### Cleanup Service (`cleanup_service.go`)
- Runs every 30 seconds (configurable)
- Multi-tier retention policy
- Emergency cleanup when disk usage exceeds limits
- Orphaned directory detection

#### Provider Manager (`provider_manager.go`)
- Registers and manages transcoding providers
- Intelligent provider selection based on:
  - Hardware availability
  - Current load
  - Provider priority
  - Format support

#### Transcode Service (`transcode_service.go`)
- Main orchestrator for all operations
- Integrates all components
- Provides dashboard data

### 2. Clean Provider Interface (`provider.go`)

```go
type TranscodingProvider interface {
    GetInfo() ProviderInfo
    GetSupportedFormats() []ContainerFormat
    GetHardwareAccelerators() []HardwareAccelerator
    GetQualityPresets() []QualityPreset
    StartTranscode(ctx context.Context, req TranscodeRequest) (*TranscodeHandle, error)
    GetProgress(handle *TranscodeHandle) (*TranscodingProgress, error)
    StopTranscode(handle *TranscodeHandle) error
    GetDashboardSections() []DashboardSection
    GetDashboardData(sectionID string) (interface{}, error)
    ExecuteDashboardAction(actionID string, params map[string]interface{}) error
}
```

### 3. Configuration

All paths and settings are configurable via environment variables:

```bash
# Paths
VIEWRA_TRANSCODING_DIR=/viewra-data/transcoding
VIEWRA_TEMP_DIR=/tmp/viewra

# Limits
VIEWRA_MAX_SESSIONS=10
VIEWRA_MAX_DISK_GB=50

# Cleanup
VIEWRA_CLEANUP_INTERVAL=30s
VIEWRA_RETENTION_HOURS=24
VIEWRA_EXTENDED_RETENTION_HOURS=48
VIEWRA_LARGE_FILE_MB=500
```

## Database Schema

Single unified table for all providers:

```sql
CREATE TABLE transcode_sessions (
    id VARCHAR(128) PRIMARY KEY,
    provider VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    request JSONB,
    progress JSONB,
    result JSONB,
    hardware JSONB,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    last_accessed TIMESTAMP WITH TIME ZONE NOT NULL,
    directory_path VARCHAR(512)
);
```

## Adding a New Provider

1. Implement the `TranscodingProvider` interface
2. Register with the transcode service
3. That's it!

Example:

```go
type MyProvider struct {
    // provider implementation
}

func (p *MyProvider) GetInfo() ProviderInfo {
    return ProviderInfo{
        ID:          "my_provider",
        Name:        "My Transcoding Provider",
        Description: "Custom transcoding implementation",
        Priority:    100,
    }
}

// ... implement other methods

// Register
transcodeService.RegisterProvider(myProvider)
```

## Dashboard Integration

Each provider can expose dashboard sections:

```go
func (p *MyProvider) GetDashboardSections() []DashboardSection {
    return []DashboardSection{{
        ID:          "my_provider_stats",
        Type:        "transcoding",
        Title:       "My Provider Statistics",
        Description: "Real-time transcoding metrics",
        Priority:    100,
    }}
}

func (p *MyProvider) GetDashboardData(sectionID string) (interface{}, error) {
    // Return provider-specific dashboard data
}
```

## Benefits

1. **Clean Architecture**: No legacy code or backwards compatibility
2. **Provider Flexibility**: Easy to add new transcoding backends
3. **Resource Efficiency**: Centralized cleanup prevents disk waste
4. **Monitoring**: Built-in dashboard support
5. **Configuration**: Everything configurable via environment
6. **Scalability**: Provider selection based on load and capabilities

## Migration from Old System

Since we're not maintaining backwards compatibility:

1. Stop all existing transcoding sessions
2. Drop old tables: `ffmpeg_sessions`, `plugin_transcode_sessions`, `direct_sessions`
3. Deploy new system
4. Update plugins to use new `TranscodingProvider` interface

## Future Enhancements

- Provider health monitoring
- Advanced load balancing algorithms
- Quality-based provider selection
- Cost-based optimization for cloud providers
- Real-time performance metrics 
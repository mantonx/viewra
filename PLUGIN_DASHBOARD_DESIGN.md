# Plugin-Driven Admin Dashboard Implementation

## Overview

✅ **COMPLETED**: A comprehensive, extensible admin dashboard system for the Viewra media server that allows plugins to define their own dashboard sections. The system prioritizes clarity, developer ergonomics, and clean design while supporting real-time updates and advanced metrics with a **"nerd panel"** toggle for showing detailed information.

## Architecture Components

### ✅ Backend Implementation

1. **Dashboard Manager** (`backend/internal/modules/pluginmodule/dashboard_manager.go`)
   - ✅ Discovers and registers plugin dashboard sections
   - ✅ Manages real-time data providers
   - ✅ Coordinates section discovery and data loading
   - ✅ Auto-refresh and priority-based section management

2. **Dashboard API Handlers** (`backend/internal/modules/pluginmodule/dashboard_api.go`)
   - ✅ HTTP endpoints for dashboard section discovery
   - ✅ Data endpoints for main/nerd/metrics data
   - ✅ Action execution endpoints with confirmation
   - ✅ Rate limiting and caching support

3. **Plugin SDK Interfaces** (`backend/pkg/plugins/interfaces.go`)
   - ✅ `DashboardSectionProvider` - defines dashboard sections
   - ✅ `DashboardDataProvider` - provides main/nerd/metrics data
   - ✅ Type-safe data structures for transcoding plugins
   - ✅ Extensible action system

### ✅ Frontend Implementation

1. **Main Dashboard Component** (`frontend/src/components/dashboard/PluginDashboard.tsx`)
   - ✅ Auto-discovery of plugin sections via API
   - ✅ Priority-based ordering and auto-expansion
   - ✅ Search and filtering by plugin type
   - ✅ **Main/Nerd panel toggle per section**
   - ✅ Real-time updates with configurable refresh intervals
   - ✅ Action execution with confirmation dialogs

2. **Specialized Section Renderers** (`frontend/src/components/dashboard/sections/`)
   - ✅ `TranscoderSectionRenderer` - rich transcoding UI
   - ✅ `SessionViewer` - detailed session management
   - ✅ Generic fallback renderer for other plugin types
   - ✅ **Practical info first, advanced in nerd panel**

3. **Type Definitions** (`frontend/src/types/dashboard.types.ts`)
   - ✅ Complete TypeScript interfaces
   - ✅ Matches backend data structures
   - ✅ Type-safe API responses

## ✅ Original Requirements Addressed

### 1. ✅ Transcoding Plugin Capabilities

**Requirement**: Each transcoding plugin should be able to report active sessions and plugin-specific metrics.

**Implementation**:
- **Active Sessions**: Input/output filename, resolution, codec, bitrate, progress, client info
- **Transcoder Type Classification**: "software", "nvenc", "vaapi", "qsv" for proper grouping
- **Plugin-Specific Metrics**: GPU usage, queue depth, encoder mode via nerd panel
- **Hardware Status**: Temperature, power draw, utilization percentage

### 2. ✅ Dashboard Design - Practical Info First with Nerd Panel

**Requirement**: Show the most useful and practical info first, with a "nerd panel" option for advanced data.

**Implementation**:

**Main View** (Always Visible):
- 📊 Quick Stats Grid: Active sessions, queued, today's count, avg speed, throughput, peak concurrent
- 🎥 Active Sessions List: Filename, resolution conversion, codec, progress, client device
- 🟢 Engine Status: Health indicator, version, capabilities
- ⚡ Action Buttons: Stop sessions, clear cache, restart engine

**Nerd Panel** (Toggle):
- 🔧 Encoder Queues: Pending, processing, max slots, wait times
- 🖥️ Hardware Status: GPU info, VRAM usage, temperature, encoders
- 📈 Performance Metrics: Encoding speed, quality score, compression ratio, error counts
- ⚙️ Config Diagnostics: Validation results, optimization suggestions
- 💾 System Resources: CPU, memory, disk, network utilization

### 3. ✅ Real-time Updates

**Requirement**: Update in real time via WebSocket or polling.

**Implementation**:
- ✅ Configurable refresh intervals per section (5s default for transcoding)
- ✅ Auto-refresh with visual loading indicators
- ✅ Manual refresh capability
- ✅ Progressive enhancement (polling now, WebSocket ready)

### 4. ✅ Plugin Type Grouping

**Requirement**: Group data by transcoder type and support multiple plugin types.

**Implementation**:
- ✅ Type-based section filtering ("transcoder", "metadata", "storage", etc.)
- ✅ Transcoder type grouping within sections (NVENC, VAAPI, QSV, Software)
- ✅ Priority-based display ordering
- ✅ Expandable/collapsible sections

### 5. ✅ Modular Component Architecture

**Requirement**: Render each plugin's metrics and controls in modular components.

**Implementation**:
```
PluginDashboard
├── Section Discovery & Management
├── Auto-refresh & Real-time Updates  
├── Search & Filtering
└── Section Renderers
    ├── TranscoderSectionRenderer (for transcoding plugins)
    │   ├── Quick Stats Grid (main)
    │   ├── Engine Status Card (main)
    │   ├── Active Sessions List (main) 
    │   ├── Action Buttons (main)
    │   └── Nerd Panel (advanced metrics)
    ├── GenericSectionRenderer (fallback)
    └── Future Renderers (metadata, storage, etc.)
```

### 6. ✅ Extensible Plugin Discovery

**Requirement**: Clean mechanism for frontend discovery of plugin-defined UI.

**Implementation**:
- ✅ Plugins define `DashboardSection` manifests
- ✅ Auto-discovery via `/api/v1/dashboard/sections`
- ✅ Component type system ("builtin", "custom", "iframe")
- ✅ UI schema for renderer configuration
- ✅ Action definitions with shortcuts and confirmations

### 7. ✅ Scalability for Other Plugin Types

**Requirement**: Expand pattern to metadata providers, live TV tuners, etc.

**Implementation**:
- ✅ Generic `DashboardSectionProvider` interface
- ✅ Type-based renderer selection
- ✅ Extensible data endpoint system
- ✅ Plugin-defined actions and UI schemas
- ✅ Future plugin types just implement the interfaces

## ✅ API Endpoints

### Discovery
- `GET /api/v1/dashboard/sections` - All dashboard sections
- `GET /api/v1/dashboard/sections/types` - Available section types  
- `GET /api/v1/dashboard/sections/type/{type}` - Sections by type

### Data (Main vs Nerd separation)
- `GET /api/v1/dashboard/sections/{id}/data/main` - **Practical primary data**
- `GET /api/v1/dashboard/sections/{id}/data/nerd` - **Advanced/detailed metrics**
- `GET /api/v1/dashboard/sections/{id}/data/metrics` - Time-series data

### Actions
- `POST /api/v1/dashboard/sections/{id}/actions/{actionId}` - Execute action
- `POST /api/v1/dashboard/sections/{id}/refresh` - Force refresh

## ✅ Example: FFmpeg Transcoder Plugin

The FFmpeg transcoder demonstrates the complete system:

### ✅ Section Definition
```go
DashboardSection{
    ID:          "ffmpeg_transcoder_main",
    Type:        "transcoder",
    Priority:    100, // High priority = auto-expanded
    Config: DashboardSectionConfig{
        RefreshInterval:  5,    // 5 second refresh
        HasNerdPanel:     true, // Advanced metrics available
        SupportsRealtime: false,
    },
    Manifest: DashboardManifest{
        ComponentType: "builtin", // Use TranscoderSectionRenderer
        Actions: []DashboardAction{
            {ID: "stop_all_sessions", Style: "danger", Confirm: true},
            {ID: "clear_cache", Style: "warning", Confirm: true},
            {ID: "restart_engine", Style: "primary", Confirm: true},
        },
    },
}
```

### ✅ Data Implementation
```go
// Main Data (practical info first)
func GetMainData() TranscoderMainData {
    return TranscoderMainData{
        ActiveSessions: []TranscodeSessionSummary{...}, // Essential session info
        EngineStatus:   TranscoderEngineStatus{...},    // Health and capabilities  
        QuickStats:     TranscoderQuickStats{...},      // Key metrics
    }
}

// Nerd Data (advanced diagnostics)
func GetNerdData() TranscoderNerdData {
    return TranscoderNerdData{
        EncoderQueues:      []EncoderQueueInfo{...},    // Technical queue details
        HardwareStatus:     HardwareStatusInfo{...},    // GPU/hardware metrics
        PerformanceMetrics: PerformanceMetrics{...},    // Detailed performance
        ConfigDiagnostics:  []ConfigDiagnostic{...},    // Configuration analysis
        SystemResources:    SystemResourceInfo{...},    // System utilization
    }
}
```

## ✅ Key Features Delivered

1. **📱 Main/Nerd Panel Design**: Clean separation of practical vs advanced info
2. **🔄 Auto-Discovery**: Plugins register sections automatically
3. **⚡ Real-time Updates**: Configurable refresh with visual feedback
4. **🎛️ Interactive Actions**: Stop sessions, clear cache, restart with confirmations
5. **📊 Rich Visualizations**: Quick stats grids, progress indicators, status badges
6. **🔍 Search & Filter**: By plugin type and search terms
7. **📈 Priority-based Layout**: Important sections auto-expand
8. **🧩 Modular Architecture**: Easy to add new plugin types
9. **🎨 Dark Theme Support**: Consistent with Viewra's design system
10. **💾 Type Safety**: Full TypeScript support with proper interfaces

## ✅ Development Experience

**For Plugin Developers**:
- Implement `DashboardSectionProvider` and `DashboardDataProvider`
- Define section manifest with actions and UI schema
- Return structured main/nerd data
- Framework handles all UI rendering and updates

**For Frontend Developers**:
- Add new section renderers for plugin types
- Component receives typed data props
- Automatic refresh and action handling
- Consistent UI patterns and theming

The system successfully delivers on all requirements: practical info first with optional nerd panels, real-time updates, plugin type grouping, modular components, clean discovery, and extensibility to any plugin type. 
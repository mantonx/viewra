# Video Player Audit - Frontend Codebase

## Overview
This document provides a comprehensive audit of all video player implementations in the Viewra frontend codebase, distinguishing between legacy and new implementations.

## Video Player Implementations

### 1. New MediaPlayer Component (Current/Recommended)
**Location**: `/frontend/src/components/MediaPlayer/`
**Status**: ✅ Active - This is the new, recommended implementation

#### Key Files:
- `MediaPlayer.tsx` - Main component
- `components/VideoElement/` - Video element wrapper
- `components/VideoControls/` - Control UI components
- `components/StatusOverlay/` - Status/error overlays
- `components/MediaInfoOverlay/` - Media information display

#### Features:
- Modern React architecture with hooks and Jotai for state management
- Modular design with separated concerns
- Built-in seek-ahead support for transcoding
- Session management for transcoding cleanup
- Keyboard shortcuts
- Fullscreen management
- Position saving/resuming
- Buffering indicators
- Debug mode support

#### Routes Using MediaPlayer:
- `/player/episode/:episodeId` → `EpisodePlayer.tsx`
- `/player/movie/:movieId` → `MoviePlayer.tsx`

### 2. Legacy VideoPlayer Component
**Location**: `/frontend/src/components/tv/VideoPlayer.tsx`
**Status**: ⚠️ Legacy - Should be migrated to MediaPlayer

#### Characteristics:
- Monolithic component (1000+ lines)
- Uses Shaka Player directly
- Inline state management with useState
- Manual session management
- Complex seek-ahead implementation embedded in component
- Direct fetch calls for API communication

#### Routes Using Legacy VideoPlayer:
- `/watch/episode/:episodeId` → Direct VideoPlayer component

#### Components/Pages Linking to Legacy Routes:
- `TVShowDetail.tsx` - Uses `/watch/episode/` route in `handlePlayEpisode()`
- `VideoPlayerTest.tsx` - Links to `/watch/episode/` route

### 3. Experimental Shaka Player Components
**Location**: `/frontend/src/components/VideoPlayer/`
**Status**: 🔬 Experimental/Unused

#### Files:
- `ShakaPlayerOptimized.tsx` - Optimized Shaka player wrapper
- `VideoPlayerExample.tsx` - Example implementation using ShakaPlayerOptimized

These components appear to be experimental and are not actively used in any routes.

### 4. Test Page
**Location**: `/frontend/src/pages/VideoPlayerTest.tsx`
**Status**: 🧪 Test/Debug page

- Test page for video playback functionality
- Currently links to legacy `/watch/episode/` routes
- Accessible via `/video-test` route

## Migration Requirements

### Components That Need Updates:

1. **TVShowDetail.tsx**
   - Current: `navigate('/watch/episode/${episodeId}')`
   - Should be: `navigate('/player/episode/${episodeId}')`

2. **VideoPlayerTest.tsx**
   - Current: Links to `/watch/episode/${episode.id}`
   - Should be: Links to `/player/episode/${episode.id}`

3. **Header.tsx**
   - Consider updating "Video Test" link to use new player routes

### Routes to Deprecate:
- `/watch/episode/:episodeId` - Replace with `/player/episode/:episodeId`

## Architecture Comparison

### New MediaPlayer Architecture:
```
MediaPlayer/
├── MediaPlayer.tsx (Main container)
├── components/
│   ├── VideoElement/ (Video element abstraction)
│   ├── VideoControls/ (UI controls)
│   ├── StatusOverlay/ (Status display)
│   └── MediaInfoOverlay/ (Media info)
├── hooks/ (Shared hooks)
│   ├── useMediaPlayer
│   ├── useVideoControls
│   ├── useSessionManager
│   └── useSeekAhead
└── atoms/ (Jotai state atoms)
```

### Legacy VideoPlayer:
- Single file with all logic embedded
- Direct DOM manipulation
- Inline event handlers
- Manual state management

## Recommendations

1. **Immediate Actions**:
   - Update `TVShowDetail.tsx` to use new player routes
   - Update `VideoPlayerTest.tsx` to use new player routes
   - Mark legacy VideoPlayer component as deprecated

2. **Short Term**:
   - Remove legacy `/watch/episode/` route from App.tsx
   - Delete legacy VideoPlayer component
   - Update any documentation referencing old routes

3. **Long Term**:
   - Consider if experimental Shaka components have value or should be removed
   - Ensure all new video playback features use MediaPlayer component
   - Add movie playback UI that links to `/player/movie/` routes

## State Management Comparison

### New MediaPlayer:
- Uses Jotai atoms for global state
- Modular state management
- Clean separation of concerns

### Legacy VideoPlayer:
- Local component state with useState
- Complex state interactions
- Difficult to share state between components

## Session Management

### New MediaPlayer:
- Centralized session management via `useSessionManager` hook
- Automatic cleanup on unmount
- UUID-based session tracking

### Legacy VideoPlayer:
- Manual session tracking with refs and state
- Complex cleanup logic
- Inline session management code

## Conclusion

The codebase currently has both legacy and new video player implementations running in parallel. The new MediaPlayer component is the recommended approach going forward, offering better architecture, maintainability, and features. Migration from the legacy VideoPlayer to MediaPlayer should be prioritized to maintain consistency and reduce technical debt.
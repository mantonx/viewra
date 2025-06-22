# Migration Guide: VideoPlayer to MediaPlayer

This guide helps you migrate from the old monolithic `VideoPlayer` component to the new modular `MediaPlayer` architecture.

## Quick Start

### Old Usage (VideoPlayer)
```tsx
// Direct route to VideoPlayer
<Route path="/watch/episode/:episodeId" element={<VideoPlayer />} />
```

### New Usage (MediaPlayer)
```tsx
// Episode player
<Route path="/player/episode/:episodeId" element={<EpisodePlayer />} />

// Movie player  
<Route path="/player/movie/:movieId" element={<MoviePlayer />} />

// Or use MediaPlayer directly
<MediaPlayer mediaType="episode" autoplay={true} />
<MediaPlayer mediaType="movie" autoplay={false} />
```

## Key Differences

### 1. Architecture
- **Old**: Single 800+ line component with mixed concerns
- **New**: Modular architecture with separate components, hooks, and utilities

### 2. State Management
- **Old**: Local React state with refs
- **New**: Jotai atoms for global state management

### 3. Media Type Support
- **Old**: TV episodes only
- **New**: Episodes and movies (extensible for more types)

### 4. Theming
- **Old**: Hardcoded colors
- **New**: Design tokens with dark/light mode support

## Component Mapping

| Old Component | New Component | Location |
|--------------|---------------|----------|
| VideoPlayer | MediaPlayer | `/components/MediaPlayer` |
| (inline controls) | VideoControls | `/components/MediaPlayer/components/VideoControls` |
| (inline progress) | ProgressBar | `/components/MediaPlayer/components/ProgressBar` |
| (inline volume) | VolumeControl | `/components/MediaPlayer/components/VolumeControl` |
| (inline status) | StatusOverlay | `/components/MediaPlayer/components/StatusOverlay` |

## Hook Mapping

| Old Logic | New Hook | Purpose |
|-----------|----------|---------|
| Player init | useMediaPlayer | Shaka player management |
| Control functions | useVideoControls | Play, pause, seek, volume |
| Session cleanup | useSessionManager | Transcoding session lifecycle |
| Seek-ahead logic | useSeekAhead | Seek-ahead transcoding |
| Navigation | useMediaNavigation | Media loading and routing |
| Keyboard events | useKeyboardShortcuts | Keyboard controls |
| Position saving | usePositionSaving | Resume playback |

## API Changes

### Props
```tsx
// Old
<VideoPlayer /> // No props, uses route params

// New
<MediaPlayer
  mediaType="episode" | "movie"
  className?: string
  autoplay?: boolean
  onBack?: () => void
/>
```

### Route Parameters
```tsx
// Old routes
/watch/episode/:episodeId

// New routes
/player/episode/:episodeId
/player/movie/:movieId
```

### URL Query Parameters
Both old and new support:
- `?t=123` - Start time in seconds
- `?autoplay=false` - Disable autoplay
- `?debug=true` - Show debug info

## Updating Your Code

### 1. Update Routes
```tsx
// Before
<Route path="/watch/episode/:episodeId" element={<VideoPlayer />} />

// After
<Route path="/player/episode/:episodeId" element={<EpisodePlayer />} />
<Route path="/player/movie/:movieId" element={<MoviePlayer />} />
```

### 2. Update Links
```tsx
// Before
<Link to={`/watch/episode/${episodeId}`}>Watch</Link>

// After
<Link to={`/player/episode/${episodeId}`}>Watch</Link>
```

### 3. Add Theme Provider
```tsx
// In App.tsx
import { ThemeProvider } from './providers/ThemeProvider';

<ThemeProvider defaultTheme="system">
  <App />
</ThemeProvider>
```

### 4. Import Design Tokens
```tsx
// In main.tsx
import './styles/tokens.css';
```

## Customization

### Custom Back Navigation
```tsx
<MediaPlayer
  mediaType="episode"
  onBack={() => {
    // Custom navigation logic
    navigate('/my-custom-route');
  }}
/>
```

### Disable Features
```tsx
<VideoControls
  showStopButton={false}
  showSkipButtons={false}
  showVolumeControl={false}
  showFullscreenButton={false}
/>
```

### Custom Styling
```tsx
// Use design tokens instead of hardcoded colors
className="bg-player-bg text-player-text"

// Instead of
className="bg-black text-white"
```

## Testing

### Unit Tests
- Test individual hooks with React Testing Library
- Mock Jotai atoms for isolated testing
- Test components with mock providers

### Integration Tests
- Test MediaPlayer with real media files
- Verify session management
- Test seek-ahead functionality

## Deprecation Timeline

1. **Phase 1** (Current): Both players available
   - New routes use MediaPlayer
   - Old routes still work with VideoPlayer

2. **Phase 2** (Next Release): Migration warnings
   - Console warnings for VideoPlayer usage
   - Migration guide in UI

3. **Phase 3** (Future): Remove VideoPlayer
   - Delete old component
   - Redirect old routes to new

## Common Issues

### Player Not Loading
- Ensure `ThemeProvider` is wrapping your app
- Import `tokens.css` in main.tsx
- Check media file permissions

### Styling Issues
- Update hardcoded colors to design tokens
- Check dark mode class application
- Verify Tailwind config includes token functions

### Session Cleanup
- New player validates session IDs
- Old sessions are ignored
- Check browser console for cleanup logs

## Need Help?

- Check component examples in Storybook
- Review hook documentation in `/hooks/README.md`
- See design token guide in `/styles/tokens.css`
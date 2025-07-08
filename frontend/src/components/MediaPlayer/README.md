# MediaPlayer Component

A comprehensive media playback component using Vidstack for video playback with support for direct play, remux, and transcode operations.

## Features

- **Vidstack Integration**: Leverages Vidstack's powerful player capabilities
- **Playback Methods**: Supports direct play, remux, and transcode
- **Progressive Download**: Start playback before transcoding completes
- **Custom Controls**: Beautiful, responsive video controls
- **Analytics**: Comprehensive playback tracking
- **Session Management**: Automatic cleanup of transcoding sessions
- **Keyboard Shortcuts**: Full keyboard navigation support
- **Touch Gestures**: Mobile-friendly touch controls

## Usage

```tsx
import { MediaPlayer } from '@/components/MediaPlayer';

// For episodes
<MediaPlayer 
  type="episode"
  tvShowId={123}
  seasonNumber={1}
  episodeNumber={1}
  autoplay={true}
  onBack={() => navigate('/shows')}
/>

// For movies
<MediaPlayer 
  type="movie"
  
  movieId={456}
  autoplay={false}
/>
```

## Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| type | `'movie' \| 'episode'` | required | Type of media |
| movieId | `number` | - | Required for movies |
| tvShowId | `number` | - | Required for episodes |
| seasonNumber | `number` | - | Required for episodes |
| episodeNumber | `number` | - | Required for episodes |
| autoplay | `boolean` | `true` | Auto-start playback |
| onBack | `() => void` | - | Custom back navigation |
| className | `string` | - | Additional CSS classes |

## Playback Decision Flow

1. **Media Analysis**: Analyzes file format, codecs, and client capabilities
2. **Decision Making**: 
   - **Direct Play**: If format is natively supported (MP4 H.264/AAC)
   - **Remux**: If only container needs changing (MKV → MP4)
   - **Transcode**: If codecs need conversion (HEVC → H.264)
3. **Session Management**: Creates and manages transcoding sessions
4. **Progressive Playback**: Starts playback as soon as enough data is available

## Components Structure

```
MediaPlayer/
├── MediaPlayer.tsx          # Main component
├── VidstackControls.tsx    # Vidstack hook integration
├── components/
│   ├── VideoControls/      # Playback controls
│   ├── ProgressBar/        # Seek bar with buffering
│   ├── VolumeControl/      # Volume slider
│   ├── StatusOverlay/      # Loading/error states
│   └── MediaInfoOverlay/   # Title and metadata
└── types.ts                # TypeScript definitions
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Space | Play/Pause |
| ← → | Seek backward/forward 10s |
| ↑ ↓ | Volume up/down |
| M | Toggle mute |
| F | Toggle fullscreen |
| Home | Seek to beginning |
| End | Seek to end |
| 0-9 | Seek to percentage |

## Touch Gestures

- **Single Tap**: Show/hide controls
- **Double Tap Left**: Skip backward 10s
- **Double Tap Right**: Skip forward 10s
- **Double Tap Center**: Toggle fullscreen
- **Swipe**: Seek through video

## State Management

The MediaPlayer uses Jotai atoms for state management:

```tsx
// Player state
playerStateAtom         // Playing, duration, volume, etc.
loadingStateAtom       // Loading and error states
currentMediaAtom       // Current media metadata

// Playback state  
playbackDecisionAtom   // Direct/remux/transcode decision
activeSessionsAtom     // Active transcoding sessions

// UI state
configAtom            // Player configuration
```

## Analytics Events

The player tracks these events:
- `play`, `pause`, `ended`
- `seek` with position
- `buffer_start`, `buffer_end`
- `quality_change` (when implemented)
- `error` with details
- `session_start`, `session_end`

## Error Handling

Errors are displayed with user-friendly messages:
- Network errors → "Connection lost"
- Codec errors → "Format not supported"
- Server errors → "Playback unavailable"

All errors include a retry option.

## Styling

The player uses CSS variables for theming:

```css
--player-accent-500: Primary accent color
--player-surface-overlay: Control background
--player-transition-fast: Animation speed
```

## Performance

- **Lazy Loading**: Components load on demand
- **Debounced Updates**: Prevents excessive re-renders
- **Efficient Seeking**: Uses HTTP range requests
- **Memory Management**: Cleans up resources on unmount

## Browser Support

- Chrome/Edge: Full support
- Firefox: Full support
- Safari: Full support (uses native HLS when needed)
- Mobile: iOS Safari, Chrome, Firefox

## Examples

### Basic Usage

```tsx
// Movie player with custom styling
<MediaPlayer
  type="movie"
  movieId={123}
  className="rounded-lg overflow-hidden"
/>
```

### Episode with Navigation

```tsx
// Episode player with custom back button
<MediaPlayer
  type="episode"
  tvShowId={1}
  seasonNumber={1}
  episodeNumber={5}
  onBack={() => navigate(`/shows/${showId}`)}
/>
```

### Programmatic Control

```tsx
// Access player instance via ref
const playerRef = useRef<MediaPlayerInstance>(null);

// Control playback
playerRef.current?.play();
playerRef.current?.pause();
playerRef.current?.seek(120); // Seek to 2 minutes
```

## Troubleshooting

### Video Won't Play
1. Check browser console for errors
2. Verify file format is supported
3. Check network connectivity
4. Ensure backend is running

### Transcoding Issues
1. Check FFmpeg is installed
2. Verify resource limits aren't exceeded
3. Check available disk space
4. Review backend logs

### Performance Issues
1. Check network bandwidth
2. Verify transcoding settings
3. Monitor concurrent sessions
4. Check browser hardware acceleration
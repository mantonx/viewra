# MusicPlayer Component

A dedicated audio playback component optimized for music with Vidstack integration and support for all playback methods.

## Features

- **Music-Optimized UI**: Clean interface designed for audio playback
- **Vidstack Powered**: Consistent playback engine with MediaPlayer
- **Playlist Support**: Queue management with next/previous navigation
- **Playback Modes**: Repeat one, repeat all, and shuffle
- **Minimizable Design**: Collapsible player for multitasking
- **Progressive Streaming**: Start playback during transcoding
- **Format Support**: MP3, FLAC, WAV, AAC, and more

## Usage

```tsx
import { MusicPlayer } from '@/components/MusicPlayer';

// Basic music player
<MusicPlayer
  mediaFileId="audio-123"
  title="Song Title"
  artist="Artist Name"
  album="Album Name"
  coverUrl="/path/to/cover.jpg"
/>

// With playlist
<MusicPlayer
  mediaFileId="audio-123"
  title="Current Song"
  artist="Current Artist"
  playlist={[
    { id: 'audio-123', title: 'Song 1', artist: 'Artist 1', duration: 180 },
    { id: 'audio-124', title: 'Song 2', artist: 'Artist 2', duration: 240 },
  ]}
  onNext={() => playNext()}
  onPrevious={() => playPrevious()}
/>
```

## Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| mediaFileId | `string` | required | ID of the audio file |
| title | `string` | 'Unknown Track' | Song title |
| artist | `string` | 'Unknown Artist' | Artist name |
| album | `string` | - | Album name |
| coverUrl | `string` | - | Album cover image URL |
| playlist | `Array<PlaylistItem>` | `[]` | Playlist items |
| onNext | `() => void` | - | Next track handler |
| onPrevious | `() => void` | - | Previous track handler |
| className | `string` | - | Additional CSS classes |

## PlaylistItem Interface

```tsx
interface PlaylistItem {
  id: string;
  title: string;
  artist?: string;
  duration?: number; // in seconds
}
```

## Playback Decision Flow

Similar to MediaPlayer, MusicPlayer makes intelligent playback decisions:

1. **Direct Play**: MP3, AAC files play directly
2. **Remux**: Container conversion (e.g., M4A → MP3)
3. **Transcode**: Format conversion (e.g., FLAC → MP3)

## UI Components

### Full Mode
- Album cover display (or default music icon)
- Track information (title, artist, album)
- Progress bar with seek
- Playback controls (shuffle, previous, play/pause, next, repeat)
- Volume control
- Playlist toggle

### Minimized Mode
- Compact view with title and artist
- Basic play/pause control
- Expand/collapse button

## Playback Controls

| Control | Function |
|---------|----------|
| Play/Pause | Toggle playback |
| Previous | Skip to previous track |
| Next | Skip to next track |
| Shuffle | Toggle random playback |
| Repeat | Cycle through none → one → all |
| Volume | Adjust playback volume |
| Mute | Toggle audio mute |
| Seek | Click on progress bar |

## Repeat Modes

- **None**: Stop after playlist ends
- **One**: Repeat current track (shows "1" badge)
- **All**: Repeat entire playlist

## State Management

The component manages its own state internally:

```tsx
// Playback state
isPlaying: boolean
currentTime: number
duration: number
volume: number
isMuted: boolean

// UI state
isMinimized: boolean
showPlaylist: boolean
repeatMode: 'none' | 'one' | 'all'
isShuffled: boolean

// Session state
playbackDecision: PlaybackDecision
sessionId: string | null
```

## Analytics

Tracks the same events as MediaPlayer:
- Playback events (play, pause, ended)
- Seek operations
- Volume changes
- Repeat/shuffle mode changes
- Error events

## Styling

The player adapts to light/dark themes:

```css
.bg-white .dark:bg-gray-900     /* Background */
.text-gray-900 .dark:text-white  /* Text */
.bg-blue-500                     /* Accent color */
```

## Format Support

### Direct Play Formats
- MP3 (.mp3)
- AAC (.aac, .m4a)
- WAV (.wav)
- OGG (.ogg)

### Transcoded Formats
- FLAC (.flac) → MP3
- ALAC (.alac) → MP3
- WMA (.wma) → MP3
- APE (.ape) → MP3

## Examples

### Basic Audio Player

```tsx
<MusicPlayer
  mediaFileId="song-001"
  title="Bohemian Rhapsody"
  artist="Queen"
  album="A Night at the Opera"
/>
```

### With Album Cover

```tsx
<MusicPlayer
  mediaFileId="song-002"
  title="Imagine"
  artist="John Lennon"
  album="Imagine"
  coverUrl="https://example.com/imagine-cover.jpg"
/>
```

### Full Playlist Example

```tsx
const [currentIndex, setCurrentIndex] = useState(0);
const playlist = [...]; // Your playlist array

<MusicPlayer
  mediaFileId={playlist[currentIndex].id}
  title={playlist[currentIndex].title}
  artist={playlist[currentIndex].artist}
  playlist={playlist}
  onNext={() => {
    setCurrentIndex((currentIndex + 1) % playlist.length);
  }}
  onPrevious={() => {
    setCurrentIndex(currentIndex > 0 ? currentIndex - 1 : playlist.length - 1);
  }}
/>
```

### Embedded Player

```tsx
// Minimized by default
const [isMinimized, setIsMinimized] = useState(true);

<MusicPlayer
  mediaFileId="podcast-001"
  title="Episode 1: Introduction"
  artist="Tech Podcast"
  className="fixed bottom-4 right-4 w-96 z-50"
/>
```

## Accessibility

- Keyboard navigation support
- ARIA labels for screen readers
- Focus indicators
- Semantic HTML structure

## Performance

- Lightweight Vidstack integration
- Efficient re-renders with React hooks
- Lazy loading of album covers
- Minimal memory footprint

## Error Handling

Displays user-friendly error messages:
- "Failed to load audio" - Network/server errors
- "Format not supported" - Unsupported audio format
- "Playback error occurred" - General playback issues

## Integration Tips

### With Media Library

```tsx
const tracks = mediaLibrary.filter(item => item.type === 'audio');
<MusicPlayer
  mediaFileId={tracks[0].id}
  playlist={tracks}
  // ... other props
/>
```

### With Queue Management

```tsx
const { queue, currentTrack, next, previous } = useAudioQueue();
<MusicPlayer
  mediaFileId={currentTrack.id}
  title={currentTrack.title}
  artist={currentTrack.artist}
  playlist={queue}
  onNext={next}
  onPrevious={previous}
/>
```

## Troubleshooting

### Audio Won't Play
1. Check file format is supported
2. Verify backend transcoding service
3. Check browser audio permissions
4. Review network connectivity

### Playlist Not Working
1. Ensure onNext/onPrevious handlers are provided
2. Verify playlist item IDs are unique
3. Check playlist array is properly formatted

### UI Issues
1. Check theme classes are applied
2. Verify Tailwind CSS is loaded
3. Ensure container has proper dimensions
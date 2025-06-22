# Utils Organization

This directory contains utility functions organized by their primary purpose. Each utility has its own folder with the main file, exports, and tests.

## Directory Structure

```
utils/
├── time/                 # Time formatting and manipulation
│   ├── time.ts
│   ├── time.test.ts
│   └── index.ts
├── video/                # Video element utilities
│   ├── video.ts
│   ├── video.test.ts
│   └── index.ts
├── storage/              # localStorage management
│   ├── storage.ts
│   ├── storage.test.ts
│   └── index.ts
├── mediaValidation/      # Media validation and type guards
│   ├── mediaValidation.ts
│   ├── mediaValidation.test.ts
│   └── index.ts
├── api/                  # API and URL utilities
│   ├── api.ts
│   ├── api.test.ts
│   └── index.ts
├── colorExtractor/       # Color extraction utilities
│   ├── colorExtractor.ts
│   ├── colorExtractor.test.ts
│   └── index.ts
├── index.ts              # Main exports
└── README.md             # This file
```

## Utility Categories

### Time (`/time`)
Time formatting, parsing, and validation utilities:
- `formatTime()` - Format seconds to HH:MM:SS or MM:SS
- `formatProgress()` - Calculate progress percentage
- `parseTimeString()` - Parse time strings to seconds
- `clampTime()` - Clamp time values within bounds
- `isValidTime()` - Validate time values
- `formatTooltipTime()` - Format for UI tooltips

### Video (`/video`)  
Video element inspection and management:
- `getBufferedRanges()` - Get buffered time ranges
- `getSeekableRanges()` - Get seekable time ranges
- `detectDuration()` - Detect duration from multiple sources
- `canSeekTo()` - Check if time is seekable
- `isPlaying()` - Check if video is playing
- `getVideoQuality()` - Get video resolution info

### Storage (`/storage`)
localStorage management with error handling:
- `getSavedPosition()` / `savePosition()` - Video position persistence
- `getSavedVolume()` / `saveVolume()` - Volume persistence
- `getPlayerSettings()` / `savePlayerSettings()` - Player preferences
- `clearPlayerStorage()` - Clear all player data
- Safe storage operations with fallbacks

### Media Validation (`/mediaValidation`)
Type guards and validation for media data:
- `isValidUUID()` - UUID format validation
- `isValidSessionId()` - Session ID validation
- `isEpisode()` / `isMovie()` - Type guards for media items
- `isValidMediaItem()` - Validate media structure
- `getDisplayTitle()` - Generate display titles
- `getMediaImage()` - Get poster/backdrop images

### API (`/api`)
API utilities and URL building:
- `buildImageUrl()` - Add quality parameters to images
- `buildArtworkUrl()` - Build artwork URLs
- `buildStreamUrl()` - Build streaming URLs  
- `getPreferredAsset()` - Fetch preferred assets
- `getEntityAssets()` - Fetch entity assets
- Async URL building with fallbacks

### Color Extractor (`/colorExtractor`)
Color extraction from images:
- Extract dominant colors from artwork
- Generate color palettes
- Color utility functions

## Usage

Import utils from the main index or specific categories:

```typescript
// Import all utilities
import { formatTime, isValidTime, getSavedPosition } from '../utils';

// Import from specific categories
import { formatTime, clampTime } from '../utils/time';
import { getBufferedRanges, detectDuration } from '../utils/video';
import { getSavedPosition, savePosition } from '../utils/storage';
```

## Guidelines

### Adding New Utilities
1. Determine the primary category (time, video, storage, etc.)
2. Add the function to the appropriate category file
3. Update the category's index.ts export
4. Add tests when Jest is set up
5. Update this README if adding a new category

### Function Guidelines
- Use descriptive names: `formatTime()` not `format()`
- Include proper TypeScript types
- Handle edge cases and invalid inputs gracefully
- Add JSDoc comments for complex functions
- Return consistent types (avoid `any`)

### Error Handling
- Use try-catch for operations that might fail
- Log warnings for non-critical errors
- Provide sensible fallback values
- Validate inputs before processing

### Testing
- Test happy path scenarios
- Test edge cases and invalid inputs
- Test error handling and fallbacks
- Mock external dependencies when needed

## Testing Status

Tests are currently placeholder files. Once Jest is set up:
1. Uncomment test code in `.test.ts` files
2. Add proper test implementations
3. Run tests with `npm test`
4. Maintain test coverage for new utilities
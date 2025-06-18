# DASH/HLS Seek-Ahead Feature

## Overview

The VideoPlayer now supports **seek-ahead functionality** for DASH and HLS adaptive streaming. This allows users to seek to any point in a video file, even beyond what has been transcoded so far.

## How It Works

### Frontend Behavior

1. **Visual Indicators**: The progress bar shows three distinct regions:
   - **Red**: Current playback position
   - **Gray**: Content that has been transcoded and is available for immediate playback
   - **Blue (translucent)**: Content that hasn't been transcoded yet but supports seek-ahead

2. **Interactive Seeking**:
   - Clicking within the gray (transcoded) area: **Immediate seeking** to that position
   - Clicking within the blue (untranscoded) area: **Seek-ahead** - starts new transcoding from that point

3. **User Feedback**:
   - Hover preview shows timestamps with ⚡ icon for seek-ahead regions
   - "Seek-ahead available" indicator appears when applicable
   - Loading state during seek-ahead operation

### Backend Implementation

1. **New API Endpoint**: `POST /api/playback/seek-ahead`
   ```json
   {
     "session_id": "current-session-id",
     "seek_time": 1800  // Time in seconds
   }
   ```

2. **Transcoding Workflow**:
   - Stops current transcoding session
   - Starts new FFmpeg process with `-ss` parameter for the requested start time
   - Returns new session ID for updated manifest URLs

3. **FFmpeg Integration**:
   - Uses `-ss` parameter before input for efficient seeking
   - Maintains all original transcoding parameters
   - Creates new DASH/HLS manifests from the seek point

## User Experience

### When to Use Seek-Ahead

- **Large video files** (movies, long episodes) where transcoding takes time
- **Random access** to specific scenes without waiting for sequential transcoding
- **Resume playback** from bookmarked positions in untranscoded content

### Visual Cues

- **⚡ Lightning bolt icon**: Indicates seek-ahead capability
- **Blue highlight**: Shows untranscoded regions that support seek-ahead
- **Smooth transitions**: Loading states during seek-ahead operations

### Performance Considerations

- Seek-ahead creates a new transcoding session
- Small delay (2-5 seconds) while new transcoding starts
- Original session is terminated to avoid resource conflicts
- New manifest needs time to generate initial segments

## Technical Details

### Frontend Components

- Enhanced `handleSeek()` function with seek-ahead detection
- Updated progress bar with multi-region visualization
- Hover interactions show seek-ahead capabilities
- Session management for switching between transcoding sessions

### Backend Components

- New `handleSeekAhead()` API handler
- Enhanced `TranscodeRequest` with `StartTime` field
- FFmpeg command builder supports `-ss` parameter
- Session lifecycle management for seek-ahead operations

### Supported Formats

- **DASH**: Full seek-ahead support with manifest regeneration
- **HLS**: Full seek-ahead support with playlist regeneration
- **Progressive**: Not supported (would require server-side range requests)

## Future Enhancements

1. **Predictive Transcoding**: Start transcoding popular seek points in advance
2. **Multi-bitrate Seek-ahead**: Allow quality selection during seek-ahead
3. **Resume Previous Sessions**: Option to resume interrupted transcoding
4. **Seek-ahead Caching**: Cache seek points for faster subsequent access

## Browser Compatibility

- **Chrome/Edge**: Full DASH support with seek-ahead
- **Firefox**: Full DASH support with seek-ahead  
- **Safari**: HLS support with seek-ahead
- **Mobile browsers**: Depends on DASH/HLS support

## Error Handling

- Network failures during seek-ahead show user-friendly error messages
- Fallback to regular seeking if seek-ahead fails
- Session cleanup ensures no orphaned transcoding processes
- Timeout handling for slow seek-ahead operations

This feature significantly improves the user experience for long-form video content by eliminating the need to wait for sequential transcoding. 
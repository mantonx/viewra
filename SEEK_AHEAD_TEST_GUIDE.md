# Seek-Ahead Testing Guide

## Implementation Status
✅ **Seek-ahead functionality is now fully implemented!**

## What Was Fixed
1. **Frontend Detection**: Fixed DASH/HLS stream detection by checking manifest URL instead of transcode params
2. **Buffered Range**: Now uses actual buffered range instead of seekableDuration to detect when to trigger seek-ahead
3. **Backend Error**: Fixed nil pointer error when session doesn't have Request field stored
4. **Session Metadata**: Sessions now store input path in metadata for seek-ahead use

## How Seek-Ahead Works
1. When you seek beyond buffered content by more than 30 seconds
2. Frontend detects this is a DASH stream (by checking for `.mpd` in manifest URL)
3. Calls `/api/playback/seek-ahead` with the seek time
4. Backend starts new FFmpeg process with `-ss <seconds>` for efficient seeking
5. Returns new manifest URL for the seeked stream
6. Frontend loads the new manifest and playback continues from seeked position

## Testing Steps
1. **Start playing a video**
   - Let it buffer for 10-20 seconds
   - Look at the progress bar - you'll see:
     - Light gray: Buffered/transcoded content
     - Blue tint: Unbuffered content

2. **Trigger seek-ahead**
   - Click on the blue-tinted area (far beyond current position)
   - You should see in console:
     ```
     🚀 Seeking beyond buffered content, starting seek-ahead transcoding
     ```

3. **Watch the result**
   - "⚡ Transcoding ahead..." indicator appears
   - Brief buffering while new session starts
   - Video resumes from your seeked position

## Debug Logs to Look For
- `📊 Buffered vs seek:` - Shows buffered end vs seek position
- `🚀 Seeking beyond buffered content` - Seek-ahead triggered
- `✅ Seek-ahead transcoding started` - Backend created new session
- `🔄 Switching to new manifest URL` - Frontend loading new stream

## Current Settings
- **Trigger threshold**: 30 seconds beyond buffered content
- **FFmpeg seek**: Uses `-ss` parameter before input for fast seeking
- **Session cleanup**: Old sessions are stopped when new ones start

## Troubleshooting
If seek-ahead isn't working:
1. Check browser console for errors
2. Ensure you're seeking beyond buffered content (30+ seconds)
3. Verify FFmpeg plugin is running: `curl http://localhost:8080/api/playback/stats`
4. Check backend logs: `docker-compose logs backend --tail 50` 
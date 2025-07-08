# Streaming Migration Plan

## Overview
This document tracks the migration of streaming functionality from the media module to the playback module as part of the refactoring effort.

## Methods to Move

### From `handlers.go.backup` to playback module:

1. **StreamMediaFile** (lines 124-138)
   - Handler method that initiates streaming
   - Endpoint: GET /api/media/files/:id/stream
   - Should become part of playback module's API

2. **streamFileWithRangeSupport** (lines 337-384) 
   - Core streaming logic with range support
   - Sets appropriate headers for video streaming
   - Handles both full file and HEAD requests
   - Should be refactored to use shared HTTP utilities

3. **handleRangeRequest** (lines 386-435)
   - Handles HTTP range requests for video seeking
   - Parses byte range headers
   - Implements partial content delivery
   - Should leverage the shared utils.ParseRangeHeader

4. **getContentType** (lines 437-467)
   - Returns MIME types for containers
   - Already duplicated in shared utils as GetMediaContentType
   - Can be removed

## Implementation Steps

1. ✅ Create shared HTTP utilities in `/internal/utils/http.go`
   - ParseRangeHeader
   - GetMediaContentType  
   - ServeFileWithRange

2. ⏳ Update playback module to handle streaming
   - Add streaming endpoints to playback API
   - Implement session-based streaming
   - Support direct play, remux, and transcode streaming

3. ⏳ Remove streaming code from media module
   - Remove StreamMediaFile handler
   - Remove helper methods
   - Update routes to remove streaming endpoint

4. ⏳ Update frontend to use playback module for streaming
   - Change API endpoints from /api/media/files/:id/stream
   - To new playback module endpoints

## Benefits

- **Better separation of concerns**: Media module focuses on library management, playback handles streaming
- **Unified streaming**: All streaming (direct, remux, transcode) goes through one module
- **Session management**: Playback module can track streaming sessions
- **Resource management**: Better control over concurrent streams

## API Changes

### Current (Media Module)
```
GET /api/media/files/:id/stream
```

### New (Playback Module)
```
GET /api/playback/stream/:sessionId
POST /api/playback/sessions (create session)
DELETE /api/playback/sessions/:sessionId (cleanup)
```
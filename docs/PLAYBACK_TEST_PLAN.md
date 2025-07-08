# Playback Test Plan

This document outlines the test plan for the refactored playback system that supports direct play, remux, and transcode operations.

## Test Environment Setup

### Backend Requirements
- Ensure transcoding module is running
- Content-addressable storage directory exists
- FFmpeg is installed and accessible
- Resource manager is configured

### Frontend Requirements
- MediaPlayer component is integrated
- MusicPlayer component is integrated
- PlaybackService is configured with correct API endpoints

## Test Scenarios

### 1. Direct Play Testing

#### Test 1.1: MP4 Video Direct Play
**Objective**: Verify that MP4 files play directly without transcoding

**Steps**:
1. Upload an H.264/AAC MP4 video file
2. Navigate to the video player
3. Check browser network tab for direct file URL
4. Verify playback starts without transcoding session

**Expected Results**:
- PlaybackDecision shows `decision: 'direct'`
- No transcoding session is created
- Video plays using browser's native capabilities
- HTTP range requests work for seeking

#### Test 1.2: MP3 Audio Direct Play
**Objective**: Verify that MP3 files play directly in MusicPlayer

**Steps**:
1. Upload an MP3 audio file
2. Open the music player
3. Verify direct playback URL is used
4. Test play/pause/seek functionality

**Expected Results**:
- Audio plays without transcoding
- Seek works with HTTP range requests
- Player controls function correctly

### 2. Remux Testing

#### Test 2.1: MKV to MP4 Remux
**Objective**: Test container format conversion without re-encoding

**Steps**:
1. Upload an MKV file with H.264/AAC codecs
2. Open in video player
3. Monitor backend logs for remux operation
4. Verify playback of remuxed stream

**Expected Results**:
- PlaybackDecision shows `decision: 'remux'`
- Backend creates session with remux parameters
- FFmpeg copies codecs without re-encoding
- Playback starts quickly (remux is fast)

#### Test 2.2: FLAC to MP3 Audio Remux
**Objective**: Test audio format conversion

**Steps**:
1. Upload a FLAC audio file
2. Open in music player
3. Verify remux session creation
4. Test playback quality

**Expected Results**:
- Audio is converted to MP3 for compatibility
- Session management works correctly
- Audio quality is acceptable

### 3. Transcode Testing

#### Test 3.1: HEVC to H.264 Transcode
**Objective**: Test full video transcoding

**Steps**:
1. Upload an HEVC (H.265) video file
2. Open in video player
3. Monitor transcoding progress
4. Test playback during transcoding

**Expected Results**:
- PlaybackDecision shows `decision: 'transcode'`
- Transcoding session created with H.264 target
- Progressive download allows playback to start before completion
- Resource manager limits concurrent sessions

#### Test 3.2: High Bitrate Transcode
**Objective**: Test bitrate reduction for bandwidth efficiency

**Steps**:
1. Upload a high bitrate (>10Mbps) video
2. Configure client to request lower bitrate
3. Verify transcoding parameters
4. Test playback quality

**Expected Results**:
- Video is transcoded to requested bitrate
- Quality settings are respected
- Playback is smooth at lower bitrate

### 4. Session Management Testing

#### Test 4.1: Session Cleanup
**Objective**: Verify sessions are cleaned up properly

**Steps**:
1. Start multiple transcoding sessions
2. Close player abruptly (browser tab close)
3. Check backend for orphaned sessions
4. Verify cleanup service removes old sessions

**Expected Results**:
- Sessions are marked as orphaned
- Cleanup service removes after timeout
- Temporary files are deleted
- Database entries are updated

#### Test 4.2: Concurrent Session Limits
**Objective**: Test resource manager limits

**Steps**:
1. Configure max concurrent sessions (e.g., 3)
2. Start 4 transcoding sessions
3. Verify 4th session is queued
4. Complete one session and verify queue processing

**Expected Results**:
- Resource manager enforces limits
- Queued sessions start when resources available
- No FFmpeg process overload

### 5. Progressive Download Testing

#### Test 5.1: Seek During Transcode
**Objective**: Test seeking in partially transcoded files

**Steps**:
1. Start transcoding a large video
2. Seek to various positions
3. Verify seek behavior
4. Monitor network requests

**Expected Results**:
- Seek within transcoded portion works immediately
- Seek beyond transcoded portion shows loading
- HTTP range requests are used efficiently

#### Test 5.2: Network Interruption
**Objective**: Test resilience to network issues

**Steps**:
1. Start video playback (transcode)
2. Simulate network interruption
3. Restore network
4. Verify playback resumes

**Expected Results**:
- Player shows appropriate loading state
- Playback resumes when network returns
- No duplicate sessions created

### 6. Content-Addressable Storage Testing

#### Test 6.1: Deduplication
**Objective**: Verify identical transcodes aren't duplicated

**Steps**:
1. Transcode a video with specific settings
2. Request same video with same settings
3. Check content store for hash collision
4. Verify existing file is served

**Expected Results**:
- Content hash matches existing file
- No new transcoding occurs
- Existing file is served immediately
- Storage space is conserved

#### Test 6.2: Hash Verification
**Objective**: Ensure content integrity

**Steps**:
1. Transcode a video
2. Note the content hash
3. Verify file exists at hash path
4. Compare file contents

**Expected Results**:
- File path matches content hash
- File integrity is maintained
- No hash collisions occur

### 7. Analytics Testing

#### Test 7.1: Playback Events
**Objective**: Verify analytics tracking

**Steps**:
1. Play a video
2. Perform various actions (pause, seek, quality change)
3. Check analytics events
4. Verify device profile capture

**Expected Results**:
- All events are tracked correctly
- Device profile includes capabilities
- Session duration is accurate
- Error events are captured

### 8. Error Handling Testing

#### Test 8.1: Invalid Media File
**Objective**: Test graceful error handling

**Steps**:
1. Attempt to play corrupted video
2. Verify error message
3. Check error recovery options
4. Ensure no backend crash

**Expected Results**:
- User-friendly error message
- Option to retry or go back
- Backend logs error appropriately
- No orphaned sessions

#### Test 8.2: FFmpeg Failure
**Objective**: Test transcoding failure handling

**Steps**:
1. Simulate FFmpeg crash during transcode
2. Verify error propagation
3. Check session cleanup
4. Test retry mechanism

**Expected Results**:
- Error is detected quickly
- User is notified
- Session is marked as failed
- Retry creates new session

## Performance Benchmarks

### Metric Targets
- Direct play start time: < 1 second
- Remux start time: < 3 seconds  
- Transcode start time: < 10 seconds
- Seek response time: < 500ms
- Session cleanup time: < 5 minutes after disconnect

### Load Testing
- Support 10 concurrent direct play streams
- Support 5 concurrent remux operations
- Support 3 concurrent transcode operations
- Memory usage < 2GB under full load
- CPU usage scales with transcoding load

## Test Automation

### Unit Tests
```bash
# Backend
cd backend
make test

# Frontend
cd frontend
npm test
```

### Integration Tests
```bash
# Run integration test suite
make test-integration
```

### Manual Test Checklist
- [ ] Direct play MP4 video
- [ ] Direct play MP3 audio
- [ ] Remux MKV to MP4
- [ ] Transcode HEVC to H264
- [ ] Seek during playback
- [ ] Session cleanup on disconnect
- [ ] Content deduplication
- [ ] Error handling
- [ ] Analytics tracking
- [ ] Performance targets met

## Known Issues and Limitations

1. **Progressive Download**: Seeking beyond transcoded portion requires waiting
2. **Resource Limits**: Concurrent transcode limit may queue requests
3. **Browser Compatibility**: Some older browsers may not support all codecs
4. **Network Requirements**: Transcoding requires stable connection

## Future Enhancements

1. **Adaptive Bitrate**: Implement quality switching based on bandwidth
2. **Hardware Acceleration**: Add GPU transcoding support
3. **Subtitle Support**: Implement subtitle extraction and display
4. **Offline Playback**: Add download for offline viewing
5. **P2P Streaming**: Reduce server load with peer-to-peer
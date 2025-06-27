# Viewra Architecture Diagrams

This directory contains Mermaid diagrams that illustrate the Viewra streaming pipeline architecture and data flow.

## Viewing the Diagrams

These diagrams are written in [Mermaid](https://mermaid.js.org/) format. You can view them in:

1. **GitHub** - GitHub automatically renders Mermaid diagrams in `.mermaid` files
2. **VS Code** - Install the "Mermaid Preview" extension
3. **Online** - Copy the content to [Mermaid Live Editor](https://mermaid.live/)
4. **Local** - Use the Mermaid CLI or any Mermaid-compatible viewer

## Diagram Files

### 1. streaming-pipeline-flow.mermaid
**Main pipeline architecture showing the complete encode → package → store → serve flow**

- Input: Media file
- Stage 1: StreamEncoder (FFmpeg segmentation)
- Stage 2: StreamPackager (Shaka packaging) 
- Stage 3: ContentStore (hash-based storage)
- Stage 4: Content API (serving)
- Output: Client player
- Events: Real-time event bus communication

### 2. streaming-sequence.mermaid
**Sequence diagram showing real-time streaming workflow**

- Client request flow
- Module communication
- Encoding and packaging process
- First segment availability
- Continuous streaming
- Event-driven updates

### 3. abr-encoding-flow.mermaid
**Adaptive Bitrate (ABR) encoding workflow**

- Device profile analysis
- Multiple quality encoding (360p to 1080p)
- Segment generation for each quality
- Adaptive packaging
- Client bandwidth adaptation
- DASH/HLS manifest structure

### 4. component-architecture.mermaid
**Complete system component architecture**

- Client layer (web, mobile, TV)
- API gateway (routing, auth, rate limiting)
- Module layer (playback, transcoding, media, scanner, plugin)
- Service registry and event bus
- Core infrastructure (database, filesystem, config)
- External tools (FFmpeg, Shaka Packager)
- Plugin ecosystem

### 5. segment-buffering.mermaid
**Timeline diagram showing segment buffering and prefetching**

- Startup phase (first segment availability)
- Linear playback (steady state buffering)
- Network adaptation scenarios
- Seeking behavior
- Buffer management strategies

## Key Concepts Illustrated

### Streaming-First Architecture
- **Real-time processing**: Segments become available as they're encoded
- **Instant playback**: First segment ready in 2-4 seconds
- **Progressive delivery**: No need to wait for full file processing

### Event-Driven Communication
- **SegmentReady**: New segment available for packaging
- **ManifestUpdated**: Manifest updated with new segments
- **TranscodeCompleted**: All segments processed
- **Real-time updates**: Clients get immediate notifications

### Content-Addressable Storage
- **Deduplication**: Same content = same hash = same storage
- **Efficient storage**: No duplicate files
- **Cache-friendly**: Predictable URLs based on content

### Adaptive Bitrate (ABR)
- **Multiple qualities**: 360p to 1080p+ based on device capabilities
- **Network adaptation**: Automatic quality switching
- **Device optimization**: Quality profiles optimized per device type

### Health Monitoring
- **Real-time metrics**: Encoding speed, buffer health, error rates
- **Performance tracking**: Segment latency, network load
- **Alerting**: Proactive issue detection

## Implementation Notes

### Performance Optimizations
- **Parallel processing**: Concurrent encoding and packaging
- **Intelligent prefetching**: Predictive segment buffering
- **Fast startup**: Keyframe-aligned segments for quick seeking
- **Resource management**: CPU/memory throttling under load

### Error Handling
- **Circuit breakers**: Prevent cascade failures
- **Retry logic**: Automatic recovery from transient errors
- **Fallback strategies**: Graceful degradation
- **Health checks**: Continuous system monitoring

### Scalability Considerations
- **Horizontal scaling**: Multiple transcoding workers
- **Load balancing**: Distribute encoding load
- **Storage efficiency**: Content deduplication
- **CDN integration**: Global content distribution

## Development Workflow

1. **Request Analysis**: Playback planner determines strategy
2. **Session Creation**: New transcoding session initiated
3. **Real-time Processing**: Segments encoded and packaged as generated
4. **Progressive Delivery**: Content available immediately
5. **Health Monitoring**: Continuous performance tracking
6. **Cleanup**: Automated resource management

## Future Enhancements

- **Low-latency streaming**: Sub-second latency with CMAF
- **Cloud integration**: S3 storage, CloudFront CDN
- **Advanced codecs**: AV1, HDR support
- **Edge computing**: Distributed transcoding
- **ML optimization**: Intelligent quality adaptation

For detailed implementation information, see [STREAMING_PIPELINE.md](../STREAMING_PIPELINE.md).
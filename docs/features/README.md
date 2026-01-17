# Feature Documentation

Technical deep-dives into specific features. These documents contain implementation details, research, and technical specifications.

## When to Use Features vs ADRs vs Guides

| Type | Purpose | Example |
|------|---------|---------|
| **ADRs** (`decisions/`) | Record *why* a decision was made | "Why we chose progressive HLS over segment-based" |
| **Features** (`features/`) | Deep technical reference for *how* something works | "FFmpeg encoding options and GPU pipelines" |
| **Guides** (`guides/`) | Step-by-step *how to* use or implement | "How to build a plugin" |

## Documents

| Document | Topic |
|----------|-------|
| [HARDWARE_ACCELERATION.md](HARDWARE_ACCELERATION.md) | GPU transcoding (NVENC, QuickSync, VAAPI, VideoToolbox) |
| [TONE_MAPPING.md](TONE_MAPPING.md) | HDR to SDR conversion techniques |
| [FFMPEG_RESEARCH.md](FFMPEG_RESEARCH.md) | FFmpeg performance research and benchmarks |
| [FFMPEG_7_8_FEATURES.md](FFMPEG_7_8_FEATURES.md) | New features in FFmpeg 7.x and 8.x |
| [LIBPLACEBO_IMPLEMENTATION_SUMMARY.md](LIBPLACEBO_IMPLEMENTATION_SUMMARY.md) | libplacebo shader library integration |
| [TRANSCODE_CLEANUP.md](TRANSCODE_CLEANUP.md) | Cleanup of transcoded segments |
| [PREDICTIVE_HEALTH_DESIGN.md](PREDICTIVE_HEALTH_DESIGN.md) | System health monitoring design |

## Related

- [../decisions/](../decisions/) - Architecture Decision Records
- [../guides/](../guides/) - Implementation guides

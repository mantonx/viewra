# FFmpeg-ViewRA

Custom FFmpeg build with patches for ViewRA media server.

## Why a Custom FFmpeg?

When seeking into a video and using the segment muxer for HLS output, stock FFmpeg has an audio/video sync issue. The segment muxer calculates segment boundaries from time 0, but when you seek to position X, the first packet has PTS=X. This mismatch causes A/V drift.

### The Problem

```
Seek to 1:06:42 (4002 seconds)
First packet PTS = 4002s

Stock FFmpeg segment boundary calculation:
  end_pts = segment_time * (segment_count + 1)
  end_pts = 6 * 1 = 6s  <- WRONG! Should be relative to 4002s

Result: Audio and video drift out of sync
```

### The Solution

We apply patches (based on [jellyfin-ffmpeg](https://github.com/jellyfin/jellyfin-ffmpeg)) that fix various streaming, encoding, and HDR issues.

## Building

### Prerequisites

Install FFmpeg build dependencies:

```bash
# Arch Linux
sudo pacman -S base-devel nasm yasm x264 x265 libfdk-aac opus libvorbis libvpx libass freetype2 openssl

# Ubuntu/Debian
sudo apt install build-essential nasm yasm libx264-dev libx265-dev libfdk-aac-dev libopus-dev libvorbis-dev libvpx-dev libass-dev libfreetype6-dev libssl-dev

# Fedora
sudo dnf install gcc gcc-c++ nasm yasm x264-devel x265-devel fdk-aac-devel opus-devel libvorbis-devel libvpx-devel libass-devel freetype-devel openssl-devel
```

### Build

```bash
cd tools/ffmpeg-viewra
./build.sh
```

The built FFmpeg will be installed to `tools/ffmpeg-viewra/dist/`.

### Configuration

Set environment variables to use the custom FFmpeg:

```bash
export VIEWRA_FFMPEG_PATH=/path/to/viewra/tools/ffmpeg-viewra/dist/bin/ffmpeg
export VIEWRA_FFPROBE_PATH=/path/to/viewra/tools/ffmpeg-viewra/dist/bin/ffprobe
export VIEWRA_FFMPEG_LIB_PATH=/path/to/viewra/tools/ffmpeg-viewra/dist/lib
```

Or copy the binaries and libraries to a standard location:

```bash
# Copy to bin directory
cp tools/ffmpeg-viewra/dist/bin/ffmpeg bin/ffmpeg-viewra
cp tools/ffmpeg-viewra/dist/bin/ffprobe bin/ffprobe-viewra
mkdir -p bin/ffmpeg-lib
cp -a tools/ffmpeg-viewra/dist/lib/*.so* bin/ffmpeg-lib/

# Set environment variables
export VIEWRA_FFMPEG_PATH=/path/to/viewra/bin/ffmpeg-viewra
export VIEWRA_FFPROBE_PATH=/path/to/viewra/bin/ffprobe-viewra
export VIEWRA_FFMPEG_LIB_PATH=/path/to/viewra/bin/ffmpeg-lib
```

## Patches

### 0001-segment-muxer-track-start-pts.patch

Fixes segment muxer to track the PTS of the first packet and use it as the base for segment boundary calculations. This ensures correct A/V sync when seeking.

**Problem:** When seeking into a video, segment boundaries are calculated from time 0 instead of the seek position, causing audio/video drift.

**Source:** [jellyfin-ffmpeg 0001](https://github.com/jellyfin/jellyfin-ffmpeg/blob/jellyfin/debian/patches/0001-add-fixes-for-segment-muxer.patch)

---

### 0002-libx265-fix-api-for-x265-4.1.patch

Fixes libx265 encoder compatibility with x265 4.1+ (build 215+). The x265 API changed the `encoder_encode` function signature back to using `x265_picture*` instead of `x265_picture**`.

**Affected versions:**
- x265 build 210-214: Uses `x265_picture**` (array of layer pointers)
- x265 build 215+: Uses `x265_picture*` (single pointer)

---

### 0003-fix-safari-hls-empty-sdtp.patch

Fixes Safari HLS playback when using libx265-encoded fMP4 segments. Safari fails when encountering an empty Sample Dependency Type (SDTP) box.

**Problem:** Safari's HLS player crashes on malformed fMP4 containers with empty SDTP boxes, which can occur with HEVC content.

**Source:** [jellyfin-ffmpeg 0045](https://github.com/jellyfin/jellyfin-ffmpeg/blob/jellyfin/debian/patches/0045-fix-libx265-encoded-fmp4-hls-playback-on-safari.patch)

---

### 0004-fix-nvdec-exceed-32-surfaces.patch

Fixes NVDEC "Cannot allocate more than 32 surfaces" error when decoding high-bitrate or 4K content.

**Problem:** NVDEC has a maximum of 32 decode surfaces. Without this patch, the surface pool can exceed this limit, causing decode failures on demanding content.

**Source:** [jellyfin-ffmpeg 0016](https://github.com/jellyfin/jellyfin-ffmpeg/blob/jellyfin/debian/patches/0016-add-fixes-for-nvdec-exceed-32-surfaces-error.patch)

---

### 0005-add-hdr-metadata-for-nvenc-hevc.patch

Adds HDR metadata passthrough for NVENC HEVC encoder. When transcoding HDR content, this ensures the output contains proper HDR10 signaling.

**Adds SEI NAL units for:**

- Mastering Display Colour Volume (SMPTE ST 2086)
- Content Light Level Info (CTA-861-3)

**Problem:** Without this patch, NVENC transcodes lose HDR metadata, causing displays to treat HDR content as SDR.

**Source:** [jellyfin-ffmpeg 0035](https://github.com/jellyfin/jellyfin-ffmpeg/blob/jellyfin/debian/patches/0035-add-hdr-metadata-for-nvenc-hevc-encoder.patch)

---

### 0006-pass-dovi-sidedata-to-hls-mpegts.patch

Enables Dolby Vision metadata passthrough for HLS and MPEG-TS streaming.

**Features:**

- Validates DV configuration against codec parameters
- Copies side data (including DV config) to HLS segment streams
- Writes DOVI Video Stream Descriptor to MPEG-TS PMT
- Registers `dvhe` and `dav1` codec tags for DV HEVC and AV1

**Problem:** When remuxing Dolby Vision content to HLS/MPEG-TS, clients don't recognize the stream as DV without proper metadata.

**Source:** [jellyfin-ffmpeg 0030](https://github.com/jellyfin/jellyfin-ffmpeg/blob/jellyfin/debian/patches/0030-pass-dovi-sidedata-to-hlsenc-and-mpegtsenc.patch)

---

### 0007-segment-muxer-report-keyframe-start-offset.patch

Reports the actual start PTS in M3U8 playlists, enabling accurate seek position feedback.

**Adds options:**

- `-segment_start_pts_report 1` - Enable start PTS reporting
- `-segment_seek_time <microseconds>` - The requested seek time (for offset calculation)

**Writes to M3U8:**

- `#EXT-X-START-PTS:<milliseconds>` - Absolute start position in source file
- `#EXT-X-START-OFFSET:<seconds>` - Offset from requested seek time

**Example:** Seeking to 3530s:

```bash
ffmpeg -ss 3530 -noaccurate_seek -i input.mkv \
  -f segment -segment_start_pts_report 1 -segment_seek_time 3530000000 \
  -segment_list playlist.m3u8 ...
```

Output in playlist:

```txt
#EXT-X-START-PTS:3530167
#EXT-X-START-OFFSET:0.167
```

**Use case:** When seeking into HEVC content with long GOP intervals (8-12 seconds), the nearest keyframe may be several seconds away from the requested position. This patch allows the server to know the exact keyframe position and report it to the frontend.

**Source:** Original ViewRA patch

---

## Patch Summary

| Patch | Category | Fixes |
|-------|----------|-------|
| 0001 | HLS Streaming | A/V sync when seeking |
| 0002 | Compatibility | x265 4.1+ API changes |
| 0003 | Safari | HLS playback with HEVC |
| 0004 | NVIDIA | NVDEC surface limits |
| 0005 | HDR | NVENC HDR10 metadata |
| 0006 | Dolby Vision | HLS/MPEG-TS DV passthrough |
| 0007 | HLS Streaming | Report actual start PTS in playlist |

## Version

This fork is based on FFmpeg 7.1 with ViewRA-specific patches.

## License

FFmpeg is licensed under LGPL/GPL. See [FFmpeg License](https://ffmpeg.org/legal.html).

Patches are derived from [jellyfin-ffmpeg](https://github.com/jellyfin/jellyfin-ffmpeg) which is also GPL-licensed.

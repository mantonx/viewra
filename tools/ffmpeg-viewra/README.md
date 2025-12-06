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
  end_pts = 6 * 1 = 6s  ← WRONG! Should be relative to 4002s

Result: Audio and video drift out of sync
```

### The Solution

We apply a patch (based on [jellyfin-ffmpeg](https://github.com/jellyfin/jellyfin-ffmpeg)) that tracks the `start_pts` of the first packet and uses it as the anchor for segment boundary calculations:

```
Patched FFmpeg segment boundary calculation:
  start_pts = 4002s (captured from first packet)
  end_pts = start_pts + segment_time * (segment_count + 1)
  end_pts = 4002 + 6 = 4008s  ← CORRECT!
```

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

**Source:** Based on [jellyfin-ffmpeg](https://github.com/jellyfin/jellyfin-ffmpeg/blob/jellyfin/debian/patches/0001-add-fixes-for-segment-muxer.patch)

### 0002-libx265-fix-api-for-x265-4.1.patch

Fixes libx265 encoder compatibility with x265 4.1+ (build 215+). The x265 API changed the `encoder_encode` function signature back to using `x265_picture*` instead of `x265_picture**` for the output picture parameter.

**Affected versions:**
- x265 build 210-214: Uses `x265_picture**` (array of layer pointers)
- x265 build 215+: Uses `x265_picture*` (single pointer)

## Version

This fork is based on FFmpeg 7.1 with ViewRA-specific patches.

## License

FFmpeg is licensed under LGPL/GPL. See [FFmpeg License](https://ffmpeg.org/legal.html).

# FFmpeg 7/8 Features for Viewra

This document outlines FFmpeg 7.x and 8.x features that Viewra should leverage for improved performance, quality, and codec support.

## Overview

Viewra currently runs **FFmpeg 8.0.1** (released August 2025), which includes significant improvements over previous versions:

- **Vulkan-based hardware acceleration** (vendor-agnostic GPU acceleration)
- **Advanced HDR processing** with color management subsystem (CMS)
- **New codec support** (VVC, AV1 improvements, ProRes RAW)
- **AI-powered features** (Whisper speech recognition)
- **Improved hardware acceleration** across all platforms

---

## Priority 1: HDR & Color Processing 🔥

### 1. libplacebo Filter (Vulkan)

**Status:** ✅ Available in your build
**Priority:** 🔥 CRITICAL - Best HDR tone mapping solution

#### What It Is
libplacebo is a Vulkan-based video renderer from MPV that provides state-of-the-art HDR tone mapping with the new FFmpeg 7 color management subsystem (CMS).

#### Why Use It
- **Superior quality:** Industry-standard BT.2390/BT.2446a algorithms
- **Better performance:** Zero-copy CUDA→Vulkan interop (vs OpenCL memory copies)
- **Dynamic adaptation:** Scene-based peak detection adjusts tone mapping per scene
- **Contrast recovery:** Preserves highlight detail better than traditional tone mapping
- **Single-pass processing:** Combines scaling, color conversion, and tone mapping in one GPU pass
- **Cross-platform:** Works on any GPU with Vulkan 1.3+ (NVIDIA, AMD, Intel, integrated GPUs)

#### Performance Comparison
| Method | Speed | Quality | Notes |
|--------|-------|---------|-------|
| **libplacebo (Vulkan)** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | Zero-copy, dynamic peak detection |
| OpenCL (current) | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Memory copies CUDA↔OpenCL |
| tonemap_vaapi | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Native GPU, good for Intel/AMD |
| CPU tonemap | ⭐⭐ | ⭐⭐⭐ | 20-30% overhead |

#### Available Tone Mapping Algorithms
```
auto         - Automatic selection
clip         - No tone mapping (hard clip)
st2094-40    - SMPTE ST 2094-40 (dynamic metadata)
st2094-10    - SMPTE ST 2094-10 (static metadata)
bt.2390      - ITU-R BT.2390 EETF (broadcast standard) ⭐ RECOMMENDED
bt.2446a     - ITU-R BT.2446 Method A (latest ITU standard) ⭐ NEW
spline       - Single-pivot polynomial spline
reinhard     - Reinhard tone mapping
mobius       - Mobius tone mapping
hable        - Filmic tone-mapping (Uncharted 2)
gamma        - Gamma function with knee
linear       - Perceptually linear stretch
```

#### Example Usage
```bash
# NVENC with libplacebo (replaces OpenCL tone mapping)
-init_hw_device cuda=cu:0 \
-hwaccel cuda \
-hwaccel_output_format cuda \
-i input.mkv \
-vf "libplacebo=w=1920:h=1080:tonemapping=bt.2390:peak_detect=true:contrast_recovery=0.3:upscaler=ewa_lanczos:downscaler=mitchell:format=yuv420p" \
-c:v h264_nvenc -preset p2 output.mp4

# Software encoding with libplacebo
-i input.mkv \
-vf "libplacebo=w=1920:h=1080:tonemapping=bt.2446a:peak_detect=true:contrast_recovery=0.3" \
-c:v libx264 output.mp4
```

#### Key Parameters
- `tonemapping=bt.2390` - Algorithm (bt.2390 recommended for broadcast quality)
- `peak_detect=true` - Dynamic peak detection (adapts per scene)
- `contrast_recovery=0.3` - Highlight detail preservation (0.0-3.0, 0.3 default)
- `smoothing_period=100` - Peak detection smoothing (ms)
- `percentile=99.995` - Peak detection percentile
- `gamut_mode=perceptual` - Gamut mapping mode
- `upscaler=ewa_lanczos` - High-quality upscaling
- `downscaler=mitchell` - High-quality downscaling

#### Implementation Plan
- [x] Research libplacebo capabilities
- [ ] Add libplacebo support to FFmpegArgsBuilder
- [ ] Add BT.2390, BT.2446a algorithms to config
- [ ] Make libplacebo default for NVENC (CUDA systems)
- [ ] Make libplacebo option for all other systems (fallback to current)
- [ ] Add advanced options (peak_detect, contrast_recovery) to config
- [ ] Update TONE_MAPPING.md documentation
- [ ] Performance testing vs OpenCL

### 2. Enhanced Color Management System (CMS)

**Status:** ✅ Available (FFmpeg 7+)
**Priority:** 🟡 MEDIUM - For future advanced features

FFmpeg 7 added a comprehensive color management subsystem:
- 3D LUT support for professional color grading
- Better color space conversions (bt709, bt2020, sRGB, DCI-P3)
- Accurate gamut mapping

**Use Cases:**
- Professional color grading workflows
- Custom LUTs for specific content (films, documentaries)
- Better handling of wide gamut content (DCI-P3, Rec.2020)

---

## Priority 2: Hardware Acceleration Improvements

### 3. Vulkan Compute Filters

**Status:** ✅ Available (FFmpeg 8.0)
**Priority:** 🟢 LOW-MEDIUM - Fallback for systems without dedicated HW accel

FFmpeg 8.0 added Vulkan compute-based filters that work on **any GPU with Vulkan 1.3**:

#### Available Filters
- `scale_vulkan` - GPU-accelerated scaling
- `transpose_vulkan` - GPU-accelerated rotation
- `flip_vulkan`, `hflip_vulkan`, `vflip_vulkan` - GPU-accelerated flipping
- `blend_vulkan` - GPU-accelerated blending
- `overlay_vulkan` - GPU-accelerated overlay
- `gblur_vulkan` - Gaussian blur
- `nlmeans_vulkan` - Non-local means denoiser
- `avgblur_vulkan` - Average blur

#### Use Cases
- Systems with integrated GPUs (Intel UHD, AMD Vega iGPU) where VAAPI/QSV isn't available
- Laptops without dedicated GPUs
- Cross-platform GPU acceleration (works on NVIDIA, AMD, Intel)

#### Example
```bash
# Use Vulkan scaling on integrated GPU
-init_hw_device vulkan \
-i input.mp4 \
-vf "scale_vulkan=1920:1080" \
-c:v libx264 output.mp4
```

### 4. AV1 Hardware Encoding/Decoding

**Status:** ✅ Available (FFmpeg 8.0)
**Priority:** 🟡 MEDIUM - Future codec support

FFmpeg 8.0 added extensive AV1 hardware acceleration:

#### Encoding
- `av1_nvenc` - NVIDIA NVENC AV1 encoder (RTX 40 series)
- `av1_qsv` - Intel Quick Sync AV1 encoder (Arc GPUs, Meteor Lake CPUs)
- `av1_amf` - AMD AMF AV1 encoder (RDNA 3+)
- `av1_vaapi` - VAAPI AV1 encoder (Intel Arc, AMD)
- `av1_vulkan` - Vulkan AV1 encoder (software, works on any GPU)

#### Decoding
- `av1_cuvid` - NVIDIA NVDEC AV1 decoder
- `av1_qsv` - Intel Quick Sync AV1 decoder
- Vulkan VP9/AV1 decoding

#### Benefits
- **50% smaller files** than H.264 at same quality
- **30% smaller** than HEVC at same quality
- **Royalty-free** (no licensing fees like HEVC)
- **Future-proof** (YouTube, Netflix standard)

#### When to Use
- Add AV1 as a quality profile option (720p AV1, 1080p AV1, etc.)
- For users with modern GPUs (RTX 40, Arc, RDNA 3)
- Significant bandwidth savings for same quality

### 5. VVC (H.266) Support

**Status:** ✅ Decoder available, stable (FFmpeg 7.1+)
**Priority:** 🔵 LOW - Future codec (not widely supported yet)

FFmpeg 7.1 declared the VVC decoder stable (was experimental in 7.0).

#### What is VVC?
- **Next-gen codec** after HEVC (H.265)
- **50% better compression** than HEVC
- **40% better** than AV1

#### Limitations
- No hardware encoding yet (CPU only)
- Patent encumbered (licensing fees)
- Not widely adopted yet (2025)

#### Hardware Decoding
- `vvc_qsv` - Intel Arc GPUs support VVC hardware decoding (FFmpeg 8.0+)
- VAAPI VVC decoding on Intel Arc

#### When to Consider
- 2026-2027 when more devices support VVC
- For archival/preservation (smallest file sizes)
- When hardware encoders become available

---

## Priority 3: Advanced Features

### 6. AI Speech Recognition (Whisper)

**Status:** ✅ Available (FFmpeg 8.0)
**Priority:** 🟡 MEDIUM - Useful for accessibility features

FFmpeg 8.0 integrated OpenAI's Whisper model for speech recognition.

#### Use Cases
- **Auto-subtitle generation** during transcoding
- **Transcription** for accessibility
- **Search/indexing** by spoken content
- **Content classification** (detect language, speakers)

#### Example
```bash
# Generate subtitles during transcode
ffmpeg -i input.mp4 \
  -filter_complex "[0:a]asplit=2[a][speech];[speech]whisper=model=base[subs]" \
  -map "[a]" -map "[subs]" -c:a aac -c:s srt output.mkv
```

#### Implementation Ideas
- Option to auto-generate subtitles during library scan
- Background job to add subtitles to content without them
- Search by spoken dialogue

### 7. Dolby Vision Support

**Status:** ✅ Improved in FFmpeg 7+
**Priority:** 🔵 LOW - Niche content

FFmpeg 7.0 added Dolby Vision support with AV1 (profile 10).

#### Benefits
- Handle Dolby Vision content without losing metadata
- Proper tone mapping of DV → SDR
- Support for high-end home theater setups

#### Current Implementation
libplacebo filter has `apply_dolbyvision=true` option (enabled by default).

### 8. Performance Optimizations

**Status:** ✅ Available (FFmpeg 8.0)
**Priority:** ✅ Already benefiting

FFmpeg 8.0 includes numerous CPU performance optimizations:
- Multi-threaded hardware decoding via Vulkan
- Improved SIMD code paths
- Better memory management

**Action:** None needed - already using FFmpeg 8.0.1

---

## Implementation Priority

### Phase 1: Critical (Do Now) 🔥
1. **Implement libplacebo support** for HDR tone mapping
   - Replace OpenCL with libplacebo for NVENC
   - Add as option for all other accelerators
   - Add BT.2390, BT.2446a algorithms
   - Enable dynamic peak detection and contrast recovery

### Phase 2: Important (Next Quarter) 🟡
2. **Add AV1 encoding support**
   - Detect AV1 hardware capability (av1_nvenc, av1_qsv, av1_vaapi)
   - Add AV1 quality profiles (720p AV1, 1080p AV1, 4K AV1)
   - UI option to select codec (H.264 vs AV1)

3. **Whisper subtitle generation**
   - Background job to generate subtitles
   - Option during library scan
   - Store in database for search

### Phase 3: Future (6+ months) 🔵
4. **VVC decoder support** when content becomes available
5. **Vulkan compute filters** for integrated GPU systems
6. **Dolby Vision** handling for high-end setups

---

## Hardware Requirements

### For libplacebo (Priority 1)
- **GPU:** Any GPU with Vulkan 1.3+ support
  - NVIDIA: GTX 900 series or newer (2014+)
  - AMD: GCN 3 or newer (2015+), RDNA 1/2/3 (2019+)
  - Intel: Gen 9 or newer (2015+), Arc GPUs (2022+)
- **Driver:** Recent drivers with Vulkan 1.3 support

### For AV1 Hardware Encoding
- **NVIDIA:** RTX 40 series (4060, 4070, 4080, 4090)
- **Intel:** Arc A-series GPUs (A380, A750, A770) or Meteor Lake CPUs
- **AMD:** RDNA 3 GPUs (RX 7600, 7700 XT, 7800 XT, 7900 XT/XTX)

### For VVC Hardware Decoding
- **Intel:** Arc A-series GPUs only (2022+)

---

## Configuration Changes Needed

### Add to config.go
```go
// Tone mapping backend
ToneMappingBackend string // "auto", "libplacebo", "opencl", "vaapi", "cpu"

// libplacebo options
LibPlaceboEnabled       bool
LibPlaceboPeakDetect    bool    // Dynamic peak detection
LibPlaceboContrastRecov float64 // Contrast recovery (0.0-3.0, default 0.3)
LibPlaceboUpscaler      string  // "ewa_lanczos", "spline36", etc.
LibPlaceboDownscaler    string  // "mitchell", "lanczos", etc.

// AV1 support
AV1EncodingEnabled bool
AV1Profiles        []QualityProfile

// Whisper subtitle generation
WhisperEnabled       bool
WhisperModel         string // "tiny", "base", "small", "medium", "large"
WhisperAutoGenerate  bool   // Auto-generate during scan
```

### Add to TONE_MAPPING.md
Document new algorithms:
- `bt.2390` - ITU-R BT.2390 EETF (broadcast standard)
- `bt.2446a` - ITU-R BT.2446 Method A (latest ITU standard)
- `st2094-40` - SMPTE ST 2094-40 (dynamic metadata)
- `spline` - Polynomial spline tone mapping

---

## Testing Plan

### Phase 1: libplacebo
1. Test NVENC with libplacebo vs OpenCL (performance, quality)
2. Test all tone mapping algorithms (bt.2390, bt.2446a, hable, etc.)
3. Test dynamic peak detection on varied HDR content
4. Test contrast recovery settings (0.0, 0.3, 0.5, 1.0)
5. Cross-platform testing (NVIDIA, AMD, Intel)

### Phase 2: AV1
1. Detect hardware AV1 support on various GPUs
2. Quality comparison H.264 vs AV1 at same bitrate
3. File size comparison at same quality
4. Performance testing (encoding speed)
5. Compatibility testing (browser playback, HLS.js)

### Phase 3: Whisper
1. Accuracy testing on various content types
2. Performance impact during transcoding
3. Language detection accuracy
4. Storage requirements for subtitle metadata

---

## References

- [FFmpeg 8.0 Release Notes](https://ffmpeg.org/download.html#release_8.0)
- [FFmpeg 7.0 Release Notes](https://ffmpeg.org/download.html#release_7.0)
- [libplacebo Documentation](https://libplacebo.org/)
- [FFmpeg libplacebo Filter](https://ffmpeg.org/ffmpeg-filters.html#libplacebo)
- [ITU-R BT.2390 Specification](https://www.itu.int/rec/R-REC-BT.2390/)
- [Vulkan Video Acceleration](https://www.khronos.org/blog/an-introduction-to-vulkan-video)
- [OpenAI Whisper](https://github.com/openai/whisper)

---

## Summary

**Top 3 actions for Viewra:**

1. 🔥 **Implement libplacebo** - Immediate quality and performance improvements for HDR tone mapping
2. 🟡 **Add AV1 support** - Future-proof codec with 50% better compression
3. 🟡 **Whisper subtitles** - Accessibility and searchability improvements

The biggest win is **libplacebo** - you already have it available in your FFmpeg build, and it will provide significant improvements over your current OpenCL implementation with minimal code changes.

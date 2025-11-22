# HDR Tone Mapping Configuration

Viewra supports automatic HDR to SDR tone mapping for HDR10/HLG content using GPU-accelerated filters when available.

## Configuration

### Enable/Disable Tone Mapping

Set the `TONE_MAPPING_ENABLED` environment variable:

```bash
# Enable tone mapping (default)
export TONE_MAPPING_ENABLED=true

# Disable tone mapping
export TONE_MAPPING_ENABLED=false
```

### Tone Mapping Backend

Choose which tone mapping backend to use with `TONE_MAPPING_BACKEND`:

```bash
# Auto-detect best backend for your system (default - recommended)
export TONE_MAPPING_BACKEND=auto

# Force libplacebo (best quality, Vulkan-based)
export TONE_MAPPING_BACKEND=libplacebo

# Force OpenCL (good quality, GPU-accelerated)
export TONE_MAPPING_BACKEND=opencl

# Force VAAPI (Intel/AMD native GPU tone mapping)
export TONE_MAPPING_BACKEND=vaapi

# Force CPU (software, slowest but most compatible)
export TONE_MAPPING_BACKEND=cpu
```

**Recommendation:** Use `auto` (default) - it automatically selects the best backend for your hardware:
- **NVENC (NVIDIA):** libplacebo (if available) → OpenCL (fallback)
- **VAAPI (Intel/AMD):** Native VAAPI tone mapping
- **QSV (Intel):** OpenCL GPU tone mapping
- **Software (no GPU):** libplacebo (if available) → CPU (fallback)

### Tone Mapping Algorithm

Choose the tone mapping algorithm with `TONE_MAPPING_ALGORITHM`:

```bash
export TONE_MAPPING_ALGORITHM=bt.2390  # Default (NEW - industry standard)
```

## Available Algorithms

| Algorithm | ID | Description | Characteristics | Backend Support |
|-----------|----|--------------| ----------------| ----------------|
| **BT.2390** (NEW default) | `bt.2390` | ITU-R BT.2390 EETF | Industry broadcast standard, excellent quality | libplacebo only |
| **BT.2446a** (NEW) | `bt.2446a` | ITU-R BT.2446 Method A | Latest ITU standard, superior color preservation | libplacebo only |
| **Spline** (NEW) | `spline` | Polynomial spline | Smooth mathematical curve | libplacebo only |
| **ST 2094-40** (NEW) | `st2094-40` | SMPTE ST 2094-40 | Dynamic metadata-based | libplacebo only |
| **ST 2094-10** (NEW) | `st2094-10` | SMPTE ST 2094-10 | Static metadata-based | libplacebo only |
| **Hable** | `hable` | Uncharted 2 filmic | Excellent for cinematic content | All backends |
| **Reinhard** | `reinhard` | Classic Reinhard | Good for general content, smooth transitions | All backends |
| **Mobius** | `mobius` | Möbius tone mapping | Smooth highlight rolloff, good color preservation | All backends |
| **Linear** | `linear` | Linear compression | Fast but may clip highlights | All backends |
| **Gamma** | `gamma` | Gamma-based | Simple gamma curve adjustment | All backends |
| **Clip** | `clip` | Hard clipping | Fastest but loses highlight detail | All backends |

**Note:** Algorithms marked as "libplacebo only" will automatically fall back to **hable** when using OpenCL, VAAPI, or CPU backends.

## Hardware Acceleration & Backend Selection

Tone mapping works across all hardware acceleration types with automatic backend selection:

### NVENC (NVIDIA GPUs)
- **Default (auto):** libplacebo (Vulkan) → OpenCL (fallback)
- **Best Quality:** `TONE_MAPPING_BACKEND=libplacebo` - BT.2390, BT.2446a algorithms available
- **Performance:** ~5-10% overhead with libplacebo, ~8-12% with OpenCL
- **Supports:** All algorithms

### VAAPI (Intel/AMD GPUs on Linux)
- **Default (auto):** Native `tonemap_vaapi` filter
- **Best Quality:** Keep default (VAAPI native is excellent)
- **Performance:** ~5-8% overhead
- **Supports:** Standard algorithms (hable, reinhard, mobius, linear, gamma, clip)
- **Note:** BT.2390/BT.2446a not available (libplacebo not used for VAAPI)

### QSV (Intel Quick Sync / Arc GPUs)
- **Default (auto):** OpenCL GPU tone mapping
- **Best Quality:** Keep default
- **Performance:** ~6-10% overhead
- **Supports:** Standard algorithms (hable, reinhard, mobius, linear, gamma, clip)

### Software (libx264)
- **Default (auto):** libplacebo → CPU tonemap (fallback)
- **Best Quality:** `TONE_MAPPING_BACKEND=libplacebo` - BT.2390, BT.2446a algorithms available
- **Performance:** ~15-20% overhead with libplacebo, ~25-35% with CPU tonemap
- **Supports:** All algorithms

### VideoToolbox (Apple Silicon / macOS)
- **Default (auto):** CPU-based tonemap filter
- **Performance:** ~20-30% overhead (hybrid GPU→CPU→GPU pipeline)
- **Supports:** Standard algorithms (hable, reinhard, mobius, linear, gamma, clip)

## Performance Comparison

| Backend | Speed | Quality | Best For |
|---------|-------|---------|----------|
| **libplacebo (Vulkan)** | ⭐⭐⭐⭐⭐ Fastest | ⭐⭐⭐⭐⭐ Best | NVENC, Software (if available) |
| **OpenCL GPU** | ⭐⭐⭐⭐ Fast | ⭐⭐⭐⭐ Good | NVENC, QSV |
| **VAAPI Native** | ⭐⭐⭐⭐ Fast | ⭐⭐⭐⭐ Good | Intel/AMD GPUs on Linux |
| **CPU (tonemap)** | ⭐⭐ Slow | ⭐⭐⭐ Good | Fallback only |

## libplacebo Advanced Options

When using `libplacebo` backend, you can configure additional options:

### Dynamic Peak Detection

```bash
# Enable dynamic peak detection (default - recommended)
export LIBPLACEBO_PEAK_DETECT=true

# Disable peak detection (static tone mapping)
export LIBPLACEBO_PEAK_DETECT=false
```

Dynamic peak detection analyzes each scene to determine the actual peak brightness, providing adaptive tone mapping that adjusts to content changes.

### Contrast Recovery

```bash
# Default contrast recovery (0.3 - recommended)
export LIBPLACEBO_CONTRAST_RECOVERY=0.3

# No contrast recovery (may lose highlight detail)
export LIBPLACEBO_CONTRAST_RECOVERY=0.0

# High contrast recovery (preserves more highlight detail)
export LIBPLACEBO_CONTRAST_RECOVERY=0.5

# Maximum contrast recovery (maximum highlight preservation)
export LIBPLACEBO_CONTRAST_RECOVERY=1.0
```

Contrast recovery preserves local contrast and highlight detail during tone mapping. Higher values preserve more detail but may reduce overall contrast.

## Algorithm Recommendations

### For Broadcast/Professional Content (Best Quality)
Use **bt.2390** (NEW default) - industry standard, requires libplacebo backend.
```bash
export TONE_MAPPING_ALGORITHM=bt.2390
export TONE_MAPPING_BACKEND=libplacebo
```

### For Latest ITU Standard (Experimental)
Try **bt.2446a** - latest ITU standard, superior color preservation, requires libplacebo.
```bash
export TONE_MAPPING_ALGORITHM=bt.2446a
export TONE_MAPPING_BACKEND=libplacebo
```

### For Cinematic Content (All Backends)
Use **hable** - excellent for films and movies, works with all backends.
```bash
export TONE_MAPPING_ALGORITHM=hable
export TONE_MAPPING_BACKEND=auto
```

### For Vibrant/Colorful Content (All Backends)
Try **mobius** - better color preservation in bright scenes, works with all backends.
```bash
export TONE_MAPPING_ALGORITHM=mobius
export TONE_MAPPING_BACKEND=auto
```

### For Documentary/Natural Content (All Backends)
Try **reinhard** - smooth, natural tone mapping, works with all backends.
```bash
export TONE_MAPPING_ALGORITHM=reinhard
export TONE_MAPPING_BACKEND=auto
```

### For Maximum Performance (All Backends)
Use **linear** - fastest tone mapping, acceptable quality loss.
```bash
export TONE_MAPPING_ALGORITHM=linear
export TONE_MAPPING_BACKEND=auto
```

## Example Configurations

### Best Quality (NEW Default)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=bt.2390     # ITU broadcast standard
export TONE_MAPPING_BACKEND=auto          # Auto-select best backend
export LIBPLACEBO_PEAK_DETECT=true        # Dynamic peak detection
export LIBPLACEBO_CONTRAST_RECOVERY=0.3   # Preserve highlights
```

### Maximum Quality (Experimental)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=bt.2446a    # Latest ITU standard
export TONE_MAPPING_BACKEND=libplacebo    # Force libplacebo
export LIBPLACEBO_PEAK_DETECT=true
export LIBPLACEBO_CONTRAST_RECOVERY=0.5   # High highlight preservation
```

### Balanced (Compatible with all backends)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=hable       # Filmic tone mapping
export TONE_MAPPING_BACKEND=auto
```

### Performance Optimized
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=linear
export TONE_MAPPING_BACKEND=auto
export LIBPLACEBO_PEAK_DETECT=false       # Disable peak detection
```

### Force OpenCL (NVIDIA)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=hable
export TONE_MAPPING_BACKEND=opencl        # Force OpenCL instead of libplacebo
```

### Disable Tone Mapping
```bash
export TONE_MAPPING_ENABLED=false
```

## Future UI Integration

These settings will be exposed in the web UI settings panel in a future update, allowing users to:
- Toggle HDR tone mapping on/off
- Select tone mapping algorithm from dropdown
- Preview tone mapping results in real-time

## Technical Details

The tone mapping backend is applied during the video encoding filter chain:

### NVENC with libplacebo (Default)
```
hwdownload → libplacebo=tonemapping={algorithm}:peak_detect={bool}:contrast_recovery={float} → hwupload_cuda
```
- Single-pass scaling + tone mapping
- Zero-copy CUDA→Vulkan on some systems
- Dynamic peak detection per scene
- Contrast recovery for highlight preservation

### NVENC with OpenCL (Fallback)
```
hwdownload → format=p010le → hwupload → tonemap_opencl=tonemap={algorithm}:desat=0 → hwdownload → format=nv12 → hwupload_cuda
```
- GPU-accelerated tone mapping
- Memory copies between CUDA ↔ OpenCL
- Standard algorithms only

### VAAPI (Intel/AMD)
```
tonemap_vaapi={algorithm} → scale_vaapi → pad_vaapi
```
- Native GPU tone mapping
- Full GPU pipeline (no CPU transfers)
- Standard algorithms only

### QSV (Intel Quick Sync)
```
hwdownload → format=p010le → hwupload → tonemap_opencl=tonemap={algorithm}:desat=0 → hwdownload → format=nv12 → hwupload
```
- OpenCL GPU tone mapping
- QSV decode → OpenCL tone map → QSV encode
- Standard algorithms only

### Software with libplacebo (Best Quality)
```
libplacebo=w={width}:h={height}:tonemapping={algorithm}:peak_detect={bool}:contrast_recovery={float}
```
- Single-pass scaling + tone mapping
- CPU-based but highly optimized
- All algorithms available including BT.2390, BT.2446a

### Software with CPU tonemap (Fallback)
```
zscale=t=linear → format=gbrpf32le → zscale=p=bt709 → tonemap={algorithm}:desat=0 → zscale=t=bt709 → scale → pad
```
- Traditional CPU tone mapping
- Multiple filter passes
- Standard algorithms only

### VideoToolbox (Apple)
```
zscale=t=linear → format=gbrpf32le → zscale=p=bt709 → tonemap={algorithm}:desat=0 → zscale=t=bt709 → scale → pad
```
- Hybrid pipeline: GPU decode → CPU tone map → GPU encode
- Standard algorithms only

**Pipeline Comparison:**
- **libplacebo:** Single-pass, best quality, supports all algorithms
- **OpenCL/VAAPI:** GPU-accelerated, good quality, standard algorithms only
- **CPU tonemap:** Multi-pass, slower, standard algorithms only

See [HARDWARE_ACCELERATION.md](HARDWARE_ACCELERATION.md) and [FFMPEG_7_8_FEATURES.md](FFMPEG_7_8_FEATURES.md) for more details.

# libplacebo Implementation Summary

## What Was Implemented

### 1. Backend Selection System
Added `TONE_MAPPING_BACKEND` environment variable with options:
- `auto` (default) - Automatically selects best backend for hardware
- `libplacebo` - Force libplacebo (Vulkan-based, best quality)
- `opencl` - Force OpenCL GPU tone mapping
- `vaapi` - Force VAAPI native tone mapping (Intel/AMD)
- `cpu` - Force CPU software tone mapping

### 2. New Tone Mapping Algorithms
Added support for industry-standard algorithms (libplacebo only):
- `bt.2390` (NEW default) - ITU-R BT.2390 EETF broadcast standard
- `bt.2446a` - Latest ITU standard
- `spline` - Polynomial spline tone mapping
- `st2094-40` / `st2094-10` - SMPTE metadata-based

All existing algorithms still work (hable, reinhard, mobius, etc.)

### 3. libplacebo Advanced Options
- `LIBPLACEBO_PEAK_DETECT=true` - Dynamic peak detection (default)
- `LIBPLACEBO_CONTRAST_RECOVERY=0.3` - Highlight preservation (0.0-3.0)

### 4. Auto-Selection Logic
When `TONE_MAPPING_BACKEND=auto` (default):
- **NVENC (NVIDIA)**: libplacebo → OpenCL fallback
- **VAAPI (Intel/AMD)**: Native VAAPI tone mapping
- **QSV (Intel)**: OpenCL GPU tone mapping
- **Software**: libplacebo → CPU fallback
- **VideoToolbox**: CPU tone mapping

## Files Modified

1. **internal/infrastructure/transcoding/config.go**
   - Added `ToneMappingBackend` field
   - Added `LibPlaceboPeakDetect` field
   - Added `LibPlaceboContrastRecovery` field
   - Updated defaults (bt.2390 algorithm, auto backend)

2. **internal/infrastructure/transcoding/ffmpeg_args_builder.go**
   - Added `shouldUseLibPlacebo()` method
   - Added `getToneMappingLibPlaceboAlgorithm()` method
   - Updated `addNVENCEncoding()` to use libplacebo
   - Updated `AddVideoEncoding()` to use libplacebo

3. **internal/infrastructure/transcoding/hardware_test_utils.go**
   - Added `CheckFFmpegFilter()` function

4. **docs/TONE_MAPPING.md**
   - Comprehensive update with all new features
   - Backend selection guide
   - Algorithm compatibility matrix
   - Configuration examples

5. **docs/FFMPEG_7_8_FEATURES.md** (NEW)
   - Complete FFmpeg 7/8 feature analysis
   - Implementation priorities
   - Future roadmap (AV1, Whisper, VVC, etc.)

## Testing Plan

### Manual Testing Commands

#### 1. Check libplacebo availability
```bash
ffmpeg -filters 2>&1 | grep libplacebo
```
Expected output: Should show the libplacebo filter

#### 2. Test default configuration (should use libplacebo on NVENC/Software)
```bash
# Should auto-select libplacebo and bt.2390 algorithm
make dev-backend

# Check logs for FFmpeg command - should see:
# - "libplacebo=tonemapping=bt.2390" for NVENC/Software
# - "tonemap_opencl" for OpenCL fallback
# - "tonemap_vaapi" for VAAPI
```

#### 3. Test forced libplacebo backend
```bash
export TONE_MAPPING_BACKEND=libplacebo
export TONE_MAPPING_ALGORITHM=bt.2390
make dev-backend
```

#### 4. Test forced OpenCL backend (to verify fallback still works)
```bash
export TONE_MAPPING_BACKEND=opencl
export TONE_MAPPING_ALGORITHM=hable
make dev-backend
```

#### 5. Test libplacebo-only algorithms with fallback
```bash
# On VAAPI (should fallback bt.2390 → hable)
export TONE_MAPPING_BACKEND=vaapi
export TONE_MAPPING_ALGORITHM=bt.2390
make dev-backend
# Check logs - should see "tonemap_vaapi=hable" not "bt.2390"
```

#### 6. Test libplacebo advanced options
```bash
export TONE_MAPPING_BACKEND=libplacebo
export TONE_MAPPING_ALGORITHM=bt.2390
export LIBPLACEBO_PEAK_DETECT=true
export LIBPLACEBO_CONTRAST_RECOVERY=0.5
make dev-backend
# Check logs - should see: peak_detect=true:contrast_recovery=0.50
```

### Build Testing

#### 1. Test Go build
```bash
cd /home/fictional/Projects/viewra2
go build ./internal/infrastructure/transcoding/...
```
Expected: Should compile without errors

#### 2. Test full backend build
```bash
make build-backend
```
Expected: Should compile without errors

#### 3. Run Go tests (if any exist for transcoding)
```bash
go test ./internal/infrastructure/transcoding/... -v
```

### Integration Testing

When backend is running, test with actual HDR content:

#### 1. Test NVENC with libplacebo
```bash
export HARDWARE_ACCEL=nvenc
export TONE_MAPPING_BACKEND=auto  # Should select libplacebo
export TONE_MAPPING_ALGORITHM=bt.2390

# Transcode HDR content through UI
# Check transcode logs for libplacebo usage
```

#### 2. Test quality comparison
Transcode the same HDR content with different backends:
```bash
# libplacebo (best quality)
export TONE_MAPPING_BACKEND=libplacebo
export TONE_MAPPING_ALGORITHM=bt.2390
# Transcode → save output as "test_libplacebo.mp4"

# OpenCL (current method)
export TONE_MAPPING_BACKEND=opencl
export TONE_MAPPING_ALGORITHM=hable
# Transcode → save output as "test_opencl.mp4"

# Compare visual quality side-by-side
```

#### 3. Test performance impact
```bash
# Monitor GPU usage with different backends
nvidia-smi -l 1  # For NVIDIA

# Compare transcoding speed:
# - libplacebo should be 5-15% faster than OpenCL
# - Quality should be noticeably better with bt.2390
```

## Verification Checklist

- [ ] Code compiles without errors
- [ ] `CheckFFmpegFilter()` correctly detects libplacebo
- [ ] `shouldUseLibPlacebo()` returns correct values for each hardware type
- [ ] NVENC uses libplacebo by default (when `auto`)
- [ ] Software encoding uses libplacebo by default (when `auto`)
- [ ] VAAPI uses native tone mapping (not libplacebo)
- [ ] OpenCL fallback works when libplacebo unavailable
- [ ] bt.2390 algorithm falls back to hable on non-libplacebo backends
- [ ] libplacebo peak detection setting is applied
- [ ] libplacebo contrast recovery setting is applied
- [ ] Backend can be manually overridden with `TONE_MAPPING_BACKEND`
- [ ] FFmpeg command logs show correct filter chains
- [ ] Transcoded HDR→SDR content looks correct
- [ ] Performance is better with libplacebo vs OpenCL

## Known Limitations

1. **libplacebo-only algorithms**: bt.2390, bt.2446a, spline, st2094-* only work with libplacebo backend. They automatically fall back to "hable" on other backends.

2. **VAAPI doesn't use libplacebo**: VAAPI has its own excellent native tone mapping, so we don't use libplacebo for VAAPI in auto mode.

3. **VideoToolbox doesn't use libplacebo**: VideoToolbox uses CPU-based tone mapping (hybrid pipeline).

4. **Algorithm fallback**: When using a libplacebo-only algorithm with a non-libplacebo backend, it silently falls back to "hable". This is by design to avoid breaking existing configurations.

## Backward Compatibility

✅ **Fully backward compatible:**
- Existing `TONE_MAPPING_ALGORITHM` values still work
- Default behavior improved (bt.2390 instead of hable)
- OpenCL fallback preserved for systems without libplacebo
- No breaking changes to configuration

## Next Steps (Future Work)

See [FFMPEG_7_8_FEATURES.md](FFMPEG_7_8_FEATURES.md) for detailed roadmap:

1. **Phase 2: AV1 Support** (3-6 months)
   - Hardware AV1 encoding (av1_nvenc, av1_qsv, av1_vaapi)
   - Quality profiles for AV1
   - 50% smaller files than H.264

2. **Phase 3: Whisper Subtitles** (6-9 months)
   - Auto-generate subtitles during transcoding
   - Background job for existing content
   - Search by dialogue

3. **Phase 4: VVC Support** (12+ months)
   - When hardware encoders become available
   - 50% better compression than HEVC

## References

- [TONE_MAPPING.md](TONE_MAPPING.md) - User configuration guide
- [FFMPEG_7_8_FEATURES.md](FFMPEG_7_8_FEATURES.md) - Comprehensive FFmpeg 7/8 feature analysis
- [libplacebo documentation](https://libplacebo.org/)
- [FFmpeg libplacebo filter](https://ffmpeg.org/ffmpeg-filters.html#libplacebo)
- [ITU-R BT.2390 Specification](https://www.itu.int/rec/R-REC-BT.2390/)

## Environment Variables Quick Reference

```bash
# Tone mapping on/off
export TONE_MAPPING_ENABLED=true

# Backend selection
export TONE_MAPPING_BACKEND=auto           # auto, libplacebo, opencl, vaapi, cpu

# Algorithm selection
export TONE_MAPPING_ALGORITHM=bt.2390      # bt.2390, bt.2446a, hable, reinhard, mobius, etc.

# libplacebo options (only apply when using libplacebo)
export LIBPLACEBO_PEAK_DETECT=true         # true/false
export LIBPLACEBO_CONTRAST_RECOVERY=0.3    # 0.0-3.0
```

## Example Configurations

### Best Quality (NEW Default)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=bt.2390
export TONE_MAPPING_BACKEND=auto
export LIBPLACEBO_PEAK_DETECT=true
export LIBPLACEBO_CONTRAST_RECOVERY=0.3
```

### Force OpenCL (Legacy)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=hable
export TONE_MAPPING_BACKEND=opencl
```

### Maximum Quality (Experimental)
```bash
export TONE_MAPPING_ENABLED=true
export TONE_MAPPING_ALGORITHM=bt.2446a
export TONE_MAPPING_BACKEND=libplacebo
export LIBPLACEBO_PEAK_DETECT=true
export LIBPLACEBO_CONTRAST_RECOVERY=0.5
```

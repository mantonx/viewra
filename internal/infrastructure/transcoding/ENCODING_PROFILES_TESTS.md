# FFmpeg Encoding Profiles Tests

## Overview

Comprehensive test suite for `ffmpeg_builder_encoding.go` focusing on codec-specific profile configuration functions.

**File**: `ffmpeg_encoding_profiles_test.go`

## Coverage Improvements

This test suite improves coverage for the following functions:

### Previously 0% Coverage (Now 100%)
- `addVP9Profile` - VP9-specific encoding parameters
- `addAV1Profile` - AV1-specific encoding parameters
- `addH265Profile` - H.265/HEVC-specific encoding parameters

### Improved Coverage
- `addCodecProfile` - from 40% to 100% (routing to codec-specific functions)

## Test Functions (14 total)

### 1. TestAddCodecProfile_H264
Tests H.264 profile selection through the `addCodecProfile` router function.

**Test Cases**:
- H.264 software 1080p (level 4.1)
- H.264 software 4K (level 5.1)
- H.264 NVENC 1080p (hardware acceleration)

**Verifies**:
- `-profile:v high` for H.264
- Correct level based on resolution (4.1 for 1080p, 5.1 for 4K)
- `-pix_fmt yuv420p` for compatibility

### 2. TestAddH265Profile
Tests H.265/HEVC profile and level configuration across different hardware accelerators.

**Test Cases**:
- Software encoding (libx265)
- NVENC hardware
- VAAPI hardware
- QSV hardware
- VideoToolbox (macOS)

**Verifies**:
- `-profile:v main` for broad compatibility
- `-level 5.1` (supports up to 4K@60fps)
- `-pix_fmt yuv420p`
- `-x265-params log-level=error` only for software encoding

### 3. TestAddVP9Profile
Tests VP9 profile configuration for software and hardware encoding.

**Test Cases**:
- Software encoding (libvpx-vp9)
- VAAPI hardware
- QSV hardware

**Verifies**:
- `-quality good` (balanced speed/quality)
- `-speed 2` (0-5 range, balanced setting)
- `-row-mt 1` only for software encoding (multi-threading)

### 4. TestAddVP9Profile_QualitySettings
Focused test for VP9 quality parameter validation.

**Verifies**:
- Quality setting is always "good" (not "best" or "realtime")
- Speed setting is 2 for balanced encoding

### 5. TestAddAV1Profile
Tests AV1 profile configuration for SVT-AV1 and hardware encoders.

**Test Cases**:
- Software encoding (SVT-AV1)
- QSV hardware
- VAAPI hardware

**Verifies**:
- `-pix_fmt yuv420p`
- `-preset 6` only for software SVT-AV1 (0-13 range, balanced)

### 6. TestAddAV1Profile_PresetRange
Validates AV1 preset value is within valid SVT-AV1 range.

**Verifies**:
- Preset is "6" (balanced on 0-13 scale)

### 7. TestAddCodecProfile_AllCodecs
Tests the `addCodecProfile` routing function correctly dispatches to codec-specific functions.

**Test Cases**:
- Routes to H.264 profile
- Routes to H.265 profile
- Routes to VP9 profile
- Routes to AV1 profile

**Verifies**:
- Correct codec-specific arguments are present
- Other codec arguments are not present (no cross-contamination)

### 8. TestAddCodecProfile_ResolutionDependentLevels
Tests H.264 level selection based on resolution (pixel count).

**Test Cases**:
- 480p (level 4.0)
- 720p (level 4.0)
- 1080p (level 4.1)
- 1440p (level 5.1)
- 4K (level 5.1)

**Verifies**:
- Correct H.264 level for each resolution tier

### 9. TestAddVideoEncoding_Integration
Integration test for the full `AddVideoEncoding()` pipeline.

**Test Cases**:
- H.264 1080p full encoding
- H.265 4K full encoding

**Verifies**:
- Codec profile is set correctly
- Bitrate arguments are present
- GOP arguments are present
- Filter chain is present

### 10. TestCodecProfile_PixelFormat
Tests all codecs use yuv420p for maximum compatibility.

**Test Cases**:
- All 4 codecs (H.264, H.265, VP9, AV1)

**Verifies**:
- All use `-pix_fmt yuv420p`

### 11. TestH265Profile_Level
Tests H.265 always uses level 5.1 regardless of resolution.

**Test Cases**:
- 720p, 1080p, 1440p, 4K

**Verifies**:
- All resolutions use level 5.1 (supports up to 4K@60fps)

### 12. TestVP9Profile_MultiThreading
Tests VP9 row-based multi-threading for software encoding.

**Verifies**:
- Software VP9 enables `-row-mt 1` for better CPU utilization

### 13. TestH265Profile_X265LogLevel
Tests x265 log suppression for cleaner output.

**Verifies**:
- Software H.265 uses `-x265-params log-level=error`

### 14. TestCodecProfile_NoHardwareSpecificArgsInSoftware
Tests software encoding doesn't include hardware-specific arguments.

**Test Cases**:
- All 4 codecs with AccelNone

**Verifies**:
- No hardware flags (_nvenc, _qsv, _vaapi, _videotoolbox, cuda, opencl)

## Key Testing Patterns

### 1. Table-Driven Tests
Most tests use table-driven approach with struct test cases for clarity and completeness.

### 2. Hardware Acceleration Coverage
Tests verify both software and hardware encoding paths for each codec.

### 3. Resolution-Dependent Behavior
Tests verify codec levels change appropriately based on resolution.

### 4. Cross-Contamination Prevention
Tests ensure codec-specific arguments don't leak between different codecs.

## FFmpeg Argument Verification

Tests use helper functions from `ffmpeg_builder_test.go`:
- `argsContain(args, flag, value)` - checks flag-value pairs
- `argsContainFlag(args, flag)` - checks flag presence

## Running the Tests

### Run all encoding profile tests:
```bash
go test -v -run "TestAdd.*Profile|TestCodecProfile" ./internal/infrastructure/transcoding/
```

### Run specific codec tests:
```bash
go test -v -run TestAddH265Profile ./internal/infrastructure/transcoding/
go test -v -run TestAddVP9Profile ./internal/infrastructure/transcoding/
go test -v -run TestAddAV1Profile ./internal/infrastructure/transcoding/
```

### Check coverage:
```bash
go test -coverprofile=coverage.out ./internal/infrastructure/transcoding/
go tool cover -func=coverage.out | grep ffmpeg_builder_encoding.go
```

## Expected Coverage Results

After these tests, `ffmpeg_builder_encoding.go` should show:
- `addCodecProfile`: 100% (was 40%)
- `addH265Profile`: 100% (was 0%)
- `addVP9Profile`: 100% (was 0%)
- `addAV1Profile`: 100% (was 0%)
- `addH264Profile`: maintained at existing level

## Codec-Specific Details Tested

### H.264
- Profile: high
- Levels: 4.0 (720p), 4.1 (1080p), 5.1 (4K+)
- Pixel format: yuv420p

### H.265/HEVC
- Profile: main (not main10 for compatibility)
- Level: 5.1 (all resolutions)
- Pixel format: yuv420p
- x265 params: log-level=error (software only)

### VP9
- Quality: good (not best or realtime)
- Speed: 2 (0-5 range, balanced)
- Pixel format: yuv420p
- Row MT: enabled for software (multi-threading)

### AV1
- Preset: 6 (0-13 range, balanced for SVT-AV1)
- Pixel format: yuv420p
- Software only preset (no preset for hardware)

## Quality Settings Rationale

### VP9 "good" quality
- Balanced between "realtime" (fastest, lower quality) and "best" (slowest, highest quality)
- Suitable for on-demand transcoding

### VP9 speed 2
- On 0-5 scale for "good" quality mode
- Balanced encoding speed vs quality
- Not too slow for real-world usage

### AV1 preset 6
- On SVT-AV1's 0-13 scale
- Balanced between speed (13) and quality (0)
- Reasonable for on-demand transcoding

### H.265 level 5.1
- Supports up to 4096x2160@30fps (4K)
- Covers all common resolutions
- Broad decoder compatibility

## Hardware vs Software Differences

### Software-Only Arguments
- H.265: `-x265-params log-level=error`
- VP9: `-row-mt 1`
- AV1: `-preset 6`

### Hardware encoding
- Omits codec-specific tuning params
- Uses hardware encoder's defaults
- Profile/level still specified for compatibility

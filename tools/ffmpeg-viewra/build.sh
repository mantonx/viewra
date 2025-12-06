#!/bin/bash
set -e

# ViewRA FFmpeg Build Script
# Builds a patched FFmpeg with fixes for HLS segment timing
#
# Based on jellyfin-ffmpeg patches for proper A/V sync when seeking

FFMPEG_VERSION="${FFMPEG_VERSION:-7.1}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="${SCRIPT_DIR}/build"
SRC_DIR="${BUILD_DIR}/ffmpeg-${FFMPEG_VERSION}"
INSTALL_PREFIX="${SCRIPT_DIR}/dist"
PATCH_DIR="${SCRIPT_DIR}/patches"

# Number of parallel jobs for compilation
JOBS="${JOBS:-$(nproc)}"

echo "=== ViewRA FFmpeg Builder ==="
echo "FFmpeg version: ${FFMPEG_VERSION}"
echo "Build directory: ${BUILD_DIR}"
echo "Install prefix: ${INSTALL_PREFIX}"
echo "Parallel jobs: ${JOBS}"
echo ""

# Create build directory
mkdir -p "${BUILD_DIR}"
cd "${BUILD_DIR}"

# Download FFmpeg source if not present
if [ ! -d "${SRC_DIR}" ]; then
    echo "Downloading FFmpeg ${FFMPEG_VERSION}..."

    # Try the release URL first, then snapshot
    FFMPEG_URL="https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VERSION}.tar.xz"

    if ! curl -fsSL "${FFMPEG_URL}" -o "ffmpeg-${FFMPEG_VERSION}.tar.xz"; then
        echo "Release not found, trying git..."
        git clone --depth 1 --branch "n${FFMPEG_VERSION}" https://git.ffmpeg.org/ffmpeg.git "${SRC_DIR}"
    else
        echo "Extracting..."
        tar xf "ffmpeg-${FFMPEG_VERSION}.tar.xz"
        rm "ffmpeg-${FFMPEG_VERSION}.tar.xz"
    fi
fi

cd "${SRC_DIR}"

# Apply patches
echo ""
echo "Applying patches..."
for patch in "${PATCH_DIR}"/*.patch; do
    if [ -f "$patch" ]; then
        echo "  Applying: $(basename "$patch")"
        # Use -p1 for standard diff format, --forward to skip already applied
        patch -p1 --forward < "$patch" || {
            # Check if already applied
            if patch -p1 --reverse --dry-run < "$patch" > /dev/null 2>&1; then
                echo "    (already applied, skipping)"
            else
                echo "    ERROR: Failed to apply patch!"
                exit 1
            fi
        }
    fi
done

# Configure FFmpeg
echo ""
echo "Configuring FFmpeg..."
echo "Detecting available libraries..."

# Helper function to check if a library is available
check_lib() {
    pkg-config --exists "$1" 2>/dev/null
}

# Helper function to check if a header exists
check_header() {
    echo "#include <$1>" | gcc -E - >/dev/null 2>&1
}

# Build configure flags dynamically based on what's available
EXTRA_FLAGS=""

# Video codecs
if check_lib x264; then
    echo "  Found: libx264"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libx264"
else
    echo "  Missing: libx264 (H.264 encoding disabled)"
fi

if check_lib x265; then
    echo "  Found: libx265"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libx265"
else
    echo "  Missing: libx265 (HEVC software encoding disabled)"
fi

if check_lib vpx; then
    echo "  Found: libvpx"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libvpx"
else
    echo "  Missing: libvpx (VP8/VP9 encoding disabled)"
fi

# Audio codecs
if check_lib fdk-aac; then
    echo "  Found: libfdk-aac"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libfdk-aac"
else
    echo "  Missing: libfdk-aac (using native AAC encoder)"
fi

if check_lib opus; then
    echo "  Found: libopus"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libopus"
else
    echo "  Missing: libopus"
fi

if check_lib vorbis; then
    echo "  Found: libvorbis"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libvorbis"
else
    echo "  Missing: libvorbis"
fi

# Subtitle/text rendering
if check_lib libass; then
    echo "  Found: libass"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libass"
else
    echo "  Missing: libass (subtitle burn-in disabled)"
fi

if check_lib freetype2; then
    echo "  Found: freetype2"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libfreetype"
else
    echo "  Missing: freetype2"
fi

# SSL/TLS
if check_lib openssl; then
    echo "  Found: openssl"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-openssl"
elif check_lib gnutls; then
    echo "  Found: gnutls"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-gnutls"
else
    echo "  Missing: openssl/gnutls (HTTPS disabled)"
fi

# Hardware acceleration - NVIDIA NVENC/NVDEC
# Requires: nv-codec-headers (provides ffnvcodec pkg-config)
# On Arch: pacman -S ffnvcodec-headers
# On Ubuntu/Debian: apt install libnvidia-encode-* or build from source
# On Fedora: dnf install nv-codec-headers
if pkg-config --exists ffnvcodec 2>/dev/null; then
    echo "  Found: ffnvcodec (enabling NVENC/NVDEC hardware encoding)"
    # --enable-cuda-llvm uses clang for CUDA kernels (no nvcc needed)
    # --enable-ffnvcodec enables the NVENC/NVDEC API
    # --enable-cuvid enables NVIDIA CUVID (hardware decoding)
    # Note: We don't need --enable-cuda-nvcc since we're not compiling CUDA kernels
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-ffnvcodec --enable-cuda-llvm --enable-cuvid"
else
    echo "  Missing: ffnvcodec-headers (NVENC/NVDEC disabled)"
    echo "           Install: pacman -S ffnvcodec-headers (Arch)"
    echo "                    apt install libnvidia-encode-* (Ubuntu)"
fi

if check_lib libva; then
    echo "  Found: VAAPI"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-vaapi"
fi

if check_lib vulkan; then
    echo "  Found: Vulkan"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-vulkan"
fi

if check_lib libplacebo; then
    echo "  Found: libplacebo (HDR tone mapping)"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libplacebo"
fi

if check_lib libdrm; then
    echo "  Found: libdrm"
    EXTRA_FLAGS="${EXTRA_FLAGS} --enable-libdrm"
fi

echo ""
echo "Running configure..."

./configure \
    --prefix="${INSTALL_PREFIX}" \
    --enable-gpl \
    --enable-version3 \
    --enable-nonfree \
    --enable-shared \
    --disable-static \
    --disable-debug \
    --disable-doc \
    ${EXTRA_FLAGS}

# Build
echo ""
echo "Building FFmpeg (this may take a while)..."
make -j"${JOBS}"

# Install
echo ""
echo "Installing to ${INSTALL_PREFIX}..."
make install

echo ""
echo "=== Build Complete ==="
echo ""
echo "FFmpeg binary: ${INSTALL_PREFIX}/bin/ffmpeg"
echo "FFprobe binary: ${INSTALL_PREFIX}/bin/ffprobe"
echo ""
echo "To use this FFmpeg with ViewRA, set:"
echo "  export VIEWRA_FFMPEG_PATH=${INSTALL_PREFIX}/bin/ffmpeg"
echo "  export VIEWRA_FFPROBE_PATH=${INSTALL_PREFIX}/bin/ffprobe"
echo ""

#!/bin/sh
set -e

echo "ViewRA Development Environment"

# Fix git ownership for mounted volume (prevents VCS stamping errors)
git config --global --add safe.directory /app

# Custom FFmpeg - prefer container-built, fall back to host-built
if [ -f /usr/local/bin/ffmpeg-viewra ]; then
    # Container has FFmpeg built-in (from Dockerfile.dev multi-stage build)
    export VIEWRA_FFMPEG_PATH=/usr/local/bin/ffmpeg-viewra
    export VIEWRA_FFPROBE_PATH=/usr/local/bin/ffprobe-viewra
    echo "FFmpeg: container build"
elif [ -f /app/bin/ffmpeg-viewra ]; then
    # Fall back to host-built FFmpeg (for native dev without Docker)
    export VIEWRA_FFMPEG_PATH=/app/bin/ffmpeg-viewra
    export VIEWRA_FFPROBE_PATH=/app/bin/ffprobe-viewra
    export LD_LIBRARY_PATH="/app/bin/ffmpeg-lib:${LD_LIBRARY_PATH}"
    echo "FFmpeg: host build"
else
    echo "ERROR: Custom FFmpeg not found."
    echo "  Container build should be automatic. If missing, rebuild with:"
    echo "    docker compose -f docker-compose.dev.yml build --no-cache backend"
    exit 1
fi

# Subtitle extractor from bin/ (built with make build-tools)
if [ -f /app/bin/subtitle-extractor ]; then
    export SUBTITLE_EXTRACTOR_PATH=/app/bin/subtitle-extractor
    echo "Subtitle extractor: ready"
else
    echo "WARNING: subtitle-extractor not found (run 'make build-tools' on host)"
fi

# Plugins from data/plugins
if [ -d /app/data/plugins ] && [ -n "$(ls -A /app/data/plugins 2>/dev/null)" ]; then
    export PLUGINS_DIR=/app/data/plugins
    echo "Plugins: $(ls /app/data/plugins | wc -l) found"
else
    echo "Plugins: none (run 'make build-plugins' on host)"
fi

# GPU hardware acceleration detection and setup
detect_hw_accel() {
    # NVIDIA: check for mounted NVIDIA libraries (nvidia-container-toolkit)
    # nvidia-smi binary may not be mounted, but libnvidia-encode always is
    if [ -f /usr/lib/x86_64-linux-gnu/libnvidia-encode.so.1 ]; then
        echo "nvenc"
        return
    fi
    # Intel/AMD: /dev/dri with render nodes = VAAPI/QSV available
    if [ -d /dev/dri ] && ls /dev/dri/render* >/dev/null 2>&1; then
        echo "vaapi"
        return
    fi
    echo "none"
}

VIEWRA_HW_ACCEL=$(detect_hw_accel)
export VIEWRA_HW_ACCEL

# Setup OpenCL ICD for NVIDIA if NVIDIA libs are mounted
# nvidia-container-toolkit mounts the libs but not the ICD registration
if [ "$VIEWRA_HW_ACCEL" = "nvenc" ] && [ -f /usr/lib/x86_64-linux-gnu/libnvidia-opencl.so.1 ]; then
    mkdir -p /etc/OpenCL/vendors
    echo "libnvidia-opencl.so.1" > /etc/OpenCL/vendors/nvidia.icd
    echo "OpenCL: NVIDIA ICD registered"
fi

case "$VIEWRA_HW_ACCEL" in
    nvenc)  echo "HW Accel: NVIDIA NVENC" ;;
    vaapi)  echo "HW Accel: VAAPI (Intel QSV / AMD)" ;;
    none)   echo "HW Accel: CPU only (enable GPU in docker-compose.dev.yml)" ;;
esac

echo ""
exec "$@"

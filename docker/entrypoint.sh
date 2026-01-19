#!/bin/sh
set -e

# ViewRA Docker Entrypoint
# Handles initialization, migrations, and process startup

echo "=== ViewRA Container Starting ==="
echo "Environment: ${ENVIRONMENT:-production}"
echo "Data directory: ${DATA_DIR:-/data}"

# Ensure data directories exist with correct structure
DATA_DIR="${DATA_DIR:-/data}"
mkdir -p "${DATA_DIR}/cache/transcodes" \
         "${DATA_DIR}/cache/images" \
         "${DATA_DIR}/plugins" \
         "${DATA_DIR}/plugins/storage"

# Run database migrations if enabled (default: true)
if [ "${AUTO_MIGRATE:-true}" = "true" ]; then
    echo "Running database migrations..."
    if /app/viewra migrate 2>&1; then
        echo "Migrations completed successfully"
    else
        echo "Warning: Migration failed or already up-to-date"
    fi
fi

# Verify FFmpeg is available
if command -v ffmpeg >/dev/null 2>&1; then
    FFMPEG_VERSION=$(ffmpeg -version | head -n1)
    echo "FFmpeg: ${FFMPEG_VERSION}"
else
    echo "Warning: FFmpeg not found in PATH"
fi

# Verify subtitle-extractor is available
if [ -x "/app/subtitle-extractor" ]; then
    echo "Subtitle extractor: available"
else
    echo "Warning: subtitle-extractor not found"
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
    none)   echo "HW Accel: CPU only" ;;
esac

echo "=== Starting ViewRA ==="

# Execute the main process
# Use exec to replace shell with viewra (proper signal handling)
exec /app/viewra "$@"

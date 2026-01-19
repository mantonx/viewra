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

echo "=== Starting ViewRA ==="

# Execute the main process
# Use exec to replace shell with viewra (proper signal handling)
exec /app/viewra "$@"

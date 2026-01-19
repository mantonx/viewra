#!/bin/sh
set -e

echo "🚀 ViewRA Development Environment"

# Build tools if not already built (persisted in volume)
if [ ! -f /app/bin/ffmpeg-viewra ]; then
    echo "📦 Building custom FFmpeg (first run only, ~10 min)..."
    make build-ffmpeg
fi

if [ ! -f /app/bin/subtitle-extractor ]; then
    echo "📦 Building subtitle-extractor..."
    make build-tools
fi

if [ ! -d /app/bin/plugins ] || [ -z "$(ls -A /app/bin/plugins 2>/dev/null)" ]; then
    echo "📦 Building plugins..."
    make build-plugins
fi

# Set up library path for custom FFmpeg
export LD_LIBRARY_PATH="/app/bin/ffmpeg-lib:${LD_LIBRARY_PATH}"

# Configure paths for ViewRA
export FFMPEG_PATH=/app/bin/ffmpeg-viewra
export FFPROBE_PATH=/app/bin/ffprobe-viewra
export SUBTITLE_EXTRACTOR_PATH=/app/bin/subtitle-extractor
export PLUGINS_DIR=/app/bin/plugins

echo "✅ Tools ready:"
echo "   FFmpeg: $FFMPEG_PATH"
echo "   Subtitle extractor: $SUBTITLE_EXTRACTOR_PATH"
echo "   Plugins: $PLUGINS_DIR"
echo ""

# Run the provided command (default: air)
exec "$@"

#!/bin/bash

# Cleanup script for Viewra temporary files
# Run this regularly to prevent Cursor performance issues

echo "🧹 Cleaning up temporary transcoding files..."

# Clean transcoding temp files older than 1 hour
find viewra-data/ -type f \( -name "*.mp4" -o -name "*.mpd" -o -name "*.m3u8" -o -name "*.ts" -o -name "*.m4s" \) -mmin +60 -delete 2>/dev/null

# Clean old session directories
find viewra-data/transcoding/ -type d -name "dash_*" -mmin +60 -exec rm -rf {} + 2>/dev/null
find viewra-data/transcoding/ -type d -name "hls_*" -mmin +60 -exec rm -rf {} + 2>/dev/null

# Clean old log files
find . -name "*.log" -size +10M -mtime +1 -delete 2>/dev/null

# Clean backend temp files
rm -rf backend/tmp/* 2>/dev/null

echo "✅ Cleanup complete!"

# Show current disk usage
echo "📊 Current viewra-data size:"
du -sh viewra-data/ 2>/dev/null || echo "Directory not accessible" 
#!/bin/sh
# ViewRA health check script
# Returns 0 if healthy, 1 if unhealthy

curl -sf http://localhost:${PORT:-8080}/health || exit 1

# ViewRA Production Dockerfile
# Multi-stage build with pre-built dependencies
#
# Usage:
#   docker build -t viewra:latest .
#   docker compose up -d
#
# GPU support is runtime-configured via docker-compose.yml
#
# Dependencies:
#   Downloads pre-built FFmpeg, subtitle-extractor, and plugins from GitHub releases.
#   Run the build-deps GitHub Action first if the release doesn't exist.
#   To build locally: see tools/ffmpeg-viewra/build.sh, tools/subtitle-extractor/, plugins/

# =============================================================================
# Stage 1: Download pre-built dependencies (FFmpeg, subtitle-extractor, plugins)
# =============================================================================
FROM ubuntu:24.04 AS deps-downloader

ARG DEPS_RELEASE_TAG=deps-latest
ARG GITHUB_REPO=mantonx/viewra
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /deps

# Download and extract all pre-built dependencies
# Fails if release doesn't exist - run the build-deps GitHub Action first
RUN RELEASE_URL="https://github.com/${GITHUB_REPO}/releases/download/${DEPS_RELEASE_TAG}" && \
    echo "Downloading FFmpeg..." && \
    curl -fSL "${RELEASE_URL}/ffmpeg-viewra-linux-x64.tar.gz" -o ffmpeg.tar.gz && \
    mkdir -p ffmpeg && tar xzf ffmpeg.tar.gz -C ffmpeg && rm ffmpeg.tar.gz && \
    echo "Downloading subtitle-extractor..." && \
    curl -fSL "${RELEASE_URL}/subtitle-extractor-linux-x64.tar.gz" -o subtitle-extractor.tar.gz && \
    mkdir -p subtitle-extractor && tar xzf subtitle-extractor.tar.gz -C subtitle-extractor && rm subtitle-extractor.tar.gz && \
    echo "Downloading plugins..." && \
    curl -fSL "${RELEASE_URL}/viewra-plugins-linux-x64.tar.gz" -o plugins.tar.gz && \
    mkdir -p plugins && tar xzf plugins.tar.gz -C plugins && rm plugins.tar.gz && \
    # Verify critical files exist
    test -f ffmpeg/bin/ffmpeg || (echo "ERROR: FFmpeg binary not found" && exit 1) && \
    test -f subtitle-extractor/subtitle-extractor || (echo "ERROR: subtitle-extractor not found" && exit 1) && \
    echo "All dependencies downloaded successfully"

# =============================================================================
# Stage 2: Build frontend
# =============================================================================
FROM node:22-alpine AS frontend

WORKDIR /build/web

# Install dependencies first (better caching)
COPY web/package.json web/package-lock.json ./
RUN npm ci

# Build frontend
COPY web/ ./
RUN npm run build

# =============================================================================
# Stage 3: Build main application
# =============================================================================
FROM golang:1.25-bookworm AS backend-builder

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built frontend for embedding
COPY --from=frontend /build/web/dist ./web/dist

# Build with CGO enabled (required for sqlite-vec)
ARG VERSION=dev
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -X github.com/mantonx/viewra/internal/version.Version=${VERSION}" \
    -o viewra \
    ./cmd/viewra

# =============================================================================
# Stage 4: Final runtime image
# =============================================================================
FROM ubuntu:24.04 AS runtime

ARG DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    tzdata \
    # FFmpeg runtime dependencies
    libx264-164 \
    libx265-199 \
    libvpx9 \
    libsvtav1enc1d2 \
    libdav1d7 \
    libopus0 \
    libvorbis0a \
    libvorbisenc2 \
    libmp3lame0 \
    libfdk-aac2 \
    libass9 \
    libfreetype6 \
    libfontconfig1 \
    libssl3t64 \
    # Hardware acceleration runtime
    libva2 \
    libva-drm2 \
    libvdpau1 \
    libdrm2 \
    ocl-icd-libopencl1 \
    # Intel GPU support
    intel-media-va-driver \
    # AMD GPU support (mesa)
    mesa-va-drivers \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -u 1000 -m -s /sbin/nologin viewra

# Create app directory structure
WORKDIR /app

# Copy pre-built FFmpeg
COPY --from=deps-downloader /deps/ffmpeg/bin/ffmpeg /usr/local/bin/
COPY --from=deps-downloader /deps/ffmpeg/bin/ffprobe /usr/local/bin/
COPY --from=deps-downloader /deps/ffmpeg/lib/lib*.so* /usr/local/lib/

# Update library cache
RUN ldconfig

# Copy application binaries
COPY --from=backend-builder /build/viewra /app/
COPY --from=deps-downloader /deps/subtitle-extractor/subtitle-extractor /app/
COPY --from=deps-downloader /deps/plugins/ /app/plugins/

# Copy helper scripts
COPY docker/entrypoint.sh docker/healthcheck.sh /app/
RUN chmod +x /app/entrypoint.sh /app/healthcheck.sh

# Copy migrations
COPY migrations/ /app/migrations/

# Create data directory
RUN mkdir -p /data && chown viewra:viewra /data

# Set environment defaults
ENV ENVIRONMENT=production \
    GIN_MODE=release \
    DATA_DIR=/data \
    PORT=8080

# Switch to non-root user
USER viewra

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD ["/app/healthcheck.sh"]

ENTRYPOINT ["/app/entrypoint.sh"]

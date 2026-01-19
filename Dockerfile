# syntax=docker/dockerfile:1
# ViewRA Production Dockerfile
#
# Usage:
#   docker build -t viewra:latest .
#   docker compose up -d
#
# Pre-built FFmpeg and subtitle-extractor are downloaded from GitHub releases.
# Plugins are managed at runtime via the marketplace in /data/plugins.

# =============================================================================
# Stage 1: Download pre-built dependencies
# =============================================================================
FROM alpine:3.21 AS deps-downloader

ARG GITHUB_REPO=mantonx/viewra
ARG FFMPEG_VERSION=""
ARG SUBTITLE_EXTRACTOR_VERSION=""

RUN apk add --no-cache curl jq

WORKDIR /deps

# Download FFmpeg from GitHub releases
RUN if [ -n "$FFMPEG_VERSION" ]; then TAG="ffmpeg-v${FFMPEG_VERSION}"; else \
    TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases" | \
      jq -r '[.[] | select(.tag_name | startswith("ffmpeg-v"))] | sort_by(.created_at) | last | .tag_name'); \
    fi && \
    echo "Downloading FFmpeg ${TAG}..." && \
    curl -fSL "https://github.com/${GITHUB_REPO}/releases/download/${TAG}/ffmpeg-viewra-linux-x64.tar.gz" | tar xz && \
    test -f ffmpeg-viewra || (echo "ERROR: FFmpeg not found" && exit 1)

# Download subtitle-extractor from GitHub releases
RUN if [ -n "$SUBTITLE_EXTRACTOR_VERSION" ]; then TAG="subtitle-extractor-v${SUBTITLE_EXTRACTOR_VERSION}"; else \
    TAG=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases" | \
      jq -r '[.[] | select(.tag_name | startswith("subtitle-extractor-v"))] | sort_by(.created_at) | last | .tag_name'); \
    fi && \
    echo "Downloading subtitle-extractor ${TAG}..." && \
    curl -fSL "https://github.com/${GITHUB_REPO}/releases/download/${TAG}/subtitle-extractor-linux-x64.tar.gz" | tar xz && \
    test -f subtitle-extractor || (echo "ERROR: subtitle-extractor not found" && exit 1)

# =============================================================================
# Stage 2: Build frontend
# =============================================================================
FROM node:22-alpine AS frontend

WORKDIR /build/web

COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY web/ ./
RUN npm run build

# =============================================================================
# Stage 3: Build backend
# =============================================================================
FROM golang:1.25-bookworm AS backend-builder

WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy go.mod, go.sum, and local replace modules first for better caching
COPY go.mod go.sum ./
COPY api/proto/plugin/ api/proto/plugin/
COPY pkg/plugin/sdk/ pkg/plugin/sdk/

# Download dependencies (cached unless go.mod/go.sum change)
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy rest of source and frontend
COPY . .
COPY --from=frontend /build/web/dist ./web/dist

# Build with cache mounts
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build \
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
    # FFmpeg runtime - video codecs
    libx264-164 \
    libx265-199 \
    libvpx9 \
    libsvtav1enc1d1 \
    libdav1d7 \
    # FFmpeg runtime - audio codecs
    libopus0 \
    libvorbis0a \
    libvorbisenc2 \
    libmp3lame0 \
    libfdk-aac2 \
    # FFmpeg runtime - subtitles/text
    libass9 \
    libfreetype6 \
    libfontconfig1 \
    # SSL
    libssl3t64 \
    # VAAPI (Intel/AMD hardware encoding)
    libva2 \
    libva-drm2 \
    libva-x11-2 \
    libdrm2 \
    # Intel QSV (oneVPL)
    libvpl2 \
    # Vulkan + libplacebo (HDR tone mapping)
    libvulkan1 \
    libplacebo338 \
    libshaderc1 \
    # OpenCL (GPU-accelerated filters)
    ocl-icd-libopencl1 \
    # VDPAU (legacy NVIDIA)
    libvdpau1 \
    # Intel VAAPI driver
    intel-media-va-driver \
    # AMD VAAPI driver (mesa)
    mesa-va-drivers \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user (use existing UID 1000 if available, otherwise create)
RUN useradd -r -u 1000 -m -s /sbin/nologin viewra 2>/dev/null || \
    (userdel -r ubuntu 2>/dev/null; useradd -r -u 1000 -m -s /sbin/nologin viewra)

# Create app directory structure
WORKDIR /app

# Copy pre-built FFmpeg (tar extracts ffmpeg-viewra, ffprobe-viewra, ffmpeg-lib/)
COPY --from=deps-downloader /deps/ffmpeg-viewra /usr/local/bin/ffmpeg
COPY --from=deps-downloader /deps/ffprobe-viewra /usr/local/bin/ffprobe
COPY --from=deps-downloader /deps/ffmpeg-lib/ /usr/local/lib/

# Update library cache
RUN ldconfig

# Copy application binaries
COPY --from=backend-builder /build/viewra /app/
COPY --from=deps-downloader /deps/subtitle-extractor /app/

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

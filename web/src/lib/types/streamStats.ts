/**
 * Stream statistics types for the "Stats for Nerds" panel
 */

// Source file information (from media API)
export interface SourceStreamInfo {
  container: string           // MKV, MP4, AVI, etc.
  fileSize: number            // bytes
  bitrate: number             // bits per second
  duration: number            // seconds
}

export interface SourceVideoInfo {
  codec: string               // H264, HEVC, VP9, AV1
  profile?: string            // High, Main, Baseline
  level?: string              // 4.0, 4.1, 5.1
  width: number
  height: number
  bitrate?: number            // bits per second (if available separately)
  frameRate: number           // fps
  hdrFormat?: string          // HDR10, Dolby Vision, HLG, SDR
  colorSpace?: string         // bt709, bt2020nc
}

export interface SourceAudioInfo {
  codec: string               // AAC, EAC3, DTS, TrueHD
  channels: string            // stereo, 5.1, 7.1, Atmos
  language: string            // English, Spanish, etc.
  bitrate?: number            // bits per second
  sampleRate?: number         // Hz (48000, 44100)
  isDefault: boolean
  title?: string              // Track title if available
}

// Output/playback information
export type PlaybackMode = 'direct' | 'remux' | 'transcode'

export interface OutputStreamInfo {
  mode: PlaybackMode
  container: string           // HLS, MP4
  bitrate?: number            // Current output bitrate
}

export interface OutputVideoInfo {
  codec: string               // Output video codec
  bitrate?: number            // Target bitrate
  width?: number
  height?: number
  reason?: string             // "Quality setting", "Incompatible codec", etc.
}

export interface OutputAudioInfo {
  codec: string               // Output audio codec (AAC usually)
  bitrate?: number            // Target bitrate (192kbps, etc.)
  reason?: string             // "Incompatible codec", "Downmix to stereo"
}

// Playback quality stats (from browser)
export interface PlaybackQualityStats {
  droppedFrames: number
  totalFrames: number
  corruptedFrames: number
  droppedFrameRate: number    // percentage
}

// HLS-specific stats
export interface HLSStats {
  currentLevel: number        // Current quality level index
  currentLevelHeight?: number // e.g., 1080
  currentLevelBitrate?: number
  availableLevels: Array<{
    height: number
    bitrate: number
  }>
  fragmentLoadTime?: number   // ms for last fragment
  bandwidthEstimate?: number  // HLS.js bandwidth estimate
}

// Buffer information
export interface BufferStats {
  forwardBuffer: number       // seconds buffered ahead
  backBuffer: number          // seconds buffered behind
  ranges: Array<{
    start: number
    end: number
  }>
  hasGaps: boolean
}

// Session/debug info
export interface SessionStats {
  sessionId?: string
  streamOffset: number        // Current offset for seek debugging
  playbackRate: number        // 1, 1.5, 2, etc.
  timeSinceStart: number      // ms since playback started
}

// Combined stats object
export interface StreamStats {
  source: {
    stream: SourceStreamInfo
    video: SourceVideoInfo
    audio: SourceAudioInfo[]
  }
  output: {
    stream: OutputStreamInfo
    video?: OutputVideoInfo
    audio?: OutputAudioInfo
  }
  playback: PlaybackQualityStats
  hls?: HLSStats
  buffer: BufferStats
  session: SessionStats
}

// Helper to format bitrate for display
export const formatBitrate = (bps: number): string => {
  if (bps >= 1_000_000) {
    return `${(bps / 1_000_000).toFixed(1)} Mbps`
  }
  if (bps >= 1_000) {
    return `${(bps / 1_000).toFixed(0)} Kbps`
  }
  return `${bps} bps`
}

// Helper to format file size
export const formatFileSize = (bytes: number): string => {
  if (bytes >= 1_000_000_000) {
    return `${(bytes / 1_000_000_000).toFixed(2)} GB`
  }
  if (bytes >= 1_000_000) {
    return `${(bytes / 1_000_000).toFixed(1)} MB`
  }
  return `${(bytes / 1_000).toFixed(0)} KB`
}

// Helper to format resolution
export const formatResolution = (height: number): string => {
  if (height >= 2160) return '4K'
  if (height >= 1440) return '1440p'
  if (height >= 1080) return '1080p'
  if (height >= 720) return '720p'
  if (height >= 480) return '480p'
  return `${height}p`
}

// Helper to format frame rate
export const formatFrameRate = (fps: number): string => {
  // Common frame rates
  if (Math.abs(fps - 23.976) < 0.01) return '23.976'
  if (Math.abs(fps - 24) < 0.01) return '24'
  if (Math.abs(fps - 25) < 0.01) return '25'
  if (Math.abs(fps - 29.97) < 0.01) return '29.97'
  if (Math.abs(fps - 30) < 0.01) return '30'
  if (Math.abs(fps - 50) < 0.01) return '50'
  if (Math.abs(fps - 59.94) < 0.01) return '59.94'
  if (Math.abs(fps - 60) < 0.01) return '60'
  return fps.toFixed(2)
}

// Helper to get codec display name
export const getCodecDisplayName = (codec: string): string => {
  const codecMap: Record<string, string> = {
    'h264': 'H.264',
    'avc': 'H.264',
    'avc1': 'H.264',
    'hevc': 'H.265',
    'h265': 'H.265',
    'hvc1': 'H.265',
    'vp9': 'VP9',
    'vp09': 'VP9',
    'av1': 'AV1',
    'av01': 'AV1',
    'aac': 'AAC',
    'mp3': 'MP3',
    'ac3': 'AC3',
    'eac3': 'E-AC3',
    'ec3': 'E-AC3',
    'dts': 'DTS',
    'truehd': 'TrueHD',
    'flac': 'FLAC',
    'opus': 'Opus',
    'vorbis': 'Vorbis',
  }
  return codecMap[codec.toLowerCase()] || codec.toUpperCase()
}

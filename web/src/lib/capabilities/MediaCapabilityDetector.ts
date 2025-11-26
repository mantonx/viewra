import type { CodecCapability, CodecSupport } from './types'

// Codec test configurations with multiple profiles per codec
const CODEC_CONFIGS = {
  h264: {
    mimeType: 'video/mp4; codecs="avc1.64001F"', // H.264 High Profile
    profiles: [
      { width: 1920, height: 1080, fps: 30, bitrate: 5_000_000 },
      { width: 1920, height: 1080, fps: 60, bitrate: 8_000_000 },
      { width: 3840, height: 2160, fps: 30, bitrate: 15_000_000 },
      { width: 3840, height: 2160, fps: 60, bitrate: 25_000_000 },
    ],
  },
  h265: {
    mimeType: 'video/mp4; codecs="hev1.1.6.L153.B0"', // HEVC Main 10, Level 5.1 (4K60)
    profiles: [
      { width: 1920, height: 1080, fps: 30, bitrate: 3_000_000 },
      { width: 1920, height: 1080, fps: 60, bitrate: 5_000_000 },
      { width: 3840, height: 2160, fps: 30, bitrate: 10_000_000 },
      { width: 3840, height: 2160, fps: 60, bitrate: 15_000_000 },
    ],
  },
  vp9: {
    mimeType: 'video/webm; codecs="vp9"',
    profiles: [
      { width: 1920, height: 1080, fps: 30, bitrate: 3_000_000 },
      { width: 1920, height: 1080, fps: 60, bitrate: 5_000_000 },
      { width: 3840, height: 2160, fps: 30, bitrate: 10_000_000 },
      { width: 3840, height: 2160, fps: 60, bitrate: 15_000_000 },
    ],
  },
  av1: {
    mimeType: 'video/mp4; codecs="av01.0.12M.08"', // AV1 Main Profile, Level 5.1
    profiles: [
      { width: 1920, height: 1080, fps: 30, bitrate: 2_500_000 },
      { width: 1920, height: 1080, fps: 60, bitrate: 4_000_000 },
      { width: 3840, height: 2160, fps: 30, bitrate: 8_000_000 },
      { width: 3840, height: 2160, fps: 60, bitrate: 12_000_000 },
    ],
  },
} as const

type CodecName = 'h264' | 'h265' | 'vp9' | 'av1'

const canPlayCodec = (mimeType: string): boolean => {
  const video = document.createElement('video')
  const canPlay = video.canPlayType(mimeType)
  return canPlay === 'probably' || canPlay === 'maybe'
}

const createUnsupportedCodec = (codec: CodecCapability['codec']): CodecCapability => ({
  codec,
  supported: false,
  hardwareAccelerated: false,
  powerEfficient: false,
  smooth: false,
  maxWidth: 0,
  maxHeight: 0,
  maxFps: 0,
})

// Detect detailed capability for a single codec using Media Capabilities API
const detectCodecCapability = async (codec: CodecName): Promise<CodecCapability> => {
  const config = CODEC_CONFIGS[codec]

  // First check basic support with canPlayType
  if (!canPlayCodec(config.mimeType)) {
    return createUnsupportedCodec(codec)
  }

  // If Media Capabilities API not available, return basic support
  if (!('mediaCapabilities' in navigator)) {
    return {
      codec,
      supported: true,
      hardwareAccelerated: false,
      powerEfficient: false,
      smooth: true,
      maxWidth: 1920,
      maxHeight: 1080,
      maxFps: 30,
    }
  }

  // Test each profile from highest to lowest to find max capability
  let maxWidth = 0
  let maxHeight = 0
  let maxFps = 0
  let hardwareAccelerated = false
  let powerEfficient = false
  let smooth = false

  // Test profiles in reverse order (highest first)
  const profiles = [...config.profiles].reverse()

  for (const profile of profiles) {
    try {
      const result = await navigator.mediaCapabilities.decodingInfo({
        type: 'file',
        video: {
          contentType: config.mimeType,
          width: profile.width,
          height: profile.height,
          bitrate: profile.bitrate,
          framerate: profile.fps,
        },
      })

      if (result.supported) {
        // First supported profile is our max
        if (maxWidth === 0) {
          maxWidth = profile.width
          maxHeight = profile.height
          maxFps = profile.fps
          hardwareAccelerated = result.powerEfficient
          powerEfficient = result.powerEfficient
          smooth = result.smooth
        }

        // If this profile is smooth, it's definitely supported well
        if (result.smooth && profile.width >= maxWidth && profile.height >= maxHeight) {
          maxWidth = profile.width
          maxHeight = profile.height
          maxFps = profile.fps
          hardwareAccelerated = result.powerEfficient
          powerEfficient = result.powerEfficient
          smooth = true
        }
      }
    } catch {
      // Skip this profile
    }
  }

  // If no profiles were supported via Media Capabilities, use canPlayType result
  if (maxWidth === 0) {
    return {
      codec,
      supported: true,
      hardwareAccelerated: false,
      powerEfficient: false,
      smooth: true,
      maxWidth: 1920,
      maxHeight: 1080,
      maxFps: 30,
    }
  }

  return {
    codec,
    supported: true,
    hardwareAccelerated,
    powerEfficient,
    smooth,
    maxWidth,
    maxHeight,
    maxFps,
  }
}

// Get browser-specific notes for codec support
const getCodecNotes = (codec: CodecName): string | undefined => {
  const ua = navigator.userAgent.toLowerCase()

  if (codec === 'h265') {
    if (ua.includes('safari') && !ua.includes('chrome')) {
      return 'Native Safari support'
    }
    if (ua.includes('edg/')) {
      return 'Hardware decode required on Windows'
    }
    if (ua.includes('chrome') || ua.includes('firefox')) {
      return 'Limited support - may require hardware decode'
    }
  }

  if (codec === 'vp9') {
    if (ua.includes('safari') && !ua.includes('chrome')) {
      return 'Limited Safari support'
    }
  }

  if (codec === 'av1') {
    if (ua.includes('safari') && !ua.includes('chrome')) {
      return 'Safari 17+ required'
    }
    return 'Chrome 90+ / Firefox 98+ / Safari 17+'
  }

  return undefined
}

// Detect detailed codec support for all codecs
export const detectCodecSupport = async (): Promise<CodecSupport> => {
  // Detect all codecs in parallel
  const [h264, h265, vp9, av1] = await Promise.all([
    detectCodecCapability('h264'),
    detectCodecCapability('h265'),
    detectCodecCapability('vp9'),
    detectCodecCapability('av1'),
  ])

  // Add browser-specific notes
  h264.notes = getCodecNotes('h264')
  h265.notes = getCodecNotes('h265')
  vp9.notes = getCodecNotes('vp9')
  av1.notes = getCodecNotes('av1')

  // Determine preferred order based on:
  // 1. Must be supported
  // 2. Prefer better compression (AV1 > H.265/VP9 > H.264)
  // 3. Prefer hardware accelerated
  // 4. Prefer power efficient
  const codecs = [
    { cap: av1, name: 'av1' as const, compression: 4 },
    { cap: h265, name: 'h265' as const, compression: 3 },
    { cap: vp9, name: 'vp9' as const, compression: 3 },
    { cap: h264, name: 'h264' as const, compression: 1 },
  ]

  const preferredOrder = codecs
    .filter(c => c.cap.supported)
    .sort((a, b) => {
      // Primary sort: compression ratio (higher is better)
      if (a.compression !== b.compression) {
        return b.compression - a.compression
      }
      // Secondary: hardware acceleration
      if (a.cap.hardwareAccelerated !== b.cap.hardwareAccelerated) {
        return a.cap.hardwareAccelerated ? -1 : 1
      }
      // Tertiary: power efficiency
      if (a.cap.powerEfficient !== b.cap.powerEfficient) {
        return a.cap.powerEfficient ? -1 : 1
      }
      return 0
    })
    .map(c => c.name)

  // Ensure h264 is always last as fallback (if supported)
  const h264Index = preferredOrder.indexOf('h264')
  if (h264Index !== -1 && h264Index !== preferredOrder.length - 1) {
    preferredOrder.splice(h264Index, 1)
    preferredOrder.push('h264')
  }

  return {
    h264,
    h265,
    vp9,
    av1,
    preferredOrder,
  }
}

export const detectHardwareAcceleration = async (): Promise<boolean> => {
  if (!('mediaCapabilities' in navigator)) {
    return false
  }

  try {
    const result = await navigator.mediaCapabilities.decodingInfo({
      type: 'file',
      video: {
        contentType: CODEC_CONFIGS.h264.mimeType,
        width: 1920,
        height: 1080,
        bitrate: 5_000_000,
        framerate: 30,
      },
    })

    return result.smooth && result.powerEfficient
  } catch {
    return false
  }
}

export const detectMaxDecodingProfile = async (): Promise<string> => {
  if (!('mediaCapabilities' in navigator)) {
    return '1080p-30fps'
  }

  const profiles = [
    { width: 1920, height: 1080, fps: 30, name: '1080p-30fps' },
    { width: 1920, height: 1080, fps: 60, name: '1080p-60fps' },
    { width: 3840, height: 2160, fps: 30, name: '4k-30fps' },
    { width: 3840, height: 2160, fps: 60, name: '4k-60fps' },
  ]

  let maxProfile = '1080p-30fps'

  for (const profile of profiles) {
    try {
      const result = await navigator.mediaCapabilities.decodingInfo({
        type: 'file',
        video: {
          contentType: CODEC_CONFIGS.h264.mimeType,
          width: profile.width,
          height: profile.height,
          bitrate: (profile.width * profile.height * profile.fps) / 10,
          framerate: profile.fps,
        },
      })

      if (result.smooth) {
        maxProfile = profile.name
      } else {
        break
      }
    } catch {
      break
    }
  }

  return maxProfile
}

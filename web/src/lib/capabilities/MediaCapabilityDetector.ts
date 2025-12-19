import type { CodecCapability, CodecSupport, HDRCapability } from './types'

// Codec variants to probe - multiple profiles per codec for broad compatibility
const CODEC_VARIANTS = {
  // Video codecs
  h264: [
    'video/mp4; codecs="avc1.640028"', // High 4.0
    'video/mp4; codecs="avc1.64002a"', // High 4.2
    'video/mp4; codecs="avc1.64001F"', // High 3.1
  ],
  h265: [
    'video/mp4; codecs="hvc1.1.4.L120.B0"', // Main 8-bit
    'video/mp4; codecs="hvc1.2.4.L120.B0"', // Main 10-bit
    'video/mp4; codecs="hev1.1.6.L153.B0"', // Main 10 Level 5.1
  ],
  av1: [
    'video/mp4; codecs="av01.0.08M.08"', // Main 8-bit
    'video/mp4; codecs="av01.0.08M.10"', // Main 10-bit
    'video/mp4; codecs="av01.0.12M.08"', // Main Profile Level 5.1
  ],
  vp9: [
    'video/webm; codecs="vp09.00.30.08"',
    'video/webm; codecs="vp9"',
  ],
} as const

// Audio codecs for future use
const AUDIO_CODEC_VARIANTS = {
  aac: ['audio/mp4; codecs="mp4a.40.2"'],
  ac3: ['audio/mp4; codecs="ac-3"'],
  eac3: ['audio/mp4; codecs="ec-3"'],
  opus: ['audio/webm; codecs="opus"'],
  flac: ['audio/mp4; codecs="flac"'],
} as const

// Resolution/fps profiles for capability testing
const CAPABILITY_PROFILES = [
  { width: 1920, height: 1080, fps: 30, bitrate: 5_000_000 },
  { width: 1920, height: 1080, fps: 60, bitrate: 8_000_000 },
  { width: 3840, height: 2160, fps: 30, bitrate: 15_000_000 },
  { width: 3840, height: 2160, fps: 60, bitrate: 25_000_000 },
] as const

type CodecName = 'h264' | 'h265' | 'vp9' | 'av1'

// Check if MediaSource supports a codec (what HLS.js actually uses)
const isMediaSourceSupported = (mimeType: string): boolean => {
  if (typeof MediaSource === 'undefined') {
    return false
  }
  return MediaSource.isTypeSupported(mimeType)
}

// Check if any variant of a codec is supported
const probeCodecSupport = (variants: readonly string[]): { supported: boolean; supportedVariant: string | null } => {
  for (const variant of variants) {
    if (isMediaSourceSupported(variant)) {
      return { supported: true, supportedVariant: variant }
    }
  }
  return { supported: false, supportedVariant: null }
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

// Detect detailed capability for a single codec
const detectCodecCapability = async (codec: CodecName): Promise<CodecCapability> => {
  const variants = CODEC_VARIANTS[codec]
  const { supported, supportedVariant } = probeCodecSupport(variants)

  if (!supported || !supportedVariant) {
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
  const profiles = [...CAPABILITY_PROFILES].reverse()

  for (const profile of profiles) {
    try {
      const result = await navigator.mediaCapabilities.decodingInfo({
        type: 'media-source', // Use media-source type for HLS compatibility
        video: {
          contentType: supportedVariant,
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

  // If no profiles were supported via Media Capabilities, use basic detection result
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

// Probe all codec support (lightweight, for debugging)
export const probeAllCodecSupport = (): Record<string, Array<{ codec: string; supported: boolean }>> => {
  const allVariants = { ...CODEC_VARIANTS, ...AUDIO_CODEC_VARIANTS }
  const support: Record<string, Array<{ codec: string; supported: boolean }>> = {}

  for (const [name, variants] of Object.entries(allVariants)) {
    support[name] = variants.map((codec) => ({
      codec,
      supported: isMediaSourceSupported(codec),
    }))
  }

  return support
}

// Fast synchronous codec detection using MediaSource.isTypeSupported()
// This is what HLS.js actually uses, and it's instant (<1ms)
const detectCodecSupportFast = (codec: CodecName): CodecCapability => {
  const variants = CODEC_VARIANTS[codec]
  const { supported } = probeCodecSupport(variants)

  if (!supported) {
    return createUnsupportedCodec(codec)
  }

  // Return basic support info - detailed capability can be detected later if needed
  return {
    codec,
    supported: true,
    hardwareAccelerated: false, // Unknown without async check
    powerEfficient: false,
    smooth: true, // Assume smooth if supported
    maxWidth: 3840, // Assume 4K capable if supported
    maxHeight: 2160,
    maxFps: 60,
    notes: getCodecNotes(codec),
  }
}

// Build preferred order from supported codecs
const buildPreferredOrder = (h264: CodecCapability, h265: CodecCapability, vp9: CodecCapability, av1: CodecCapability): CodecSupport['preferredOrder'] => {
  const codecs = [
    { cap: av1, name: 'av1' as const, compression: 4 },
    { cap: h265, name: 'h265' as const, compression: 3 },
    { cap: vp9, name: 'vp9' as const, compression: 3 },
    { cap: h264, name: 'h264' as const, compression: 1 },
  ]

  const preferredOrder = codecs
    .filter((c) => c.cap.supported)
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
    .map((c) => c.name)

  // Ensure h264 is always last as fallback (if supported)
  const h264Index = preferredOrder.indexOf('h264')
  if (h264Index !== -1 && h264Index !== preferredOrder.length - 1) {
    preferredOrder.splice(h264Index, 1)
    preferredOrder.push('h264')
  }

  return preferredOrder
}

// Fast synchronous codec detection - instant, no async calls
// Uses MediaSource.isTypeSupported() which is what HLS.js uses anyway
export const detectCodecSupportSync = (): CodecSupport => {
  const h264 = detectCodecSupportFast('h264')
  const h265 = detectCodecSupportFast('h265')
  const vp9 = detectCodecSupportFast('vp9')
  const av1 = detectCodecSupportFast('av1')

  return {
    h264,
    h265,
    vp9,
    av1,
    preferredOrder: buildPreferredOrder(h264, h265, vp9, av1),
  }
}

// Detect detailed codec support for all codecs (async, slower but more accurate)
// Use detectCodecSupportSync() for instant results during playback start
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

  return {
    h264,
    h265,
    vp9,
    av1,
    preferredOrder: buildPreferredOrder(h264, h265, vp9, av1),
  }
}

export const detectHardwareAcceleration = async (): Promise<boolean> => {
  if (!('mediaCapabilities' in navigator)) {
    return false
  }

  const { supportedVariant } = probeCodecSupport(CODEC_VARIANTS.h264)
  if (!supportedVariant) {
    return false
  }

  try {
    const result = await navigator.mediaCapabilities.decodingInfo({
      type: 'media-source',
      video: {
        contentType: supportedVariant,
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

  const { supportedVariant } = probeCodecSupport(CODEC_VARIANTS.h264)
  if (!supportedVariant) {
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
        type: 'media-source',
        video: {
          contentType: supportedVariant,
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

// HDR codec strings for testing decode capability
const HDR_CODEC_VARIANTS = {
  // HEVC HDR10 (PQ transfer function, BT.2020 color primaries)
  hevcHDR10: 'video/mp4; codecs="hvc1.2.4.L153.B0"',
  // HEVC HLG
  hevcHLG: 'video/mp4; codecs="hvc1.2.4.L153.B0"',
  // VP9 Profile 2 (10-bit HDR)
  vp9HDR: 'video/webm; codecs="vp09.02.10.10.01.09.16.09.01"',
  // AV1 HDR10
  av1HDR: 'video/mp4; codecs="av01.0.13M.10.0.110.09.16.09.0"',
} as const

// Check for 10-bit color depth via WebGL (works on Linux where CSS media queries fail)
const detect10BitColorDepth = (): boolean => {
  try {
    const canvas = document.createElement('canvas')
    const gl = canvas.getContext('webgl2') || canvas.getContext('webgl')
    if (!gl) {
      return false
    }

    // Check if we can create a 10-bit framebuffer
    // This indicates the GPU/driver supports higher bit depth
    const ext = gl.getExtension('EXT_color_buffer_float')
    if (ext) {
      return true
    }

    // Alternative: check for half-float texture support (common on HDR-capable GPUs)
    const halfFloatExt = gl.getExtension('OES_texture_half_float')
    if (halfFloatExt) {
      return true
    }

    return false
  } catch {
    return false
  }
}

// Check localStorage for user HDR override (for Linux users with HDR displays)
const getHDROverride = (): boolean | null => {
  try {
    const override = localStorage.getItem('viewra_hdr_display_override')
    if (override === 'true') {
      return true
    }
    if (override === 'false') {
      return false
    }
    return null
  } catch {
    return null
  }
}

// Detect display HDR capability using multiple methods
// Priority: 1) User override, 2) CSS media query, 3) Wide gamut + 10-bit as fallback
const detectDisplayHDR = (): { supportsHDR: boolean; colorGamut: 'srgb' | 'p3' | 'rec2020'; detectionMethod: string } => {
  // Check user override first (for Linux users who know their display supports HDR)
  const override = getHDROverride()
  if (override !== null) {
    return {
      supportsHDR: override,
      colorGamut: override ? 'rec2020' : 'srgb',
      detectionMethod: 'user_override',
    }
  }

  // Primary: CSS media query (works on Windows/macOS with HDR enabled)
  const cssSupportsHDR = typeof matchMedia !== 'undefined' && matchMedia('(dynamic-range: high)').matches

  // Check color gamut support (progressively wider gamuts)
  let colorGamut: 'srgb' | 'p3' | 'rec2020' = 'srgb'
  if (typeof matchMedia !== 'undefined') {
    if (matchMedia('(color-gamut: rec2020)').matches) {
      colorGamut = 'rec2020'
    } else if (matchMedia('(color-gamut: p3)').matches) {
      colorGamut = 'p3'
    }
  }

  if (cssSupportsHDR) {
    return { supportsHDR: true, colorGamut, detectionMethod: 'css_media_query' }
  }

  // Fallback for Linux: Wide color gamut + 10-bit color depth suggests HDR capability
  // This is a heuristic - user can override via localStorage if wrong
  const has10Bit = detect10BitColorDepth()
  const hasWideGamut = colorGamut === 'rec2020' || colorGamut === 'p3'

  if (has10Bit && hasWideGamut) {
    return { supportsHDR: true, colorGamut, detectionMethod: 'webgl_10bit_wide_gamut' }
  }

  return { supportsHDR: false, colorGamut, detectionMethod: 'none' }
}

// Test if browser can decode HDR video with specific transfer function
const testHDRDecodeCapability = async (
  transferFunction: 'pq' | 'hlg',
  colorGamut: 'rec2020'
): Promise<boolean> => {
  if (!('mediaCapabilities' in navigator)) {
    return false
  }

  // Test with HEVC HDR (most common HDR format)
  const codec = transferFunction === 'pq' ? HDR_CODEC_VARIANTS.hevcHDR10 : HDR_CODEC_VARIANTS.hevcHLG

  try {
    const result = await navigator.mediaCapabilities.decodingInfo({
      type: 'media-source',
      video: {
        contentType: codec,
        width: 3840,
        height: 2160,
        bitrate: 40_000_000,
        framerate: 24,
        transferFunction,
        colorGamut,
      },
    })
    return result.supported
  } catch {
    // Some browsers don't support transferFunction/colorGamut params yet
    return false
  }
}

// Synchronous HDR display detection (instant, for initial checks)
// Returns detection method for debugging (css_media_query, webgl_10bit_wide_gamut, user_override, none)
export const detectHDRDisplaySync = (): Pick<HDRCapability, 'displaySupportsHDR' | 'colorGamut'> & { detectionMethod: string } => {
  const { supportsHDR, colorGamut, detectionMethod } = detectDisplayHDR()
  return {
    displaySupportsHDR: supportsHDR,
    colorGamut,
    detectionMethod,
  }
}

// Set HDR display override (for Linux users with HDR displays where detection fails)
// Call from browser console: setHDROverride(true) to enable, setHDROverride(false) to disable, setHDROverride(null) to clear
// Returns a message indicating the result
export const setHDROverride = (enabled: boolean | null): string => {
  try {
    if (enabled === null) {
      localStorage.removeItem('viewra_hdr_display_override')
      return '[HDR] Override cleared. Refresh page to use auto-detection.'
    } else {
      localStorage.setItem('viewra_hdr_display_override', String(enabled))
      return `[HDR] Override set to ${enabled}. Refresh page to apply.`
    }
  } catch (e) {
    console.error('[HDR] Failed to set override:', e)
    return '[HDR] Failed to set override. Check console for details.'
  }
}

// Expose setHDROverride globally for console access
if (typeof window !== 'undefined') {
  (window as Window & { setHDROverride?: typeof setHDROverride }).setHDROverride = setHDROverride
}

// Full async HDR capability detection
export const detectHDRCapability = async (): Promise<HDRCapability> => {
  const { supportsHDR: displaySupportsHDR, colorGamut } = detectDisplayHDR()

  // Test decode capabilities for different HDR formats
  const [canDecodePQ, canDecodeHLG] = await Promise.all([
    testHDRDecodeCapability('pq', 'rec2020'),
    testHDRDecodeCapability('hlg', 'rec2020'),
  ])

  const canDecodeHDR = canDecodePQ || canDecodeHLG

  // Dolby Vision support is harder to detect - assume supported if PQ works
  // and we have a wide color gamut display
  const supportsDolbyVision = canDecodePQ && colorGamut === 'rec2020'

  // Can play HDR natively only if BOTH display and decode support exist
  const canPlayHDRNatively = displaySupportsHDR && canDecodeHDR

  return {
    displaySupportsHDR,
    supportsHDR10: canDecodePQ && displaySupportsHDR,
    supportsHLG: canDecodeHLG && displaySupportsHDR,
    supportsDolbyVision: supportsDolbyVision && displaySupportsHDR,
    colorGamut,
    canDecodeHDR,
    canDecodePQ,
    canDecodeHLG,
    canPlayHDRNatively,
  }
}

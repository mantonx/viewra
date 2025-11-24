export class MediaCapabilityDetector {
  async detectCodecSupport(): Promise<string[]> {
    const codecs = [
      'video/mp4; codecs="avc1.42E01E"', // H.264 Baseline
      'video/mp4; codecs="avc1.4D401E"', // H.264 Main
      'video/mp4; codecs="avc1.64001F"', // H.264 High
      'video/mp4; codecs="hev1.1.6.L93.B0"', // H.265/HEVC
      'video/webm; codecs="vp8"', // VP8
      'video/webm; codecs="vp9"', // VP9
      'video/mp4; codecs="av01.0.05M.08"', // AV1
    ]

    const supported: string[] = []

    for (const codec of codecs) {
      if (this.canPlayCodec(codec)) {
        supported.push(this.extractCodecName(codec))
      }
    }

    return supported
  }

  private canPlayCodec(mimeType: string): boolean {
    const video = document.createElement('video')
    const canPlay = video.canPlayType(mimeType)
    return canPlay === 'probably' || canPlay === 'maybe'
  }

  private extractCodecName(mimeType: string): string {
    // Extract codec name from MIME type
    const match = mimeType.match(/codecs="([^"]+)"/)
    if (!match) {
      return 'unknown'
    }

    const codec = match[1]
    if (codec.startsWith('avc1')) {
      return 'h264'
    }
    if (codec.startsWith('hev1') || codec.startsWith('hvc1')) {
      return 'h265'
    }
    if (codec.startsWith('vp8')) {
      return 'vp8'
    }
    if (codec.startsWith('vp9')) {
      return 'vp9'
    }
    if (codec.startsWith('av01')) {
      return 'av1'
    }
    return 'unknown'
  }

  async detectHardwareAcceleration(): Promise<boolean> {
    // Check if Media Capabilities API is available
    if (!('mediaCapabilities' in navigator)) {
      return false // Assume no HW accel if API not available
    }

    try {
      // Test with a common configuration
      const config = {
        type: 'file' as const,
        video: {
          contentType: 'video/mp4; codecs="avc1.42E01E"',
          width: 1920,
          height: 1080,
          bitrate: 5000000,
          framerate: 30,
        },
      }

      const result = await navigator.mediaCapabilities.decodingInfo(config)

      // If smooth and power efficient, likely using HW acceleration
      return result.smooth && result.powerEfficient
    } catch {
      return false
    }
  }

  async detectMaxDecodingProfile(): Promise<string> {
    // Test progressively higher profiles
    const profiles = [
      { width: 1920, height: 1080, fps: 30, name: '1080p-30fps' },
      { width: 1920, height: 1080, fps: 60, name: '1080p-60fps' },
      { width: 3840, height: 2160, fps: 30, name: '4k-30fps' },
      { width: 3840, height: 2160, fps: 60, name: '4k-60fps' },
    ]

    if (!('mediaCapabilities' in navigator)) {
      return '1080p-30fps' // Conservative default
    }

    let maxProfile = '1080p-30fps'

    for (const profile of profiles) {
      try {
        const config = {
          type: 'file' as const,
          video: {
            contentType: 'video/mp4; codecs="avc1.64001F"',
            width: profile.width,
            height: profile.height,
            bitrate: (profile.width * profile.height * profile.fps) / 10, // Rough estimate
            framerate: profile.fps,
          },
        }

        const result = await navigator.mediaCapabilities.decodingInfo(config)

        if (result.smooth) {
          maxProfile = profile.name
        } else {
          break // Stop at first non-smooth profile
        }
      } catch {
        break
      }
    }

    return maxProfile
  }
}

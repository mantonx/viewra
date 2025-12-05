/**
 * SubtitleOverlay Component
 * Renders subtitles as a positioned overlay above video controls.
 *
 * Supports two rendering modes:
 * - Text subtitles (SRT/ASS/VTT): Parses WebVTT and renders as styled text
 * - Bitmap subtitles (PGS): Fetches WebP images and renders as positioned overlays
 */

import { useEffect, useState, useRef, useMemo } from 'react'
import { authFetch } from '@/lib/utils/authFetch'

// Text subtitle cue (from WebVTT)
interface TextCue {
  start: number
  end: number
  text: string
}

// PGS subtitle frame (from API)
interface PGSFrame {
  start_ms: number
  end_ms: number
  x: number
  y: number
  width: number
  height: number
  canvas_width: number // PGS authoring resolution (e.g., 1920 for 1080p subs on 4K video)
  canvas_height: number // PGS authoring resolution (e.g., 1080 for 1080p subs on 4K video)
  image_base64: string
}

interface SubtitleOverlayProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  mediaId: number
  trackId: number | null
  /** Whether this is a bitmap (PGS) subtitle track */
  isBitmap?: boolean
  /** Stream index for embedded subtitles - enables fast streaming extraction */
  streamIndex?: number
  /** Relative index among bitmap tracks (for PGS API) */
  bitmapIndex?: number
  streamOffsetRef: React.RefObject<number>
  /** Manual subtitle timing adjustment in seconds (positive = delay subtitles, negative = advance) */
  subtitleDelay?: number
}

// Allowed HTML tags in subtitles (VTT supports formatting)
const ALLOWED_TAGS = ['b', 'i', 'u', 'c', 'ruby', 'rt', 'v', 'lang']

/**
 * Sanitize subtitle text to prevent XSS while preserving allowed formatting.
 * Strips all tags except those in ALLOWED_TAGS, then converts newlines to <br/>.
 */
const sanitizeSubtitleText = (text: string): string => {
  // First, escape any raw < or > that aren't part of valid tags
  // Then strip disallowed tags while keeping allowed ones
  const tagPattern = /<\/?([a-zA-Z][a-zA-Z0-9]*)[^>]*>/g

  const sanitized = text.replace(tagPattern, (match, tagName) => {
    const tag = tagName.toLowerCase()
    if (ALLOWED_TAGS.includes(tag)) {
      // Keep the tag but strip attributes (except class for <c>)
      if (tag === 'c') {
        // VTT <c> tags can have class for styling
        const classMatch = match.match(/class="([^"]*)"/)
        return classMatch ? `<span class="${classMatch[1]}">` : '<span>'
      }
      // Self-closing or end tag
      return match.startsWith('</') ? `</${tag}>` : `<${tag}>`
    }
    // Strip disallowed tags by returning empty string
    return ''
  })

  // Convert newlines to <br/> and escape any remaining dangerous characters
  return sanitized
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/&lt;(\/?(b|i|u|span|ruby|rt|v|lang))&gt;/g, '<$1>') // Restore allowed tags
    .replace(/\n/g, '<br/>')
}

// Parse VTT timestamp to seconds
const parseTimestamp = (ts: string): number => {
  const parts = ts.split(':')
  if (parts.length === 3) {
    return parseInt(parts[0]) * 3600 + parseInt(parts[1]) * 60 + parseFloat(parts[2])
  } else if (parts.length === 2) {
    return parseInt(parts[0]) * 60 + parseFloat(parts[1])
  }
  return parseFloat(ts)
}

// Parse VTT content into cues
const parseVTT = (content: string): TextCue[] => {
  const cues: TextCue[] = []
  const lines = content.split('\n')
  let i = 0

  // Skip header
  while (i < lines.length && !lines[i].includes('-->')) {
    i++
  }

  while (i < lines.length) {
    const line = lines[i].trim()

    // Look for timestamp line
    if (line.includes('-->')) {
      const match = line.match(/([\d:.]+)\s*-->\s*([\d:.]+)/)
      if (match) {
        const start = parseTimestamp(match[1])
        const end = parseTimestamp(match[2])

        // Collect text lines until empty line
        const textLines: string[] = []
        i++
        while (i < lines.length && lines[i].trim() !== '') {
          textLines.push(lines[i].trim())
          i++
        }

        if (textLines.length > 0) {
          cues.push({ start, end, text: textLines.join('\n') })
        }
      }
    }
    i++
  }

  return cues
}

/**
 * Text Subtitle Renderer
 * Handles WebVTT-based subtitles (SRT, ASS, VTT)
 */
const TextSubtitleRenderer = ({
  videoRef,
  mediaId,
  trackId,
  streamIndex,
  streamOffsetRef,
  subtitleDelay = 0,
}: Omit<SubtitleOverlayProps, 'isBitmap' | 'bitmapIndex'>) => {
  const [cues, setCues] = useState<TextCue[]>([])
  const [currentText, setCurrentText] = useState('')
  const lastTrackIdRef = useRef<number | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Fetch and parse subtitle when track changes
  useEffect(() => {
    if (trackId === null) {
      setCues([])
      setCurrentText('')
      lastTrackIdRef.current = null
      return
    }

    // Don't refetch if same track
    if (trackId === lastTrackIdRef.current) {
      return
    }

    // Abort previous request if still in progress
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }

    lastTrackIdRef.current = trackId
    setCues([]) // Clear old cues while loading

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    const fetchSubtitle = async () => {
      try {
        // Use fast streaming endpoint when streamIndex is available (embedded subtitles)
        const url =
          streamIndex !== undefined
            ? `/api/media/${mediaId}/subtitles/text/${streamIndex}/stream`
            : `/api/media/${mediaId}/subtitles/${trackId}`

        const response = await authFetch(url, {
          signal: abortController.signal,
        })
        if (!response.ok) {
          return
        }

        const vttContent = await response.text()
        const parsedCues = parseVTT(vttContent)
        setCues(parsedCues)
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') {
          return
        }
        console.error('Failed to load text subtitle:', err)
      }
    }

    fetchSubtitle()

    return () => {
      abortController.abort()
    }
  }, [mediaId, trackId, streamIndex])

  // Use requestAnimationFrame for precise subtitle timing
  useEffect(() => {
    const video = videoRef.current
    if (!video || cues.length === 0) {
      return
    }

    let animationId: number
    let lastText = ''

    const updateCue = () => {
      const actualTime = video.currentTime + (streamOffsetRef.current || 0) - subtitleDelay
      const activeCue = cues.find((c) => actualTime >= c.start && actualTime < c.end)
      const newText = activeCue?.text ?? ''

      if (newText !== lastText) {
        lastText = newText
        setCurrentText(newText)
      }

      animationId = requestAnimationFrame(updateCue)
    }

    animationId = requestAnimationFrame(updateCue)
    return () => cancelAnimationFrame(animationId)
  }, [videoRef, cues, streamOffsetRef, subtitleDelay])

  if (!currentText) {
    return null
  }

  return (
    <div className="absolute left-0 right-0 bottom-32 flex justify-center pointer-events-none z-10 px-16">
      <div
        className="text-white text-center"
        style={{
          fontSize: '2.75rem',
          fontWeight: 400,
          lineHeight: 1.5,
          letterSpacing: '0.02em',
          textShadow: '0 0 8px rgba(0,0,0,0.9), 0 0 16px rgba(0,0,0,0.7), 2px 2px 4px rgba(0,0,0,0.9)',
        }}
        dangerouslySetInnerHTML={{ __html: sanitizeSubtitleText(currentText) }}
      />
    </div>
  )
}

// Micro-window for immediate subtitle display after seek (5 seconds)
// This gives near-instant feedback while the full window loads in background
const PGS_MICRO_WINDOW_MS = 5 * 1000
// Full window size for progressive PGS fetching (2 minutes in ms)
const PGS_WINDOW_MS = 2 * 60 * 1000
// Prefetch threshold - start fetching next window when this close to end (60 seconds)
const PGS_PREFETCH_THRESHOLD_MS = 60 * 1000

/**
 * PGS (Bitmap) Subtitle Renderer
 * Handles PGS/VobSub subtitles rendered as WebP images.
 * Uses progressive windowed fetching to minimize initial load time and memory usage.
 */
const PGSSubtitleRenderer = ({
  videoRef,
  mediaId,
  trackId,
  bitmapIndex,
  streamOffsetRef,
  subtitleDelay = 0,
}: Omit<SubtitleOverlayProps, 'isBitmap' | 'streamIndex'>) => {
  const [frames, setFrames] = useState<PGSFrame[]>([])
  const [currentFrame, setCurrentFrame] = useState<PGSFrame | null>(null)
  // Ref to access latest frames in animation loop without triggering effect restart
  const framesRef = useRef<PGSFrame[]>([])
  // Native video dimensions (e.g., 1920x1080 for Blu-ray)
  const [videoSize, setVideoSize] = useState({ width: 0, height: 0 })
  // Current display dimensions of the video element (changes on resize)
  const [elementSize, setElementSize] = useState({ width: 0, height: 0 })
  const abortControllerRef = useRef<AbortController | null>(null)
  // Track which full windows we've already fetched
  const fetchedWindowsRef = useRef<Set<number>>(new Set())
  // Track which micro-windows we've fetched (for immediate display after seek)
  const fetchedMicroWindowsRef = useRef<Set<string>>(new Set())

  // Keep framesRef in sync with frames state
  useEffect(() => {
    framesRef.current = frames
  }, [frames])

  // Parse NDJSON stream into frames
  const parseNDJSON = async (response: Response): Promise<PGSFrame[]> => {
    const text = await response.text()
    const lines = text.trim().split('\n').filter(Boolean)
    return lines.map((line) => JSON.parse(line) as PGSFrame)
  }

  // Fetch a specific time window of PGS frames
  const fetchWindow = async (startMs: number, endMs: number, signal: AbortSignal, bmpIndex: number): Promise<PGSFrame[]> => {
    const url = `/api/media/${mediaId}/subtitles/pgs/${bmpIndex}/stream?start=${startMs}&end=${endMs}`
    const response = await authFetch(url, { signal })
    if (!response.ok) {
      console.error('Failed to fetch PGS frames:', response.status, await response.text())
      return []
    }
    return await parseNDJSON(response)
  }

  // Merge new frames into state, avoiding duplicates
  const mergeFrames = (newFrames: PGSFrame[]) => {
    if (newFrames.length === 0) return
    setFrames((prev) => {
      const existingStarts = new Set(prev.map((f) => f.start_ms))
      const uniqueNew = newFrames.filter((f) => !existingStarts.has(f.start_ms))
      if (uniqueNew.length === 0) return prev
      const merged = [...prev, ...uniqueNew]
      return merged.sort((a, b) => a.start_ms - b.start_ms)
    })
  }

  // Calculate which window a timestamp falls into
  const getWindowStart = (timeMs: number): number => {
    return Math.floor(timeMs / PGS_WINDOW_MS) * PGS_WINDOW_MS
  }

  // Two-phase fetching: micro-window first for instant display, then full window in background
  // actualTimeMs is the current playback position for micro-window targeting
  const ensureWindowLoaded = async (windowStart: number, signal: AbortSignal, bmpIndex: number, actualTimeMs?: number) => {
    // Check if full window already fetched
    if (fetchedWindowsRef.current.has(windowStart)) {
      return
    }

    const windowEnd = windowStart + PGS_WINDOW_MS
    // Use actual position for micro-window if provided, otherwise use window start
    const microStart = actualTimeMs !== undefined ? Math.floor(actualTimeMs / 1000) * 1000 : windowStart
    const microKey = `${microStart}-micro`

    // Phase 1: Fetch micro-window immediately for fast subtitle display
    if (!fetchedMicroWindowsRef.current.has(microKey)) {
      fetchedMicroWindowsRef.current.add(microKey)
      try {
        const microEnd = microStart + PGS_MICRO_WINDOW_MS
        const microFrames = await fetchWindow(microStart, microEnd, signal, bmpIndex)
        if (!signal.aborted) {
          mergeFrames(microFrames)
        }
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') return
        console.error('Failed to fetch PGS micro-window:', err)
      }
    }

    // Phase 2: Fetch full window in background (will include micro-window frames too, but we dedupe)
    if (!fetchedWindowsRef.current.has(windowStart)) {
      fetchedWindowsRef.current.add(windowStart)
      try {
        const fullFrames = await fetchWindow(windowStart, windowEnd, signal, bmpIndex)
        if (!signal.aborted) {
          mergeFrames(fullFrames)
        }
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') return
        console.error('Failed to fetch PGS window:', err)
      }
    }
  }

  // Reset and fetch initial window when track changes
  useEffect(() => {
    if (trackId === null || bitmapIndex === undefined) {
      setFrames([])
      setCurrentFrame(null)
      fetchedWindowsRef.current.clear()
      fetchedMicroWindowsRef.current.clear()
      return
    }

    // Abort previous requests
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
    }

    setFrames([])
    setCurrentFrame(null)
    fetchedWindowsRef.current.clear()
    fetchedMicroWindowsRef.current.clear()

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    // Get initial playback position and fetch that window
    const video = videoRef.current
    const initialTimeMs = video ? (video.currentTime + (streamOffsetRef.current || 0)) * 1000 : 0
    const initialWindow = getWindowStart(initialTimeMs)

    ensureWindowLoaded(initialWindow, abortController.signal, bitmapIndex)

    return () => {
      abortController.abort()
    }
    // Note: videoRef and streamOffsetRef are stable refs, ensureWindowLoaded is defined in component
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mediaId, trackId, bitmapIndex])

  // Track video size (native resolution) and element size (display size)
  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    const updateVideoSize = () => {
      setVideoSize({ width: video.videoWidth, height: video.videoHeight })
    }

    const updateElementSize = () => {
      setElementSize({ width: video.clientWidth, height: video.clientHeight })
    }

    // Update native video size when metadata loads
    video.addEventListener('loadedmetadata', updateVideoSize)
    updateVideoSize()

    // Use ResizeObserver to track element size changes (window resize, fullscreen, etc.)
    const resizeObserver = new ResizeObserver(() => {
      updateElementSize()
    })
    resizeObserver.observe(video)
    updateElementSize()

    return () => {
      video.removeEventListener('loadedmetadata', updateVideoSize)
      resizeObserver.disconnect()
    }
  }, [videoRef])

  // Use requestAnimationFrame for precise frame timing and prefetching
  // Animation loop for frame timing - uses framesRef to avoid effect restart on frame updates
  useEffect(() => {
    const video = videoRef.current
    if (!video || bitmapIndex === undefined) {
      return
    }

    let animationId: number
    let lastFrameStartMs = -1

    const updateFrame = () => {
      // Convert to milliseconds
      const actualTimeMs = (video.currentTime + (streamOffsetRef.current || 0) - subtitleDelay) * 1000
      const currentFrames = framesRef.current

      // Check if we need to prefetch the next window
      const currentWindow = getWindowStart(actualTimeMs)
      const nextWindow = currentWindow + PGS_WINDOW_MS
      const timeUntilNextWindow = nextWindow - actualTimeMs

      if (timeUntilNextWindow < PGS_PREFETCH_THRESHOLD_MS && abortControllerRef.current && bitmapIndex !== undefined) {
        ensureWindowLoaded(nextWindow, abortControllerRef.current.signal, bitmapIndex, nextWindow)
      }

      // Also ensure current window is loaded (for seeks)
      // Pass actualTimeMs so micro-window targets the exact seek position
      if (!fetchedWindowsRef.current.has(currentWindow) && abortControllerRef.current && bitmapIndex !== undefined) {
        ensureWindowLoaded(currentWindow, abortControllerRef.current.signal, bitmapIndex, actualTimeMs)
      }

      // Find active frame
      if (currentFrames.length > 0) {
        let activeFrame: PGSFrame | null = null
        for (const frame of currentFrames) {
          if (actualTimeMs >= frame.start_ms && actualTimeMs < frame.end_ms) {
            activeFrame = frame
            break
          }
        }

        // Only update state if frame changed
        if (activeFrame) {
          if (activeFrame.start_ms !== lastFrameStartMs) {
            lastFrameStartMs = activeFrame.start_ms
            setCurrentFrame(activeFrame)
          }
        } else if (lastFrameStartMs !== -1) {
          lastFrameStartMs = -1
          setCurrentFrame(null)
        }
      }

      animationId = requestAnimationFrame(updateFrame)
    }

    animationId = requestAnimationFrame(updateFrame)
    return () => cancelAnimationFrame(animationId)
  }, [videoRef, streamOffsetRef, subtitleDelay, bitmapIndex])

  // Calculate the actual video content area (accounting for letterboxing from object-fit: contain)
  // and scaling factors for PGS positioning
  const videoLayout = useMemo(() => {
    if (videoSize.width === 0 || videoSize.height === 0 || elementSize.width === 0 || elementSize.height === 0) {
      return { offsetX: 0, offsetY: 0, scale: 1 }
    }

    const videoAspect = videoSize.width / videoSize.height
    const elementAspect = elementSize.width / elementSize.height

    let renderWidth: number
    let renderHeight: number

    if (videoAspect > elementAspect) {
      // Video is wider than element - letterbox top/bottom
      renderWidth = elementSize.width
      renderHeight = elementSize.width / videoAspect
    } else {
      // Video is taller than element - letterbox left/right (pillarbox)
      renderHeight = elementSize.height
      renderWidth = elementSize.height * videoAspect
    }

    // Calculate offset to center the video content
    const offsetX = (elementSize.width - renderWidth) / 2
    const offsetY = (elementSize.height - renderHeight) / 2

    // Scale factor from native video resolution to rendered size
    const scale = renderWidth / videoSize.width

    return { offsetX, offsetY, scale }
  }, [videoSize, elementSize])

  if (!currentFrame) {
    return null
  }

  // Create data URL for the WebP image
  const imageUrl = `data:image/webp;base64,${currentFrame.image_base64}`

  // Position the subtitle image within the actual video content area
  // PGS coordinates are relative to the PGS canvas resolution (e.g., 1920x1080 for most Blu-rays)
  // which may differ from the video resolution (e.g., 4K video with 1080p subtitles)
  //
  // We need to:
  // 1. Scale from canvas resolution to video resolution (if different)
  // 2. Then scale from video resolution to rendered size (videoLayout.scale handles this)
  // 3. Apply letterbox offset

  // Calculate the scale factor from PGS canvas to video resolution
  // If canvas dimensions match video, this is 1.0
  // If canvas is 1080p and video is 4K, this is 2.0
  const canvasToVideoScale =
    currentFrame.canvas_width > 0 && videoSize.width > 0
      ? videoSize.width / currentFrame.canvas_width
      : 1

  // Combined scale from canvas to rendered size
  const totalScale = canvasToVideoScale * videoLayout.scale

  const style: React.CSSProperties = {
    position: 'absolute',
    left: videoLayout.offsetX + currentFrame.x * totalScale,
    top: videoLayout.offsetY + currentFrame.y * totalScale,
    width: currentFrame.width * totalScale,
    height: currentFrame.height * totalScale,
    pointerEvents: 'none',
  }

  return (
    <div className="absolute inset-0 pointer-events-none z-10">
      <img src={imageUrl} alt="" style={style} />
    </div>
  )
}

/**
 * Main SubtitleOverlay component
 * Routes to the appropriate renderer based on subtitle type
 */
export const SubtitleOverlay = (props: SubtitleOverlayProps) => {
  const { isBitmap, ...rest } = props

  if (props.trackId === null) {
    return null
  }

  if (isBitmap) {
    return <PGSSubtitleRenderer {...rest} />
  }

  return <TextSubtitleRenderer {...rest} />
}

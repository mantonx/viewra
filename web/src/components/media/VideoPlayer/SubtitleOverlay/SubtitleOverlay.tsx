/**
 * SubtitleOverlay Component
 * Renders subtitles as a positioned overlay above video controls.
 *
 * Supports two rendering modes:
 * - Text subtitles (SRT/ASS/VTT): Parses WebVTT and renders as styled text
 * - Bitmap subtitles (PGS): Fetches WebP images and renders as positioned overlays
 */

import { useEffect, useState, useMemo, useCallback } from 'react'
import { authFetch } from '@/lib/utils/authFetch'
import {
  useWindowedFetching,
  useSubtitleAnimation,
  FULL_WINDOW_MS,
  PREFETCH_THRESHOLD_MS,
} from '@/lib/hooks/useWindowedFetching'
import { subtitleUrls } from '@/lib/types/subtitles'
import type {
  TextCue,
  PGSFrame,
  SubtitleOverlayProps,
  TextSubtitleRendererProps,
  PGSSubtitleRendererProps,
} from './SubtitleOverlay.types'

// Allowed HTML tags in subtitles
const ALLOWED_TAGS = ['b', 'i', 'u', 'c', 'ruby', 'rt', 'v', 'lang']

const sanitizeSubtitleText = (text: string): string => {
  const tagPattern = /<\/?([a-zA-Z][a-zA-Z0-9]*)[^>]*>/g

  const sanitized = text.replace(tagPattern, (match, tagName) => {
    const tag = tagName.toLowerCase()
    if (ALLOWED_TAGS.includes(tag)) {
      if (tag === 'c') {
        const classMatch = match.match(/class="([^"]*)"/)
        return classMatch ? `<span class="${classMatch[1]}">` : '<span>'
      }
      return match.startsWith('</') ? `</${tag}>` : `<${tag}>`
    }
    return ''
  })

  return sanitized
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/&lt;(\/?(b|i|u|span|ruby|rt|v|lang))&gt;/g, '<$1>')
    .replace(/\n/g, '<br/>')
}

const parseTimestamp = (ts: string): number => {
  const parts = ts.split(':')
  if (parts.length === 3) {
    return parseInt(parts[0]) * 3600 + parseInt(parts[1]) * 60 + parseFloat(parts[2])
  } else if (parts.length === 2) {
    return parseInt(parts[0]) * 60 + parseFloat(parts[1])
  }
  return parseFloat(ts)
}

const parseVTT = (content: string): TextCue[] => {
  const cues: TextCue[] = []
  const lines = content.split('\n')
  let i = 0

  while (i < lines.length && !lines[i].includes('-->')) {
    i++
  }

  while (i < lines.length) {
    const line = lines[i].trim()

    if (line.includes('-->')) {
      const match = line.match(/([\d:.]+)\s*-->\s*([\d:.]+)/)
      if (match) {
        const start = parseTimestamp(match[1])
        const end = parseTimestamp(match[2])

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

const parseNDJSON = async (response: Response): Promise<PGSFrame[]> => {
  const text = await response.text()
  const lines = text.trim().split('\n').filter(Boolean)
  return lines.map((line) => JSON.parse(line) as PGSFrame)
}

/**
 * Text Subtitle Renderer
 */
const TextSubtitleRenderer = ({
  videoRef,
  mediaId,
  trackId,
  streamIndex,
  streamOffsetRef,
  subtitleDelay = 0,
  onError,
}: TextSubtitleRendererProps) => {
  const [currentText, setCurrentText] = useState('')

  const initialTimeMs = useMemo(() => {
    const video = videoRef.current
    return video ? (video.currentTime + (streamOffsetRef.current || 0)) * 1000 : 0
  }, [videoRef, streamOffsetRef])

  const fetchWindow = useCallback(async (startMs: number, endMs: number, signal: AbortSignal): Promise<TextCue[]> => {
    if (streamIndex === undefined) { return [] }
    const url = subtitleUrls.textStream(mediaId, streamIndex, { start: startMs, end: endMs })
    const response = await authFetch(url, { signal })
    if (!response.ok) {
      throw new Error(`Failed to fetch subtitles: ${response.status} ${response.statusText}`)
    }
    return parseVTT(await response.text())
  }, [mediaId, streamIndex])

  const fetchFull = useCallback(async (signal: AbortSignal): Promise<TextCue[]> => {
    // External subtitles use trackId, embedded use streamIndex
    if (streamIndex === undefined && trackId === null) { return [] }
    const url = streamIndex !== undefined
      ? subtitleUrls.textStream(mediaId, streamIndex)
      : subtitleUrls.byTrackId(mediaId, trackId as number)
    const response = await authFetch(url, { signal })
    if (!response.ok) {
      throw new Error(`Failed to fetch subtitles: ${response.status} ${response.statusText}`)
    }
    return parseVTT(await response.text())
  }, [mediaId, trackId, streamIndex])

  const {
    itemsRef: cuesRef,
    ensureWindowLoaded,
    getWindowStart,
    abortControllerRef,
    fetchedWindowsRef,
    errors,
  } = useWindowedFetching<TextCue>({
    sourceKey: trackId !== null ? `${mediaId}-${trackId}` : null,
    fetchWindow,
    getItemKey: (cue) => cue.start,
    initialTimeMs,
    windowedEnabled: streamIndex !== undefined,
    fetchFull,
  })

  // Notify parent of errors
  useEffect(() => {
    if (errors.length > 0 && onError) {
      onError(errors)
    }
  }, [errors, onError])

  const findActiveCue = useCallback((cues: TextCue[], timeSeconds: number): TextCue | null => {
    return cues.find((c) => timeSeconds >= c.start && timeSeconds < c.end) ?? null
  }, [])

  const onActiveItemChange = useCallback((cue: TextCue | null) => {
    setCurrentText(cue?.text ?? '')
  }, [])

  const onFrame = useCallback((timeMs: number) => {
    if (streamIndex === undefined || !abortControllerRef.current) {return}

    const currentWindow = getWindowStart(timeMs)
    const nextWindow = currentWindow + FULL_WINDOW_MS
    const timeUntilNextWindow = nextWindow - timeMs

    // Prefetch next window
    if (timeUntilNextWindow < PREFETCH_THRESHOLD_MS) {
      ensureWindowLoaded(nextWindow, nextWindow)
    }

    // Handle seeks
    if (!fetchedWindowsRef.current.has(currentWindow)) {
      ensureWindowLoaded(currentWindow, timeMs)
    }
  }, [streamIndex, getWindowStart, ensureWindowLoaded, abortControllerRef, fetchedWindowsRef])

  useSubtitleAnimation({
    videoRef,
    streamOffsetRef,
    subtitleDelay,
    itemsRef: cuesRef,
    findActiveItem: findActiveCue,
    onActiveItemChange,
    onFrame: streamIndex !== undefined ? onFrame : undefined,
    enabled: trackId !== null,
  })

  if (!currentText) {return null}

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

/**
 * PGS (Bitmap) Subtitle Renderer
 */
const PGSSubtitleRenderer = ({
  videoRef,
  mediaId,
  trackId,
  bitmapIndex,
  streamOffsetRef,
  subtitleDelay = 0,
  onError,
}: PGSSubtitleRendererProps) => {
  const [currentFrame, setCurrentFrame] = useState<PGSFrame | null>(null)
  const [videoSize, setVideoSize] = useState({ width: 0, height: 0 })
  const [elementSize, setElementSize] = useState({ width: 0, height: 0 })

  const initialTimeMs = useMemo(() => {
    const video = videoRef.current
    return video ? (video.currentTime + (streamOffsetRef.current || 0)) * 1000 : 0
  }, [videoRef, streamOffsetRef])

  const fetchWindow = useCallback(async (startMs: number, endMs: number, signal: AbortSignal): Promise<PGSFrame[]> => {
    if (bitmapIndex === undefined) { return [] }
    const url = subtitleUrls.pgsStream(mediaId, bitmapIndex, { start: startMs, end: endMs })
    const response = await authFetch(url, { signal })
    if (!response.ok) {
      throw new Error(`Failed to fetch PGS subtitles: ${response.status} ${response.statusText}`)
    }
    return parseNDJSON(response)
  }, [mediaId, bitmapIndex])

  const {
    itemsRef: framesRef,
    ensureWindowLoaded,
    getWindowStart,
    abortControllerRef,
    fetchedWindowsRef,
    errors,
  } = useWindowedFetching<PGSFrame>({
    sourceKey: trackId !== null && bitmapIndex !== undefined ? `${mediaId}-${trackId}` : null,
    fetchWindow,
    getItemKey: (frame) => frame.start_ms,
    initialTimeMs,
    windowedEnabled: true,
  })

  // Notify parent of errors
  useEffect(() => {
    if (errors.length > 0 && onError) {
      onError(errors)
    }
  }, [errors, onError])

  // Track video dimensions
  useEffect(() => {
    const video = videoRef.current
    if (!video) {return}

    const updateVideoSize = () => {
      setVideoSize({ width: video.videoWidth, height: video.videoHeight })
    }

    const updateElementSize = () => {
      setElementSize({ width: video.clientWidth, height: video.clientHeight })
    }

    video.addEventListener('loadedmetadata', updateVideoSize)
    updateVideoSize()

    const resizeObserver = new ResizeObserver(updateElementSize)
    resizeObserver.observe(video)
    updateElementSize()

    return () => {
      video.removeEventListener('loadedmetadata', updateVideoSize)
      resizeObserver.disconnect()
    }
  }, [videoRef])

  const findActiveFrame = useCallback((frames: PGSFrame[], timeSeconds: number): PGSFrame | null => {
    const timeMs = timeSeconds * 1000
    return frames.find((f) => timeMs >= f.start_ms && timeMs < f.end_ms) ?? null
  }, [])

  const onActiveItemChange = useCallback((frame: PGSFrame | null) => {
    setCurrentFrame(frame)
  }, [])

  const onFrame = useCallback((timeMs: number) => {
    if (bitmapIndex === undefined || !abortControllerRef.current) {return}

    const currentWindow = getWindowStart(timeMs)
    const nextWindow = currentWindow + FULL_WINDOW_MS
    const timeUntilNextWindow = nextWindow - timeMs

    // Prefetch next window
    if (timeUntilNextWindow < PREFETCH_THRESHOLD_MS) {
      ensureWindowLoaded(nextWindow, nextWindow)
    }

    // Handle seeks
    if (!fetchedWindowsRef.current.has(currentWindow)) {
      ensureWindowLoaded(currentWindow, timeMs)
    }
  }, [bitmapIndex, getWindowStart, ensureWindowLoaded, abortControllerRef, fetchedWindowsRef])

  useSubtitleAnimation({
    videoRef,
    streamOffsetRef,
    subtitleDelay,
    itemsRef: framesRef,
    findActiveItem: findActiveFrame,
    onActiveItemChange,
    onFrame,
    enabled: trackId !== null && bitmapIndex !== undefined,
  })

  // Calculate video layout for positioning
  const videoLayout = useMemo(() => {
    if (videoSize.width === 0 || videoSize.height === 0 || elementSize.width === 0 || elementSize.height === 0) {
      return { offsetX: 0, offsetY: 0, scale: 1 }
    }

    const videoAspect = videoSize.width / videoSize.height
    const elementAspect = elementSize.width / elementSize.height

    let renderWidth: number
    let renderHeight: number

    if (videoAspect > elementAspect) {
      renderWidth = elementSize.width
      renderHeight = elementSize.width / videoAspect
    } else {
      renderHeight = elementSize.height
      renderWidth = elementSize.height * videoAspect
    }

    const offsetX = (elementSize.width - renderWidth) / 2
    const offsetY = (elementSize.height - renderHeight) / 2
    const scale = renderWidth / videoSize.width

    return { offsetX, offsetY, scale }
  }, [videoSize, elementSize])

  if (!currentFrame) {return null}

  const imageUrl = `data:image/webp;base64,${currentFrame.image_base64}`

  const canvasToVideoScale =
    currentFrame.canvas_width > 0 && videoSize.width > 0
      ? videoSize.width / currentFrame.canvas_width
      : 1

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
 */
export const SubtitleOverlay = (props: SubtitleOverlayProps) => {
  const { isBitmap, ...rest } = props

  if (props.trackId === null) {return null}

  if (isBitmap) {
    return <PGSSubtitleRenderer {...rest} />
  }

  return <TextSubtitleRenderer {...rest} />
}

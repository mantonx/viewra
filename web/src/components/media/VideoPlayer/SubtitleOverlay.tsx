/**
 * SubtitleOverlay Component
 * Renders subtitles as a positioned overlay above video controls.
 * Parses VTT directly instead of using HTML5 TextTrack to avoid blob URL security issues.
 */

import { useEffect, useState, useRef } from 'react'
import { authFetch } from '@/lib/utils/authFetch'

interface Cue {
  start: number
  end: number
  text: string
}

interface SubtitleOverlayProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  mediaId: number
  trackId: number | null
  /** Stream index for embedded subtitles - enables fast streaming extraction */
  streamIndex?: number
  streamOffsetRef: React.RefObject<number>
  /** Manual subtitle timing adjustment in seconds (positive = delay subtitles, negative = advance) */
  subtitleDelay?: number
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
const parseVTT = (content: string): Cue[] => {
  const cues: Cue[] = []
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

export const SubtitleOverlay = ({ videoRef, mediaId, trackId, streamIndex, streamOffsetRef, subtitleDelay = 0 }: SubtitleOverlayProps) => {
  const [cues, setCues] = useState<Cue[]>([])
  const [currentText, setCurrentText] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const lastTrackIdRef = useRef<number | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // Fetch and parse subtitle when track changes
  useEffect(() => {
    if (trackId === null) {
      setCues([])
      setCurrentText('')
      setIsLoading(false)
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
    setIsLoading(true)
    setCues([]) // Clear old cues while loading

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    const fetchSubtitle = async () => {
      try {
        // Use fast streaming endpoint when streamIndex is available (embedded subtitles)
        // This extracts via demuxing rather than scanning the entire file
        // Fall back to trackId endpoint for external subtitles (no streamIndex)
        const url =
          streamIndex !== undefined
            ? `/api/media/${mediaId}/subtitles/text/${streamIndex}/stream`
            : `/api/media/${mediaId}/subtitles/${trackId}`

        const response = await authFetch(url, {
          signal: abortController.signal,
        })
        if (!response.ok) {
          setIsLoading(false)
          return
        }

        const vttContent = await response.text()
        const parsedCues = parseVTT(vttContent)
        setCues(parsedCues)
        setIsLoading(false)
      } catch (err) {
        if (err instanceof Error && err.name === 'AbortError') {
          return // Request was aborted, ignore
        }
        console.error('Failed to load subtitle:', err)
        setIsLoading(false)
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
      // Get actual media time: video element time + stream offset - subtitle delay
      // subtitleDelay > 0 means subtitles appear later, so we subtract from actualTime
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
        dangerouslySetInnerHTML={{ __html: currentText.replace(/\n/g, '<br/>') }}
      />
    </div>
  )
}

// Re-export types from centralized location
export type { PGSFrame, TextCue } from '@/lib/types/subtitles'
export type { FetchError } from '@/lib/hooks/useWindowedFetching'

export interface SubtitleOverlayProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  mediaId: number
  trackId: number | null
  isBitmap?: boolean
  streamIndex?: number
  bitmapIndex?: number
  streamOffsetRef: React.RefObject<number>
  subtitleDelay?: number
  /** Optional callback when subtitle fetch errors occur */
  onError?: (errors: import('@/lib/hooks/useWindowedFetching').FetchError[]) => void
}

export type TextSubtitleRendererProps = Omit<SubtitleOverlayProps, 'isBitmap' | 'bitmapIndex'>

export type PGSSubtitleRendererProps = Omit<SubtitleOverlayProps, 'isBitmap' | 'streamIndex'>

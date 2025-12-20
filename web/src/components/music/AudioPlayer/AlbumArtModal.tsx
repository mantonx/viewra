import { useEffect } from 'react'
import { MediaPoster } from '@/components/media/MediaPoster'
import type { MusicTrackResponse } from '@/lib/types/music'

type AlbumArtModalProps = {
  track: MusicTrackResponse
  isOpen: boolean
  onClose: () => void
}

export const AlbumArtModal = ({ track, isOpen, onClose }: AlbumArtModalProps) => {
  // Handle Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
      }
    }

    if (isOpen) {
      document.addEventListener('keydown', handleKeyDown)
      return () => document.removeEventListener('keydown', handleKeyDown)
    }
  }, [isOpen, onClose])

  if (!isOpen) {
    return null
  }

  return (
    <div
      className="fixed inset-0 z-100 flex items-center justify-center bg-black/90 backdrop-blur-md animate-in fade-in duration-200"
      onClick={onClose}
    >
      <div className="relative w-[85vw] h-[85vh] max-w-5xl p-8 animate-in zoom-in-95 duration-300">
        <button
          onClick={onClose}
          className="absolute -top-2 -right-2 p-3 bg-black/60 rounded-full hover:bg-black/80 hover:scale-110 transition-all duration-200 z-10 shadow-xl cursor-pointer"
          title="Close (Esc)"
          aria-label="Close album art view"
        >
          <svg
            className="w-6 h-6 text-white"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
        <div
          className="w-full h-full rounded-2xl overflow-hidden shadow-2xl ring-1 ring-white/10 animate-in slide-in-from-bottom-4 duration-300"
          onClick={(e) => e.stopPropagation()}
        >
          <MediaPoster
            mediaId={track.id}
            mediaType="media"
            alt={track.album || track.title}
            className="w-full h-full object-contain"
            preset="xlarge"
            aspectRatio="square"
            fallbackIcon="🎵"
          />
        </div>
        <div className="mt-6 text-center animate-in fade-in slide-in-from-bottom-2 duration-500 delay-100">
          <h3 className="text-2xl font-bold text-white drop-shadow-lg">{track.title}</h3>
          <p className="text-base text-neutral-200 mt-2 drop-shadow-md">
            {track.artist || 'Unknown Artist'}
            {track.album && ` • ${track.album}`}
          </p>
        </div>
      </div>
    </div>
  )
}

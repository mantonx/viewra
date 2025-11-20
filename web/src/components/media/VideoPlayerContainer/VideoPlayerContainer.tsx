import { VideoPlayer } from '../VideoPlayer'
import type { PlaybackState } from '@/lib/hooks/useMediaPlayback'

interface Media {
  id: number
  duration: number
}

interface VideoPlayerContainerProps {
  playbackState: PlaybackState
  media: Media | null | undefined
  onClose: () => void
  overlay?: React.ReactNode
}

export const VideoPlayerContainer = ({
  playbackState,
  media,
  onClose,
  overlay,
}: VideoPlayerContainerProps) => {
  // Don't render if not playing or no media
  if (!playbackState.isPlaying || !media) {
    return null
  }

  return (
    <div className="relative">
      <VideoPlayer
        mediaId={media.id}
        streamUrl={playbackState.streamUrl || ''}
        initialPosition={playbackState.initialPosition}
        duration={media.duration}
        onClose={onClose}
      />
      {overlay}
    </div>
  )
}

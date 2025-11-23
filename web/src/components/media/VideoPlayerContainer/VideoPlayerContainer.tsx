import { VideoPlayer, type MediaMetadata } from '../VideoPlayer'
import type { PlaybackState } from '@/lib/hooks/useMediaPlayback'
import { useTVShowImages, useMovieImages } from '@/lib/hooks/useMediaImages'
import { getPosterImage, getImageUrl } from '@/lib/types/images'

interface Media {
  id?: number
  duration?: number
  title?: string
  year?: number
  // For TV shows
  show_title?: string
  show_id?: number
  season?: number
  episode?: number
  episode_title?: string
}

interface VideoPlayerContainerProps {
  playbackState: PlaybackState
  media: Media | null | undefined
  onClose: () => void
  onTimeUpdate?: (time: number) => void
  overlay?: React.ReactNode
}

export const VideoPlayerContainer = ({
  playbackState,
  media,
  onClose,
  onTimeUpdate,
  overlay,
}: VideoPlayerContainerProps) => {
  // Fetch images for TV shows or movies
  const tvShowImages = useTVShowImages(media?.show_id || 0, {
    enabled: !!media?.show_id && !!media?.show_title,
  })
  const movieImages = useMovieImages(media?.id || 0, {
    enabled: !!media?.id && !media?.show_title,
  })

  // Don't render if not playing or no media
  if (!playbackState.isPlaying || !media) {
    return null
  }

  // Get poster URL based on media type
  let posterUrl: string | undefined
  if (media.show_title) {
    // TV Show - use show-level poster
    const images = tvShowImages.data?.images || []
    const poster = getPosterImage(images)
    posterUrl = poster ? getImageUrl(poster.id, 'medium') : undefined
  } else {
    // Movie - use movie poster
    const images = movieImages.data?.images || []
    const poster = getPosterImage(images)
    posterUrl = poster ? getImageUrl(poster.id, 'medium') : undefined
  }

  // Build metadata based on media type
  const metadata: MediaMetadata = media.show_title
    ? {
        // TV Show
        title: media.show_title || 'Unknown',
        subtitle: `S${media.season || 0}:E${media.episode || 0}${media.episode_title ? ` • ${media.episode_title}` : ''}`,
        posterUrl,
      }
    : {
        // Movie
        title: media.title || 'Unknown',
        subtitle: media.year?.toString(),
        posterUrl,
      }

  return (
    <div className="relative">
      <VideoPlayer
        mediaId={media.id || 0}
        streamUrl={playbackState.streamUrl || ''}
        initialPosition={playbackState.initialPosition}
        duration={media.duration}
        metadata={metadata}
        onClose={onClose}
        onTimeUpdate={onTimeUpdate}
      />
      {overlay}
    </div>
  )
}

import { useBatchProgress } from '@/lib/hooks'
import { useBadgePreferences } from '@/lib/hooks/useBadgePreferences'
import { formatResolutionLabel } from '@/lib/utils/quality'
import { MediaCard } from '@/components/media/MediaCard'
import { MediaBadges } from '@/components/media/MediaBadges'
import { MediaMetadata } from '@/components/media/MediaMetadata'
import { ProgressBar } from '@/components/media/ProgressBar'
import type { MovieCardProps } from './MovieCard.types'

const MovieCard = ({ movie, onClick }: MovieCardProps) => {
  const { progress } = useBatchProgress(movie.id)
  const { preferences } = useBadgePreferences()

  // Check if movie is newly added (within last 7 days)
  const isNew = Boolean(
    movie.created_at &&
    Date.now() - new Date(movie.created_at).getTime() < 7 * 24 * 60 * 60 * 1000
  )

  return (
    <MediaCard
      mediaId={movie.id}
      mediaType="movie"
      imageAlt={movie.title}
      aspectRatio="2/3"
      onClick={onClick}
      playIconType="play"
      badges={
        <MediaBadges
          preferences={preferences}
          badges={{
            isNew,
            isExtra: movie.is_extra,
            resolution: formatResolutionLabel(movie.height) ?? undefined,
            contentRating: movie.content_rating ?? undefined,
            codec: movie.video_codec ?? undefined,
          }}
        />
      }
      overlays={<ProgressBar progress={progress} />}
      infoContent={
        <MediaMetadata
          title={movie.title}
          year={movie.year}
          duration={movie.duration}
          genres={movie.genre}
          plot={movie.plot}
          director={movie.director}
          fileSize={movie.file_size}
          progress={progress}
          links={{
            imdb: movie.imdb_id,
            tmdb: movie.tmdb_id?.toString(),
          }}
        />
      }
    />
  )
}

export type { MovieCardProps } from './MovieCard.types'
export { MovieCard }

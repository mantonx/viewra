import { cn } from '@/lib/utils'
import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import { MediaCard } from '@/components/media'
import { ScrollableRow } from '@/components/common'
import type { MediaRowData, TrendingItem, TrendingRowData } from './widget.types'
import { normalizeMediaType } from './utils'

interface MediaRowProps {
  data: MediaRowData | TrendingRowData
  className?: string
}

/**
 * Type guard to check if data is TrendingRowData
 */
const isTrendingData = (data: MediaRowData | TrendingRowData): data is TrendingRowData => {
  return 'window' in data && 'source' in data
}

/**
 * MediaRow - Horizontal scrolling row of media items
 *
 * Renders movies and TV shows in a horizontal scrollable row.
 * Supports trending items from external sources (TMDb).
 */
export const MediaRow = ({ data, className }: MediaRowProps) => {
  const navigate = useNavigate()

  // Determine what data we have
  const isTrending = isTrendingData(data)
  const mediaRowData = !isTrending ? (data as MediaRowData) : null

  // Get items based on data type
  const trendingItems = isTrending ? (data as TrendingRowData).items : []
  const movies = mediaRowData?.movies ?? []
  const shows = mediaRowData?.shows ?? []

  const seeAllUrl = mediaRowData?.see_all_url
  const subtitle = mediaRowData?.subtitle

  // Check if we have any content to show
  const hasContent = trendingItems.length > 0 || movies.length > 0 || shows.length > 0
  if (!hasContent) {
    return null
  }

  const handleMovieClick = (movieId: number) => {
    navigate({ to: `/movies?id=${movieId}` })
  }

  const handleShowClick = (showId: number) => {
    navigate({ to: `/tv/${showId}` })
  }

  const handleMediaClick = (mediaType: string, mediaId: number) => {
    const normalizedType = normalizeMediaType(mediaType)
    if (normalizedType === 'movie') {
      handleMovieClick(mediaId)
    } else if (normalizedType === 'tv-show') {
      handleShowClick(mediaId)
    }
  }

  return (
    <section className={className}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-xl text-neutral-900 dark:text-neutral-50 font-display tracking-tight">
            {data.title}
          </h2>
          {subtitle && (
            <p className="text-sm text-neutral-500 dark:text-neutral-400 mt-0.5">{subtitle}</p>
          )}
        </div>
        {seeAllUrl && (
          <Link
            to={seeAllUrl as never}
            className={cn(
              'flex items-center gap-1 text-sm font-medium',
              'text-primary-600 dark:text-primary-400',
              'hover:text-primary-700 dark:hover:text-primary-300',
              'transition-colors'
            )}
          >
            See all
            <ChevronRight className="w-4 h-4" />
          </Link>
        )}
      </div>

      {/* Scrollable row with affordances */}
      <ScrollableRow gap={4}>
        {/* Trending items */}
        {isTrending &&
          trendingItems.map((item, index) => (
            <TrendingCard
              key={item.external_id || index}
              item={item}
              onClick={handleMediaClick}
            />
          ))}

        {/* Movies */}
        {movies.map((movie) => (
          <div key={`movie-${movie.id}`} className="w-48 shrink-0">
            <MediaCard
              mediaId={movie.id}
              mediaType="movie"
              imageAlt={movie.title}
              title={movie.title}
              year={movie.year}
              onClick={() => handleMovieClick(movie.id)}
            />
          </div>
        ))}

        {/* TV Shows */}
        {shows.map((show) => (
          <div key={`show-${show.id}`} className="w-48 shrink-0">
            <MediaCard
              mediaId={show.id ?? 0}
              mediaType="tv-show"
              imageAlt={show.title ?? 'TV Show'}
              imageFallback="📺"
              title={show.title ?? 'Unknown Show'}
              year={show.year}
              onClick={() => show.id && handleShowClick(show.id)}
            />
          </div>
        ))}
      </ScrollableRow>
    </section>
  )
}

/**
 * TrendingCard - Card for external trending items (not in local library)
 */
interface TrendingCardProps {
  item: TrendingItem
  onClick?: (mediaType: string, mediaId: number) => void
}

const TrendingCard = ({ item, onClick }: TrendingCardProps) => {
  // If matched to local library, use MediaCard
  if (item.local_matched && item.local_id) {
    const localId = item.local_id
    return (
      <div className="w-48 shrink-0">
        <MediaCard
          mediaId={localId}
          mediaType={normalizeMediaType(item.media_type)}
          imageAlt={item.title}
          title={item.title}
          year={item.year}
          onClick={() => onClick?.(item.media_type, localId)}
        />
      </div>
    )
  }

  // External item - render a simpler card with external poster
  return (
    <div className="w-48 shrink-0">
      <div
        className={cn(
          'rounded-xl overflow-hidden relative',
          'bg-white dark:bg-neutral-900',
          'shadow-sm dark:shadow-none',
          'border border-neutral-200/50 dark:border-white/5',
          'transition-all duration-200 ease-out',
          'hover:scale-[1.03] hover:z-50',
          'hover:shadow-xl hover:shadow-neutral-900/10',
          'dark:hover:shadow-2xl dark:hover:shadow-primary-500/10'
        )}
      >
        {/* Poster */}
        <div className="aspect-2/3 relative bg-neutral-200 dark:bg-neutral-800">
          {item.poster_path ? (
            <img
              src={item.poster_path}
              alt={item.title}
              className="w-full h-full object-cover"
              loading="lazy"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center text-4xl">
              {item.media_type === 'tv' ? '📺' : '🎬'}
            </div>
          )}

          {/* Not in library badge */}
          <div className="absolute top-2 right-2">
            <span className="px-2 py-1 text-xs font-medium rounded-full bg-neutral-900/75 text-white backdrop-blur-sm">
              Not in library
            </span>
          </div>
        </div>

        {/* Info */}
        <div className="p-3">
          <h3 className="font-medium text-sm text-neutral-900 dark:text-white line-clamp-1">
            {item.title}
          </h3>
          {item.year > 0 && (
            <p className="text-xs text-neutral-500 dark:text-neutral-400 mt-0.5">{item.year}</p>
          )}
        </div>
      </div>
    </div>
  )
}

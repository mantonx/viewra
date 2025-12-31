import { MediaCard } from '@/components/media'
import { cn } from '@/lib/utils'
import { Link } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import type { MediaItem, MediaRowData, TrendingItem, TrendingRowData } from './widget.types'

interface MediaRowProps {
  data: MediaRowData | TrendingRowData
  className?: string
}

/**
 * Normalize media type from various formats to MediaCard format
 */
const normalizeMediaType = (type: string): 'movie' | 'tv-show' | 'music-album' | 'music-artist' => {
  switch (type.toLowerCase()) {
    case 'movie':
    case 'movies':
      return 'movie'
    case 'tv':
    case 'tv_show':
    case 'tv-show':
    case 'tvshow':
      return 'tv-show'
    case 'music_album':
    case 'music-album':
    case 'album':
      return 'music-album'
    case 'music_artist':
    case 'music-artist':
    case 'artist':
      return 'music-artist'
    default:
      return 'movie'
  }
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
 * Used for recommendations, trending, similar items, etc.
 * Supports both local media items and external trending items.
 */
export const MediaRow = ({ data, className }: MediaRowProps) => {
  const items = isTrendingData(data) ? data.items : data.items
  const seeAllUrl = !isTrendingData(data) ? data.see_all_url : undefined
  const subtitle = !isTrendingData(data) ? data.subtitle : undefined

  if (!items || items.length === 0) {
    return null
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

      {/* Scrollable row */}
      <div className="overflow-x-auto -mx-4 px-4 pb-2">
        <div className="flex gap-4" style={{ minWidth: 'max-content' }}>
          {items.map((item, index) => {
            if (isTrendingData(data)) {
              // Trending item from external source
              const trendingItem = item as TrendingItem
              return <TrendingCard key={trendingItem.external_id || index} item={trendingItem} />
            } else {
              // Local media item
              const mediaItem = item as MediaItem
              return (
                <div key={mediaItem.entity_id} className="w-48 shrink-0">
                  <MediaCard
                    mediaId={mediaItem.entity_id}
                    mediaType={normalizeMediaType(mediaItem.entity_type)}
                    imageAlt={mediaItem.title}
                    infoContent={
                      mediaItem.reason ? (
                        <p className="text-xs text-neutral-500 dark:text-neutral-400 line-clamp-2">
                          {mediaItem.reason}
                        </p>
                      ) : undefined
                    }
                  />
                </div>
              )
            }
          })}
        </div>
      </div>
    </section>
  )
}

/**
 * TrendingCard - Card for external trending items (not in local library)
 */
interface TrendingCardProps {
  item: TrendingItem
}

const TrendingCard = ({ item }: TrendingCardProps) => {
  // If matched to local library, use MediaCard
  if (item.local_matched && item.local_id) {
    return (
      <div className="w-48 shrink-0">
        <MediaCard
          mediaId={item.local_id}
          mediaType={normalizeMediaType(item.media_type)}
          imageAlt={item.title}
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

import { useNavigate } from '@tanstack/react-router'
import { ScrollableRow } from '@/components/common'
import { ContinueWatchingCard } from './ContinueWatchingCard'
import { MediaRow } from './MediaRow'
import type { ContinueWatchingData, ContinueWatchingItem } from './widget.types'

interface ContinueRowProps {
  data: ContinueWatchingData
  className?: string
}

/**
 * ContinueRow - Continue Watching widget
 *
 * Displays movies and TV shows the user is currently watching.
 * Uses horizontal 16:9 cards with progress bars when the new format is available,
 * falls back to the legacy MediaRow format for backward compatibility.
 */
export const ContinueRow = ({ data, className }: ContinueRowProps) => {
  const navigate = useNavigate()

  // Check if we have the new items format with progress data
  const hasNewFormat = data.items && data.items.length > 0

  // If no new format, fall back to legacy MediaRow rendering
  if (!hasNewFormat) {
    return (
      <MediaRow
        data={{
          title: data.title,
          movies: data.movies,
          shows: data.shows,
        }}
        className={className}
      />
    )
  }

  const handleItemClick = (item: ContinueWatchingItem) => {
    if (item.entity_type === 'movie') {
      navigate({ to: `/movies?id=${item.entity_id}` })
    } else {
      navigate({ to: `/tv/${item.entity_id}` })
    }
  }

  const handleItemPlay = (item: ContinueWatchingItem) => {
    // For TV shows, navigate to the specific episode for playback
    if (item.entity_type === 'tv_show' && item.episode_context?.episode_media_id) {
      // Navigate to episode playback (adjust route as needed)
      navigate({ to: `/tv/${item.entity_id}` })
    } else {
      handleItemClick(item)
    }
  }

  return (
    <section className={className}>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <h2 className="text-xl text-neutral-900 dark:text-neutral-50 font-display tracking-tight">
          {data.title}
        </h2>
      </div>

      {/* Horizontal scrollable row with new cards */}
      <ScrollableRow gap={4}>
        {data.items?.map((item) => (
          <ContinueWatchingCard
            key={`${item.entity_type}-${item.entity_id}`}
            item={item}
            onClick={() => handleItemClick(item)}
            onPlay={() => handleItemPlay(item)}
          />
        ))}
      </ScrollableRow>
    </section>
  )
}

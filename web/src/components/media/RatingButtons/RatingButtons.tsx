import { useQueryClient } from '@tanstack/react-query'
import { ThumbsUp, ThumbsDown, Heart } from 'lucide-react'
import { cn } from '@/lib/utils'
import {
  useGetApiRatingsEntityTypeEntityId,
  usePostApiRatings,
  useDeleteApiRatingsEntityTypeEntityId,
  getGetApiRatingsEntityTypeEntityIdQueryKey,
} from '@/lib/api/generated/ratings/ratings'

type RatingType = 'up' | 'down' | 'favorite'

interface RatingButtonsProps {
  entityType: 'movie' | 'tv_show' | 'tv_episode'
  entityId: number
  className?: string
  size?: 'sm' | 'md' | 'lg'
}

/**
 * RatingButtons - User rating controls for media items
 *
 * Displays thumbs up, thumbs down, and favorite buttons.
 * Clicking a selected rating removes it; clicking a different rating replaces it.
 */
export const RatingButtons = ({
  entityType,
  entityId,
  className,
  size = 'md',
}: RatingButtonsProps) => {
  const queryClient = useQueryClient()

  // Fetch current rating
  const { data: ratingData } = useGetApiRatingsEntityTypeEntityId(entityType, entityId, {
    query: {
      retry: false,
      staleTime: 30000,
    },
  })

  // Get current rating value (null if not rated or 404)
  const currentRating: RatingType | null =
    ratingData?.status === 200 ? (ratingData.data.rating as RatingType) : null

  // Mutations
  const { mutate: setRating, isPending: isSettingRating } = usePostApiRatings({
    mutation: {
      onSuccess: () => {
        queryClient.invalidateQueries({
          queryKey: getGetApiRatingsEntityTypeEntityIdQueryKey(entityType, entityId),
        })
        // Also invalidate home sections to update favorites widget
        queryClient.invalidateQueries({ queryKey: ['home'] })
      },
    },
  })

  const { mutate: deleteRating, isPending: isDeletingRating } =
    useDeleteApiRatingsEntityTypeEntityId({
      mutation: {
        onSuccess: () => {
          queryClient.invalidateQueries({
            queryKey: getGetApiRatingsEntityTypeEntityIdQueryKey(entityType, entityId),
          })
          // Also invalidate home sections to update favorites widget
          queryClient.invalidateQueries({ queryKey: ['home'] })
        },
      },
    })

  const isPending = isSettingRating || isDeletingRating

  const handleRate = (rating: RatingType) => {
    if (isPending) return

    if (currentRating === rating) {
      // Toggle off - delete the rating
      deleteRating({ entityType, entityId })
    } else {
      // Set new rating
      setRating({
        data: {
          entity_type: entityType,
          entity_id: entityId,
          rating,
        },
      })
    }
  }

  const iconSize = {
    sm: 'w-4 h-4',
    md: 'w-5 h-5',
    lg: 'w-6 h-6',
  }[size]

  const buttonSize = {
    sm: 'p-1.5',
    md: 'p-2',
    lg: 'p-2.5',
  }[size]

  const getTooltip = (rating: RatingType): string => {
    const isSelected = currentRating === rating
    switch (rating) {
      case 'up':
        return isSelected ? 'Remove like' : 'Like - helps improve recommendations'
      case 'down':
        return isSelected ? 'Remove dislike' : 'Dislike - helps improve recommendations'
      case 'favorite':
        return isSelected ? 'Remove from favorites' : 'Add to favorites'
    }
  }

  return (
    <div className={cn('flex items-center gap-1', className)}>
      {/* Thumbs Up */}
      <button
        onClick={() => handleRate('up')}
        disabled={isPending}
        className={cn(
          buttonSize,
          'rounded-lg transition-colors cursor-pointer',
          currentRating === 'up'
            ? 'bg-green-500/20 text-green-500'
            : 'hover:bg-neutral-100 dark:hover:bg-white/10 text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300',
          isPending && 'opacity-50 !cursor-not-allowed'
        )}
        aria-label={getTooltip('up')}
        aria-pressed={currentRating === 'up'}
        title={getTooltip('up')}
      >
        <ThumbsUp className={iconSize} />
      </button>

      {/* Thumbs Down */}
      <button
        onClick={() => handleRate('down')}
        disabled={isPending}
        className={cn(
          buttonSize,
          'rounded-lg transition-colors cursor-pointer',
          currentRating === 'down'
            ? 'bg-red-500/20 text-red-500'
            : 'hover:bg-neutral-100 dark:hover:bg-white/10 text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300',
          isPending && 'opacity-50 !cursor-not-allowed'
        )}
        aria-label={getTooltip('down')}
        aria-pressed={currentRating === 'down'}
        title={getTooltip('down')}
      >
        <ThumbsDown className={iconSize} />
      </button>

      {/* Favorite */}
      <button
        onClick={() => handleRate('favorite')}
        disabled={isPending}
        className={cn(
          buttonSize,
          'rounded-lg transition-colors cursor-pointer',
          currentRating === 'favorite'
            ? 'bg-pink-500/20 text-pink-500'
            : 'hover:bg-neutral-100 dark:hover:bg-white/10 text-neutral-500 hover:text-neutral-700 dark:hover:text-neutral-300',
          isPending && 'opacity-50 !cursor-not-allowed'
        )}
        aria-label={getTooltip('favorite')}
        aria-pressed={currentRating === 'favorite'}
        title={getTooltip('favorite')}
      >
        <Heart className={cn(iconSize, currentRating === 'favorite' && 'fill-current')} />
      </button>
    </div>
  )
}

import { useInProgressList } from '@/lib/hooks/useProgress'
import { useQuery } from '@tanstack/react-query'
import { getGetApiMediaQueryOptions } from '@/lib/api'
import { MediaCard } from '@/components/media'
import { Loading } from '@/components/ui'
import { extractMedia } from '@/lib/utils/api'
import type { GithubComMantonxViewraInternalApplicationMediaMediaResponse } from '@/lib/api/generated/models'

export const ContinueWatching = () => {
  const { data: progressData, isLoading: isLoadingProgress } = useInProgressList({ limit: 20 })
  const { data: mediaData, isLoading: isLoadingMedia } = useQuery(getGetApiMediaQueryOptions())

  const isLoading = isLoadingProgress || isLoadingMedia

  // Extract media items from API response
  const allMedia = extractMedia(mediaData)

  // Get in-progress items
  const inProgressItems = progressData?.progress || []

  // Filter media to only include items that have progress
  const inProgressMedia = inProgressItems
    .map((progressItem) => {
      const mediaItem = allMedia.find((m) => m.id === progressItem.media_id)
      return mediaItem
    })
    .filter((item): item is GithubComMantonxViewraInternalApplicationMediaMediaResponse =>
      Boolean(item)
    )

  // Sort by last watched date (most recent first)
  const sortedMedia = [...inProgressMedia].sort((a, b) => {
    const aProgress = inProgressItems.find((p) => p.media_id === a.id)
    const bProgress = inProgressItems.find((p) => p.media_id === b.id)

    const aDate = aProgress?.last_watched_at ? new Date(aProgress.last_watched_at).getTime() : 0
    const bDate = bProgress?.last_watched_at ? new Date(bProgress.last_watched_at).getTime() : 0

    return bDate - aDate
  })

  // Don't render section if no in-progress items
  if (!isLoading && sortedMedia.length === 0) {
    return null
  }

  return (
    <section>
      <h2 className="text-xl font-semibold mb-5 text-neutral-900 dark:text-neutral-50 font-display tracking-tight">
        Continue Watching
      </h2>

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <Loading size="lg" text="Loading your progress..." />
        </div>
      ) : (
        <div className="overflow-x-auto -mx-4 px-4 pb-2">
          <div className="flex gap-4" style={{ minWidth: 'max-content' }}>
            {sortedMedia
              .filter((media) => media.id !== undefined)
              .map((media) => (
                <div key={media.id} className="w-48 shrink-0">
                  <MediaCard
                    mediaId={media.id as number}
                    mediaType={media.type === 'movie' ? 'movie' : 'tv-show'}
                    imageAlt={media.title || 'Media item'}
                  />
                </div>
              ))}
          </div>
        </div>
      )}
    </section>
  )
}

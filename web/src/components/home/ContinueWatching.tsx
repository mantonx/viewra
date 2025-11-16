import { useInProgressList } from '@/lib/hooks/useProgress'
import { useQuery } from '@tanstack/react-query'
import { getGetApiMediaQueryOptions } from '@/lib/api'
import { MediaCard } from '@/components/media'
import { Loading } from '@/components/ui'
import { extractMedia } from '@/lib/utils/api'
import type { GithubComViewraViewraInternalApplicationMediaMediaResponse } from '@/lib/api/generated/models'

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
    .filter((item): item is GithubComViewraViewraInternalApplicationMediaMediaResponse =>
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
    <section className="mb-8">
      <h2 className="text-2xl font-bold mb-4">Continue Watching</h2>

      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <Loading size="lg" text="Loading your progress..." />
        </div>
      ) : (
        <div className="overflow-x-auto -mx-4 px-4 pb-4">
          <div className="flex gap-4" style={{ minWidth: 'max-content' }}>
            {sortedMedia.map((media) => (
              <div key={media.id} className="w-48 flex-shrink-0">
                <MediaCard media={media} />
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}

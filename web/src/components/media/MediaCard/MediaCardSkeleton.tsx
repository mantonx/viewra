/**
 * MediaCardSkeleton Component
 * Loading placeholder for MediaCard to prevent layout shifts during infinite scroll
 */

interface MediaCardSkeletonProps {
  /**
   * Aspect ratio for the skeleton (matches MediaCard)
   */
  aspectRatio?: '2/3' | 'square'
}

export const MediaCardSkeleton = ({ aspectRatio = '2/3' }: MediaCardSkeletonProps) => {
  const aspectClass = aspectRatio === 'square' ? 'aspect-square' : 'aspect-[2/3]'

  return (
    <div className="group cursor-pointer">
      {/* Skeleton poster */}
      <div className={`relative ${aspectClass} w-full overflow-hidden rounded-lg mb-2`}>
        <div className="absolute inset-0 bg-linear-to-br from-neutral-700 to-neutral-900 animate-pulse" />
      </div>

      {/* Skeleton info */}
      <div className="space-y-2">
        {/* Title skeleton */}
        <div className="h-4 bg-neutral-300 dark:bg-neutral-700 rounded animate-pulse w-3/4" />
        {/* Metadata skeleton */}
        <div className="h-3 bg-neutral-300 dark:bg-neutral-700 rounded animate-pulse w-1/2" />
      </div>
    </div>
  )
}

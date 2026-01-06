import { useMediaImages, useTVShowImages } from '@/lib/hooks/useMediaImages'
import { getFanartImage, getImageUrl } from '@/lib/types/images'
import { cn } from '@/lib/utils'

interface HeroBackdropProps {
  /** Media ID to fetch backdrop from */
  mediaId: number
  /** Type of media (movie or tv_show) */
  mediaType: 'movie' | 'tv_show'
  /** Optional greeting text */
  greeting?: string
  /** Optional date text */
  dateText?: string
  /** Additional CSS classes */
  className?: string
}

/**
 * HeroBackdrop - Dynamic backdrop with blur and gradient overlay
 *
 * Displays a large backdrop image sourced from the user's continue watching
 * or trending content, with a blurred/desaturated effect and gradient overlay.
 */
export const HeroBackdrop = ({
  mediaId,
  mediaType,
  greeting,
  dateText,
  className,
}: HeroBackdropProps) => {
  // Fetch backdrop image based on media type
  const isMovie = mediaType === 'movie'
  const movieQuery = useMediaImages(isMovie ? mediaId : undefined)
  const showQuery = useTVShowImages(!isMovie ? mediaId : undefined)

  const images = isMovie ? movieQuery.data?.images : showQuery.data?.images
  const fanartImage = images ? getFanartImage(images) : null
  const backdropUrl = fanartImage ? getImageUrl(fanartImage.id, 'xlarge') : null

  const isLoading = isMovie ? movieQuery.isLoading : showQuery.isLoading

  // Don't render if no backdrop available and not loading
  if (!backdropUrl && !isLoading) {
    return null
  }

  return (
    <div className={cn('absolute inset-0 -z-10 overflow-hidden', className)}>
      {/* Backdrop image with blur and desaturation */}
      {backdropUrl && (
        <img
          src={backdropUrl}
          alt=""
          className={cn(
            'absolute inset-0 w-full h-full object-cover',
            'scale-110 blur-sm saturate-50 opacity-30',
            'transition-opacity duration-500'
          )}
          loading="eager"
          onLoad={(e) => {
            // Fade in when loaded
            e.currentTarget.style.opacity = '0.3'
          }}
        />
      )}

      {/* Gradient overlay - fade to background color */}
      <div
        className={cn(
          'absolute inset-0',
          'bg-gradient-to-b from-transparent via-transparent to-neutral-50',
          'dark:to-neutral-950'
        )}
      />

      {/* Side gradients for vignette effect */}
      <div
        className={cn(
          'absolute inset-0',
          'bg-gradient-to-r from-neutral-50/50 via-transparent to-neutral-50/50',
          'dark:from-neutral-950/50 dark:to-neutral-950/50'
        )}
      />

      {/* Greeting and date overlay */}
      {(greeting || dateText) && (
        <div className="absolute top-0 left-0 p-8 z-10">
          {dateText && (
            <p className="text-sm text-neutral-600 dark:text-neutral-400 mb-1">
              {dateText}
            </p>
          )}
          {greeting && (
            <h1 className="text-2xl font-display text-neutral-900 dark:text-white">
              {greeting}
            </h1>
          )}
        </div>
      )}
    </div>
  )
}

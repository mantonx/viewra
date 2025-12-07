import { getProgressPercentage } from '@/lib/utils'
import { Progress } from '@/components/ui/Progress'
import type { ProgressBarProps } from './ProgressBar.types'

/**
 * Progress bar overlay component for media cards
 * Shows playback progress at the bottom of media thumbnails
 * Automatically hides when there is no progress or when fully complete
 */
export const ProgressBar = ({ progress }: ProgressBarProps) => {
  const percentage = getProgressPercentage(progress)

  // Don't show if no progress or fully complete (>= 95% is considered complete)
  if (!progress || percentage <= 0 || percentage >= 95) {
    return null
  }

  return (
    <div className="absolute bottom-0 left-0 right-0 z-10">
      <Progress
        value={percentage}
        size="xs"
        className="[&>div]:rounded-none [&>div]:bg-black/30 [&_[role=progressbar]]:rounded-none"
      />
    </div>
  )
}

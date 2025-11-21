import { getProgressPercentage } from '@/lib/utils'
import type { ProgressBarProps } from './ProgressBar.types'

/**
 * Progress bar overlay component for media cards
 * Shows playback progress at the bottom of media thumbnails
 * Shows when there is incomplete progress (not 100% complete)
 */
export const ProgressBar = ({ progress }: ProgressBarProps) => {
  const percentage = getProgressPercentage(progress)

  // Don't show if no progress or fully complete (>= 95% is considered complete)
  if (!progress || percentage <= 0 || percentage >= 95) {
    return null
  }

  return (
    <div className="absolute bottom-0 left-0 right-0 h-1 bg-black bg-opacity-30 z-10">
      <div
        className="h-full bg-blue-500 transition-all"
        style={{ width: `${Math.min(percentage, 100)}%` }}
      />
    </div>
  )
}

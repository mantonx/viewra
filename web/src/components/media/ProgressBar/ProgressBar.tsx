import { getProgressPercentage } from '@/lib/utils'
import type { ProgressBarProps } from './ProgressBar.types'

/**
 * Progress bar overlay component for media cards
 * Shows playback progress at the bottom of media thumbnails
 */
export const ProgressBar = ({ progress }: ProgressBarProps) => {
  if (!progress || getProgressPercentage(progress) <= 0) {
    return null
  }

  return (
    <div className="absolute bottom-0 left-0 right-0 h-1 bg-black bg-opacity-30 z-10">
      <div
        className={`h-full transition-all ${
          progress.is_watched ? 'bg-green-500' : 'bg-blue-500'
        }`}
        style={{ width: `${Math.min(getProgressPercentage(progress), 100)}%` }}
      />
    </div>
  )
}

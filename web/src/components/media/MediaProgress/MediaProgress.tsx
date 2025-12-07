import { ProgressBar } from '@/components/media/ProgressBar'
import type { MediaProgressProps } from './MediaProgress.types'

export const MediaProgress = ({
  progress,
  showPercentage: _showPercentage = true,
  showWatchedIndicator: _showWatchedIndicator = true,
}: MediaProgressProps) => {
  if (!progress) {return null}

  return (
    <>
      <ProgressBar progress={progress} />
      {/* Additional progress indicators can be added here */}
    </>
  )
}

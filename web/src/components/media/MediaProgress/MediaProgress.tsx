import { ProgressBar } from '@/components/media/ProgressBar'
import type { MediaProgressProps } from './MediaProgress.types'

export const MediaProgress = ({
  progress,
  showPercentage = true,
  showWatchedIndicator = true,
}: MediaProgressProps) => {
  if (!progress) return null

  return (
    <>
      <ProgressBar progress={progress} />
      {/* Additional progress indicators can be added here */}
    </>
  )
}

import { useState } from 'react'
import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'
import { formatFileSize, formatDuration, DEFAULT_USER_ID, getProgressPercentage, getProgressSeconds, getDurationSeconds, hasProgress } from '@/lib/utils'
import type { MediaDetailsModalProps } from './MediaDetailsModal.types'
import { useMediaProgress, useMarkWatched, useMarkUnwatched } from '@/lib/hooks/useProgress'
import { VideoPlayer } from '../VideoPlayer'

const MediaDetailsModal = ({ media, onClose }: MediaDetailsModalProps) => {
  const { data: progress } = useMediaProgress(media.id)
  const markWatched = useMarkWatched()
  const markUnwatched = useMarkUnwatched()
  const [showPlayer, setShowPlayer] = useState(false)
  const [playFromStart, setPlayFromStart] = useState(false)

  const handleToggleWatched = () => {
    if (progress?.is_watched) {
      markUnwatched.mutate({ media_id: media.id, user_id: DEFAULT_USER_ID })
    } else {
      markWatched.mutate({ media_id: media.id, user_id: DEFAULT_USER_ID })
    }
  }

  const handlePlay = (fromStart: boolean = false) => {
    setPlayFromStart(fromStart)
    setShowPlayer(true)
  }

  const handleClosePlayer = () => {
    setShowPlayer(false)
  }

  const streamUrl = `http://localhost:8080/api/stream/${media.id}`
  const progressSecs = getProgressSeconds(progress)
  const showProgress = hasProgress(progress)
  const canResume = showProgress && progressSecs > 0 && !progress?.is_watched

  // If video player is showing, render it instead of modal
  if (showPlayer) {
    return (
      <VideoPlayer
        mediaId={media.id || 0}
        streamUrl={streamUrl}
        initialPosition={playFromStart ? 0 : progressSecs}
        duration={media.duration}
        onClose={handleClosePlayer}
      />
    )
  }

  return (
    <Modal isOpen={true} onClose={onClose} title={media.title} size="md">
      <ModalContent>
        <div className="space-y-3">
          {/* Progress indicator */}
          {showProgress && (
            <div className="bg-blue-50 border border-blue-200 rounded p-3">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-semibold text-blue-900">
                  {progress?.is_watched ? '✓ Watched' : `${Math.floor(getProgressPercentage(progress))}% Complete`}
                </span>
                {getProgressSeconds(progress) > 0 && (
                  <span className="text-xs text-blue-600">
                    {formatDuration(getProgressSeconds(progress))} / {formatDuration(getDurationSeconds(progress))}
                  </span>
                )}
              </div>
              <div className="w-full bg-blue-200 rounded-full h-2">
                <div
                  className={`h-full rounded-full transition-all ${
                    progress?.is_watched ? 'bg-green-500' : 'bg-blue-500'
                  }`}
                  style={{ width: `${Math.min(getProgressPercentage(progress), 100)}%` }}
                />
              </div>
            </div>
          )}

          <div>
            <span className="font-semibold">File Path:</span>
            <p className="text-sm text-gray-600 break-all">{media.file_path}</p>
          </div>
          {media.file_size && (
            <div>
              <span className="font-semibold">File Size:</span> {formatFileSize(media.file_size)}
            </div>
          )}
          {media.duration && (
            <div>
              <span className="font-semibold">Duration:</span> {formatDuration(media.duration)}
            </div>
          )}
        </div>
      </ModalContent>
      <ModalFooter>
        <div className="flex gap-2 w-full flex-wrap">
          {canResume ? (
            <>
              <Button
                variant="primary"
                onClick={() => handlePlay(false)}
                className="flex-1"
              >
                Resume from {formatDuration(progressSecs)}
              </Button>
              <Button
                variant="secondary"
                onClick={() => handlePlay(true)}
              >
                Play from Start
              </Button>
            </>
          ) : (
            <Button
              variant="primary"
              onClick={() => handlePlay(true)}
              className="flex-1"
            >
              Play
            </Button>
          )}
          <Button
            variant={progress?.is_watched ? 'secondary' : 'ghost'}
            onClick={handleToggleWatched}
            disabled={markWatched.isPending || markUnwatched.isPending}
          >
            {progress?.is_watched ? 'Mark Unwatched' : 'Mark Watched'}
          </Button>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
        </div>
      </ModalFooter>
    </Modal>
  )
}

export { MediaDetailsModal }
export type { MediaDetailsModalProps } from './MediaDetailsModal.types'

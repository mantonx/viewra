import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'
import { formatFileSize, formatDuration, DEFAULT_USER_ID, getProgressPercentage, getProgressSeconds, getDurationSeconds, hasProgress } from '@/lib/utils'
import type { MediaDetailsModalProps } from './MediaDetailsModal.types'
import { useMediaProgress, useMarkWatched, useMarkUnwatched } from '@/lib/hooks/useProgress'

const MediaDetailsModal = ({ media, onClose }: MediaDetailsModalProps) => {
  const { data: progress } = useMediaProgress(media.id)
  const markWatched = useMarkWatched()
  const markUnwatched = useMarkUnwatched()

  const handleToggleWatched = () => {
    if (progress?.is_watched) {
      markUnwatched.mutate({ media_id: media.id, user_id: DEFAULT_USER_ID })
    } else {
      markWatched.mutate({ media_id: media.id, user_id: DEFAULT_USER_ID })
    }
  }

  // Build stream URL with resume timestamp if available
  const getStreamUrl = () => {
    const baseUrl = `http://localhost:8080/api/stream/${media.id}`
    const progressSecs = getProgressSeconds(progress)
    if (progress && progressSecs > 0 && !progress.is_watched) {
      return `${baseUrl}#t=${progressSecs}`
    }
    return baseUrl
  }

  const showProgress = hasProgress(progress)

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
        <div className="flex gap-2 w-full">
          <a
            href={getStreamUrl()}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center px-4 py-2 bg-blue-600 text-white rounded font-medium hover:bg-blue-700 transition-colors"
          >
            {showProgress && !progress?.is_watched ? '▶ Resume' : '▶ Play'}
          </a>
          <Button
            variant={progress?.is_watched ? 'secondary' : 'primary'}
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

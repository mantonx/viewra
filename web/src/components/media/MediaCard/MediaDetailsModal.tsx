import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'
import { formatFileSize, formatDuration } from '@/lib/utils'
import type { MediaDetailsModalProps } from './MediaDetailsModal.types'

const MediaDetailsModal = ({ media, onClose }: MediaDetailsModalProps) => {
  return (
    <Modal isOpen={true} onClose={onClose} title={media.title} size="md">
      <ModalContent>
        <div className="space-y-3">
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
        <a
          href={`http://localhost:8080/api/stream/${media.id}`}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center justify-center px-4 py-2 bg-blue-600 text-white rounded font-medium hover:bg-blue-700 transition-colors"
        >
          ▶ Play
        </a>
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      </ModalFooter>
    </Modal>
  )
}

export { MediaDetailsModal }
export type { MediaDetailsModalProps } from './MediaDetailsModal.types'

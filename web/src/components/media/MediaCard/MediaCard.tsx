import { useState } from 'react'
import { MediaDetailsModal } from './MediaDetailsModal'
import type { MediaCardProps } from './MediaCard.types'

const MediaCard = ({ media, onClick }: MediaCardProps) => {
  const [showDetails, setShowDetails] = useState(false)

  const handleClick = () => {
    setShowDetails(true)
    onClick?.()
  }

  return (
    <>
      <div
        className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
        onClick={handleClick}
      >
        {/* Thumbnail placeholder */}
        <div className="aspect-[2/3] bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white text-4xl">
          🎬
        </div>

        {/* Info */}
        <div className="p-3">
          <h3 className="font-semibold text-sm line-clamp-2 mb-1">{media.title}</h3>
        </div>
      </div>

      {/* Modal */}
      {showDetails && <MediaDetailsModal media={media} onClose={() => setShowDetails(false)} />}
    </>
  )
}

export { MediaCard }
export type { MediaCardProps } from './MediaCard.types'

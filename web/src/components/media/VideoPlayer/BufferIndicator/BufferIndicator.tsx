import { memo } from 'react'
import type { BufferIndicatorProps } from './BufferIndicator.types'

const getBufferStatus = (
  bufferLength: number,
  minBuffer: number,
  criticalBuffer: number
): { color: string; label: string } => {
  if (bufferLength < criticalBuffer) {
    return { color: 'text-red-400', label: 'Critical' }
  }
  if (bufferLength < minBuffer) {
    return { color: 'text-yellow-400', label: 'Low' }
  }
  return { color: 'text-green-400', label: 'OK' }
}

/**
 * Minimal buffer health indicator
 *
 * Shows a small pill in the bottom-left corner with buffer status.
 * Only prominent when buffer is low/critical.
 */
export const BufferIndicator = memo(({
  bufferLength,
  minBuffer = 5,
  criticalBuffer = 2,
  isVisible,
}: BufferIndicatorProps) => {
  if (!isVisible) {
    return null
  }

  const status = getBufferStatus(bufferLength, minBuffer, criticalBuffer)
  const isHealthy = bufferLength >= minBuffer

  return (
    <div
      className={`
        absolute bottom-24 left-4 z-20
        px-2 py-1 rounded-md text-[10px] font-mono
        bg-black/60 backdrop-blur-sm border border-white/10
        transition-opacity duration-300
        ${isHealthy ? 'opacity-40 hover:opacity-100' : 'opacity-100'}
      `}
    >
      <span className="text-white/50">Buffer </span>
      <span className={status.color}>
        {bufferLength.toFixed(1)}s
      </span>
    </div>
  )
})

BufferIndicator.displayName = 'BufferIndicator'

export default BufferIndicator

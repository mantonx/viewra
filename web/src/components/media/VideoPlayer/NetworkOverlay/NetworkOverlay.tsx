import { memo, useEffect, useRef, useState, useCallback } from 'react'
import { X, GripHorizontal } from 'lucide-react'
import type { NetworkStats } from '@/lib/network/NetworkMonitor'
import type { NetworkOverlayProps } from './NetworkOverlay.types'

// Keep last 60 samples for the graph (30 seconds at 2s intervals)
const MAX_GRAPH_SAMPLES = 60
const GRAPH_WIDTH = 200
const GRAPH_HEIGHT = 60

const getTrendIcon = (trend: NetworkStats['trend']): string => {
  switch (trend) {
    case 'improving':
      return '↑'
    case 'degrading':
      return '↓'
    default:
      return '→'
  }
}

const getTrendColor = (trend: NetworkStats['trend']): string => {
  switch (trend) {
    case 'improving':
      return 'text-green-400'
    case 'degrading':
      return 'text-red-400'
    default:
      return 'text-white/70'
  }
}

const getStabilityColor = (stability: number): string => {
  if (stability > 0.7) {
    return 'text-green-400'
  }
  if (stability > 0.4) {
    return 'text-yellow-400'
  }
  return 'text-red-400'
}

const getBufferColor = (bufferLength: number): string => {
  if (bufferLength > 10) {
    return 'text-green-400'
  }
  if (bufferLength > 5) {
    return 'text-yellow-400'
  }
  return 'text-red-400'
}

const getConnectionQuality = (throughputMbps: number): { label: string; color: string } => {
  if (throughputMbps >= 20) {
    return { label: 'Excellent', color: 'text-green-400' }
  }
  if (throughputMbps >= 8) {
    return { label: 'Good', color: 'text-green-400' }
  }
  if (throughputMbps >= 2) {
    return { label: 'Fair', color: 'text-yellow-400' }
  }
  return { label: 'Poor', color: 'text-red-400' }
}

/**
 * Draw a smooth line graph on canvas
 */
const drawGraph = (
  ctx: CanvasRenderingContext2D,
  samples: number[],
  width: number,
  height: number
) => {
  ctx.clearRect(0, 0, width, height)

  if (samples.length < 2) {
    return
  }

  // Find max for scaling (minimum 10 Mbps for scale)
  const maxValue = Math.max(10, ...samples) * 1.1

  // Draw grid lines
  ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)'
  ctx.lineWidth = 1
  for (let i = 1; i < 4; i++) {
    const y = (height / 4) * i
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(width, y)
    ctx.stroke()
  }

  // Draw the throughput line with gradient
  const gradient = ctx.createLinearGradient(0, 0, 0, height)
  gradient.addColorStop(0, 'rgba(59, 130, 246, 0.8)') // Blue at top
  gradient.addColorStop(1, 'rgba(59, 130, 246, 0.2)') // Faded at bottom

  // Draw fill area
  ctx.beginPath()
  ctx.moveTo(0, height)

  const stepX = width / (MAX_GRAPH_SAMPLES - 1)
  samples.forEach((value, index) => {
    const x = index * stepX
    const y = height - (value / maxValue) * height
    if (index === 0) {
      ctx.lineTo(x, y)
    } else {
      ctx.lineTo(x, y)
    }
  })

  ctx.lineTo((samples.length - 1) * stepX, height)
  ctx.closePath()
  ctx.fillStyle = gradient
  ctx.fill()

  // Draw the line on top
  ctx.beginPath()
  ctx.strokeStyle = 'rgba(96, 165, 250, 1)' // Brighter blue
  ctx.lineWidth = 2
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'

  samples.forEach((value, index) => {
    const x = index * stepX
    const y = height - (value / maxValue) * height
    if (index === 0) {
      ctx.moveTo(x, y)
    } else {
      ctx.lineTo(x, y)
    }
  })
  ctx.stroke()

  // Draw current value dot
  if (samples.length > 0) {
    const lastX = (samples.length - 1) * stepX
    const lastY = height - (samples[samples.length - 1] / maxValue) * height
    ctx.beginPath()
    ctx.arc(lastX, lastY, 3, 0, Math.PI * 2)
    ctx.fillStyle = 'rgba(96, 165, 250, 1)'
    ctx.fill()
    ctx.strokeStyle = 'white'
    ctx.lineWidth = 1
    ctx.stroke()
  }
}

/**
 * Debug overlay showing network statistics and quality decisions
 * Draggable panel positioned on the right side of the video player
 */
const MIN_SAMPLES_REQUIRED = 3

export const NetworkOverlay = memo(({
  stats,
  decision,
  currentQuality,
  bufferLength,
  sampleCount,
  isVisible,
  onClose,
}: NetworkOverlayProps) => {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const samplesRef = useRef<number[]>([])
  const overlayRef = useRef<HTMLDivElement>(null)
  const [position, setPosition] = useState({ x: 0, y: 0 })
  const [isDragging, setIsDragging] = useState(false)
  const dragStartRef = useRef({ x: 0, y: 0, posX: 0, posY: 0 })

  // Handle drag start
  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    setIsDragging(true)
    dragStartRef.current = {
      x: e.clientX,
      y: e.clientY,
      posX: position.x,
      posY: position.y,
    }
  }, [position])

  // Handle dragging
  useEffect(() => {
    if (!isDragging) {
      return
    }

    const handleMouseMove = (e: MouseEvent) => {
      const deltaX = e.clientX - dragStartRef.current.x
      const deltaY = e.clientY - dragStartRef.current.y
      setPosition({
        x: dragStartRef.current.posX + deltaX,
        y: dragStartRef.current.posY + deltaY,
      })
    }

    const handleMouseUp = () => {
      setIsDragging(false)
    }

    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)

    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }
  }, [isDragging])

  // Update graph when stats change
  useEffect(() => {
    if (!stats || !isVisible) {
      return
    }

    // Add new sample
    samplesRef.current.push(stats.currentThroughputMbps)

    // Keep only last MAX_GRAPH_SAMPLES
    if (samplesRef.current.length > MAX_GRAPH_SAMPLES) {
      samplesRef.current = samplesRef.current.slice(-MAX_GRAPH_SAMPLES)
    }

    // Draw graph
    const canvas = canvasRef.current
    if (canvas) {
      const ctx = canvas.getContext('2d')
      if (ctx) {
        drawGraph(ctx, samplesRef.current, GRAPH_WIDTH, GRAPH_HEIGHT)
      }
    }
  }, [stats, isVisible])

  // Clear samples when hidden
  useEffect(() => {
    if (!isVisible) {
      samplesRef.current = []
    }
  }, [isVisible])

  if (!isVisible) {
    return null
  }

  return (
    <div
      ref={overlayRef}
      className="absolute top-4 right-4 z-30 bg-black/85 backdrop-blur-md rounded-lg text-xs font-mono text-white/90 min-w-56 shadow-xl border border-white/10"
      style={{
        transform: `translate(${position.x}px, ${position.y}px)`,
        cursor: isDragging ? 'grabbing' : 'auto',
      }}
    >
      {/* Drag Handle */}
      <div
        className="flex items-center justify-center py-1.5 cursor-grab active:cursor-grabbing border-b border-white/10 hover:bg-white/5 transition-colors rounded-t-lg"
        onMouseDown={handleMouseDown}
      >
        <GripHorizontal className="w-4 h-4 text-white/30" />
      </div>

      {/* Content */}
      <div className="p-3">
        {/* Header */}
        <div className="flex items-center justify-between mb-2 pb-2 border-b border-white/10">
          <span className="text-white/50 text-[10px] uppercase tracking-wider">Network Stats</span>
          <button
            onClick={onClose}
            className="p-1 -m-1 text-white/40 hover:text-white/80 hover:bg-white/10 rounded transition-colors cursor-pointer"
            title="Close (D)"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>

      {stats ? (
        <>
          {/* Metrics Grid */}
          <div className="space-y-1.5">
            {/* Speed */}
            <div className="flex justify-between items-center">
              <span className="text-white/50">Speed</span>
              <span className={getTrendColor(stats.trend)}>
                {stats.averageThroughputMbps.toFixed(1)} Mbps {getTrendIcon(stats.trend)}
              </span>
            </div>

            {/* Buffer */}
            <div className="flex justify-between items-center">
              <span className="text-white/50">Buffer</span>
              <span className={getBufferColor(bufferLength)}>
                {bufferLength.toFixed(1)}s
              </span>
            </div>

            {/* Stability */}
            <div className="flex justify-between items-center">
              <span className="text-white/50">Stability</span>
              <span className={getStabilityColor(stats.stability)}>
                {(stats.stability * 100).toFixed(0)}%
              </span>
            </div>

            {/* Quality */}
            <div className="flex justify-between items-center">
              <span className="text-white/50">Quality</span>
              <span className="text-blue-400">{currentQuality}</span>
            </div>

            {/* Connection quality - inferred from measured throughput */}
            <div className="flex justify-between items-center">
              <span className="text-white/50">Connection</span>
              <span className={getConnectionQuality(stats.averageThroughputMbps).color}>
                {getConnectionQuality(stats.averageThroughputMbps).label}
              </span>
            </div>

            {/* Range */}
            <div className="flex justify-between items-center">
              <span className="text-white/50">Range</span>
              <span className="text-white/50">
                {stats.minThroughputMbps.toFixed(1)}–{stats.maxThroughputMbps.toFixed(1)} Mbps
              </span>
            </div>

            {/* Stalls */}
            {stats.stallCount > 0 && (
              <div className="flex justify-between items-center text-red-400">
                <span>Stalls</span>
                <span>{stats.stallCount}</span>
              </div>
            )}

            {/* Metered */}
            {stats.isMetered && (
              <div className="flex justify-between items-center text-yellow-400">
                <span>Metered</span>
                <span>Yes</span>
              </div>
            )}
          </div>

          {/* Throughput Graph */}
          <div className="mt-3 pt-3 border-t border-white/10">
            <div className="flex justify-between items-center mb-1.5">
              <span className="text-white/50 text-[10px]">Throughput</span>
              <span className="text-white/30 text-[10px]">
                {stats.currentThroughputMbps.toFixed(1)} Mbps
              </span>
            </div>
            <div className="bg-white/5 rounded overflow-hidden">
              <canvas
                ref={canvasRef}
                width={GRAPH_WIDTH}
                height={GRAPH_HEIGHT}
                className="w-full"
                style={{ height: `${GRAPH_HEIGHT}px` }}
              />
            </div>
          </div>

          {/* Decision */}
          {decision && decision.action !== 'maintain' && (
            <div className="mt-3 pt-3 border-t border-white/10">
              <div className={`flex items-center gap-1.5 ${decision.action === 'upgrade' ? 'text-green-400' : 'text-yellow-400'}`}>
                <span>{decision.action === 'upgrade' ? '↑' : '↓'}</span>
                <span>
                  {decision.action === 'upgrade' ? 'Upgrading' : 'Downgrading'}
                  {decision.targetLevel && ` to ${decision.targetLevel.name}`}
                </span>
              </div>
              <div className="text-white/40 text-[10px] mt-0.5">{decision.reason}</div>
              <div className="text-white/30 text-[10px]">
                Confidence: {(decision.confidence * 100).toFixed(0)}%
              </div>
            </div>
          )}
        </>
      ) : (
        <div className="text-center py-3">
          <div className="text-white/60 mb-2">Collecting network data...</div>

          {/* Sample progress indicator */}
          <div className="flex items-center justify-center gap-1.5 mb-2">
            {[...Array(MIN_SAMPLES_REQUIRED)].map((_, i) => (
              <div
                key={i}
                className={`w-2 h-2 rounded-full transition-colors ${
                  i < sampleCount ? 'bg-blue-400' : 'bg-white/20'
                }`}
              />
            ))}
          </div>

          <div className="text-white/40 text-[10px]">
            {sampleCount} of {MIN_SAMPLES_REQUIRED} samples
          </div>

          {/* Show buffer while waiting */}
          <div className="mt-3 pt-3 border-t border-white/10">
            <div className="flex justify-between items-center">
              <span className="text-white/50">Buffer</span>
              <span className={getBufferColor(bufferLength)}>
                {bufferLength.toFixed(1)}s
              </span>
            </div>
            <div className="flex justify-between items-center mt-1.5">
              <span className="text-white/50">Quality</span>
              <span className="text-blue-400">{currentQuality}</span>
            </div>
          </div>
        </div>
      )}
      </div>
    </div>
  )
})

NetworkOverlay.displayName = 'NetworkOverlay'

export default NetworkOverlay

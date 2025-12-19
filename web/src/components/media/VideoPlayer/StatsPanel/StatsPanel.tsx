import { memo, useState, useRef, useEffect, useCallback } from 'react'
import { X, GripHorizontal, ChevronDown, ChevronRight } from 'lucide-react'
import type { NetworkStats } from '@/lib/network/NetworkMonitor'
import {
  formatBitrate,
  formatFileSize,
  formatResolution,
  formatFrameRate,
  getCodecDisplayName,
} from '@/lib/types/streamStats'
import type { StatRowProps, SectionProps, StatsPanelProps } from './StatsPanel.types'

// Graph constants
const MAX_GRAPH_SAMPLES = 60
const GRAPH_WIDTH = 240
const GRAPH_HEIGHT = 50

const StatRow = memo(({ label, value, valueColor = 'text-white/90', subValue }: StatRowProps) => {
  if (value === undefined || value === null || value === '') {
    return null
  }
  return (
    <div className="flex justify-between items-start gap-2 py-0.5">
      <span className="text-white/50 shrink-0">{label}</span>
      <div className="text-right">
        <span className={valueColor}>{value}</span>
        {subValue && <div className="text-white/30 text-[10px]">{subValue}</div>}
      </div>
    </div>
  )
})
StatRow.displayName = 'StatRow'

const Section = memo(({ title, children, defaultOpen = false }: SectionProps) => {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  return (
    <div className="border-t border-white/10 first:border-t-0">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full flex items-center justify-between py-2 text-white/50 text-[10px] uppercase tracking-wider hover:text-white/70 transition-colors cursor-pointer"
      >
        <span>{title}</span>
        {isOpen ? (
          <ChevronDown className="w-3 h-3" />
        ) : (
          <ChevronRight className="w-3 h-3" />
        )}
      </button>
      {isOpen && <div className="pb-2 space-y-0.5">{children}</div>}
    </div>
  )
})
Section.displayName = 'Section'

// Network helper functions
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
  if (stability > 0.7) {return 'text-green-400'}
  if (stability > 0.4) {return 'text-yellow-400'}
  return 'text-red-400'
}

const getConnectionQuality = (throughputMbps: number): { label: string; color: string } => {
  if (throughputMbps >= 20) {return { label: 'Excellent', color: 'text-green-400' }}
  if (throughputMbps >= 8) {return { label: 'Good', color: 'text-green-400' }}
  if (throughputMbps >= 2) {return { label: 'Fair', color: 'text-yellow-400' }}
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
  gradient.addColorStop(0, 'rgba(59, 130, 246, 0.8)')
  gradient.addColorStop(1, 'rgba(59, 130, 246, 0.2)')

  // Draw fill area
  ctx.beginPath()
  ctx.moveTo(0, height)

  const stepX = width / (MAX_GRAPH_SAMPLES - 1)
  samples.forEach((value, index) => {
    const x = index * stepX
    const y = height - (value / maxValue) * height
    ctx.lineTo(x, y)
  })

  ctx.lineTo((samples.length - 1) * stepX, height)
  ctx.closePath()
  ctx.fillStyle = gradient
  ctx.fill()

  // Draw the line on top
  ctx.beginPath()
  ctx.strokeStyle = 'rgba(96, 165, 250, 1)'
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
 * StatsPanel - Detailed stream statistics panel ("Stats for Nerds")
 *
 * Shows comprehensive information about:
 * - Source stream (container, bitrate, file size)
 * - Network (throughput graph, speed, stability)
 * - Video (codec, resolution, frame rate, HDR)
 * - Audio tracks (codec, channels, language)
 * - Output/playback (mode, transcoding info)
 * - Buffer health and dropped frames
 * - HLS quality levels
 */
export const StatsPanel = memo(({
  stats,
  networkStats,
  isVisible,
  onClose,
  isLoading = false,
  selectedQuality,
}: StatsPanelProps) => {
  const overlayRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const samplesRef = useRef<number[]>([])
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

  // Update graph when networkStats change
  useEffect(() => {
    if (!networkStats || !isVisible) {
      return
    }

    // Add new sample
    samplesRef.current.push(networkStats.currentThroughputMbps)

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
  }, [networkStats, isVisible])

  // Clear samples when hidden
  useEffect(() => {
    if (!isVisible) {
      samplesRef.current = []
    }
  }, [isVisible])

  if (!isVisible) {
    return null
  }

  // Get color for playback mode - green for direct play modes, blue for remux, yellow for transcode
  const getModeColor = (mode: string) => {
    switch (mode) {
      case 'direct':
        return 'text-green-400'
      case 'remux':
        return 'text-blue-400'
      case 'remux_audio':
        return 'text-blue-400'
      case 'remux_hevc':
        return 'text-green-400' // HEVC video is direct play
      case 'transcode':
        return 'text-yellow-400'
      default:
        return 'text-white/90'
    }
  }

  // Get display label for playback mode
  const getModeLabel = (mode: string) => {
    switch (mode) {
      case 'direct':
        return 'Direct Play'
      case 'remux':
        return 'Remux'
      case 'remux_audio':
        return 'Remux (Audio Trans.)'
      case 'remux_hevc':
        return 'Direct (HEVC)'
      case 'transcode':
        return 'Transcode'
      default:
        return mode.charAt(0).toUpperCase() + mode.slice(1)
    }
  }

  const getBufferColor = (buffer: number) => {
    if (buffer > 10) {return 'text-green-400'}
    if (buffer > 5) {return 'text-yellow-400'}
    return 'text-red-400'
  }

  const getDroppedFrameColor = (rate: number) => {
    if (rate < 0.1) {return 'text-green-400'}
    if (rate < 1) {return 'text-yellow-400'}
    return 'text-red-400'
  }

  return (
    <div
      ref={overlayRef}
      className="absolute top-4 right-4 z-30 bg-black/90 backdrop-blur-md rounded-lg text-xs font-mono text-white/90 w-72 max-h-[80%] overflow-hidden shadow-xl border border-white/10 flex flex-col"
      style={{
        transform: `translate(${position.x}px, ${position.y}px)`,
        cursor: isDragging ? 'grabbing' : 'auto',
      }}
    >
      {/* Drag Handle */}
      <div
        className="flex items-center justify-center py-1.5 cursor-grab active:cursor-grabbing border-b border-white/10 hover:bg-white/5 transition-colors rounded-t-lg shrink-0"
        onMouseDown={handleMouseDown}
      >
        <GripHorizontal className="w-4 h-4 text-white/30" />
      </div>

      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-white/10 shrink-0">
        <span className="text-white/50 text-[10px] uppercase tracking-wider">Stats for Nerds</span>
        <button
          onClick={onClose}
          className="p-1 -m-1 text-white/40 hover:text-white/80 hover:bg-white/10 rounded transition-colors cursor-pointer"
          title="Close (D)"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Content - Scrollable */}
      <div className="p-3 overflow-y-auto">
        {isLoading ? (
          <div className="text-center py-4 text-white/50">
            Loading stream info...
          </div>
        ) : !stats ? (
          <div className="text-center py-4 text-white/50">
            No stream data available
          </div>
        ) : (
          <>
            {/* Stream Section - Open by default */}
            <Section title="Stream" defaultOpen={true}>
              <StatRow
                label="Container"
                value={stats.source.stream.container.toUpperCase()}
              />
              <StatRow
                label="File Size"
                value={formatFileSize(stats.source.stream.fileSize)}
              />
              <StatRow
                label="Bitrate"
                value={formatBitrate(stats.source.stream.bitrate)}
              />
              <StatRow
                label="Playback"
                value={getModeLabel(stats.output.stream.mode)}
                valueColor={getModeColor(stats.output.stream.mode)}
              />
              {stats.output.stream.bitrate && (
                <StatRow
                  label="Output Bitrate"
                  value={formatBitrate(stats.output.stream.bitrate)}
                />
              )}
              {selectedQuality && (
                <StatRow
                  label="Quality"
                  value={`${selectedQuality.displayName}${selectedQuality.isOriginalQuality ? ' · Original' : ''}`}
                  valueColor={selectedQuality.isOriginalQuality ? 'text-amber-400' : 'text-white/90'}
                />
              )}
            </Section>

            {/* Network Section with Graph */}
            <Section title="Network">
              {networkStats ? (
                <>
                  <StatRow
                    label="Speed"
                    value={`${networkStats.averageThroughputMbps.toFixed(1)} Mbps ${getTrendIcon(networkStats.trend)}`}
                    valueColor={getTrendColor(networkStats.trend)}
                  />
                  <StatRow
                    label="Connection"
                    value={getConnectionQuality(networkStats.averageThroughputMbps).label}
                    valueColor={getConnectionQuality(networkStats.averageThroughputMbps).color}
                  />
                  <StatRow
                    label="Stability"
                    value={`${(networkStats.stability * 100).toFixed(0)}%`}
                    valueColor={getStabilityColor(networkStats.stability)}
                  />
                  <StatRow
                    label="Range"
                    value={`${networkStats.minThroughputMbps.toFixed(1)}–${networkStats.maxThroughputMbps.toFixed(1)} Mbps`}
                  />
                  {networkStats.stallCount > 0 && (
                    <StatRow
                      label="Stalls"
                      value={networkStats.stallCount}
                      valueColor="text-red-400"
                    />
                  )}
                  {networkStats.isMetered && (
                    <StatRow
                      label="Metered"
                      value="Yes"
                      valueColor="text-yellow-400"
                    />
                  )}
                  {/* Throughput Graph */}
                  <div className="mt-2 pt-2 border-t border-white/5">
                    <div className="flex justify-between items-center mb-1.5">
                      <span className="text-white/50 text-[10px]">Throughput</span>
                      <span className="text-white/30 text-[10px]">
                        {networkStats.currentThroughputMbps.toFixed(1)} Mbps
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
                </>
              ) : (
                <div className="text-white/40 text-[10px] py-1">Collecting network data...</div>
              )}
            </Section>

            {/* Video Section */}
            <Section title="Video">
              <StatRow
                label="Codec"
                value={getCodecDisplayName(stats.source.video.codec)}
                subValue={stats.source.video.profile ? `${stats.source.video.profile}` : undefined}
              />
              <StatRow
                label="Resolution"
                value={`${stats.source.video.width}x${stats.source.video.height}`}
                subValue={formatResolution(stats.source.video.height)}
              />
              <StatRow
                label="Frame Rate"
                value={`${formatFrameRate(stats.source.video.frameRate)} fps`}
              />
              {stats.source.video.bitrate && (
                <StatRow
                  label="Video Bitrate"
                  value={formatBitrate(stats.source.video.bitrate)}
                />
              )}
              {stats.source.video.hdrFormat && (
                <StatRow
                  label="HDR"
                  value={stats.source.video.hdrFormat}
                  valueColor="text-purple-400"
                />
              )}
              {stats.output.toneMapping?.enabled && (
                <StatRow
                  label="Tone Mapping"
                  value={stats.output.toneMapping.algorithm?.toUpperCase() ?? 'Enabled'}
                  valueColor="text-orange-400"
                  subValue={stats.output.toneMapping.backend}
                />
              )}
              {stats.source.video.colorSpace && (
                <StatRow
                  label="Color Space"
                  value={stats.source.video.colorSpace}
                />
              )}
              {stats.output.video && (
                <>
                  <div className="mt-1 pt-1 border-t border-white/5">
                    <StatRow
                      label="Output Codec"
                      value={getCodecDisplayName(stats.output.video.codec)}
                    />
                    {stats.output.video.height && (
                      <StatRow
                        label="Output Res"
                        value={formatResolution(stats.output.video.height)}
                      />
                    )}
                    {stats.output.video.reason && (
                      <div className="text-white/40 text-[10px] mt-1">
                        {stats.output.video.reason}
                      </div>
                    )}
                  </div>
                </>
              )}
            </Section>

            {/* Audio Section */}
            <Section title="Audio">
              {stats.source.audio.map((track, index) => (
                <div
                  key={index}
                  className={`${index > 0 ? 'mt-2 pt-2 border-t border-white/5' : ''}`}
                >
                  <div className="flex items-center gap-1.5 mb-1">
                    <span className="text-white/70">
                      {track.language}
                      {track.isDefault && (
                        <span className="ml-1 text-[9px] text-blue-400">(Default)</span>
                      )}
                    </span>
                  </div>
                  <StatRow
                    label="Codec"
                    value={getCodecDisplayName(track.codec)}
                  />
                  <StatRow
                    label="Channels"
                    value={track.channels}
                  />
                  {track.bitrate && (
                    <StatRow
                      label="Bitrate"
                      value={formatBitrate(track.bitrate)}
                    />
                  )}
                  {track.title && (
                    <StatRow
                      label="Title"
                      value={track.title}
                    />
                  )}
                </div>
              ))}
              {stats.output.audio && (
                <div className="mt-2 pt-2 border-t border-white/5">
                  <StatRow
                    label="Output Codec"
                    value={getCodecDisplayName(stats.output.audio.codec)}
                  />
                  {stats.output.audio.bitrate && (
                    <StatRow
                      label="Output Bitrate"
                      value={formatBitrate(stats.output.audio.bitrate)}
                    />
                  )}
                  {stats.output.audio.reason && (
                    <div className="text-white/40 text-[10px] mt-1">
                      {stats.output.audio.reason}
                    </div>
                  )}
                </div>
              )}
            </Section>

            {/* Playback Section */}
            <Section title="Playback">
              <StatRow
                label="Buffer"
                value={`${stats.buffer.forwardBuffer.toFixed(1)}s`}
                valueColor={getBufferColor(stats.buffer.forwardBuffer)}
              />
              <StatRow
                label="Dropped Frames"
                value={stats.playback.droppedFrames}
                valueColor={getDroppedFrameColor(stats.playback.droppedFrameRate)}
                subValue={stats.playback.totalFrames > 0
                  ? `${stats.playback.droppedFrameRate.toFixed(2)}% of ${stats.playback.totalFrames}`
                  : undefined
                }
              />
              {stats.buffer.hasGaps && (
                <StatRow
                  label="Buffer Gaps"
                  value="Yes"
                  valueColor="text-yellow-400"
                />
              )}
              <StatRow
                label="Playback Rate"
                value={`${stats.session.playbackRate}x`}
              />
              {stats.session.streamOffset > 0 && (
                <StatRow
                  label="Stream Offset"
                  value={`${stats.session.streamOffset.toFixed(1)}s`}
                />
              )}
            </Section>

            {/* HLS Section */}
            {stats.hls && (
              <Section title="HLS">
                <StatRow
                  label="Current Level"
                  value={stats.hls.currentLevelHeight
                    ? `${stats.hls.currentLevelHeight}p`
                    : `Level ${stats.hls.currentLevel}`
                  }
                />
                {stats.hls.currentLevelBitrate && (
                  <StatRow
                    label="Level Bitrate"
                    value={formatBitrate(stats.hls.currentLevelBitrate)}
                  />
                )}
                {stats.hls.bandwidthEstimate && (
                  <StatRow
                    label="Bandwidth Est."
                    value={formatBitrate(stats.hls.bandwidthEstimate)}
                  />
                )}
                <div className="mt-2 pt-2 border-t border-white/5">
                  <div className="text-white/50 text-[10px] mb-1">Available Levels</div>
                  <div className="space-y-0.5">
                    {stats.hls.availableLevels.map((level, index) => (
                      <div
                        key={index}
                        className={`flex justify-between items-center ${
                          index === stats.hls?.currentLevel ? 'text-blue-400' : 'text-white/50'
                        }`}
                      >
                        <span>{level.height}p</span>
                        <span>{formatBitrate(level.bitrate)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              </Section>
            )}

            {/* Buffer Visualization */}
            <Section title="Buffer Ranges">
              {stats.buffer.ranges.length === 0 ? (
                <div className="text-white/40 text-[10px]">No buffer data</div>
              ) : (
                <div className="space-y-1">
                  {stats.buffer.ranges.map((range, index) => (
                    <div key={index} className="flex justify-between text-white/50">
                      <span>Range {index + 1}</span>
                      <span>
                        {range.start.toFixed(1)}s - {range.end.toFixed(1)}s
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </Section>
          </>
        )}
      </div>
    </div>
  )
})

StatsPanel.displayName = 'StatsPanel'

export default StatsPanel

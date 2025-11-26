import { useState, useRef, useEffect } from 'react'
import type { QualityRecommendationResponse } from '@/lib/api/adaptive'

interface QualityOption {
  height: number
  bandwidth: number
  // Extended properties from adaptive profiles
  displayName?: string
  dataUsageMBPerHour?: number
  canDirectPlay?: boolean
  needsTranscode?: boolean
  isRecommended?: boolean
  description?: string
}

interface QualitySelectorProps {
  currentQuality: number | null
  availableQualities: QualityOption[]
  recommendedQuality: QualityRecommendationResponse | null
  autoMode: boolean
  onQualityChange: (height: number) => void
  onAutoToggle: () => void
}

// Format bandwidth to human readable string
const formatBandwidth = (bps: number): string => {
  if (bps >= 1_000_000) {
    return `${(bps / 1_000_000).toFixed(1)} Mbps`
  }
  return `${(bps / 1_000).toFixed(0)} kbps`
}

// Estimate data usage from bandwidth
const estimateDataUsage = (bps: number): string => {
  // MB per hour = (bps / 8) * 3600 / 1_000_000
  const mbPerHour = (bps / 8) * 3600 / 1_000_000
  if (mbPerHour >= 1000) {
    return `${(mbPerHour / 1000).toFixed(1)} GB/hr`
  }
  return `${mbPerHour.toFixed(0)} MB/hr`
}

export const QualitySelector = ({
  currentQuality,
  availableQualities,
  recommendedQuality,
  autoMode,
  onQualityChange,
  onAutoToggle,
}: QualitySelectorProps) => {
  const [showPanel, setShowPanel] = useState(false)
  const [hoveredQuality, setHoveredQuality] = useState<number | null>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  // Close panel when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        panelRef.current &&
        buttonRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        !buttonRef.current.contains(e.target as Node)
      ) {
        setShowPanel(false)
      }
    }

    if (showPanel) {
      document.addEventListener('mousedown', handleClickOutside)
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [showPanel])

  // Get display text for current quality
  const getQualityDisplayText = () => {
    if (autoMode) {
      return currentQuality ? `Auto (${currentQuality}p)` : 'Auto'
    }
    return currentQuality ? `${currentQuality}p` : 'Auto'
  }

  // Check if a quality is recommended
  const isRecommended = (height: number) => {
    return recommendedQuality?.height === height
  }

  // Sort qualities by height descending
  const sortedQualities = [...availableQualities].sort((a, b) => b.height - a.height)

  return (
    <div className="relative">
      {/* Quality button */}
      <button
        ref={buttonRef}
        onClick={() => setShowPanel(!showPanel)}
        className="bg-white/10 backdrop-blur-sm text-white text-xs sm:text-sm rounded-md px-2 sm:px-3 py-1.5 hover:bg-white/20 transition-all cursor-pointer border border-white/20 focus:outline-none focus:ring-2 focus:ring-primary-500/50 flex items-center gap-1"
        style={{ minWidth: '80px' }}
        aria-label="Video quality"
        aria-expanded={showPanel}
        aria-haspopup="listbox"
      >
        {/* Settings icon */}
        <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
          <path d="M19.14 12.94c.04-.31.06-.63.06-.94 0-.31-.02-.63-.06-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z" />
        </svg>
        <span>{getQualityDisplayText()}</span>
        {/* Chevron */}
        <svg
          className={`w-3 h-3 transition-transform ${showPanel ? 'rotate-180' : ''}`}
          fill="currentColor"
          viewBox="0 0 20 20"
        >
          <path
            fillRule="evenodd"
            d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
            clipRule="evenodd"
          />
        </svg>
      </button>

      {/* Quality panel */}
      {showPanel && (
        <div
          ref={panelRef}
          className="absolute bottom-full right-0 mb-2 w-72 bg-black/95 backdrop-blur-md rounded-lg shadow-xl border border-white/20 overflow-hidden z-50"
          role="listbox"
          aria-label="Video quality options"
        >
          {/* Header */}
          <div className="px-3 py-2 border-b border-white/10">
            <div className="text-white text-sm font-semibold">Video Quality</div>
            {recommendedQuality && (
              <div className="text-white/60 text-xs mt-0.5">
                {recommendedQuality.reason}
              </div>
            )}
          </div>

          {/* Quality options */}
          <div className="max-h-64 overflow-y-auto">
            {/* Auto option */}
            <button
              onClick={() => {
                onAutoToggle()
                setShowPanel(false)
              }}
              onMouseEnter={() => setHoveredQuality(0)}
              onMouseLeave={() => setHoveredQuality(null)}
              className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${
                autoMode ? 'bg-primary-500/20' : ''
              }`}
              role="option"
              aria-selected={autoMode}
            >
              <div className="flex items-center gap-2">
                <span className="text-white text-sm font-medium">Auto</span>
                <span className="text-white/50 text-xs">
                  {currentQuality ? `Currently ${currentQuality}p` : 'Adaptive'}
                </span>
              </div>
              {autoMode && (
                <svg className="w-4 h-4 text-primary-400" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                    clipRule="evenodd"
                  />
                </svg>
              )}
            </button>

            {/* Divider */}
            <div className="border-t border-white/10" />

            {/* Quality levels */}
            {sortedQualities.map((quality) => {
              const isSelected = !autoMode && currentQuality === quality.height
              const isRec = isRecommended(quality.height)
              const isHovered = hoveredQuality === quality.height

              return (
                <button
                  key={quality.height}
                  onClick={() => {
                    onQualityChange(quality.height)
                    setShowPanel(false)
                  }}
                  onMouseEnter={() => setHoveredQuality(quality.height)}
                  onMouseLeave={() => setHoveredQuality(null)}
                  className={`w-full px-3 py-2.5 flex items-start justify-between hover:bg-white/10 transition-colors cursor-pointer ${
                    isSelected ? 'bg-primary-500/20' : ''
                  }`}
                  role="option"
                  aria-selected={isSelected}
                >
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="text-white text-sm font-medium">{quality.height}p</span>
                      {isRec && (
                        <span className="text-primary-400 text-xs flex items-center gap-0.5">
                          <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                            <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                          </svg>
                          Recommended
                        </span>
                      )}
                    </div>

                    {/* Expanded info on hover */}
                    <div
                      className={`overflow-hidden transition-all duration-200 ${
                        isHovered || isSelected ? 'max-h-20 opacity-100 mt-1' : 'max-h-0 opacity-0'
                      }`}
                    >
                      <div className="text-white/60 text-xs space-y-0.5">
                        <div className="flex items-center gap-3">
                          <span>{formatBandwidth(quality.bandwidth)}</span>
                          <span>{estimateDataUsage(quality.bandwidth)}</span>
                        </div>
                        {quality.canDirectPlay !== undefined && (
                          <div className="flex items-center gap-1">
                            {quality.canDirectPlay ? (
                              <>
                                <svg className="w-3 h-3 text-yellow-400" fill="currentColor" viewBox="0 0 20 20">
                                  <path
                                    fillRule="evenodd"
                                    d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z"
                                    clipRule="evenodd"
                                  />
                                </svg>
                                <span className="text-yellow-400">Instant (no transcoding)</span>
                              </>
                            ) : (
                              <span className="text-white/40">Transcoding required</span>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>

                  {isSelected && (
                    <svg className="w-4 h-4 text-primary-400 shrink-0 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                        clipRule="evenodd"
                      />
                    </svg>
                  )}
                </button>
              )
            })}
          </div>

          {/* Footer with data usage warning if on metered connection */}
          {recommendedQuality?.dataUsageMBPerHour && (
            <div className="px-3 py-2 border-t border-white/10 bg-white/5">
              <div className="text-white/50 text-xs flex items-center gap-1">
                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                    clipRule="evenodd"
                  />
                </svg>
                <span>
                  Current: ~{recommendedQuality.dataUsageMBPerHour.toFixed(0)} MB/hr
                </span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

export default QualitySelector

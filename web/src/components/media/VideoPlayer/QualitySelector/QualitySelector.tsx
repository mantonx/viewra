/**
 * QualitySelector Component
 *
 * Dropdown for selecting video quality with two modes:
 * - Auto: HLS.js ABR handles quality (shows current quality with "Auto" badge)
 * - Manual: User selects specific quality (shows selected quality)
 */

import { useState, useMemo, useEffect } from 'react'
import { DropdownBase } from '../DropdownBase'
import type { QualityOption } from '@/lib/types/video'
import type { QualitySelectorProps } from './QualitySelector.types'

const formatBandwidth = (bps: number): string => {
  if (bps >= 1_000_000) {
    return `${(bps / 1_000_000).toFixed(0)} Mbps`
  }
  return `${(bps / 1_000).toFixed(0)} kbps`
}

const SettingsIcon = () => (
  <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
    <path d="M19.14 12.94c.04-.31.06-.63.06-.94 0-.31-.02-.63-.06-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z" />
  </svg>
)

const CheckIcon = () => (
  <svg className="w-4 h-4 text-primary-400" fill="currentColor" viewBox="0 0 20 20">
    <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
  </svg>
)

const ChevronIcon = ({ expanded }: { expanded: boolean }) => (
  <svg className={`w-4 h-4 text-white/50 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="currentColor" viewBox="0 0 20 20">
    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
  </svg>
)

export const QualitySelector = ({
  currentQuality,
  currentBandwidth,
  availableQualities,
  autoMode,
  onQualityChange,
  onAutoToggle,
  showBitrateVariants = true,
}: QualitySelectorProps) => {
  // Track which resolution groups are expanded
  const [expandedResolutions, setExpandedResolutions] = useState<Set<number>>(new Set())

  // Auto-expand the resolution group containing the selected quality
  useEffect(() => {
    if (currentQuality) {
      setExpandedResolutions(prev => {
        if (!prev.has(currentQuality)) {
          return new Set([...prev, currentQuality])
        }
        return prev
      })
    }
  }, [currentQuality])

  // Group qualities by resolution
  const qualitiesByResolution = useMemo(() => {
    const grouped = new Map<number, QualityOption[]>()
    for (const q of availableQualities) {
      const existing = grouped.get(q.height) || []
      existing.push(q)
      grouped.set(q.height, existing)
    }
    // Sort each group by bandwidth (highest first)
    for (const [height, options] of grouped) {
      grouped.set(height, options.sort((a, b) => b.bandwidth - a.bandwidth))
    }
    return grouped
  }, [availableQualities])

  const hasMultipleBitrates = useMemo(() => {
    for (const options of qualitiesByResolution.values()) {
      if (options.length > 1) return true
    }
    return false
  }, [qualitiesByResolution])

  const sortedHeights = useMemo(() => {
    return Array.from(qualitiesByResolution.keys()).sort((a, b) => b - a)
  }, [qualitiesByResolution])

  const toggleExpanded = (height: number) => {
    setExpandedResolutions(prev => {
      const next = new Set(prev)
      if (next.has(height)) {
        next.delete(height)
      } else {
        next.add(height)
      }
      return next
    })
  }

  // Get display text for the button
  const getQualityDisplayText = () => {
    if (!currentQuality) {
      return autoMode ? 'Auto' : 'Quality'
    }
    if (autoMode) {
      return `Auto · ${currentQuality}p`
    }
    return `${currentQuality}p`
  }

  // Check if a specific quality option is currently active
  const isQualityActive = (q: QualityOption) => {
    if (currentQuality !== q.height) return false
    if (currentBandwidth && q.bandwidth !== currentBandwidth) return false
    return true
  }

  // Get the selected option within a resolution group
  const getActiveOptionInGroup = (height: number): QualityOption | null => {
    if (currentQuality !== height) return null
    const options = qualitiesByResolution.get(height) || []
    return options.find(q => q.bandwidth === currentBandwidth) || null
  }

  const renderQualityOption = (quality: QualityOption, close: () => void, isNested = false) => {
    const isActive = isQualityActive(quality)

    return (
      <button
        key={`${quality.height}-${quality.bandwidth}`}
        onClick={(e) => {
          e.stopPropagation()
          onQualityChange(quality.height, quality.bandwidth)
          close()
        }}
        className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${isActive ? 'bg-primary-500/20' : ''} ${isNested ? 'pl-6 bg-white/5' : ''}`}
        role="option"
        aria-selected={isActive}
      >
        <div className="flex items-center gap-2">
          {isNested ? (
            <>
              <span className="text-white text-sm">{formatBandwidth(quality.bandwidth)}</span>
              {quality.isOriginal && (
                <span className="text-amber-400 text-xs font-medium px-1.5 py-0.5 bg-amber-400/20 rounded">Original</span>
              )}
            </>
          ) : (
            <>
              <span className="text-white text-sm font-medium">{quality.height}p</span>
              <span className="text-white/50 text-xs">{formatBandwidth(quality.bandwidth)}</span>
              {quality.isOriginal && (
                <span className="text-amber-400 text-xs font-medium px-1.5 py-0.5 bg-amber-400/20 rounded">Original</span>
              )}
            </>
          )}
        </div>
        {isActive && <CheckIcon />}
      </button>
    )
  }

  const renderResolutionGroup = (height: number, close: () => void) => {
    const options = qualitiesByResolution.get(height) || []
    const hasBitrateVariants = options.length > 1 && showBitrateVariants
    const isExpanded = expandedResolutions.has(height)
    const activeOption = getActiveOptionInGroup(height)
    const isGroupActive = activeOption !== null

    // Single bitrate at this resolution - render as simple option
    if (!hasBitrateVariants) {
      return options[0] ? renderQualityOption(options[0], close) : null
    }

    // Multiple bitrates - render as expandable group
    return (
      <div key={height}>
        <button
          onClick={() => toggleExpanded(height)}
          className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${isGroupActive ? 'bg-primary-500/10' : ''}`}
        >
          <div className="flex items-center gap-2">
            <span className="text-white text-sm font-medium">{height}p</span>
            {isGroupActive && !isExpanded ? (
              <span className="text-primary-400 text-xs">
                {formatBandwidth(activeOption.bandwidth)}
              </span>
            ) : (
              <span className="text-white/50 text-xs">{options.length} bitrates</span>
            )}
          </div>
          <div className="flex items-center gap-1.5">
            {isGroupActive && <CheckIcon />}
            <ChevronIcon expanded={isExpanded} />
          </div>
        </button>
        {isExpanded && (
          <div className="border-l-2 border-white/10 ml-3">
            {options.map((opt) => renderQualityOption(opt, close, true))}
          </div>
        )}
      </div>
    )
  }

  return (
    <DropdownBase
      buttonContent={getQualityDisplayText()}
      icon={<SettingsIcon />}
      minButtonWidth="80px"
      panelWidth="w-72"
      ariaLabel="Video quality"
    >
      {({ close }) => (
        <>
          {/* Header */}
          <div className="px-3 py-2.5 border-b border-white/10">
            <div className="text-white text-sm font-semibold">Video Quality</div>
          </div>

          <div className="max-h-80 overflow-y-auto">
            {/* Auto mode toggle */}
            <div className="px-3 py-3 flex items-center justify-between border-b border-white/10">
              <div>
                <div className="text-white text-sm font-medium">Auto</div>
                <div className="text-white/50 text-xs">Adapts to network speed</div>
              </div>
              <button
                onClick={() => onAutoToggle()}
                className={`relative w-11 h-6 rounded-full transition-colors cursor-pointer ${autoMode ? 'bg-primary-500' : 'bg-white/20'}`}
                role="switch"
                aria-checked={autoMode}
              >
                <div className={`absolute top-1 w-4 h-4 bg-white rounded-full shadow transition-transform ${autoMode ? 'translate-x-6' : 'translate-x-1'}`} />
              </button>
            </div>

            {/* Quality options */}
            {sortedHeights.map((height) => renderResolutionGroup(height, close))}
          </div>

          {/* Footer - current quality info */}
          {currentQuality && currentBandwidth && (
            <div className="px-3 py-2 border-t border-white/10 bg-white/5">
              <div className="text-white/60 text-xs flex items-center gap-1.5">
                <span className={`w-2 h-2 rounded-full ${autoMode ? 'bg-green-400' : 'bg-primary-400'}`} />
                <span>
                  {autoMode ? 'Auto' : 'Manual'}: {currentQuality}p @ {formatBandwidth(currentBandwidth)}
                </span>
              </div>
            </div>
          )}
        </>
      )}
    </DropdownBase>
  )
}

export default QualitySelector

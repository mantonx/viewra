/**
 * QualitySelector Component
 *
 * Dropdown for selecting video quality with two modes:
 * - Auto: HLS.js ABR handles quality (shows current quality with "Auto" badge)
 * - Manual: User selects specific quality (shows selected quality)
 */

import { useState, useMemo, useEffect } from 'react'
import { cn } from '@/lib/utils'
import { videoOverlay } from '@/styles/semantic'
import { DropdownBase } from '../DropdownBase'
import { SettingsIcon, CheckIcon, ChevronIcon } from '../icons'
import { formatBitrate } from '@/lib/types/streamStats'
import type { QualityOption } from '@/lib/types/video'
import type { QualitySelectorProps } from './QualitySelector.types'

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
    if (currentQuality !== q.height) { return false }
    if (currentBandwidth && q.bandwidth !== currentBandwidth) { return false }
    return true
  }

  // Get the selected option within a resolution group
  const getActiveOptionInGroup = (height: number): QualityOption | null => {
    if (currentQuality !== height) { return null }
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
        className={cn(
          'w-full px-3 py-2.5 flex items-center justify-between cursor-pointer',
          videoOverlay.patterns.listItem,
          isActive && videoOverlay.bg.active,
          isNested && 'pl-6 bg-white/5'
        )}
        role="option"
        aria-selected={isActive}
      >
        <div className="flex items-center gap-2">
          {isNested ? (
            <>
              <span className={cn('text-sm', videoOverlay.text.primary)}>{formatBitrate(quality.bandwidth)}</span>
              {quality.isOriginal && (
                <span className="text-amber-400 text-xs font-medium px-1.5 py-0.5 bg-amber-400/20 rounded">Original</span>
              )}
            </>
          ) : (
            <>
              <span className={cn('text-sm font-medium', videoOverlay.text.primary)}>{quality.height}p</span>
              <span className={cn('text-xs', videoOverlay.text.tertiary)}>{formatBitrate(quality.bandwidth)}</span>
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
          className={cn(
            'w-full px-3 py-2.5 flex items-center justify-between cursor-pointer',
            videoOverlay.patterns.listItem,
            isGroupActive && 'bg-primary-500/10'
          )}
        >
          <div className="flex items-center gap-2">
            <span className={cn('text-sm font-medium', videoOverlay.text.primary)}>{height}p</span>
            {isGroupActive && !isExpanded ? (
              <span className="text-primary-400 text-xs">
                {formatBitrate(activeOption.bandwidth)}
              </span>
            ) : (
              <span className={cn('text-xs', videoOverlay.text.tertiary)}>{options.length} bitrates</span>
            )}
          </div>
          <div className="flex items-center gap-1.5">
            {isGroupActive && <CheckIcon />}
            <ChevronIcon expanded={isExpanded} />
          </div>
        </button>
        {isExpanded && (
          <div className={cn('border-l-2 ml-3', videoOverlay.border.subtle)}>
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
          <div className={cn('px-3 py-2.5 border-b', videoOverlay.border.subtle)}>
            <div className={cn('text-sm font-semibold', videoOverlay.text.primary)}>Video Quality</div>
          </div>

          <div className="max-h-80 overflow-y-auto">
            {/* Auto mode toggle */}
            <div className={cn('px-3 py-3 flex items-center justify-between border-b', videoOverlay.border.subtle)}>
              <div>
                <div className={cn('text-sm font-medium', videoOverlay.text.primary)}>Auto</div>
                <div className={cn('text-xs', videoOverlay.text.tertiary)}>Adapts to network speed</div>
              </div>
              <button
                onClick={() => onAutoToggle()}
                className={cn(
                  'relative w-11 h-6 rounded-full transition-colors cursor-pointer',
                  autoMode ? 'bg-primary-500' : videoOverlay.bg.prominent
                )}
                role="switch"
                aria-checked={autoMode}
              >
                <div className={cn(
                  'absolute top-1 w-4 h-4 bg-white rounded-full shadow transition-transform',
                  autoMode ? 'translate-x-6' : 'translate-x-1'
                )} />
              </button>
            </div>

            {/* Quality options */}
            {sortedHeights.map((height) => renderResolutionGroup(height, close))}
          </div>

          {/* Footer - current quality info */}
          {currentQuality && currentBandwidth && (
            <div className={cn('px-3 py-2 border-t', videoOverlay.border.subtle, videoOverlay.patterns.sectionHeader)}>
              <div className={cn('text-xs flex items-center gap-1.5', videoOverlay.text.secondary)}>
                <span className={cn('w-2 h-2 rounded-full', autoMode ? 'bg-green-400' : 'bg-primary-400')} />
                <span>
                  {autoMode ? 'Auto' : 'Manual'}: {currentQuality}p @ {formatBitrate(currentBandwidth)}
                </span>
              </div>
            </div>
          )}
        </>
      )}
    </DropdownBase>
  )
}

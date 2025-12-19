/**
 * QualitySelector Component
 *
 * Dropdown for selecting video quality in single-quality mode.
 * No ABR - each quality change triggers stream reload from current position.
 */

import { useMemo } from 'react'
import { cn } from '@/lib/utils'
import { videoOverlay } from '@/styles/semantic'
import { DropdownBase } from '../DropdownBase'
import { SettingsIcon, CheckIcon } from '../icons'
import type { QualitySelectorProps } from './QualitySelector.types'

export const QualitySelector = ({
  availableQualities,
  selectedQualityId,
  onQualityChange,
}: QualitySelectorProps) => {
  // Find the currently selected quality
  const selectedQuality = useMemo(
    () => availableQualities.find(q => q.id === selectedQualityId),
    [availableQualities, selectedQualityId]
  )

  // Find the original quality (highest resolution)
  const originalQuality = useMemo(() => {
    if (availableQualities.length === 0) return null
    return availableQualities.reduce((best, q) =>
      q.height > best.height ? q : (q.height === best.height && q.bandwidth > best.bandwidth ? q : best)
    )
  }, [availableQualities])

  const isSelectedOriginal = selectedQuality && originalQuality && selectedQuality.id === originalQuality.id

  // Get display text for the button
  const getQualityDisplayText = () => {
    if (!selectedQuality) {
      return 'Quality'
    }
    if (isSelectedOriginal) {
      return `${selectedQuality.height}p ✦`
    }
    return `${selectedQuality.height}p`
  }

  // Sort qualities by height (highest first), then by bandwidth (highest first)
  const sortedQualities = useMemo(() => {
    return [...availableQualities].sort((a, b) => {
      if (a.height !== b.height) return b.height - a.height
      return b.bandwidth - a.bandwidth
    })
  }, [availableQualities])

  return (
    <DropdownBase
      buttonContent={getQualityDisplayText()}
      icon={<SettingsIcon />}
      minButtonWidth="80px"
      panelWidth="w-64"
      ariaLabel="Video quality"
    >
      {({ close }) => (
        <>
          {/* Header */}
          <div className={cn('px-3 py-2.5 border-b', videoOverlay.border.subtle)}>
            <div className={cn('text-sm font-semibold', videoOverlay.text.primary)}>Video Quality</div>
            <div className={cn('text-xs mt-0.5', videoOverlay.text.tertiary)}>
              Quality change restarts from current position
            </div>
          </div>

          <div className="max-h-80 overflow-y-auto">
            {sortedQualities.map((quality) => {
              const isSelected = quality.id === selectedQualityId
              const isOriginal = quality.id === originalQuality?.id

              return (
                <button
                  key={quality.id}
                  onClick={() => {
                    if (!isSelected) {
                      onQualityChange(quality.id)
                    }
                    close()
                  }}
                  className={cn(
                    'w-full px-3 py-2.5 flex items-center justify-between cursor-pointer',
                    videoOverlay.patterns.listItem,
                    isSelected && videoOverlay.bg.active
                  )}
                  role="option"
                  aria-selected={isSelected}
                >
                  <div className="flex items-center gap-2">
                    <span className={cn('text-sm font-medium', videoOverlay.text.primary)}>
                      {quality.displayName}
                    </span>
                    {isOriginal && (
                      <span className="text-amber-400 text-xs font-medium px-1.5 py-0.5 bg-amber-400/20 rounded">
                        Original
                      </span>
                    )}
                  </div>
                  {isSelected && <CheckIcon />}
                </button>
              )
            })}
          </div>

          {/* Footer - current quality info */}
          {selectedQuality && (
            <div className={cn('px-3 py-2 border-t', videoOverlay.border.subtle, videoOverlay.patterns.sectionHeader)}>
              <div className={cn('text-xs flex items-center gap-1.5', videoOverlay.text.secondary)}>
                <span className={cn('w-2 h-2 rounded-full', isSelectedOriginal ? 'bg-amber-400' : 'bg-primary-400')} />
                <span>
                  {selectedQuality.height}p @ {Math.round(selectedQuality.bandwidth / 1_000_000)} Mbps
                  {isSelectedOriginal && ' · Original'}
                </span>
              </div>
            </div>
          )}
        </>
      )}
    </DropdownBase>
  )
}

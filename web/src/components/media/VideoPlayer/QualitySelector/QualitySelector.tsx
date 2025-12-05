import { useState, useMemo } from 'react'
import { DropdownBase } from '../DropdownBase'
import type { QualityOption } from '@/lib/types/video'
import type { QualitySelectorProps } from './QualitySelector.types'

const formatBandwidth = (bps: number): string => {
  if (bps >= 1_000_000) {
    return `${(bps / 1_000_000).toFixed(1)} Mbps`
  }
  return `${(bps / 1_000).toFixed(0)} kbps`
}

const estimateDataUsage = (bps: number): string => {
  const mbPerMin = (bps / 8) * 60 / 1_000_000
  if (mbPerMin >= 100) {
    return `${mbPerMin.toFixed(0)} MB/min`
  }
  return `${mbPerMin.toFixed(1)} MB/min`
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

const StarIcon = () => (
  <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
    <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
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
  recommendedQuality,
  autoMode,
  onQualityChange,
  onAutoToggle,
  showBitrateVariants = true,
}: QualitySelectorProps) => {
  const [hoveredQuality, setHoveredQuality] = useState<string | null>(null)
  const [expandedResolutions, setExpandedResolutions] = useState<Set<number>>(new Set())

  const qualitiesByResolution = useMemo(() => {
    const grouped = new Map<number, QualityOption[]>()
    for (const q of availableQualities) {
      const existing = grouped.get(q.height) || []
      existing.push(q)
      grouped.set(q.height, existing)
    }
    for (const [height, options] of grouped) {
      grouped.set(height, options.sort((a, b) => b.bandwidth - a.bandwidth))
    }
    return grouped
  }, [availableQualities])

  const hasMultipleBitrates = useMemo(() => {
    for (const options of qualitiesByResolution.values()) {
      if (options.length > 1) {return true}
    }
    return false
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

  const getQualityKey = (q: QualityOption) => `${q.height}-${q.bandwidth}`

  // Check if the current quality is the original
  const isCurrentOriginal = useMemo(() => {
    if (!currentQuality || !currentBandwidth) return false
    const currentOption = availableQualities.find(
      q => q.height === currentQuality && q.bandwidth === currentBandwidth
    )
    return currentOption?.isOriginal ?? false
  }, [currentQuality, currentBandwidth, availableQualities])

  const getQualityDisplayText = () => {
    if (autoMode) {
      return currentQuality ? `Auto (${currentQuality}p)` : 'Auto'
    }
    if (currentQuality && currentBandwidth) {
      const mbps = currentBandwidth / 1_000_000
      if (isCurrentOriginal) {
        return `${currentQuality}p Original`
      }
      if (mbps >= 1) {
        return `${currentQuality}p ${mbps.toFixed(0)}M`
      }
      return `${currentQuality}p`
    }
    return currentQuality ? `${currentQuality}p` : 'Auto'
  }

  const isRecommended = (height: number) => recommendedQuality?.height === height

  const sortedHeights = useMemo(() => {
    return Array.from(qualitiesByResolution.keys()).sort((a, b) => b - a)
  }, [qualitiesByResolution])

  const isQualitySelected = (q: QualityOption) => {
    if (autoMode) {return false}
    if (currentQuality !== q.height) {return false}
    if (currentBandwidth && q.bandwidth !== currentBandwidth) {return false}
    return true
  }

  const renderQualityOption = (quality: QualityOption, close: () => void, isNested = false) => {
    const isSelected = isQualitySelected(quality)
    const qualityKey = getQualityKey(quality)
    const isHovered = hoveredQuality === qualityKey
    const isRec = isRecommended(quality.height)

    return (
      <button
        key={qualityKey}
        onClick={(e) => { e.stopPropagation(); onQualityChange(quality.height, quality.bandwidth); close() }}
        onMouseEnter={() => setHoveredQuality(qualityKey)}
        onMouseLeave={() => setHoveredQuality(null)}
        className={`w-full px-3 py-2 flex items-start justify-between hover:bg-white/10 transition-colors cursor-pointer ${isSelected ? 'bg-primary-500/20' : ''} ${isNested ? 'pl-6 bg-white/5' : ''}`}
        role="option"
        aria-selected={isSelected}
      >
        <div className="flex-1">
          <div className="flex items-center gap-2">
            {isNested ? (
              <>
                <span className="text-white text-sm">{formatBandwidth(quality.bandwidth)}</span>
                {quality.isOriginal && (
                  <span className="text-amber-400 text-xs font-medium px-1.5 py-0.5 bg-amber-400/20 rounded">Original</span>
                )}
              </>
            ) : (
              <span className="text-white text-sm font-medium">{quality.height}p</span>
            )}
            {isRec && !isNested && (
              <span className="text-primary-400 text-xs flex items-center gap-0.5">
                <StarIcon />
                Recommended
              </span>
            )}
            {!isNested && (
              <span className="text-white/50 text-xs">{formatBandwidth(quality.bandwidth)}</span>
            )}
          </div>
          <div className={`overflow-hidden transition-all duration-200 ${isHovered || isSelected ? 'max-h-20 opacity-100 mt-1' : 'max-h-0 opacity-0'}`}>
            <div className="text-white/60 text-xs space-y-0.5">
              <div className="flex items-center gap-3">
                <span>{estimateDataUsage(quality.bandwidth)}</span>
                {quality.displayName && <span>{quality.displayName}</span>}
              </div>
              {quality.canDirectPlay !== undefined && (
                <div className="flex items-center gap-1">
                  {quality.canDirectPlay ? (
                    <>
                      <svg className="w-3 h-3 text-yellow-400" fill="currentColor" viewBox="0 0 20 20">
                        <path fillRule="evenodd" d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z" clipRule="evenodd" />
                      </svg>
                      <span className="text-yellow-400">Direct play</span>
                    </>
                  ) : (
                    <span className="text-white/40">Transcoding</span>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
        {isSelected && <div className="shrink-0 mt-0.5"><CheckIcon /></div>}
      </button>
    )
  }

  const renderResolutionGroup = (height: number, close: () => void) => {
    const options = qualitiesByResolution.get(height) || []
    const hasBitrateVariants = options.length > 1 && showBitrateVariants
    const isExpanded = expandedResolutions.has(height)
    const isRec = isRecommended(height)
    const anySelected = !autoMode && currentQuality === height

    if (!hasBitrateVariants) {
      return options[0] ? renderQualityOption(options[0], close) : null
    }

    return (
      <div key={height}>
        <button
          onClick={() => toggleExpanded(height)}
          className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${anySelected ? 'bg-primary-500/10' : ''}`}
        >
          <div className="flex items-center gap-2">
            <span className="text-white text-sm font-medium">{height}p</span>
            {isRec && <span className="text-primary-400 text-xs flex items-center gap-0.5"><StarIcon /></span>}
            <span className="text-white/50 text-xs">{options.length} bitrates</span>
          </div>
          <div className="flex items-center gap-1.5">
            {anySelected && <CheckIcon />}
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
      panelWidth="w-80"
      ariaLabel="Video quality"
    >
      {({ close }) => (
        <>
          <div className="px-3 py-2 border-b border-white/10">
            <div className="text-white text-sm font-semibold">Video Quality</div>
            {recommendedQuality && <div className="text-white/60 text-xs mt-0.5">{recommendedQuality.reason}</div>}
            {hasMultipleBitrates && showBitrateVariants && <div className="text-white/40 text-xs mt-1">Tap resolution to see bitrate options</div>}
          </div>

          <div className="max-h-80 overflow-y-auto">
            <button
              onClick={() => { onAutoToggle(); close() }}
              onMouseEnter={() => setHoveredQuality('auto')}
              onMouseLeave={() => setHoveredQuality(null)}
              className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${autoMode ? 'bg-primary-500/20' : ''}`}
              role="option"
              aria-selected={autoMode}
            >
              <div className="flex items-center gap-2">
                <span className="text-white text-sm font-medium">Auto</span>
                <span className="text-white/50 text-xs">{currentQuality ? `Currently ${currentQuality}p` : 'Adaptive'}</span>
              </div>
              {autoMode && <CheckIcon />}
            </button>
            <div className="border-t border-white/10" />
            {sortedHeights.map((height) => renderResolutionGroup(height, close))}
          </div>

          {(recommendedQuality?.dataUsageMBPerHour || currentBandwidth) && (
            <div className="px-3 py-2 border-t border-white/10 bg-white/5">
              <div className="text-white/50 text-xs flex items-center gap-1">
                <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clipRule="evenodd" />
                </svg>
                <span>
                  {currentBandwidth
                    ? `Current: ${formatBandwidth(currentBandwidth)} (~${estimateDataUsage(currentBandwidth)})`
                    : recommendedQuality?.dataUsageMBPerHour
                    ? `Current: ~${recommendedQuality.dataUsageMBPerHour.toFixed(0)} MB/hr`
                    : null}
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

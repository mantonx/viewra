import { useMemo } from 'react'
import { cn } from '@/lib/utils'
import { videoOverlay } from '@/styles/semantic'
import { DropdownBase } from '../DropdownBase'
import { SubtitleIcon, CheckIcon } from '../icons'
import { getLanguageName } from '@/lib/utils/language'
import type { SubtitleTrack } from '@/lib/types/subtitles'
import type { SubtitleSelectorProps } from './SubtitleSelector.types'

export const SubtitleSelector = ({
  availableSubtitles,
  currentSubtitle,
  onSubtitleChange,
}: SubtitleSelectorProps) => {
  // Separate text and bitmap subtitles
  const textSubtitles = useMemo(() => {
    return availableSubtitles.filter((sub) => !sub.isBitmap)
  }, [availableSubtitles])

  const bitmapSubtitles = useMemo(() => {
    return availableSubtitles.filter((sub) => sub.isBitmap)
  }, [availableSubtitles])

  // Build display name for a subtitle track
  const getTrackDisplayName = (track: SubtitleTrack): string => {
    if (track.title) {
      return track.title
    }

    let name = getLanguageName(track.language)

    // Add special indicators
    const indicators: string[] = []
    if (track.isForced) { indicators.push('Forced') }
    if (track.isSDH) { indicators.push('SDH') }
    if (track.isCommentary) { indicators.push('Commentary') }
    if (track.sourceType === 'external') { indicators.push('External') }

    if (indicators.length > 0) {
      name += ` (${indicators.join(', ')})`
    }

    return name
  }

  // Get current display text for button
  const getCurrentDisplayText = (): string => {
    if (currentSubtitle === null) {
      return 'Off'
    }
    // Check text subtitles first, then bitmap
    let track = textSubtitles.find((t) => t.id === currentSubtitle)
    let isBitmap = false
    if (!track) {
      track = bitmapSubtitles.find((t) => t.id === currentSubtitle)
      isBitmap = true
    }
    if (!track) {
      return 'Off'
    }
    // Short form for button
    const lang = getLanguageName(track.language)
    if (isBitmap) { return `${lang} (B)` }
    if (track.isForced) { return `${lang} (F)` }
    if (track.isSDH) { return `${lang} (SDH)` }
    return lang
  }

  // Group subtitles by language for better organization
  const subtitlesByLanguage = useMemo(() => {
    const grouped = new Map<string, SubtitleTrack[]>()
    for (const sub of textSubtitles) {
      const lang = sub.language || 'und'
      const existing = grouped.get(lang) || []
      existing.push(sub)
      grouped.set(lang, existing)
    }
    return grouped
  }, [textSubtitles])

  // Sort languages: prioritize English, then alphabetically
  const sortedLanguages = useMemo(() => {
    const langs = Array.from(subtitlesByLanguage.keys())
    return langs.sort((a, b) => {
      if (a === 'eng') { return -1 }
      if (b === 'eng') { return 1 }
      return getLanguageName(a).localeCompare(getLanguageName(b))
    })
  }, [subtitlesByLanguage])

  const renderSubtitleOption = (
    track: SubtitleTrack,
    close: () => void,
    showLanguage: boolean
  ) => {
    const isSelected = currentSubtitle === track.id
    const displayName = showLanguage
      ? getTrackDisplayName(track)
      : track.title || buildTrackDescription(track)

    const handleClick = () => {
      onSubtitleChange(track.id)
      close()
    }

    return (
      <button
        key={track.id}
        onClick={handleClick}
        className={cn(
          'w-full px-3 py-2 flex items-center justify-between cursor-pointer',
          videoOverlay.patterns.listItem,
          isSelected && videoOverlay.bg.active
        )}
        role="option"
        aria-selected={isSelected}
      >
        <div className="flex items-center gap-2">
          <span className={cn('text-sm', videoOverlay.text.primary)}>{displayName}</span>
          {track.isDefault && !isSelected && (
            <span className={cn('text-xs', videoOverlay.text.disabled)}>(Default)</span>
          )}
        </div>
        {isSelected && <CheckIcon />}
      </button>
    )
  }

  // Build description without language
  const buildTrackDescription = (track: SubtitleTrack): string => {
    const indicators: string[] = []
    if (track.isForced) { indicators.push('Forced') }
    if (track.isSDH) { indicators.push('SDH') }
    if (track.isCommentary) { indicators.push('Commentary') }
    if (track.sourceType === 'external') { indicators.push('External') }

    if (indicators.length > 0) {
      return indicators.join(', ')
    }
    if (track.isDefault) {
      return 'Default'
    }
    return 'Track'
  }

  // Don't render if no subtitles available at all
  if (availableSubtitles.length === 0) {
    return null
  }

  return (
    <DropdownBase
      buttonContent={getCurrentDisplayText()}
      icon={<SubtitleIcon />}
      minButtonWidth="70px"
      panelWidth="w-64"
      ariaLabel="Subtitle track"
    >
      {({ close }) => (
        <>
          <div className={cn('px-3 py-2 border-b', videoOverlay.border.subtle)}>
            <div className={cn('text-sm font-semibold', videoOverlay.text.primary)}>Subtitles</div>
            <div className={cn('text-xs mt-0.5', videoOverlay.text.tertiary)}>
              {availableSubtitles.length} track{availableSubtitles.length !== 1 ? 's' : ''}{' '}
              available
            </div>
          </div>

          <div className="max-h-80 overflow-y-auto">
            {/* Off option */}
            <button
              onClick={() => {
                onSubtitleChange(null)
                close()
              }}
              className={cn(
                'w-full px-3 py-2.5 flex items-center justify-between cursor-pointer',
                videoOverlay.patterns.listItem,
                currentSubtitle === null && videoOverlay.bg.active
              )}
              role="option"
              aria-selected={currentSubtitle === null}
            >
              <span className={cn('text-sm font-medium', videoOverlay.text.primary)}>Off</span>
              {currentSubtitle === null && <CheckIcon />}
            </button>

            <div className={cn('border-t', videoOverlay.border.subtle)} />

            {/* Text subtitles section */}
            {textSubtitles.length > 0 && (
              <>
                {/* If only one language, show flat list */}
                {sortedLanguages.length === 1
                  ? textSubtitles.map((track) => renderSubtitleOption(track, close, true))
                  : /* Multiple languages - group by language */
                    sortedLanguages.map((lang) => {
                      const tracks = subtitlesByLanguage.get(lang) || []
                      const hasMultipleTracks = tracks.length > 1

                      return (
                        <div key={lang}>
                          {/* Language header */}
                          <div className={cn(
                            'px-3 py-1.5 text-xs font-medium uppercase tracking-wider',
                            videoOverlay.patterns.sectionHeader
                          )}>
                            {getLanguageName(lang)}
                          </div>
                          {/* Tracks for this language */}
                          {tracks.map((track) =>
                            renderSubtitleOption(track, close, !hasMultipleTracks)
                          )}
                        </div>
                      )
                    })}
              </>
            )}

            {/* Bitmap subtitles section (PGS/VobSub - rendered as image overlay) */}
            {bitmapSubtitles.length > 0 && (
              <>
                <div className={cn(
                  'px-3 py-1.5 text-xs font-medium uppercase tracking-wider border-t flex items-center gap-1.5',
                  videoOverlay.border.subtle,
                  videoOverlay.patterns.sectionHeader
                )}>
                  {/* Image icon for bitmap */}
                  <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M4 3a2 2 0 00-2 2v10a2 2 0 002 2h12a2 2 0 002-2V5a2 2 0 00-2-2H4zm12 12H4l4-8 3 6 2-4 3 6z" clipRule="evenodd" />
                  </svg>
                  Image Subtitles (PGS)
                </div>
                {bitmapSubtitles.map((track) => renderSubtitleOption(track, close, true))}
              </>
            )}
          </div>
        </>
      )}
    </DropdownBase>
  )
}

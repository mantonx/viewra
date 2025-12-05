import { useMemo } from 'react'
import { DropdownBase } from '../DropdownBase'
import type { SubtitleTrack } from '@/lib/types/subtitles'
import type { SubtitleSelectorProps } from './SubtitleSelector.types'

// Language code to display name mapping
const languageNames: Record<string, string> = {
  eng: 'English',
  spa: 'Spanish',
  fra: 'French',
  deu: 'German',
  ita: 'Italian',
  por: 'Portuguese',
  jpn: 'Japanese',
  kor: 'Korean',
  zho: 'Chinese',
  rus: 'Russian',
  ara: 'Arabic',
  hin: 'Hindi',
  tha: 'Thai',
  vie: 'Vietnamese',
  nld: 'Dutch',
  pol: 'Polish',
  swe: 'Swedish',
  nor: 'Norwegian',
  dan: 'Danish',
  fin: 'Finnish',
  tur: 'Turkish',
  ell: 'Greek',
  heb: 'Hebrew',
  ind: 'Indonesian',
  ces: 'Czech',
  hun: 'Hungarian',
  ron: 'Romanian',
  ukr: 'Ukrainian',
  und: 'Unknown',
}

const getLanguageName = (code: string): string => {
  return languageNames[code.toLowerCase()] || code.toUpperCase()
}

const SubtitleIcon = () => (
  <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
    <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 14H4V6h16v12zM6 10h2v2H6v-2zm0 4h8v2H6v-2zm10 0h2v2h-2v-2zm-6-4h8v2h-8v-2z" />
  </svg>
)

const CheckIcon = () => (
  <svg className="w-4 h-4 text-primary-400" fill="currentColor" viewBox="0 0 20 20">
    <path
      fillRule="evenodd"
      d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
      clipRule="evenodd"
    />
  </svg>
)

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
    if (track.isForced) indicators.push('Forced')
    if (track.isSDH) indicators.push('SDH')
    if (track.isCommentary) indicators.push('Commentary')
    if (track.sourceType === 'external') indicators.push('External')

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
    if (isBitmap) return `${lang} (B)`
    if (track.isForced) return `${lang} (F)`
    if (track.isSDH) return `${lang} (SDH)`
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
      if (a === 'eng') return -1
      if (b === 'eng') return 1
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
        className={`w-full px-3 py-2 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${
          isSelected ? 'bg-primary-500/20' : ''
        }`}
        role="option"
        aria-selected={isSelected}
      >
        <div className="flex items-center gap-2">
          <span className="text-white text-sm">{displayName}</span>
          {track.isDefault && !isSelected && (
            <span className="text-white/40 text-xs">(Default)</span>
          )}
        </div>
        {isSelected && <CheckIcon />}
      </button>
    )
  }

  // Build description without language
  const buildTrackDescription = (track: SubtitleTrack): string => {
    const indicators: string[] = []
    if (track.isForced) indicators.push('Forced')
    if (track.isSDH) indicators.push('SDH')
    if (track.isCommentary) indicators.push('Commentary')
    if (track.sourceType === 'external') indicators.push('External')

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
          <div className="px-3 py-2 border-b border-white/10">
            <div className="text-white text-sm font-semibold">Subtitles</div>
            <div className="text-white/50 text-xs mt-0.5">
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
              className={`w-full px-3 py-2.5 flex items-center justify-between hover:bg-white/10 transition-colors cursor-pointer ${
                currentSubtitle === null ? 'bg-primary-500/20' : ''
              }`}
              role="option"
              aria-selected={currentSubtitle === null}
            >
              <span className="text-white text-sm font-medium">Off</span>
              {currentSubtitle === null && <CheckIcon />}
            </button>

            <div className="border-t border-white/10" />

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
                          <div className="px-3 py-1.5 text-white/60 text-xs font-medium uppercase tracking-wider bg-white/5">
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
                <div className="px-3 py-1.5 text-xs font-medium uppercase tracking-wider border-t border-white/10 flex items-center gap-1.5 text-white/60 bg-white/5">
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

export default SubtitleSelector

import { useMemo } from 'react'
import { DropdownBase } from './DropdownBase'
import type { SubtitleSelection } from '@/lib/hooks/useSubtitles'

export type { SubtitleSelection }

export interface SubtitleTrack {
  id: number
  language: string
  title?: string
  isDefault?: boolean
  isForced?: boolean
  isSDH?: boolean
  isCommentary?: boolean
  isBitmap?: boolean
  sourceType?: 'embedded' | 'external'
  streamIndex?: number // Absolute stream index in the file (for FFmpeg)
}

interface SubtitleSelectorProps {
  availableSubtitles: SubtitleTrack[]
  currentSubtitle: number | null // null means "Off"
  onSubtitleChange: (trackId: number | null, selection?: SubtitleSelection) => void
}

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
    <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
  </svg>
)

export const SubtitleSelector = ({
  availableSubtitles,
  currentSubtitle,
  onSubtitleChange,
}: SubtitleSelectorProps) => {
  // Separate text and bitmap subtitles
  const textSubtitles = useMemo(() => {
    return availableSubtitles.filter(sub => !sub.isBitmap)
  }, [availableSubtitles])

  const bitmapSubtitles = useMemo(() => {
    return availableSubtitles.filter(sub => sub.isBitmap)
  }, [availableSubtitles])

  // Calculate relative stream index for a subtitle (0-based among subtitle streams only)
  // FFmpeg's 0:s:N expects the relative index among subtitle streams, not absolute stream index
  const getSubtitleStreamIndex = (track: SubtitleTrack): number => {
    // Sort by absolute stream index to get the correct order
    const sortedTracks = [...availableSubtitles]
      .filter(t => t.streamIndex !== undefined)
      .sort((a, b) => (a.streamIndex ?? 0) - (b.streamIndex ?? 0))

    // Find the position of this track (0-based relative index among subtitle streams)
    return sortedTracks.findIndex(t => t.id === track.id)
  }

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
    let track = textSubtitles.find(t => t.id === currentSubtitle)
    let isBitmap = false
    if (!track) {
      track = bitmapSubtitles.find(t => t.id === currentSubtitle)
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
    showLanguage: boolean,
    isBurnIn: boolean = false
  ) => {
    const isSelected = currentSubtitle === track.id
    const displayName = showLanguage
      ? getTrackDisplayName(track)
      : track.title || buildTrackDescription(track)

    const handleClick = () => {
      if (isBurnIn) {
        // For bitmap subtitles, pass the full selection with burn-in info
        const selection: SubtitleSelection = {
          trackId: track.id,
          requiresBurnIn: true,
          streamIndex: getSubtitleStreamIndex(track),
        }
        onSubtitleChange(track.id, selection)
      } else {
        // For text subtitles, simple selection
        onSubtitleChange(track.id, {
          trackId: track.id,
          requiresBurnIn: false,
        })
      }
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
              {availableSubtitles.length} track{availableSubtitles.length !== 1 ? 's' : ''} available
            </div>
          </div>

          <div className="max-h-80 overflow-y-auto">
            {/* Off option */}
            <button
              onClick={() => {
                onSubtitleChange(null, { trackId: null, requiresBurnIn: false })
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
                {sortedLanguages.length === 1 ? (
                  textSubtitles.map(track => renderSubtitleOption(track, close, true, false))
                ) : (
                  /* Multiple languages - group by language */
                  sortedLanguages.map(lang => {
                    const tracks = subtitlesByLanguage.get(lang) || []
                    const hasMultipleTracks = tracks.length > 1

                    return (
                      <div key={lang}>
                        {/* Language header */}
                        <div className="px-3 py-1.5 text-white/60 text-xs font-medium uppercase tracking-wider bg-white/5">
                          {getLanguageName(lang)}
                        </div>
                        {/* Tracks for this language */}
                        {tracks.map(track =>
                          renderSubtitleOption(track, close, !hasMultipleTracks, false)
                        )}
                      </div>
                    )
                  })
                )}
              </>
            )}

            {/* Bitmap subtitles section (burn-in) */}
            {bitmapSubtitles.length > 0 && (
              <>
                <div className="px-3 py-1.5 text-amber-400/80 text-xs font-medium uppercase tracking-wider bg-amber-500/10 border-t border-white/10 flex items-center gap-1.5">
                  <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                    <path
                      fillRule="evenodd"
                      d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                      clipRule="evenodd"
                    />
                  </svg>
                  Burn-in (PGS/DVD)
                </div>
                {bitmapSubtitles.map(track =>
                  renderSubtitleOption(track, close, true, true)
                )}
              </>
            )}
          </div>
        </>
      )}
    </DropdownBase>
  )
}

export default SubtitleSelector

import { useMemo } from 'react'
import { DropdownBase } from '../DropdownBase'
import type { AudioTrack } from '@/lib/types/video'
import type { AudioSelectorProps } from './AudioSelector.types'

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

// Format channel count to human-readable string
const formatChannels = (channels?: number): string | null => {
  if (!channels) return null
  switch (channels) {
    case 1:
      return 'Mono'
    case 2:
      return 'Stereo'
    case 6:
      return '5.1'
    case 8:
      return '7.1'
    default:
      return `${channels}ch`
  }
}

const AudioIcon = () => (
  <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
    <path d="M12 3v10.55c-.59-.34-1.27-.55-2-.55-2.21 0-4 1.79-4 4s1.79 4 4 4 4-1.79 4-4V7h4V3h-6zm-2 16c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2z" />
  </svg>
)

const CheckIcon = () => (
  <svg className="w-4 h-4 text-primary-400" fill="currentColor" viewBox="0 0 20 20">
    <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
  </svg>
)

export const AudioSelector = ({
  availableAudioTracks,
  currentAudioTrack,
  onAudioTrackChange,
}: AudioSelectorProps) => {
  // Get current track info for button display
  const currentTrack = useMemo(() => {
    return availableAudioTracks.find(t => t.id === currentAudioTrack)
  }, [availableAudioTracks, currentAudioTrack])

  // Get short display text for button
  const getCurrentDisplayText = (): string => {
    if (!currentTrack) {
      return 'Audio'
    }
    const lang = getLanguageName(currentTrack.language)
    const channels = formatChannels(currentTrack.channels)
    if (channels) {
      return `${lang} ${channels}`
    }
    return lang
  }

  // Group audio tracks by language for better organization
  const tracksByLanguage = useMemo(() => {
    const grouped = new Map<string, AudioTrack[]>()
    for (const track of availableAudioTracks) {
      const lang = track.language || 'und'
      const existing = grouped.get(lang) || []
      existing.push(track)
      grouped.set(lang, existing)
    }
    return grouped
  }, [availableAudioTracks])

  // Sort languages: prioritize current track's language, then English, then alphabetically
  const sortedLanguages = useMemo(() => {
    const langs = Array.from(tracksByLanguage.keys())
    const currentLang = currentTrack?.language
    return langs.sort((a, b) => {
      if (a === currentLang) return -1
      if (b === currentLang) return 1
      if (a === 'eng') return -1
      if (b === 'eng') return 1
      return getLanguageName(a).localeCompare(getLanguageName(b))
    })
  }, [tracksByLanguage, currentTrack])

  // Build display name for an audio track
  const getTrackDisplayName = (track: AudioTrack): string => {
    const parts: string[] = []

    // Add language name
    parts.push(getLanguageName(track.language))

    // Add track name if it provides additional info
    if (track.name && !track.name.toLowerCase().includes(track.language.toLowerCase())) {
      parts.push(`(${track.name})`)
    }

    // Add channel info
    const channels = formatChannels(track.channels)
    if (channels) {
      parts.push(`- ${channels}`)
    }

    // Add codec if available
    if (track.codec) {
      parts.push(`[${track.codec.toUpperCase()}]`)
    }

    return parts.join(' ')
  }

  // Build short description for tracks when grouped by language
  const getTrackDescription = (track: AudioTrack): string => {
    const parts: string[] = []

    // Add track name if it provides useful info
    if (track.name && !track.name.toLowerCase().includes('default')) {
      parts.push(track.name)
    }

    // Add channel info
    const channels = formatChannels(track.channels)
    if (channels) {
      parts.push(channels)
    }

    // Add codec if available
    if (track.codec) {
      parts.push(track.codec.toUpperCase())
    }

    if (parts.length === 0) {
      return track.isDefault ? 'Default' : 'Track'
    }

    return parts.join(' - ')
  }

  const renderAudioOption = (
    track: AudioTrack,
    close: () => void,
    showLanguage: boolean
  ) => {
    const isSelected = currentAudioTrack === track.id
    const displayName = showLanguage
      ? getTrackDisplayName(track)
      : getTrackDescription(track)

    return (
      <button
        key={track.id}
        onClick={() => {
          onAudioTrackChange(track.id)
          close()
        }}
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

  // Don't render if only one audio track
  if (availableAudioTracks.length <= 1) {
    return null
  }

  return (
    <DropdownBase
      buttonContent={getCurrentDisplayText()}
      icon={<AudioIcon />}
      minButtonWidth="90px"
      panelWidth="w-72"
      ariaLabel="Audio track"
    >
      {({ close }) => (
        <>
          <div className="px-3 py-2 border-b border-white/10">
            <div className="text-white text-sm font-semibold">Audio</div>
            <div className="text-white/50 text-xs mt-0.5">
              {availableAudioTracks.length} track{availableAudioTracks.length !== 1 ? 's' : ''} available
            </div>
          </div>

          <div className="max-h-80 overflow-y-auto">
            {/* If only one language, show flat list */}
            {sortedLanguages.length === 1 ? (
              availableAudioTracks.map(track => renderAudioOption(track, close, true))
            ) : (
              /* Multiple languages - group by language */
              sortedLanguages.map(lang => {
                const tracks = tracksByLanguage.get(lang) || []
                const hasMultipleTracks = tracks.length > 1

                return (
                  <div key={lang}>
                    {/* Language header */}
                    <div className="px-3 py-1.5 text-white/60 text-xs font-medium uppercase tracking-wider bg-white/5">
                      {getLanguageName(lang)}
                    </div>
                    {/* Tracks for this language */}
                    {tracks.map(track =>
                      renderAudioOption(track, close, !hasMultipleTracks)
                    )}
                  </div>
                )
              })
            )}
          </div>
        </>
      )}
    </DropdownBase>
  )
}

export default AudioSelector

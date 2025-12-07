import { useMemo } from 'react'
import { cn } from '@/lib/utils'
import { videoOverlay } from '@/styles/semantic'
import { DropdownBase } from '../DropdownBase'
import { AudioIcon, CheckIcon } from '../icons'
import { getLanguageName, formatChannels } from '@/lib/utils/language'
import type { AudioTrack } from '@/lib/types/video'
import type { AudioSelectorProps } from './AudioSelector.types'

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
      if (a === currentLang) { return -1 }
      if (b === currentLang) { return 1 }
      if (a === 'eng') { return -1 }
      if (b === 'eng') { return 1 }
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
          <div className={cn('px-3 py-2 border-b', videoOverlay.border.subtle)}>
            <div className={cn('text-sm font-semibold', videoOverlay.text.primary)}>Audio</div>
            <div className={cn('text-xs mt-0.5', videoOverlay.text.tertiary)}>
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
                    <div className={cn(
                      'px-3 py-1.5 text-xs font-medium uppercase tracking-wider',
                      videoOverlay.patterns.sectionHeader
                    )}>
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

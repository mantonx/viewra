/**
 * Language utilities for ISO 639-2/B code handling.
 * Used for audio track and subtitle language display.
 */

/**
 * Maps ISO 639-2/B language codes to human-readable names.
 * This is a curated list of the most common languages in media files.
 */
export const languageNames: Record<string, string> = {
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

/**
 * Gets the display name for a language code.
 * Falls back to uppercase code if not found in the mapping.
 */
export const getLanguageName = (code: string): string => {
  return languageNames[code.toLowerCase()] || code.toUpperCase()
}

/**
 * Formats audio channel count to human-readable string.
 */
export const formatChannels = (channels?: number): string | null => {
  if (!channels) {
    return null
  }
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

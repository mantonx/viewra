/**
 * Format utilities for display values
 */

/**
 * Format bytes to human-readable file size
 * @param bytes - Number of bytes
 * @returns Formatted string like "1.23 MB"
 * @example
 * formatFileSize(1234567) // "1.18 MB"
 */
const formatFileSize = (bytes: number): string => {
  if (bytes === 0) {
    return '0 Bytes'
  }
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${Math.round((bytes / Math.pow(k, i)) * 100) / 100} ${sizes[i]}`
}

/**
 * Format seconds to human-readable duration
 * @param seconds - Number of seconds
 * @returns Formatted string like "1h 23m" or "45m"
 * @example
 * formatDuration(3661) // "1h 1m"
 * formatDuration(120) // "2m"
 */
const formatDuration = (seconds: number): string => {
  if (seconds === 0) {
    return '0m'
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  if (hours > 0) {
    return `${hours}h ${minutes}m`
  }

  return `${minutes}m`
}

/**
 * Format date to human-readable format
 * @param date - Date string or Date object
 * @returns Formatted string like "Jan 12, 2024, 2:30 PM"
 * @example
 * formatDate("2024-01-12T14:30:00Z") // "Jan 12, 2024, 2:30 PM"
 */
const formatDate = (date: string | Date): string => {
  const dateObj = typeof date === 'string' ? new Date(date) : date
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(dateObj)
}

/**
 * Format seconds to MM:SS or HH:MM:SS format for video player
 * @param seconds - Number of seconds
 * @returns Formatted string like "1:23:45" or "23:45"
 * @example
 * formatTime(3661) // "1:01:01"
 * formatTime(125) // "2:05"
 */
const formatTime = (seconds: number): string => {
  if (isNaN(seconds) || seconds < 0) {
    return '0:00'
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)

  const pad = (num: number) => num.toString().padStart(2, '0')

  if (hours > 0) {
    return `${hours}:${pad(minutes)}:${pad(secs)}`
  }

  return `${minutes}:${pad(secs)}`
}

/**
 * Pluralize a word based on count with optional custom plural form
 * @param count - Number to determine singular/plural
 * @param singular - Singular form of the word
 * @param plural - Optional custom plural form (defaults to singular + 's')
 * @returns Formatted string with count and properly pluralized word
 * @example
 * pluralize(1, 'error') // "1 error"
 * pluralize(5, 'error') // "5 errors"
 * pluralize(0, 'file') // "0 files"
 */
const pluralize = (count: number | undefined, singular: string, plural?: string): string => {
  const n = count ?? 0
  const pluralForm = plural ?? `${singular}s`
  return `${n} ${n !== 1 ? pluralForm : singular}`
}

/**
 * Format ETA (estimated time remaining) in a clean, human-readable way
 * @param seconds - Number of seconds remaining
 * @returns Formatted string like "~5m", "~1h 23m", or "~2h"
 * @example
 * formatETA(300) // "~5m"
 * formatETA(5000) // "~1h 23m"
 * formatETA(120) // "~2m"
 * formatETA(45) // "<1m"
 */
const formatETA = (seconds: number | undefined | null): string | null => {
  if (seconds === null || seconds === undefined || seconds <= 0) {
    return null
  }

  // Less than a minute
  if (seconds < 60) {
    return '<1m'
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.round((seconds % 3600) / 60)

  if (hours > 0) {
    // Show hours and minutes, but omit minutes if 0
    if (minutes > 0) {
      return `~${hours}h ${minutes}m`
    }
    return `~${hours}h`
  }

  return `~${minutes}m`
}

export { formatDate, formatDuration, formatETA, formatFileSize, formatTime, pluralize }

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

export { formatFileSize, formatDuration }

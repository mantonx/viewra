/**
 * Constants related to library scanning operations
 */

/**
 * Polling interval in milliseconds for checking scan status updates
 */
export const SCAN_POLL_INTERVAL_MS = 5000

/**
 * Maximum number of error entries to display in the UI
 * Prevents memory issues with very large error lists
 */
export const MAX_ERROR_DISPLAY_LIMIT = 1000

/**
 * Error category color mappings for UI display
 */
export const ERROR_CATEGORY_COLORS: Record<string, string> = {
  ffmpeg: 'text-red-600 bg-red-50 border-red-200',
  parsing: 'text-orange-600 bg-orange-50 border-orange-200',
  database: 'text-purple-600 bg-purple-50 border-purple-200',
  filesystem: 'text-blue-600 bg-blue-50 border-blue-200',
  metadata: 'text-yellow-600 bg-yellow-50 border-yellow-200',
  unknown: 'text-gray-600 bg-gray-50 border-gray-200',
}

/**
 * Development-only logging utility
 *
 * Provides a centralized logging interface that automatically strips out
 * debug/info logs in production builds while preserving error logs.
 *
 * Usage:
 *   logger.debug('Processing data:', data)
 *   logger.warn('Deprecated API usage')
 *   logger.error('Failed to load resource:', error)
 */

/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable no-console */

export const logger = {
  /**
   * Debug-level logging - only shown in development
   * Use for detailed diagnostic information
   */
  debug: (...args: any[]): void => {
    if (import.meta.env.DEV) {
      console.log(...args)
    }
  },

  /**
   * Info-level logging - only shown in development
   * Use for general informational messages
   */
  info: (...args: any[]): void => {
    if (import.meta.env.DEV) {
      console.info(...args)
    }
  },

  /**
   * Warning-level logging - only shown in development
   * Use for potentially problematic situations
   */
  warn: (...args: any[]): void => {
    if (import.meta.env.DEV) {
      console.warn(...args)
    }
  },

  /**
   * Error-level logging - always shown
   * Use for error conditions and exceptions
   */
  error: (...args: any[]): void => {
    console.error(...args)
  },
}

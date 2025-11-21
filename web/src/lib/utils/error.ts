/**
 * Error handling utilities
 */

/**
 * Extracts a user-friendly error message from various error types.
 * Handles Error objects, strings, and unknown types.
 *
 * @param error - The error to extract a message from
 * @param fallback - Default message if error type is unknown
 * @returns User-friendly error message
 *
 * @example
 * try {
 *   await dangerousOperation()
 * } catch (error) {
 *   const message = getErrorMessage(error, 'Operation failed')
 *   showToast(message)
 * }
 */
export const getErrorMessage = (error: unknown, fallback = 'An error occurred'): string => {
  if (error instanceof Error) {
    return error.message
  }

  if (typeof error === 'string') {
    return error
  }

  if (error && typeof error === 'object' && 'message' in error) {
    const msg = (error as { message: unknown }).message
    if (typeof msg === 'string') {
      return msg
    }
  }

  return fallback
}

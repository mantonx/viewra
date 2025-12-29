/**
 * Type guard utilities for API response validation
 */

import type { InternalApiHandlersScanErrorsResponse } from '@/lib/api'

/**
 * Type guard to check if response is successful scan errors response
 * @param data - Response data to check
 * @returns True if data is a valid ScanErrorsResponse
 */
export const isScanErrorsResponse = (
  data: unknown
): data is InternalApiHandlersScanErrorsResponse => {
  return (
    typeof data === 'object' &&
    data !== null &&
    typeof (data as { total_errors?: unknown }).total_errors === 'number' &&
    'by_category' in data
  )
}

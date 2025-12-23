/**
 * Types for useListData hook.
 */

export type ListItem = Record<string, unknown>

export interface UseListDataOptions {
  /** Whether to fetch data on mount */
  enabled?: boolean
  /** Query parameters to pass to the endpoint */
  params?: Record<string, string>
}

export interface UseListDataReturn {
  /** List items */
  items: ListItem[]
  /** Loading state */
  isLoading: boolean
  /** Error from fetch */
  error: Error | null
  /** Refresh the list */
  refresh: () => Promise<void>
}

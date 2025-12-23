/**
 * Types for ListHeader component.
 */

export interface ListHeaderProps {
  /** Title to display */
  title: string
  /** Item count */
  count: number
  /** Loading state */
  isLoading: boolean
  /** Called when refresh is clicked */
  onRefresh: () => void
}

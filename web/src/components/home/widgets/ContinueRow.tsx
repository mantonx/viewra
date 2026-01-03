import { MediaRow } from './MediaRow'
import type { MediaRowData } from './widget.types'

interface ContinueRowProps {
  data: MediaRowData
  className?: string
}

/**
 * ContinueRow - Continue Watching widget
 *
 * Displays movies and TV shows the user is currently watching.
 */
export const ContinueRow = ({ data, className }: ContinueRowProps) => {
  return <MediaRow data={data} className={className} />
}

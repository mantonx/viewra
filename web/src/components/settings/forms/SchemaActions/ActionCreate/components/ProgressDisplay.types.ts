/**
 * Types for ProgressDisplay component.
 */

import type { StreamingProgress } from '@/lib/hooks/useStreamingAction.types'

export interface ProgressDisplayProps {
  /** Progress state */
  progress: StreamingProgress
  /** Label to show (e.g., form values joined) */
  label?: string
}

interface EnrichmentIndicatorProps {
  /** Library ID to track enrichment progress */
  libraryId: number
  /** Whether to enable the SSE connection (default: true) */
  enabled?: boolean
  /** Called when enrichment completes */
  onComplete?: () => void
  /** Compact inline mode for use alongside other progress indicators (default: false) */
  compact?: boolean
}

export type { EnrichmentIndicatorProps }

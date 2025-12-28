export type EnrichmentStatus = 'pending' | 'processing' | 'failed' | 'enriched'

export interface EnrichmentBadgeProps {
  /** Enrichment status of the media item */
  status: EnrichmentStatus
  /** Additional CSS classes */
  className?: string
}

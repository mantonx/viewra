import { useCallback, useEffect, useRef } from 'react'
import { useMutation } from '@tanstack/react-query'
import { customInstance } from '@/lib/api/mutator'

interface PrioritizeRequest {
  media_id: number
  media_type: string
  library_id: number
}

interface PrioritizeResponse {
  media_id: number
  priority: number
  status: 'boosted' | 'enqueued' | 'already_complete'
}

const postApiEnrichmentPrioritize = async (
  request: PrioritizeRequest
): Promise<{ data: PrioritizeResponse; status: number }> => {
  return customInstance('/api/enrichment/prioritize', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })
}

interface UseAutoEnrichOptions {
  /** Media ID to auto-enrich */
  mediaId: number
  /** Media type: 'movie', 'tv', 'tv_show', 'music', etc. */
  mediaType: string
  /** Library ID the media belongs to */
  libraryId: number
  /** Whether enrichment is complete (has metadata) */
  isEnriched: boolean
  /** Enable/disable auto-enrichment */
  enabled?: boolean
  /** Callback when prioritization succeeds */
  onSuccess?: (status: PrioritizeResponse['status']) => void
  /** Callback when prioritization fails */
  onError?: (error: Error) => void
}

/**
 * Automatically prioritizes a media item for enrichment when viewed.
 * Only triggers if the item is not already enriched.
 *
 * This hook calls POST /api/enrichment/prioritize to boost the item's
 * priority to 1000 (interactive), ensuring it's processed immediately
 * instead of waiting behind the entire enrichment queue.
 *
 * @example
 * ```tsx
 * useAutoEnrich({
 *   mediaId: movie.id,
 *   mediaType: 'movie',
 *   libraryId: movie.library_id,
 *   isEnriched: !!movie.plot, // Has metadata = enriched
 * })
 * ```
 */
export const useAutoEnrich = ({
  mediaId,
  mediaType,
  libraryId,
  isEnriched,
  enabled = true,
  onSuccess,
  onError,
}: UseAutoEnrichOptions) => {
  // Track which items we've already prioritized to avoid duplicate calls
  const prioritizedRef = useRef<Set<string>>(new Set())

  const { mutate, isPending } = useMutation({
    mutationFn: postApiEnrichmentPrioritize,
    onSuccess: (response) => {
      onSuccess?.(response.data.status)
    },
    onError: (error: Error) => {
      onError?.(error)
    },
  })

  const prioritize = useCallback(() => {
    const key = `${mediaType}:${mediaId}`

    // Don't re-prioritize same item in this session
    if (prioritizedRef.current.has(key)) return

    prioritizedRef.current.add(key)

    mutate({
      media_id: mediaId,
      media_type: mediaType,
      library_id: libraryId,
    })
  }, [mediaId, mediaType, libraryId, mutate])

  useEffect(() => {
    // Only prioritize if:
    // 1. enabled
    // 2. not already enriched
    // 3. valid IDs
    if (!enabled || isEnriched || mediaId <= 0 || libraryId <= 0) return

    prioritize()
  }, [enabled, isEnriched, mediaId, libraryId, prioritize])

  return { isPending }
}

export default useAutoEnrich

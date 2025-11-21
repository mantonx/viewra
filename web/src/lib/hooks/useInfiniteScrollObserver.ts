/**
 * useInfiniteScrollObserver Hook
 * Encapsulates IntersectionObserver logic for infinite scroll
 */

import { useEffect, useRef } from 'react'

interface UseInfiniteScrollObserverOptions {
  onLoadMore: () => void
  enabled?: boolean
  threshold?: number
  rootMargin?: string
}

export const useInfiniteScrollObserver = ({
  onLoadMore,
  enabled = true,
  threshold = 0.1,
  rootMargin = '1000px', // Aggressive prefetch by default
}: UseInfiniteScrollObserverOptions) => {
  const observerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!enabled) {return}

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          onLoadMore()
        }
      },
      { threshold, rootMargin }
    )

    const currentTarget = observerRef.current
    if (currentTarget) {
      observer.observe(currentTarget)
    }

    return () => {
      if (currentTarget) {
        observer.unobserve(currentTarget)
      }
    }
  }, [onLoadMore, enabled, threshold, rootMargin])

  return observerRef
}

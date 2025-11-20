import type { Movie } from '@/lib/types/movies'

interface MovieCardProps {
  movie: Movie
  onClick?: () => void
}

export type { MovieCardProps }

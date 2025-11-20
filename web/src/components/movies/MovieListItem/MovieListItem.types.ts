import type { Movie } from '@/lib/types/movies'

export interface MovieListItemProps {
  movie: Movie
  onClick?: () => void
}

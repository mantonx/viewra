import { Loading } from '@/components/ui'
import type { LoadingPageProps } from './LoadingPage.types'

/**
 * Standardized loading page component with consistent padding and styling.
 * Used across routes to display loading states uniformly.
 */
export const LoadingPage = ({ text = 'Loading...', size = 'lg' }: LoadingPageProps) => (
  <div className="p-8">
    <Loading size={size} text={text} />
  </div>
)

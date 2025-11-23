export interface ProgressBarProps {
  progress: number
  label?: string
  variant?: 'default' | 'success' | 'warning' | 'error'
  size?: 'sm' | 'md'
  showPercentage?: boolean
}

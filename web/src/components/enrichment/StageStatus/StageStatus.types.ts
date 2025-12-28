interface StageStatusProps {
  /** Whether to show stages only when there are problems (default: true) */
  showOnlyProblems?: boolean
  /** Enable status fetching (default: true) */
  enabled?: boolean
  /** Optional className for the container */
  className?: string
}

export type { StageStatusProps }

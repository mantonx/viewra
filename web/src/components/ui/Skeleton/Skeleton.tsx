const Skeleton = ({ className = '' }: { className?: string }) => {
  return (
    <div
      className={`animate-pulse bg-neutral-200 dark:bg-neutral-800 rounded ${className}`}
      aria-hidden="true"
    />
  )
}

export { Skeleton }

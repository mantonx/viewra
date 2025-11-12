import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Utility function to merge Tailwind CSS classes with proper precedence
 */
const cn = (...inputs: ClassValue[]) => {
  return twMerge(clsx(inputs))
}

export { cn }

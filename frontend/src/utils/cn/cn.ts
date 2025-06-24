import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/**
 * Combines class names using clsx and tailwind-merge
 * 
 * This utility function merges CSS class names intelligently, resolving
 * conflicts between Tailwind CSS classes and combining conditional classes.
 * It's a combination of clsx for conditional logic and tailwind-merge for
 * Tailwind-specific conflict resolution.
 * 
 * @param inputs - Class values to combine (strings, objects, arrays, etc.)
 * @returns Combined and deduped class string
 * 
 * @example
 * ```typescript
 * // Basic usage
 * cn('px-4', 'py-2', 'bg-blue-500') // 'px-4 py-2 bg-blue-500'
 * 
 * // Conditional classes
 * cn('base-class', {
 *   'active-class': isActive,
 *   'disabled-class': isDisabled
 * })
 * 
 * // Tailwind conflict resolution
 * cn('px-4', 'px-6') // 'px-6' (later class wins)
 * cn('text-red-500', 'text-blue-500') // 'text-blue-500'
 * 
 * // Mixed usage
 * cn(
 *   'base-class',
 *   someCondition && 'conditional-class',
 *   {
 *     'state-class': hasState,
 *     'variant-primary': variant === 'primary'
 *   },
 *   props.className
 * )
 * ```
 * 
 * @remarks
 * - Uses clsx for flexible class name handling
 * - Uses tailwind-merge to resolve Tailwind CSS conflicts
 * - Handles conditional classes, arrays, objects, and undefined values
 * - Ensures later Tailwind classes override earlier conflicting ones
 * - Perfect for component prop merging with default classes
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
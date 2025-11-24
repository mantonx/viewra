/**
 * Semantic Utility Classes
 *
 * Pre-composed utility class strings that include both light and dark mode variants.
 * These reduce repetition of `dark:` classes across components.
 *
 * Usage:
 * import { bg, text, border } from '@/styles/semantic'
 * <div className={cn(bg.primary, text.primary, border.primary)}>
 */

/**
 * Background Colors
 *
 * Semantic background color utilities with built-in dark mode support
 */
export const bg = {
  /** Primary page background (neutral-50 / neutral-950) */
  primary: 'bg-neutral-50 dark:bg-neutral-950',

  /** Secondary surface background (neutral-100 / neutral-900) */
  secondary: 'bg-neutral-100 dark:bg-neutral-900',

  /** Tertiary/subtle background (neutral-200 / neutral-800) */
  tertiary: 'bg-neutral-200 dark:bg-neutral-800',

  /** Elevated surface (cards, modals) - white / neutral-900 */
  elevated: 'bg-white dark:bg-neutral-900',

  /** Inverse background (dark in light mode, light in dark mode) */
  inverse: 'bg-neutral-900 dark:bg-neutral-50',

  /** Hover states */
  hover: {
    /** Subtle hover (neutral-50 / neutral-900) */
    subtle: 'hover:bg-neutral-50 dark:hover:bg-neutral-900',

    /** Default hover (neutral-100 / neutral-800) */
    default: 'hover:bg-neutral-100 dark:hover:bg-neutral-800',

    /** Prominent hover (neutral-200 / neutral-700) */
    prominent: 'hover:bg-neutral-200 dark:hover:bg-neutral-700',
  },
} as const

/**
 * Text Colors
 *
 * Semantic text color utilities with built-in dark mode support
 */
export const text = {
  /** Primary text (neutral-900 / neutral-50) */
  primary: 'text-neutral-900 dark:text-neutral-50',

  /** Secondary text (neutral-600 / neutral-400) */
  secondary: 'text-neutral-600 dark:text-neutral-400',

  /** Tertiary/muted text (neutral-500 / neutral-500) */
  tertiary: 'text-neutral-500 dark:text-neutral-500',

  /** Disabled text (neutral-400 / neutral-600) */
  disabled: 'text-neutral-400 dark:text-neutral-600',

  /** Inverse text (light in light mode, dark in dark mode) */
  inverse: 'text-neutral-50 dark:text-neutral-900',

  /** Link text with hover */
  link: 'text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300',
} as const

/**
 * Border Colors
 *
 * Semantic border color utilities with built-in dark mode support
 */
export const border = {
  /** Primary border (neutral-200 / neutral-800) */
  primary: 'border-neutral-200 dark:border-neutral-800',

  /** Secondary/prominent border (neutral-300 / neutral-700) */
  secondary: 'border-neutral-300 dark:border-neutral-700',

  /** Subtle border (neutral-100 / neutral-900) */
  subtle: 'border-neutral-100 dark:border-neutral-900',

  /** Focus ring border */
  focus: 'border-primary-500 dark:border-primary-500',
} as const

/**
 * Shadow Utilities
 *
 * Shadow utilities with dark mode variants
 */
export const shadow = {
  /** Small shadow with dark mode */
  sm: 'shadow-sm dark:shadow-neutral-950/50',

  /** Default shadow with dark mode */
  default: 'shadow dark:shadow-neutral-950/50',

  /** Medium shadow with dark mode */
  md: 'shadow-md dark:shadow-neutral-950/50',

  /** Large shadow with dark mode */
  lg: 'shadow-lg dark:shadow-neutral-950/50',

  /** Extra large shadow with dark mode */
  xl: 'shadow-xl dark:shadow-neutral-950/50',

  /** 2XL shadow with dark mode */
  '2xl': 'shadow-2xl dark:shadow-neutral-950/50',

  /** No shadow */
  none: 'shadow-none',
} as const

/**
 * Ring Utilities (Focus States)
 *
 * Focus ring utilities with proper offset for dark mode
 */
export const ring = {
  /** Default focus ring (primary-500 with offset) */
  focus: 'focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-neutral-900',

  /** Focus ring without offset */
  focusInset: 'focus-visible:ring-2 focus-visible:ring-primary-500',

  /** Error focus ring */
  error: 'focus-visible:ring-2 focus-visible:ring-error-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-neutral-900',

  /** Success focus ring */
  success: 'focus-visible:ring-2 focus-visible:ring-success-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-neutral-900',
} as const

/**
 * Status/Semantic Colors
 *
 * Color utilities for success, warning, error, info states
 */
export const status = {
  success: {
    text: 'text-green-600 dark:text-green-500',
    bg: 'bg-green-50 dark:bg-green-950',
    border: 'border-green-300 dark:border-green-800',
  },
  warning: {
    text: 'text-yellow-600 dark:text-yellow-500',
    bg: 'bg-yellow-50 dark:bg-yellow-950',
    border: 'border-yellow-300 dark:border-yellow-800',
  },
  error: {
    text: 'text-red-600 dark:text-red-500',
    bg: 'bg-red-50 dark:bg-red-950',
    border: 'border-red-300 dark:border-red-800',
  },
  info: {
    text: 'text-blue-600 dark:text-blue-500',
    bg: 'bg-blue-50 dark:bg-blue-950',
    border: 'border-blue-300 dark:border-blue-800',
  },
} as const

/**
 * Common Component Patterns
 *
 * Pre-composed class strings for common component patterns
 */
export const patterns = {
  /** Card component classes */
  card: `${bg.elevated} ${border.primary} ${shadow.default} rounded-lg border`,

  /** Input component classes */
  input: `${bg.elevated} ${text.primary} ${border.primary} ${ring.focus} rounded-md border px-3 py-2 outline-none transition-colors`,

  /** Button base classes (add variant-specific colors) */
  buttonBase: `${ring.focus} rounded-md px-4 py-2 font-medium transition-colors outline-none`,

  /** Modal overlay */
  modalOverlay: 'bg-black/50 dark:bg-black/70',

  /** Modal content */
  modalContent: `${bg.elevated} ${shadow['2xl']} rounded-lg`,

  /** Sidebar/navigation */
  sidebar: `${bg.elevated} ${text.primary} ${border.primary} border-r`,

  /** Page header */
  pageHeader: `${bg.elevated} ${border.primary} border-b`,
} as const

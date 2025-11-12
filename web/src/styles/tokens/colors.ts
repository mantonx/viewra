/**
 * Design Tokens - Colors
 *
 * Centralized color system for ViewRA
 * Supports light and dark themes
 */

export const colors = {
  // Brand colors
  primary: {
    50: '#eff6ff',
    100: '#dbeafe',
    200: '#bfdbfe',
    300: '#93c5fd',
    400: '#60a5fa',
    500: '#3b82f6', // Main brand color
    600: '#2563eb',
    700: '#1d4ed8',
    800: '#1e40af',
    900: '#1e3a8a',
    950: '#172554',
  },

  // Neutral colors (grays)
  neutral: {
    50: '#fafafa',
    100: '#f5f5f5',
    200: '#e5e5e5',
    300: '#d4d4d4',
    400: '#a3a3a3',
    500: '#737373',
    600: '#525252',
    700: '#404040',
    800: '#262626',
    900: '#171717',
    950: '#0a0a0a',
  },

  // Semantic colors
  success: {
    50: '#f0fdf4',
    100: '#dcfce7',
    200: '#bbf7d0',
    300: '#86efac',
    400: '#4ade80',
    500: '#22c55e',
    600: '#16a34a',
    700: '#15803d',
    800: '#166534',
    900: '#14532d',
    950: '#052e16',
  },
  warning: {
    50: '#fefce8',
    100: '#fef9c3',
    200: '#fef08a',
    300: '#fde047',
    400: '#facc15',
    500: '#eab308',
    600: '#ca8a04',
    700: '#a16207',
    800: '#854d0e',
    900: '#713f12',
    950: '#422006',
  },
  error: {
    50: '#fef2f2',
    100: '#fee2e2',
    200: '#fecaca',
    300: '#fca5a5',
    400: '#f87171',
    500: '#ef4444',
    600: '#dc2626',
    700: '#b91c1c',
    800: '#991b1b',
    900: '#7f1d1d',
    950: '#450a0a',
  },
  info: {
    50: '#eff6ff',
    100: '#dbeafe',
    200: '#bfdbfe',
    300: '#93c5fd',
    400: '#60a5fa',
    500: '#3b82f6',
    600: '#2563eb',
    700: '#1d4ed8',
    800: '#1e40af',
    900: '#1e3a8a',
    950: '#172554',
  },
} as const

/**
 * Semantic color mappings for light theme
 */
export const lightTheme = {
  // Background colors
  background: {
    primary: colors.neutral[50], // Main background
    secondary: colors.neutral[100], // Cards, elevated surfaces
    tertiary: colors.neutral[200], // Hover states
    inverse: colors.neutral[900], // Dark backgrounds
  },

  // Text colors
  text: {
    primary: colors.neutral[900], // Primary text
    secondary: colors.neutral[600], // Secondary text
    tertiary: colors.neutral[500], // Disabled/muted text
    inverse: colors.neutral[50], // Text on dark backgrounds
    link: colors.primary[600], // Links
    linkHover: colors.primary[700], // Link hover
  },

  // Border colors
  border: {
    primary: colors.neutral[200], // Default borders
    secondary: colors.neutral[300], // Prominent borders
    focus: colors.primary[500], // Focus rings
  },

  // Component-specific colors
  button: {
    primary: colors.primary[600],
    primaryHover: colors.primary[700],
    secondary: colors.neutral[100],
    secondaryHover: colors.neutral[200],
    danger: colors.error[600],
    dangerHover: colors.error[700],
    success: colors.success[600],
    successHover: colors.success[700],
  },

  // Status colors
  status: {
    success: colors.success[600],
    successBg: colors.success[50],
    warning: colors.warning[600],
    warningBg: colors.warning[50],
    error: colors.error[600],
    errorBg: colors.error[50],
    info: colors.info[600],
    infoBg: colors.info[50],
  },
} as const

/**
 * Semantic color mappings for dark theme
 */
export const darkTheme = {
  // Background colors
  background: {
    primary: colors.neutral[950], // Main background
    secondary: colors.neutral[900], // Cards, elevated surfaces
    tertiary: colors.neutral[800], // Hover states
    inverse: colors.neutral[50], // Light backgrounds
  },

  // Text colors
  text: {
    primary: colors.neutral[50], // Primary text
    secondary: colors.neutral[400], // Secondary text
    tertiary: colors.neutral[500], // Disabled/muted text
    inverse: colors.neutral[900], // Text on light backgrounds
    link: colors.primary[400], // Links
    linkHover: colors.primary[300], // Link hover
  },

  // Border colors
  border: {
    primary: colors.neutral[800], // Default borders
    secondary: colors.neutral[700], // Prominent borders
    focus: colors.primary[500], // Focus rings
  },

  // Component-specific colors
  button: {
    primary: colors.primary[600],
    primaryHover: colors.primary[500],
    secondary: colors.neutral[800],
    secondaryHover: colors.neutral[700],
    danger: colors.error[600],
    dangerHover: colors.error[500],
    success: colors.success[600],
    successHover: colors.success[500],
  },

  // Status colors
  status: {
    success: colors.success[500],
    successBg: colors.success[950],
    warning: colors.warning[500],
    warningBg: colors.warning[950],
    error: colors.error[500],
    errorBg: colors.error[950],
    info: colors.info[500],
    infoBg: colors.info[950],
  },
} as const

export type Theme = typeof lightTheme
export type ThemeMode = 'light' | 'dark'

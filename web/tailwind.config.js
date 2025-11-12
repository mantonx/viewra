import { colors, spacing, sizes, borderRadius, borderWidth, boxShadow, zIndex, fontFamily, fontSize, fontWeight, lineHeight, letterSpacing } from './src/styles/tokens'

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class', // Enable class-based dark mode
  theme: {
    extend: {
      colors: {
        // Base color palette
        primary: colors.primary,
        neutral: colors.neutral,
        success: colors.success,
        warning: colors.warning,
        error: colors.error,
        info: colors.info,
      },
      spacing,
      width: sizes,
      maxWidth: sizes,
      borderRadius,
      borderWidth,
      boxShadow,
      zIndex,
      fontFamily: {
        sans: fontFamily.sans.split(', '),
        mono: fontFamily.mono.split(', '),
      },
      fontSize: Object.entries(fontSize).reduce((acc, [key, value]) => {
        acc[key] = Array.isArray(value) ? value : [value, {}]
        return acc
      }, {}),
      fontWeight,
      lineHeight,
      letterSpacing,
    },
  },
  plugins: [],
}

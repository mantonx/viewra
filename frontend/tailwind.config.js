// tailwind.config.js
import { defineConfig } from '@tailwindcss/vite';

function withOpacity(cssVariable) {
  return ({ opacityValue }) => {
    if (opacityValue !== undefined) {
      return `rgb(var(${cssVariable}) / ${opacityValue})`;
    }
    return `rgb(var(${cssVariable}))`;
  };
}

export default defineConfig({
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  darkMode: ['class', '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        // Primary colors
        primary: {
          50: withOpacity('--color-primary-50'),
          100: withOpacity('--color-primary-100'),
          200: withOpacity('--color-primary-200'),
          300: withOpacity('--color-primary-300'),
          400: withOpacity('--color-primary-400'),
          500: withOpacity('--color-primary-500'),
          600: withOpacity('--color-primary-600'),
          700: withOpacity('--color-primary-700'),
          800: withOpacity('--color-primary-800'),
          900: withOpacity('--color-primary-900'),
          950: withOpacity('--color-primary-950'),
          DEFAULT: withOpacity('--color-primary-500'),
        },
        // Secondary colors
        secondary: {
          50: withOpacity('--color-secondary-50'),
          100: withOpacity('--color-secondary-100'),
          200: withOpacity('--color-secondary-200'),
          300: withOpacity('--color-secondary-300'),
          400: withOpacity('--color-secondary-400'),
          500: withOpacity('--color-secondary-500'),
          600: withOpacity('--color-secondary-600'),
          700: withOpacity('--color-secondary-700'),
          800: withOpacity('--color-secondary-800'),
          900: withOpacity('--color-secondary-900'),
          950: withOpacity('--color-secondary-950'),
          DEFAULT: withOpacity('--color-secondary-500'),
        },
        // Background colors
        background: {
          DEFAULT: withOpacity('--color-background'),
          secondary: withOpacity('--color-background-secondary'),
          tertiary: withOpacity('--color-background-tertiary'),
          elevated: withOpacity('--color-background-elevated'),
          overlay: withOpacity('--color-background-overlay'),
        },
        // Foreground colors
        foreground: {
          DEFAULT: withOpacity('--color-foreground'),
          secondary: withOpacity('--color-foreground-secondary'),
          tertiary: withOpacity('--color-foreground-tertiary'),
          muted: withOpacity('--color-foreground-muted'),
        },
        // Border colors
        border: {
          DEFAULT: withOpacity('--color-border'),
          secondary: withOpacity('--color-border-secondary'),
          focus: withOpacity('--color-border-focus'),
        },
        // Semantic colors
        success: withOpacity('--color-success'),
        warning: withOpacity('--color-warning'),
        error: withOpacity('--color-error'),
        info: withOpacity('--color-info'),
        // Player specific colors
        player: {
          bg: withOpacity('--color-player-bg'),
          'controls-bg': withOpacity('--color-player-controls-bg'),
          'progress-bg': withOpacity('--color-player-progress-bg'),
          'progress-buffered': withOpacity('--color-player-progress-buffered'),
          'progress-played': withOpacity('--color-player-progress-played'),
          'progress-hover': withOpacity('--color-player-progress-hover'),
          text: withOpacity('--color-player-text'),
          'text-secondary': withOpacity('--color-player-text-secondary'),
        },
      },
      backgroundColor: {
        background: {
          DEFAULT: withOpacity('--color-background'),
          secondary: withOpacity('--color-background-secondary'),
          tertiary: withOpacity('--color-background-tertiary'),
          elevated: withOpacity('--color-background-elevated'),
          overlay: withOpacity('--color-background-overlay'),
        },
      },
      textColor: {
        foreground: {
          DEFAULT: withOpacity('--color-foreground'),
          secondary: withOpacity('--color-foreground-secondary'),
          tertiary: withOpacity('--color-foreground-tertiary'),
          muted: withOpacity('--color-foreground-muted'),
        },
      },
      borderColor: {
        border: {
          DEFAULT: withOpacity('--color-border'),
          secondary: withOpacity('--color-border-secondary'),
          focus: withOpacity('--color-border-focus'),
        },
      },
      opacity: {
        'overlay-light': 'var(--opacity-overlay-light)',
        'overlay-medium': 'var(--opacity-overlay-medium)',
        'overlay-heavy': 'var(--opacity-overlay-heavy)',
      },
      transitionDuration: {
        fast: 'var(--duration-fast)',
        normal: 'var(--duration-normal)',
        slow: 'var(--duration-slow)',
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        DEFAULT: 'var(--radius)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
        full: 'var(--radius-full)',
      },
      boxShadow: {
        sm: 'var(--shadow-sm)',
        DEFAULT: 'var(--shadow)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
        xl: 'var(--shadow-xl)',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'fade-in': 'fadeIn var(--duration-normal) ease-out',
        'zoom-in': 'zoomIn var(--duration-normal) ease-out',
        'zoom-in-slow': 'zoomIn var(--duration-slow) ease-out',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        zoomIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
      },
    },
  },
  plugins: [],
});
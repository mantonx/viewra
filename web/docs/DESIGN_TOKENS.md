# Design Tokens

A comprehensive design token system for ViewRA that provides consistent theming, dark mode support, and a scalable foundation for the UI.

## Overview

Design tokens are the atomic values of the design system - colors, spacing, typography, etc. They provide a single source of truth for design decisions and make it easy to maintain consistency across the application.

## Structure

```
web/src/styles/tokens/
├── colors.ts      # Color palette and theme definitions
├── spacing.ts     # Spacing, sizing, borders, shadows, z-index
├── typography.ts  # Fonts, sizes, weights, line heights
└── index.ts       # Main export
```

## Color Tokens

### Base Palette

We use a systematic color palette with 11 shades for each color family:

- **Primary**: Blue - Main brand color
- **Neutral**: Gray - Text, backgrounds, borders
- **Success**: Green - Success states
- **Warning**: Yellow - Warning states
- **Error**: Red - Error states
- **Info**: Blue - Informational states

Each color has shades from 50 (lightest) to 950 (darkest).

### Theme Support

The color system includes both light and dark theme mappings:

```typescript
import { lightTheme, darkTheme } from '@/styles/tokens'

// Light theme
lightTheme.background.primary  // neutral-50
lightTheme.text.primary         // neutral-900

// Dark theme
darkTheme.background.primary   // neutral-950
darkTheme.text.primary         // neutral-50
```

### Semantic Colors

Colors are organized by semantic meaning for easier use:

```typescript
const theme = lightTheme // or darkTheme

// Backgrounds
theme.background.primary   // Main background
theme.background.secondary // Cards, elevated surfaces
theme.background.tertiary  // Hover states

// Text
theme.text.primary        // Primary text
theme.text.secondary      // Secondary text
theme.text.link           // Link color

// Borders
theme.border.primary      // Default borders
theme.border.focus        // Focus rings

// Buttons
theme.button.primary
theme.button.danger
theme.button.success

// Status
theme.status.success
theme.status.error
theme.status.warning
```

## Spacing Tokens

Based on a 4px base unit for consistency:

```typescript
import { spacing } from '@/styles/tokens'

spacing[1]  // 0.25rem (4px)
spacing[2]  // 0.5rem (8px)
spacing[4]  // 1rem (16px)
spacing[8]  // 2rem (32px)
// ... up to spacing[96] (24rem / 384px)
```

### Usage with Tailwind

```tsx
<div className="p-4 m-8">      {/* padding: 1rem, margin: 2rem */}
<div className="space-y-6">    {/* vertical spacing: 1.5rem */}
```

## Sizing Tokens

Predefined sizes for components and containers:

```typescript
import { sizes } from '@/styles/tokens'

sizes.xs   // 20rem (320px)
sizes.md   // 28rem (448px)
sizes.xl   // 36rem (576px)
sizes['4xl'] // 56rem (896px)
```

### Container Widths

```typescript
sizes.container.sm  // 640px
sizes.container.md  // 768px
sizes.container.lg  // 1024px
sizes.container.xl  // 1280px
```

## Typography Tokens

### Font Families

```typescript
import { fontFamily } from '@/styles/tokens'

fontFamily.sans  // Inter, system-ui, ...
fontFamily.mono  // Fira Code, JetBrains Mono, ...
```

### Font Sizes

Includes both size and line-height:

```typescript
import { fontSize } from '@/styles/tokens'

fontSize.xs    // ['0.75rem', { lineHeight: '1rem' }]
fontSize.base  // ['1rem', { lineHeight: '1.5rem' }]
fontSize['3xl'] // ['1.875rem', { lineHeight: '2.25rem' }]
```

### Semantic Typography

Predefined text styles for common use cases:

```typescript
import { typography } from '@/styles/tokens'

typography.h1         // Heading 1
typography.body       // Body text
typography.caption    // Small caption text
typography.code       // Code blocks
```

## Border Tokens

### Border Radius

```typescript
import { borderRadius } from '@/styles/tokens'

borderRadius.DEFAULT  // 0.25rem (4px)
borderRadius.lg       // 0.5rem (8px)
borderRadius.full     // 9999px (pill shape)
```

### Border Width

```typescript
import { borderWidth } from '@/styles/tokens'

borderWidth.DEFAULT  // 1px
borderWidth[2]       // 2px
```

## Shadows

Consistent elevation system:

```typescript
import { boxShadow } from '@/styles/tokens'

boxShadow.sm       // Subtle shadow
boxShadow.DEFAULT  // Standard shadow
boxShadow.lg       // Prominent shadow
boxShadow.xl       // Very prominent shadow
```

## Z-Index

Layering system for overlays:

```typescript
import { zIndex } from '@/styles/tokens'

zIndex.dropdown       // 1000
zIndex.modal          // 1050
zIndex.tooltip        // 1070
```

## Using Design Tokens

### In Tailwind Classes

The tokens are integrated with Tailwind, so you can use them directly in className:

```tsx
<div className="bg-neutral-50 dark:bg-neutral-950">
  <h1 className="text-3xl font-bold text-neutral-900 dark:text-neutral-50">
    Title
  </h1>
  <p className="text-neutral-600 dark:text-neutral-400">
    Description
  </p>
</div>
```

### In TypeScript

Import and use tokens directly in your components:

```tsx
import { lightTheme, spacing } from '@/styles/tokens'

const buttonStyle = {
  padding: `${spacing[2]} ${spacing[4]}`,
  backgroundColor: lightTheme.button.primary,
}
```

### With Theme Context

Use the theme provider for dynamic theming:

```tsx
import { useTheme } from '@/contexts/ThemeContext'

function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()

  return (
    <button onClick={toggleTheme}>
      {theme === 'light' ? '🌙 Dark' : '☀️ Light'}
    </button>
  )
}
```

## Dark Mode

Dark mode is implemented using Tailwind's class-based dark mode.

### Setup

Wrap your app with the ThemeProvider:

```tsx
import { ThemeProvider } from '@/contexts/ThemeContext'

function App() {
  return (
    <ThemeProvider defaultTheme="light">
      <YourApp />
    </ThemeProvider>
  )
}
```

### Usage

Add dark mode variants to any Tailwind class:

```tsx
<div className="bg-white dark:bg-neutral-900">
  <p className="text-gray-900 dark:text-gray-100">
    This text adapts to the theme
  </p>
</div>
```

### Toggling Theme

```tsx
import { useTheme } from '@/contexts/ThemeContext'

function Component() {
  const { theme, setTheme, toggleTheme } = useTheme()

  return (
    <button onClick={toggleTheme}>
      Toggle Theme (Current: {theme})
    </button>
  )
}
```

## Best Practices

1. **Use semantic tokens**: Prefer `text-neutral-900` over specific hex colors
2. **Think in scales**: Use the spacing scale consistently (4, 8, 12, 16, etc.)
3. **Respect the palette**: Stick to the defined color shades (50-950)
4. **Always provide dark mode**: Add dark: variants for all themed elements
5. **Use CSS variables sparingly**: Prefer Tailwind classes for consistency
6. **Test both themes**: Ensure your UI works in both light and dark mode

## Adding New Tokens

To add new design tokens:

1. Add the token to the appropriate file in `src/styles/tokens/`
2. Update the Tailwind config in `tailwind.config.js` if needed
3. Document the new token in this file
4. Add usage examples

Example:

```typescript
// src/styles/tokens/colors.ts
export const colors = {
  // ... existing colors
  accent: {
    500: '#your-color',
    // ... other shades
  },
}

// tailwind.config.js
theme: {
  extend: {
    colors: {
      accent: colors.accent,
    },
  },
}
```

## Migration Guide

When updating existing components to use design tokens:

1. Replace hardcoded colors with semantic tokens
2. Replace magic numbers with spacing tokens
3. Add dark mode support with dark: variants
4. Test in both light and dark themes

### Before

```tsx
<div style={{ padding: '16px', backgroundColor: '#f5f5f5' }}>
  <h1 style={{ fontSize: '24px', color: '#333' }}>Title</h1>
</div>
```

### After

```tsx
<div className="p-4 bg-neutral-100 dark:bg-neutral-900">
  <h1 className="text-2xl text-neutral-900 dark:text-neutral-50">Title</h1>
</div>
```

## Resources

- [Tailwind CSS Documentation](https://tailwindcss.com)
- [Design Tokens W3C Standard](https://design-tokens.github.io/community-group/format/)
- [Color Palette Tool](https://uicolors.app)

# Design Tokens

A comprehensive design token system for ViewRA that provides consistent theming, dark mode support, and a scalable foundation for the UI.

## Overview

Design tokens are the atomic values of the design system - colors, spacing, typography, etc. They provide a single source of truth for design decisions and make it easy to maintain consistency across the application.

ViewRA uses **Tailwind CSS v4's `@theme` directive** to define design tokens natively in CSS, providing better performance and alignment with web standards.

## Structure

**Primary Source (Tailwind v4):**

```
web/src/index.css
└── @theme directive  # All design tokens defined in CSS
```

**Legacy (Deprecated but maintained for reference):**

```
web/src/styles/tokens/
├── colors.ts      # Color palette definitions (TypeScript reference)
├── spacing.ts     # Spacing, sizing tokens (TypeScript reference)
├── typography.ts  # Typography tokens (TypeScript reference)
└── index.ts       # Main export
```

> **Note**: The TypeScript token files in `src/styles/tokens/` are kept for backward compatibility and type safety, but the source of truth is now the `@theme` block in `index.css`.

## Color Tokens

### Base Palette

We use a systematic color palette with 11 shades for each color family, defined in the `@theme` directive:

- **Primary**: Blue - Main brand color
- **Neutral**: Gray - Text, backgrounds, borders
- **Success**: Green - Success states
- **Warning**: Yellow - Warning states
- **Error**: Red - Error states
- **Info**: Blue - Informational states

Each color has shades from 50 (lightest) to 950 (darkest).

**Definition** ([index.css](../src/index.css)):

```css
@theme {
  /* Primary colors (Blue) */
  --color-primary-50: #eff6ff;
  --color-primary-500: #3b82f6;
  --color-primary-900: #1e3a8a;

  /* Neutral colors (Grays) */
  --color-neutral-50: #fafafa;
  --color-neutral-500: #737373;
  --color-neutral-900: #171717;

  /* Success, Warning, Error, Info... */
}
```

**Usage in Tailwind:**

```tsx
<div className="bg-primary-500 text-white">
  <p className="text-neutral-900 dark:text-neutral-50">
    Colored text
  </p>
</div>
```

### Theme Support

Dark mode is implemented using Tailwind v4's class-based system with the `dark:` variant.

**Configuration** ([index.css](../src/index.css)):

```css
/* Configure class-based dark mode */
@variant dark (.dark &);
```

**Usage:**

```tsx
<div className="bg-neutral-100 dark:bg-neutral-900">
  <p className="text-neutral-900 dark:text-neutral-50">
    Adapts to theme
  </p>
</div>
```

### Semantic Colors (Custom CSS Variables)

For advanced theming, semantic CSS variables are defined separately from Tailwind tokens:

**Definition** ([index.css](../src/index.css)):

```css
:root {
  /* Light theme */
  --color-bg-primary: 250 250 250;     /* neutral-50 */
  --color-text-primary: 23 23 23;      /* neutral-900 */
  --color-border-primary: 229 229 229; /* neutral-200 */
}

.dark {
  /* Dark theme */
  --color-bg-primary: 10 10 10;        /* neutral-950 */
  --color-text-primary: 250 250 250;   /* neutral-50 */
  --color-border-primary: 38 38 38;    /* neutral-800 */
}
```

**Usage in custom CSS:**

```css
.custom-component {
  background-color: rgb(var(--color-bg-primary));
  color: rgb(var(--color-text-primary));
}
```

> **Note**: Prefer Tailwind utilities (`bg-neutral-100 dark:bg-neutral-900`) over custom CSS variables for consistency.

## Spacing Tokens

Based on a 4px base unit for consistency, defined in `@theme`:

**Definition** ([index.css](../src/index.css)):

```css
@theme {
  --spacing-1: 0.25rem;   /* 4px */
  --spacing-2: 0.5rem;    /* 8px */
  --spacing-4: 1rem;      /* 16px */
  --spacing-8: 2rem;      /* 32px */
  /* ... up to spacing-96 (24rem / 384px) */
}
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

Dark mode is implemented using **Tailwind v4's class-based dark mode** system.

### Setup

The dark mode variant is configured in [index.css](../src/index.css):

```css
@variant dark (.dark &);
```

Wrap your app with the ThemeProvider ([App.tsx](../src/App.tsx)):

```tsx
import { ThemeProvider } from '@/contexts/ThemeContext'

function App() {
  return (
    <ThemeProvider>
      <YourApp />
    </ThemeProvider>
  )
}
```

The ThemeProvider:

- Respects system preferences (`prefers-color-scheme`)
- Persists user choice to localStorage
- Adds/removes `.dark` class on `<html>` element

### Usage

Add dark mode variants to any Tailwind class:

```tsx
<div className="bg-white dark:bg-neutral-900">
  <p className="text-neutral-900 dark:text-neutral-50">
    This text adapts to the theme
  </p>
</div>
```

### Toggling Theme

```tsx
import { useTheme } from '@/contexts/ThemeContext'

function Component() {
  const { theme, toggleTheme } = useTheme()

  return (
    <button onClick={toggleTheme}>
      {theme === 'dark' ? '☀️ Light' : '🌙 Dark'}
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

## Adding New Tokens (Tailwind v4)

To add new design tokens in Tailwind v4:

1. Add the token to the `@theme` block in [index.css](../src/index.css)
2. Document the new token in this file
3. Add usage examples

Example - Adding an accent color:

```css
/* web/src/index.css */
@theme {
  /* Existing tokens... */

  /* Accent colors (Purple) */
  --color-accent-50: #faf5ff;
  --color-accent-500: #a855f7;
  --color-accent-900: #581c87;
}
```

Usage:

```tsx
<button className="bg-accent-500 hover:bg-accent-600 text-white">
  Accent Button
</button>
```

> **Note**: Tailwind v4 automatically generates utilities from `@theme` tokens. No additional configuration needed in `tailwind.config.js`.

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

## Tailwind v4 Migration (2025-11-23)

ViewRA migrated from Tailwind CSS v3 to v4, moving all design tokens from JavaScript to CSS.

### Key Changes

**Before (v3):**

- Tokens defined in TypeScript files (`src/styles/tokens/*.ts`)
- Imported into `tailwind.config.js`
- 42-line configuration file

**After (v4):**

- Tokens defined in CSS `@theme` directive
- Minimal 6-line `tailwind.config.js`
- No JavaScript imports required

### Benefits

✅ Faster build times and HMR
✅ Better alignment with web standards
✅ Smaller configuration
✅ All Tailwind utilities work correctly
✅ No bundler required for tokens

### Migration Path

If you have legacy code using TypeScript tokens:

```typescript
// ❌ Old way (deprecated)
import { colors } from '@/styles/tokens'
const bg = colors.neutral[500]

// ✅ New way (Tailwind v4)
<div className="bg-neutral-500" />
```

For more details, see [ADR 0001](../../docs/adr/0001-design-tokens-and-dark-mode.md).

## Resources

- [Tailwind CSS v4 Documentation](https://tailwindcss.com/docs/v4-beta)
- [Tailwind v4 @theme Directive](https://tailwindcss.com/docs/functions-and-directives#theme)
- [Design Tokens W3C Standard](https://design-tokens.github.io/community-group/format/)
- [Color Palette Tool](https://uicolors.app)

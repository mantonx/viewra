# Glassmorphism Design Utilities

Modern translucent glass-like effects for ViewRA's UI components. These utilities provide consistent, cross-browser glassmorphism effects with automatic dark mode support.

## Overview

Glassmorphism creates a frosted glass effect with:
- **Translucent backgrounds** (partial opacity)
- **Backdrop blur** (content behind the element is blurred)
- **Subtle borders** (usually light colored)
- **Depth with shadows** (optional inset highlights)

## Usage

Import from the semantic design system:

```tsx
import { glass, glassStyles } from '@/styles/semantic'
import { useTheme } from '@/contexts'
```

## Tailwind Class Utilities

### 1. Light Glassmorphism
Subtle effect for secondary UI elements.

```tsx
// Individual pieces
<div className={`${glass.light.bg} ${glass.light.border} ${glass.light.blur}`}>

// Or use the full preset
<div className={glass.light.full}>
  Content
</div>
```

**Use cases:**
- Floating toolbars
- Secondary navigation
- Subtle overlays

### 2. Medium Glassmorphism
Balanced effect for primary interactive elements.

```tsx
<div className={glass.medium.full}>
  Content
</div>
```

**Use cases:**
- Search bars
- Filter panels
- Card overlays

### 3. Strong Glassmorphism
Prominent effect for hero sections and modals.

```tsx
<div className={glass.strong.full}>
  Content
</div>
```

**Use cases:**
- Modal dialogs
- Hero sections
- Featured content cards

### 4. Enhanced Glassmorphism
Custom enhanced effect with saturation boost (requires inline styles).

```tsx
<div
  className="rounded-xl border"
  style={glassStyles.enhanced(theme === 'dark')}
>
  Content
</div>
```

**Use cases:**
- Main search bars (like MediaBrowsePage)
- Premium UI sections
- Branded components

## Inline Style Objects

When Tailwind classes aren't sufficient (e.g., Safari webkit prefixes, custom saturation), use `glassStyles`:

```tsx
const { theme } = useTheme()

// Light effect
<div style={glassStyles.light(theme === 'dark')}>

// Medium effect
<div style={glassStyles.medium(theme === 'dark')}>

// Enhanced effect (with saturation and custom shadows)
<div style={glassStyles.enhanced(theme === 'dark')}>
```

### Why Inline Styles?

Some glassmorphism effects require:
1. **Safari compatibility** (`WebkitBackdropFilter` prefix)
2. **Custom saturation** (`backdrop-filter: saturate(130%)`)
3. **Complex shadows** (inset highlights + depth shadows)

These work better as inline styles than trying to configure them all in Tailwind.

## Pre-composed Patterns

Use these for common component patterns:

```tsx
import { patterns } from '@/styles/semantic'

// Glass card
<div className={patterns.glassCard}>

// Glass panel (search bars, toolbars)
<div className={patterns.glassPanel}>
```

## Examples

### Search Bar with Enhanced Glass

```tsx
import { glassStyles } from '@/styles/semantic'
import { useTheme } from '@/contexts'

const SearchBar = () => {
  const { theme } = useTheme()

  return (
    <div
      className="px-4 py-3 rounded-xl border transition-all"
      style={glassStyles.enhanced(theme === 'dark')}
    >
      <input
        type="search"
        placeholder="Search..."
        className="bg-transparent w-full outline-none"
      />
    </div>
  )
}
```

### Floating Toolbar

```tsx
import { glass } from '@/styles/semantic'

const FloatingToolbar = () => (
  <div className={`${glass.medium.full} px-4 py-2 rounded-lg shadow-lg`}>
    <button>Action 1</button>
    <button>Action 2</button>
  </div>
)
```

### Modal with Strong Glass

```tsx
import { glass } from '@/styles/semantic'

const GlassModal = ({ children }) => (
  <>
    {/* Backdrop */}
    <div className="fixed inset-0 bg-black/50" />

    {/* Modal */}
    <div className={`${glass.strong.full} rounded-2xl p-6 shadow-2xl`}>
      {children}
    </div>
  </>
)
```

## Best Practices

### ✅ Do:
- Use glassmorphism for **floating** or **overlay** UI elements
- Ensure there's **visual content behind** the glass (images, gradients)
- Keep **text contrast high** (avoid placing glass over busy backgrounds)
- Use **appropriate intensity** (light for subtle, strong for prominent)
- Test in both **light and dark modes**

### ❌ Don't:
- Use glass effects on **empty backgrounds** (looks flat)
- Stack **multiple glass layers** (becomes unreadable)
- Use glass for **primary content** (reserve for UI chrome)
- Forget **fallbacks** for browsers without backdrop-filter support
- Overuse glass effects (use sparingly for impact)

## Browser Support

| Feature | Support |
|---------|---------|
| `backdrop-filter` | Chrome 76+, Safari 9+, Firefox 103+ |
| `-webkit-backdrop-filter` | Safari 9+ (prefixed) |
| `backdrop-filter: saturate()` | Chrome 76+, Safari 9+ |

**Fallback:** Components gracefully degrade to semi-transparent backgrounds in older browsers.

## Accessibility

- **Contrast:** Ensure text on glass backgrounds meets WCAG AA standards (4.5:1 for body text)
- **Motion:** Consider `prefers-reduced-transparency` for users with vestibular disorders
- **Focus states:** Make focus indicators visible against glass backgrounds

## Related

- [Semantic Utilities](../design-tokens/semantic.md)
- [Dark Mode Guide](../design-tokens/dark-mode.md)
- [ADR 0001: Design Token System](../adr/0001-design-tokens-and-dark-mode.md)

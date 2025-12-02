# ADR 031: Design System Improvements - Cinema at Home

## Status

Accepted (Implemented)

### Implementation Progress

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1: Foundation | ✅ Complete | Color system, glass utilities, animations, layout background |
| Phase 2: Sidebar | ✅ Complete | Glass sidebar with ambient glow, refined nav states |
| Phase 3: Dashboard | ✅ Complete | Cinematic hero with ambient glows, glass stat cards, icon grid |
| Phase 4: Media Cards | ✅ Complete | Premium hover effects, glass badges, resolution colors |
| Phase 5: Components | ✅ Complete | Modal glass treatment, button/input refinements |
| Phase 6: Micro-interactions | ✅ Complete | Modal animations, skeleton shimmer, page transitions |

## Context

ViewRA's current frontend design has a disconnect between its cinematic authentication pages (login, setup) and its utilitarian interior application experience. The auth pages feature:

- Glass-morphism effects with backdrop blur
- Ambient glow animations
- Staggered form animations
- Premium streaming aesthetic

However, the main application interior (dashboard, sidebar, media browsing) feels like a different product:

- Plain white/neutral backgrounds without atmosphere
- Basic hover states without personality
- No visual continuity with the auth experience
- Generic admin-panel aesthetic rather than media-app aesthetic

### Current Design Assets

**Strengths:**
- Tailwind CSS 4 with well-organized design tokens
- CVA (class-variance-authority) for component variants
- Semantic utility system in `web/src/styles/semantic.ts`
- Outfit display font + Inter body font pairing
- Full light/dark mode support

**Weaknesses:**
- Dashboard is generic text + unstyled buttons
- Sidebar feels administrative, not media-focused
- No atmospheric elements (gradients, glows) in main app
- Media cards lack premium treatment
- Inconsistent visual language between auth and main app

### Design Vision

**"Cinema at Home"** - Dark-dominant streaming aesthetic with cinematic depth. Every screen should feel like you're in a personal theater, with content taking center stage and subtle atmospheric enhancement.

**Design Principles:**
1. **Refined animations** - Subtle, Apple-like transitions rather than flashy
2. **Light/dark parity** - Both themes should feel equally polished
3. **Blue accent** - Keep the current blue (primary-500/600) as the signature color
4. **Atmospheric depth** - Subtle gradients and glows, not flat surfaces

## Decision

Implement a phased design system improvement focusing on **Phase 1: Foundation Refresh** first, with subsequent phases for Sidebar, Dashboard, Media Cards, and Polish.

### Phase 1: Foundation Refresh (This ADR)

#### 1.1 Color System Enhancement

Extend the current neutral palette with atmospheric colors:

```css
/* New CSS variables in index.css */
:root {
  /* Atmospheric backgrounds */
  --color-surface-glass: 255 255 255 / 0.85;
  --color-glow-blue: 59 130 246 / 0.08;
  --color-glow-purple: 139 92 246 / 0.05;
}

.dark {
  --color-surface-glass: 23 23 23 / 0.7;
  --color-glow-blue: 59 130 246 / 0.15;
  --color-glow-purple: 139 92 246 / 0.1;
}
```

#### 1.2 Typography Hierarchy

Strengthen the existing Outfit + Inter pairing:

| Element | Font | Weight | Size | Letter Spacing |
|---------|------|--------|------|----------------|
| Hero titles | Outfit | 600 | 4xl | tracking-tight |
| Page titles | Outfit | 600 | 3xl | tracking-tight |
| Section headers | Outfit | 500 | 2xl | tracking-tight |
| Navigation labels | Outfit | 500 | xs | tracking-wider, uppercase |
| Body text | Inter | 400 | base | normal |

#### 1.3 Cinematic Background in Main App

Apply subtle atmospheric background to the main layout (not as intense as auth pages):

```css
.app-background {
  background:
    radial-gradient(ellipse 100% 80% at 50% -30%, rgb(var(--color-glow-blue)), transparent),
    linear-gradient(to bottom, rgb(var(--color-bg-primary)), rgb(var(--color-bg-secondary)));
}
```

#### 1.4 Glass Component Enhancement

Create reusable glass utility classes for both light and dark modes:

```css
.glass-card {
  background: rgb(var(--color-surface-glass));
  backdrop-filter: blur(20px) saturate(150%);
  border: 1px solid rgb(var(--color-border-primary) / 0.5);
}
```

### Future Phases (Separate ADRs)

**Phase 2: Sidebar Redesign**
- Glass effect sidebar with ambient glow
- Active states with accent bar + soft glow
- User section with avatar and cleaner layout

**Phase 3: Dashboard Transformation**
- Hero section with backdrop imagery
- Continue Watching with progress indicators
- Glass stat cards

**Phase 4: Media Cards Overhaul**
- Hover effects with scale + glow
- Quality badges with glass styling
- Progress bar integration

**Phase 5: Component Polish**
- Button glow effects on primary
- Input focus animations
- Modal glass treatment

**Phase 6: Micro-interactions**
- Page transitions
- Loading skeleton shimmer
- Toast animations

## Consequences

### Positive

- **Visual consistency** - Auth pages and main app share the same design language
- **Premium feel** - Users will perceive ViewRA as a polished, professional media server
- **Maintainability** - CSS variables enable easy theme adjustments
- **Accessibility** - Light mode support ensures readability for all users
- **Performance** - CSS-only effects (no JS animations for core styling)

### Negative

- **Safari support** - Backdrop-filter has quirks; requires `-webkit-` prefixes and fallbacks
- **Light mode complexity** - Glass effects are harder to make look good in light mode
- **Design debt** - Existing pages need updates to use new patterns

### Neutral

- Requires careful testing on lower-powered devices for blur performance
- Team needs to learn the new utility patterns

## Alternatives Considered

### 1. Adopt a Third-Party Component Library (Shadcn, Radix)

**Rejected because:**
- Would require significant migration effort
- Loses the custom cinematic aesthetic
- Already have a functional component system

### 2. Dark Mode Only

**Rejected because:**
- User preference should be respected
- Light mode is important for accessibility and daylight use
- Many media apps support both (Netflix, Plex, Jellyfin)

### 3. Full Redesign from Scratch

**Rejected because:**
- Disrupts existing functional UI
- Much higher risk and effort
- Current foundation is solid; just needs polish

### 4. More Expressive/Flashy Animations

**Rejected because:**
- User preference for refined, Apple-like animations
- Flashy animations can feel cheap over time
- Refined animations are more universally appealing

## Implementation Details

### File Changes

| File | Changes |
|------|---------|
| `web/src/index.css` | Add atmospheric CSS variables, glass utilities |
| `web/src/styles/semantic.ts` | Add glass patterns, atmospheric utilities |
| `web/src/routes/_layout.tsx` | Apply cinematic background |
| `web/src/components/ui/Button/Button.tsx` | Enhanced hover states |
| `web/src/components/ui/Input/Input.tsx` | Focus glow improvements |
| `web/src/routes/_layout/index.tsx` | Dashboard improvements (Phase 3) |

### Testing Checklist

- [x] Light mode appearance matches dark mode quality
- [ ] Backdrop-filter works in Safari
- [ ] No performance issues on low-end devices
- [x] Animations respect `prefers-reduced-motion`
- [x] All existing functionality preserved

### Files Modified (Implementation)

| File | Changes |
|------|---------|
| `web/src/index.css` | Atmospheric CSS variables, glass utilities, animations (page-enter, modal-enter, skeleton-shimmer), reduced-motion support |
| `web/src/routes/_layout.tsx` | Applied app-background, glass-sidebar with sidebar-glow, refined nav link states |
| `web/src/routes/_layout/index.tsx` | Cinematic hero with ambient glows, glass stat cards, library counts, icon grid quick actions |
| `web/src/components/ui/Button/Button.tsx` | Refined hover states with primary glow, rounded-lg |
| `web/src/components/ui/Input/Input.tsx` | Improved focus rings with primary-500/30, hover states |
| `web/src/components/ui/Modal/Modal.tsx` | Glass treatment, backdrop blur, refined borders, animate-in entrance |
| `web/src/components/ui/Skeleton/Skeleton.tsx` | Shimmer effect animation |
| `web/src/components/ui/Loading/Loading.tsx` | Uses primary color for spinner |
| `web/src/components/common/PageHeader/PageHeader.tsx` | Better spacing (mb-8), leading-tight/relaxed |
| `web/src/components/media/MediaCard/MediaCard.tsx` | Premium hover with scale, shadow, blue glow in dark mode |
| `web/src/components/media/MediaBadges/MediaBadges.tsx` | Glass badges, resolution color hierarchy (amber=4K, white=1080p), emerald NEW gradient |
| `web/src/components/home/ContinueWatching.tsx` | Consistent section styling with font-display headings |

## References

- [ADR 013: Library Browsing UX Improvements](013-library-browsing-ux-improvements.md) - Previous UX work
- [ADR 015: Player Enhancement Strategy](015-player-enhancement-strategy.md) - Player UI context
- Current design tokens: `web/src/index.css`
- Semantic utilities: `web/src/styles/semantic.ts`
- Auth page styling: `.auth-background`, `.auth-card` classes in index.css

# Component Library Structure

This directory contains all React components organized by domain and functionality.

## Directory Structure

```
components/
├── admin/          # Admin-specific components
├── audio/          # Audio playback components
├── layout/         # Layout components (Header, Footer, etc.)
├── media/          # Media library and file components
├── plugins/        # Plugin system components
├── system/         # System monitoring and utilities
└── ui/             # Reusable UI primitives
```

## Component Guidelines

1. **Domain Organization**: Components are organized by their primary domain/feature
2. **Barrel Exports**: Each folder has an `index.ts` for clean imports
3. **Type Safety**: All components use TypeScript with proper interfaces
4. **Composition**: Favor composition over inheritance
5. **Single Responsibility**: Each component should have one clear purpose

## Import Patterns

```tsx
// Import from barrel exports
import { AudioPlayer, MediaCard, SystemInfo } from '@/components';

// Import from specific domains
import { PluginManager } from '@/components/plugins';

// Import UI primitives
import { IconButton } from '@/components/ui';
import { Play, Pause } from '@/components/ui/icons';
```

## Adding New Components

1. Determine the appropriate domain folder
2. Create the component file with TypeScript
3. Add proper type definitions
4. Export from the domain's `index.ts`
5. Add to the main `components/index.ts` if it's a public API

## Component Patterns

### Container/Presentational Pattern

- Container components handle data and logic
- Presentational components handle rendering

### Compound Components

- Used for complex UI like `MusicLibrary` with `MediaCard`
- Provides flexible composition

### Controlled/Uncontrolled

- Form inputs use controlled pattern
- Complex state managed by parent components

# ViewRA Component Library - Ideal Architecture

## Overview

This document outlines the ideal component library structure for ViewRA, designed to scale from MVP to a full-featured media server application.

## Directory Structure

```
web/src/
├── components/
│   ├── ui/                      # Base UI primitives
│   │   ├── Button/
│   │   │   ├── Button.tsx
│   │   │   ├── Button.test.tsx
│   │   │   └── index.ts
│   │   ├── Input/
│   │   ├── Select/
│   │   ├── Card/
│   │   ├── Modal/
│   │   ├── Alert/
│   │   ├── Loading/
│   │   ├── Badge/
│   │   ├── Tooltip/
│   │   ├── Dropdown/
│   │   └── index.ts
│   │
│   ├── form/                    # Form-specific components
│   │   ├── FormField/
│   │   ├── FormGroup/
│   │   ├── FormError/
│   │   ├── SearchInput/
│   │   ├── FileInput/
│   │   └── index.ts
│   │
│   ├── layout/                  # Layout components
│   │   ├── Container/
│   │   ├── Grid/
│   │   ├── Stack/
│   │   ├── Sidebar/
│   │   ├── Header/
│   │   ├── Footer/
│   │   └── index.ts
│   │
│   ├── media/                   # Media-specific components
│   │   ├── MediaCard/
│   │   │   ├── MediaCard.tsx
│   │   │   ├── MediaCardSkeleton.tsx
│   │   │   └── index.ts
│   │   ├── MediaGrid/
│   │   ├── MediaList/
│   │   ├── VideoPlayer/
│   │   ├── AudioPlayer/
│   │   ├── Thumbnail/
│   │   ├── ProgressBar/
│   │   └── index.ts
│   │
│   ├── library/                 # Library-specific components
│   │   ├── LibraryCard/
│   │   ├── LibraryForm/
│   │   ├── LibraryStats/
│   │   ├── ScanProgress/
│   │   └── index.ts
│   │
│   ├── common/                  # Shared business components
│   │   ├── EmptyState/
│   │   ├── ErrorBoundary/
│   │   ├── PageHeader/
│   │   ├── PageLoader/
│   │   ├── SearchBar/
│   │   ├── FilterPanel/
│   │   ├── Pagination/
│   │   └── index.ts
│   │
│   └── feedback/                # User feedback components
│       ├── Toast/
│       ├── ConfirmDialog/
│       ├── ProgressIndicator/
│       └── index.ts
│
├── hooks/                       # Custom React hooks
│   ├── useInvalidateLibraries.ts
│   ├── useMediaFilters.ts
│   ├── useDebounce.ts
│   ├── useLocalStorage.ts
│   ├── useKeyPress.ts
│   └── index.ts
│
├── lib/
│   ├── utils/                   # Utility functions
│   │   ├── cn.ts               # className merger
│   │   ├── format.ts           # Formatters (file size, duration, date)
│   │   ├── validation.ts       # Validation helpers
│   │   └── index.ts
│   └── constants/               # App constants
│       ├── media.ts            # Media type constants
│       ├── routes.ts           # Route constants
│       └── index.ts
│
└── types/                       # Shared TypeScript types
    ├── media.ts
    ├── library.ts
    └── index.ts
```

## Component Hierarchy

### Level 1: Primitive Components (ui/)
**Purpose:** Atomic, reusable UI elements with no business logic

**Characteristics:**
- Pure presentation
- Highly configurable via props
- No API calls or business logic
- Fully documented with Storybook (future)
- 100% test coverage goal

**Examples:**
```tsx
<Button variant="primary" size="md" />
<Input label="Name" error="Required" />
<Card variant="elevated" />
```

### Level 2: Form Components (form/)
**Purpose:** Specialized form inputs and validation

**Characteristics:**
- Built on top of ui/ primitives
- Form-specific behavior (validation, error handling)
- Integration with form libraries (React Hook Form)

**Examples:**
```tsx
<FormField name="libraryName" label="Library Name" required />
<SearchInput onSearch={handleSearch} debounce={300} />
```

### Level 3: Layout Components (layout/)
**Purpose:** Page structure and responsive layouts

**Characteristics:**
- Composition patterns
- Responsive by default
- CSS Grid/Flexbox utilities

**Examples:**
```tsx
<Container maxWidth="xl">
  <Grid cols={{ sm: 1, md: 2, lg: 3 }}>
    {items.map(item => <MediaCard key={item.id} {...item} />)}
  </Grid>
</Container>
```

### Level 4: Domain Components (media/, library/)
**Purpose:** Feature-specific, business logic components

**Characteristics:**
- Domain knowledge embedded
- May include API calls
- Composed of lower-level components

**Examples:**
```tsx
<MediaCard
  media={media}
  onPlay={handlePlay}
  showProgress
  showActions
/>

<LibraryForm
  onSubmit={handleSubmit}
  initialData={library}
  mode="edit"
/>
```

### Level 5: Common Components (common/)
**Purpose:** Shared patterns across features

**Characteristics:**
- Reusable across different domains
- Common UI patterns
- Smart defaults

**Examples:**
```tsx
<EmptyState
  icon="📚"
  title="No libraries yet"
  description="Add a library to get started"
  action={<Button onClick={handleAdd}>Add Library</Button>}
/>

<PageHeader
  title="Libraries"
  description="Manage your media libraries"
  actions={<Button>Add Library</Button>}
/>
```

## Component Design Patterns

### 1. Composition Pattern

```tsx
// Bad: Monolithic component
<MediaCard
  showThumbnail
  showTitle
  showYear
  showProgress
  showActions
  thumbnailSize="large"
  actionsPosition="bottom"
/>

// Good: Composable components
<MediaCard>
  <MediaCard.Thumbnail size="large" />
  <MediaCard.Content>
    <MediaCard.Title>{title}</MediaCard.Title>
    <MediaCard.Meta year={year} />
    <MediaCard.Progress value={progress} />
  </MediaCard.Content>
  <MediaCard.Actions>
    <Button size="sm">Play</Button>
    <Button size="sm" variant="ghost">Info</Button>
  </MediaCard.Actions>
</MediaCard>
```

### 2. Compound Components Pattern

```tsx
// FormField compound component
<FormField>
  <FormField.Label>Library Name</FormField.Label>
  <FormField.Input placeholder="My Movies" />
  <FormField.Helper>Choose a descriptive name</FormField.Helper>
  <FormField.Error>{errors.name}</FormField.Error>
</FormField>
```

### 3. Render Props Pattern

```tsx
// Flexible filtering
<FilterPanel>
  {({ filters, setFilter }) => (
    <>
      <SearchInput
        value={filters.search}
        onChange={(value) => setFilter('search', value)}
      />
      <Select
        value={filters.library}
        onChange={(value) => setFilter('library', value)}
        options={libraries}
      />
    </>
  )}
</FilterPanel>
```

### 4. Polymorphic Components

```tsx
// Button that can render as different elements
<Button as="a" href="/libraries">
  View Libraries
</Button>

<Button as={Link} to="/media">
  Browse Media
</Button>
```

## Folder-by-Component Structure

Each component gets its own folder with:

```
Button/
├── Button.tsx           # Main component
├── Button.test.tsx      # Unit tests
├── Button.stories.tsx   # Storybook stories (future)
├── Button.types.ts      # TypeScript types
├── useButton.ts         # Component-specific hook (if needed)
└── index.ts            # Exports
```

**Benefits:**
- Co-location of related files
- Easy to find tests and types
- Clear ownership
- Better for code splitting

## File Naming Conventions

```tsx
// Component files
Button.tsx              // PascalCase for components
useButton.ts           // camelCase with 'use' prefix for hooks
Button.test.tsx        // .test.tsx for tests
Button.stories.tsx     // .stories.tsx for Storybook
button.utils.ts        // camelCase for utilities

// Index files
index.ts               // Barrel exports

// Type files
Button.types.ts        // Component-specific types
media.ts               // Domain types (in types/ folder)
```

## TypeScript Patterns

### Shared Props Pattern

```tsx
// base.types.ts
export interface BaseComponentProps {
  className?: string
  children?: React.ReactNode
  testId?: string
}

// Button.types.ts
import { BaseComponentProps } from '../base.types'

export interface ButtonProps extends BaseComponentProps {
  variant?: 'primary' | 'secondary'
  size?: 'sm' | 'md' | 'lg'
  isLoading?: boolean
}
```

### Polymorphic Component Types

```tsx
type AsProp<C extends React.ElementType> = {
  as?: C
}

type PropsToOmit<C extends React.ElementType, P> = keyof (AsProp<C> & P)

type PolymorphicComponentProp<
  C extends React.ElementType,
  Props = {}
> = React.PropsWithChildren<Props & AsProp<C>> &
  Omit<React.ComponentPropsWithoutRef<C>, PropsToOmit<C, Props>>

// Usage
export const Button = <C extends React.ElementType = 'button'>({
  as,
  children,
  ...props
}: PolymorphicComponentProp<C, ButtonProps>) => {
  const Component = as || 'button'
  return <Component {...props}>{children}</Component>
}
```

## Hook Organization

### Custom Hooks Structure

```tsx
// hooks/useMediaFilters.ts
export function useMediaFilters(initialFilters?: MediaFilters) {
  const [filters, setFilters] = useState(initialFilters || {})

  const setFilter = useCallback((key: string, value: any) => {
    setFilters(prev => ({ ...prev, [key]: value }))
  }, [])

  const resetFilters = useCallback(() => {
    setFilters(initialFilters || {})
  }, [initialFilters])

  const filteredItems = useMemo(() => {
    // Filtering logic
  }, [filters])

  return { filters, setFilter, resetFilters, filteredItems }
}

// Usage in component
function MediaPage() {
  const { filters, setFilter, filteredItems } = useMediaFilters()

  return (
    <FilterPanel>
      <SearchInput
        value={filters.search}
        onChange={(v) => setFilter('search', v)}
      />
      <MediaGrid items={filteredItems} />
    </FilterPanel>
  )
}
```

## Utility Functions

### Formatters

```tsx
// lib/utils/format.ts
export const formatters = {
  fileSize: (bytes: number): string => {
    // Implementation
  },

  duration: (seconds: number): string => {
    // Implementation
  },

  date: (date: Date, format: string): string => {
    // Implementation
  },

  number: (num: number, locale: string = 'en-US'): string => {
    // Implementation
  }
}

// Usage
import { formatters } from '@/lib/utils'

<p>Size: {formatters.fileSize(media.file_size)}</p>
<p>Duration: {formatters.duration(media.duration)}</p>
```

### Validators

```tsx
// lib/utils/validation.ts
export const validators = {
  isValidPath: (path: string): boolean => {
    // Implementation
  },

  isValidLibraryName: (name: string): boolean => {
    // Implementation
  },

  isMediaFile: (filename: string): boolean => {
    // Implementation
  }
}
```

## Theme System (Future Enhancement)

```tsx
// lib/theme/
├── colors.ts
├── typography.ts
├── spacing.ts
├── breakpoints.ts
└── index.ts

// colors.ts
export const colors = {
  primary: {
    50: '#eff6ff',
    100: '#dbeafe',
    // ...
    900: '#1e3a8a',
  },
  // ...
}

// Usage with Tailwind CSS variables
<Button className="bg-primary-600 hover:bg-primary-700" />
```

## Testing Strategy

### Component Tests

```tsx
// Button.test.tsx
import { render, screen, userEvent } from '@testing-library/react'
import { Button } from './Button'

describe('Button', () => {
  it('renders children', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByText('Click me')).toBeInTheDocument()
  })

  it('calls onClick when clicked', async () => {
    const handleClick = jest.fn()
    render(<Button onClick={handleClick}>Click me</Button>)
    await userEvent.click(screen.getByRole('button'))
    expect(handleClick).toHaveBeenCalledTimes(1)
  })

  it('shows loading state', () => {
    render(<Button isLoading>Submit</Button>)
    expect(screen.getByRole('button')).toBeDisabled()
    expect(screen.getByText('Submit')).toBeInTheDocument()
  })
})
```

### Hook Tests

```tsx
// useMediaFilters.test.ts
import { renderHook, act } from '@testing-library/react'
import { useMediaFilters } from './useMediaFilters'

describe('useMediaFilters', () => {
  it('initializes with default filters', () => {
    const { result } = renderHook(() => useMediaFilters())
    expect(result.current.filters).toEqual({})
  })

  it('updates filter value', () => {
    const { result } = renderHook(() => useMediaFilters())
    act(() => {
      result.current.setFilter('search', 'movie')
    })
    expect(result.current.filters.search).toBe('movie')
  })
})
```

## Migration Path

### Phase 1: Current State ✅
- Basic ui/ components
- Single-file components
- Essential props

### Phase 2: Immediate Improvements (Next Sprint)
- [ ] Move to folder-based structure
- [ ] Add media/ domain components
- [ ] Create common/ components
- [ ] Extract formatters and validators

### Phase 3: Enhancement (Future)
- [ ] Add Storybook
- [ ] Implement compound components
- [ ] Add comprehensive tests
- [ ] Create theme system

### Phase 4: Maturity (Long-term)
- [ ] Design tokens
- [ ] Component library documentation site
- [ ] Accessibility audit
- [ ] Performance optimization

## Best Practices Summary

1. **Single Responsibility**: Each component does one thing well
2. **Composition over Configuration**: Build complex UIs from simple pieces
3. **Progressive Enhancement**: Start simple, add features as needed
4. **Type Safety**: Full TypeScript coverage
5. **Accessibility**: ARIA attributes, keyboard navigation
6. **Performance**: Memoization, lazy loading, code splitting
7. **Testability**: Easy to test in isolation
8. **Documentation**: Clear props, examples, and use cases
9. **Consistency**: Follow established patterns
10. **Maintainability**: Easy to understand and modify

## References

- [React Component Patterns](https://www.patterns.dev/posts/react-component-patterns)
- [Compound Components](https://kentcdodds.com/blog/compound-components-with-react-hooks)
- [Polymorphic Components](https://www.benmvp.com/blog/polymorphic-react-components-typescript/)
- [Testing Library Best Practices](https://kentcdodds.com/blog/common-mistakes-with-react-testing-library)

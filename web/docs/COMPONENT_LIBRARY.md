# ViewRA Component Library

A comprehensive, reusable UI component library built with React, TypeScript, and Tailwind CSS.

## Design Principles

1. **Consistency**: All components follow the same design patterns and styling conventions
2. **Reusability**: Components are designed to be used across the entire application
3. **Type Safety**: Full TypeScript support with proper type definitions
4. **Accessibility**: Components follow WAI-ARIA guidelines where applicable
5. **Customization**: All components accept className prop for easy customization

## Components

### Button

A versatile button component with multiple variants and sizes.

**Props:**
- `variant`: 'primary' | 'secondary' | 'danger' | 'success' | 'ghost' (default: 'primary')
- `size`: 'sm' | 'md' | 'lg' (default: 'md')
- `isLoading`: boolean - Shows loading spinner
- All standard HTML button attributes

**Example:**
```tsx
import { Button } from '@/components/ui'

<Button variant="primary" size="md" onClick={handleClick}>
  Click Me
</Button>

<Button variant="danger" isLoading={isDeleting}>
  Delete
</Button>
```

### Input

A form input component with label, error, and helper text support.

**Props:**
- `label`: string - Input label
- `error`: string - Error message to display
- `helperText`: string - Helper text below input
- All standard HTML input attributes

**Example:**
```tsx
import { Input } from '@/components/ui'

<Input
  label="Library Name"
  value={name}
  onChange={(e) => setName(e.target.value)}
  placeholder="My Movies"
  error={errors.name}
  helperText="Choose a descriptive name"
  required
/>
```

### Select

A dropdown select component with label and error support.

**Props:**
- `label`: string - Select label
- `options`: SelectOption[] - Array of {value, label} objects
- `error`: string - Error message to display
- `helperText`: string - Helper text below select
- All standard HTML select attributes

**Example:**
```tsx
import { Select } from '@/components/ui'

<Select
  label="Library Type"
  value={type}
  onChange={(e) => setType(e.target.value)}
  options={[
    { value: 'movie', label: 'Movies' },
    { value: 'tv', label: 'TV Shows' },
    { value: 'music', label: 'Music' },
  ]}
/>
```

### Card

A flexible container component with header, content, and footer sections.

**Props:**
- `variant`: 'default' | 'bordered' | 'elevated' (default: 'default')
- All standard HTML div attributes

**Sub-components:**
- `CardHeader` - Header section with border
- `CardContent` - Main content area
- `CardFooter` - Footer section with border

**Example:**
```tsx
import { Card, CardHeader, CardContent, CardFooter } from '@/components/ui'

<Card>
  <CardHeader>
    <h2>Card Title</h2>
  </CardHeader>
  <CardContent>
    <p>Card content goes here</p>
  </CardContent>
  <CardFooter>
    <Button>Action</Button>
  </CardFooter>
</Card>
```

### Modal

A modal dialog component with backdrop and keyboard support (Escape to close).

**Props:**
- `isOpen`: boolean - Controls modal visibility
- `onClose`: () => void - Close handler
- `title`: string - Modal title
- `size`: 'sm' | 'md' | 'lg' | 'xl' (default: 'md')
- All standard HTML div attributes

**Sub-components:**
- `ModalContent` - Main content area
- `ModalFooter` - Footer section with actions

**Example:**
```tsx
import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'

<Modal
  isOpen={showModal}
  onClose={() => setShowModal(false)}
  title="Confirm Action"
  size="md"
>
  <ModalContent>
    <p>Are you sure you want to proceed?</p>
  </ModalContent>
  <ModalFooter>
    <Button variant="secondary" onClick={() => setShowModal(false)}>
      Cancel
    </Button>
    <Button variant="danger" onClick={handleConfirm}>
      Confirm
    </Button>
  </ModalFooter>
</Modal>
```

### Alert

An alert component for displaying messages with different severity levels.

**Props:**
- `variant`: 'info' | 'success' | 'warning' | 'error' (default: 'info')
- `title`: string - Optional alert title
- All standard HTML div attributes

**Example:**
```tsx
import { Alert } from '@/components/ui'

<Alert variant="error" title="Error">
  Failed to load data. Please try again.
</Alert>

<Alert variant="success">
  Library created successfully!
</Alert>
```

### Loading

A loading spinner component with optional text.

**Props:**
- `size`: 'sm' | 'md' | 'lg' (default: 'md')
- `text`: string - Optional loading text
- All standard HTML div attributes

**Example:**
```tsx
import { Loading } from '@/components/ui'

<Loading size="lg" text="Loading libraries..." />
```

## Utilities

### cn()

A utility function for merging Tailwind CSS classes with proper precedence.

**Location:** `@/lib/utils`

**Usage:**
```tsx
import { cn } from '@/lib/utils'

<div className={cn('base-class', isActive && 'active-class', className)} />
```

## Styling Guide

All components use Tailwind CSS utility classes with the following color scheme:

- **Primary**: Blue (600/700)
- **Secondary**: Gray (100/200)
- **Success**: Green (600/700)
- **Danger**: Red (600/700)
- **Warning**: Yellow (600/700)
- **Info**: Blue (600/700)

### Spacing

- Small: `px-3 py-1`
- Medium: `px-4 py-2`
- Large: `px-6 py-3`

### Borders

- Radius: `rounded` or `rounded-md` or `rounded-lg`
- Color: `border-gray-200` or `border-gray-300`

### Focus States

All interactive components include focus states with ring:
- `focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2`

## Best Practices

1. **Always use components from the library** instead of creating custom styled elements
2. **Extend with className** when you need custom styling
3. **Use semantic variants** (primary, danger, success) instead of custom colors
4. **Provide labels** for all form inputs for accessibility
5. **Show loading states** during async operations
6. **Display errors** inline for better UX
7. **Use proper sizes** based on context (sm for compact, md for normal, lg for prominent)

## Adding New Components

When adding a new component to the library:

1. Create the component in `web/src/components/ui/ComponentName.tsx`
2. Follow the existing patterns (forwardRef, displayName, TypeScript props)
3. Use Tailwind CSS for styling
4. Support customization via className prop
5. Export from `web/src/components/ui/index.ts`
6. Update this documentation with examples

## Dependencies

- `clsx`: Conditionally join classNames together
- `tailwind-merge`: Merge Tailwind CSS classes without style conflicts
- `@tanstack/react-router`: For navigation and routing
- `@tanstack/react-query`: For data fetching and caching

## Migration Guide

### Migrating from Raw HTML to Components

**Before:**
```tsx
<button
  onClick={handleClick}
  className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
  disabled={isLoading}
>
  {isLoading ? 'Loading...' : 'Submit'}
</button>
```

**After:**
```tsx
<Button onClick={handleClick} isLoading={isLoading}>
  Submit
</Button>
```

**Before:**
```tsx
<div className="bg-white rounded-lg shadow p-4">
  <div className="border-b pb-2 mb-2">
    <h2>Title</h2>
  </div>
  <div>Content</div>
</div>
```

**After:**
```tsx
<Card>
  <CardHeader>
    <h2>Title</h2>
  </CardHeader>
  <CardContent>
    Content
  </CardContent>
</Card>
```

## Performance Considerations

- All components are memoized using `forwardRef`
- Components are tree-shakeable (import only what you need)
- Tailwind classes are purged in production
- No runtime CSS-in-JS overhead

## Browser Support

The component library supports all modern browsers:
- Chrome/Edge (latest 2 versions)
- Firefox (latest 2 versions)
- Safari (latest 2 versions)

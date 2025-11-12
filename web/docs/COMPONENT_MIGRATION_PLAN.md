# Component Library Migration Plan

## Current State Analysis

### What We Have ✅
```
web/src/components/ui/
├── Alert.tsx
├── Button.tsx
├── Card.tsx
├── Input.tsx
├── Loading.tsx
├── Modal.tsx
├── Select.tsx
└── index.ts
```

**Strengths:**
- Working implementations
- Type-safe with TypeScript
- Linted and formatted
- Used in production pages

**Limitations:**
- All components in single files
- No tests
- No domain-specific components
- Limited reusability patterns
- Missing common utilities

## Migration Strategy

### Phase 1: Restructure Existing (Week 1)

**Goal:** Move to folder-based structure without changing functionality

#### Step 1.1: Convert to Folder Structure

```bash
# Current
ui/Button.tsx

# Target
ui/Button/
├── Button.tsx
├── index.ts
```

**Tasks:**
- [ ] Create folders for each existing component
- [ ] Move component files into folders
- [ ] Create index.ts barrel exports
- [ ] Update all imports in pages
- [ ] Verify no breaking changes

**Estimated Time:** 2 hours

#### Step 1.2: Extract Utilities

```bash
# Create
lib/utils/
├── format.ts      # formatFileSize, formatDuration
├── validation.ts  # path validation, etc.
└── index.ts
```

**Extract from current code:**
- `formatFileSize()` from media.tsx → `lib/utils/format.ts`
- Path validation logic → `lib/utils/validation.ts`

**Estimated Time:** 1 hour

### Phase 2: Add Domain Components (Week 1-2)

**Goal:** Create media and library specific components

#### Step 2.1: Create MediaCard Component

Extract from `routes/_layout/media.tsx`:

```tsx
// Before (in page)
function MediaCard({ media }: { media: MediaResponse }) {
  const [showDetails, setShowDetails] = useState(false)
  return (
    <>
      <div className="...">
        {/* Thumbnail */}
        {/* Title */}
      </div>
      {showDetails && <MediaDetailsModal ... />}
    </>
  )
}

// After (in components/media/MediaCard/)
export function MediaCard({ media, onPlay }: MediaCardProps) {
  // Extracted component with clean API
}
```

**Files to create:**
```
components/media/
├── MediaCard/
│   ├── MediaCard.tsx
│   ├── MediaCardSkeleton.tsx
│   ├── index.ts
├── MediaGrid/
│   ├── MediaGrid.tsx
│   ├── index.ts
└── index.ts
```

**Estimated Time:** 3 hours

#### Step 2.2: Create LibraryCard Component

Extract from `routes/_layout/libraries.tsx`:

```
components/library/
├── LibraryCard/
│   ├── LibraryCard.tsx
│   ├── index.ts
├── LibraryForm/
│   ├── LibraryForm.tsx
│   ├── index.ts
└── index.ts
```

**Estimated Time:** 2 hours

### Phase 3: Add Common Components (Week 2)

**Goal:** Extract repeated patterns

#### Step 3.1: Create EmptyState Component

Extract empty state pattern used in multiple places:

```tsx
// components/common/EmptyState/EmptyState.tsx
export function EmptyState({
  icon,
  title,
  description,
  action,
}: EmptyStateProps) {
  return (
    <div className="text-center py-12">
      {icon && <div className="text-6xl mb-4">{icon}</div>}
      <h3 className="text-lg font-semibold text-gray-900 mb-2">{title}</h3>
      {description && <p className="text-gray-600 mb-4">{description}</p>}
      {action}
    </div>
  )
}

// Usage
<EmptyState
  icon="📚"
  title="No libraries yet"
  description="Add a library to get started"
  action={<Button onClick={handleAdd}>Add Library</Button>}
/>
```

**Estimated Time:** 2 hours

#### Step 3.2: Create PageHeader Component

```tsx
// components/common/PageHeader/PageHeader.tsx
export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="mb-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-3xl font-bold mb-2">{title}</h1>
          {description && <p className="text-gray-600">{description}</p>}
        </div>
        {actions && <div className="flex gap-2">{actions}</div>}
      </div>
    </div>
  )
}
```

**Estimated Time:** 1 hour

### Phase 4: Advanced Patterns (Week 3)

**Goal:** Implement composition patterns

#### Step 4.1: Compound Components for Card

```tsx
// Before
<Card>
  <CardHeader>Title</CardHeader>
  <CardContent>Content</CardContent>
</Card>

// After (compound pattern)
<Card>
  <Card.Header>Title</Card.Header>
  <Card.Content>Content</Card.Content>
  <Card.Footer>Actions</Card.Footer>
</Card>
```

**Implementation:**
```tsx
// components/ui/Card/Card.tsx
export const Card = ({ children, ...props }: CardProps) => {
  return <div {...props}>{children}</div>
}

Card.Header = CardHeader
Card.Content = CardContent
Card.Footer = CardFooter
```

**Estimated Time:** 3 hours

#### Step 4.2: Polymorphic Button

Add support for rendering Button as different elements:

```tsx
<Button as="a" href="/libraries">Link Button</Button>
<Button as={Link} to="/media">Router Link</Button>
```

**Estimated Time:** 2 hours

### Phase 5: Testing (Week 3-4)

**Goal:** Add comprehensive tests

#### Step 5.1: Setup Testing Infrastructure

```bash
npm install --save-dev @testing-library/react @testing-library/jest-dom @testing-library/user-event vitest jsdom
```

**Create test setup:**
```tsx
// src/test/setup.ts
import '@testing-library/jest-dom'
```

**Estimated Time:** 1 hour

#### Step 5.2: Write Component Tests

Priority order:
1. ui/ components (Button, Input, Select, etc.)
2. common/ components (EmptyState, PageHeader)
3. domain/ components (MediaCard, LibraryCard)

**Test coverage goal:** 80%+

**Estimated Time:** 8 hours

### Phase 6: Documentation (Week 4)

**Goal:** Document all components

#### Step 6.1: Add JSDoc Comments

```tsx
/**
 * A flexible button component with multiple variants and sizes.
 *
 * @example
 * ```tsx
 * <Button variant="primary" size="md" onClick={handleClick}>
 *   Click Me
 * </Button>
 * ```
 */
export function Button({ variant = 'primary', ... }: ButtonProps) {
  // ...
}
```

**Estimated Time:** 3 hours

#### Step 6.2: Update Component Library Docs

- Add new components to COMPONENT_LIBRARY.md
- Include usage examples
- Document props and variants

**Estimated Time:** 2 hours

## Detailed Migration Steps

### Converting Button to Folder Structure

**Step 1:** Create folder
```bash
mkdir -p web/src/components/ui/Button
```

**Step 2:** Move file
```bash
mv web/src/components/ui/Button.tsx web/src/components/ui/Button/Button.tsx
```

**Step 3:** Create index.ts
```tsx
// web/src/components/ui/Button/index.ts
export { Button, type ButtonProps } from './Button'
```

**Step 4:** Update main index.ts
```tsx
// web/src/components/ui/index.ts
export { Button, type ButtonProps } from './Button'
// ... other exports
```

**Step 5:** Verify imports still work
```bash
npm run lint
npm run dev
```

### Creating MediaCard Component

**Step 1:** Create structure
```bash
mkdir -p web/src/components/media/MediaCard
```

**Step 2:** Extract component
```tsx
// web/src/components/media/MediaCard/MediaCard.tsx
import { useState } from 'react'
import { MediaDetailsModal } from './MediaDetailsModal'
import type { GithubComViewraViewraInternalApplicationMediaMediaResponse } from '@/lib/api'

export interface MediaCardProps {
  media: GithubComViewraViewraInternalApplicationMediaMediaResponse
  onClick?: () => void
}

export function MediaCard({ media, onClick }: MediaCardProps) {
  const [showDetails, setShowDetails] = useState(false)

  const handleClick = () => {
    setShowDetails(true)
    onClick?.()
  }

  return (
    <>
      <div
        className="bg-white rounded-lg shadow overflow-hidden cursor-pointer hover:shadow-lg transition-shadow"
        onClick={handleClick}
      >
        <div className="aspect-[2/3] bg-gradient-to-br from-gray-700 to-gray-900 flex items-center justify-center text-white text-4xl">
          🎬
        </div>
        <div className="p-3">
          <h3 className="font-semibold text-sm line-clamp-2 mb-1">{media.title}</h3>
        </div>
      </div>

      {showDetails && (
        <MediaDetailsModal
          media={media}
          onClose={() => setShowDetails(false)}
        />
      )}
    </>
  )
}
```

**Step 3:** Create MediaDetailsModal in same folder
```tsx
// web/src/components/media/MediaCard/MediaDetailsModal.tsx
import { Modal, ModalContent, ModalFooter, Button } from '@/components/ui'
import { formatFileSize } from '@/lib/utils/format'

export function MediaDetailsModal({ media, onClose }) {
  // ... implementation
}
```

**Step 4:** Export from index
```tsx
// web/src/components/media/MediaCard/index.ts
export { MediaCard, type MediaCardProps } from './MediaCard'
```

**Step 5:** Update media page
```tsx
// web/src/routes/_layout/media.tsx
import { MediaCard } from '@/components/media/MediaCard'

// Replace inline MediaCard component with imported one
```

## Success Criteria

### Phase 1 Complete ✅
- [x] All ui/ components in folder structure
- [x] All imports updated and working
- [x] Utilities extracted (formatFileSize, formatDuration, cn)
- [x] No regressions

### Phase 2 Complete ✅
- [x] MediaCard component created (with MediaDetailsModal)
- [x] LibraryCard, LibraryForm components created
- [x] Pages using new components
- [x] Code reduction in route files (196 lines → 106 media, 262 lines → 77 libraries)

### Phase 3 Complete ✅
- [x] EmptyState component created
- [x] PageHeader component created
- [x] Both components reused across media and libraries pages
- [x] Improved user experience with icons and better empty state messaging

### Phase 4 Complete
- [ ] Compound component patterns implemented
- [ ] Polymorphic components working
- [ ] Render props where beneficial

### Phase 5 Complete
- [ ] 80%+ test coverage for ui/ components
- [ ] 60%+ test coverage for domain components
- [ ] All tests passing
- [ ] Tests integrated in CI

### Phase 6 Complete
- [ ] All components documented
- [ ] Usage examples provided
- [ ] Migration guide updated
- [ ] Architecture docs complete

## Rollback Plan

If migration causes issues:

1. **Git branches**: Each phase in separate branch
2. **Feature flags**: Toggle new components via env var
3. **Gradual rollout**: Migrate one page at a time
4. **Monitoring**: Track errors in new components

## Timeline Summary

| Phase | Duration | Dependencies |
|-------|----------|--------------|
| Phase 1 | Week 1 | None |
| Phase 2 | Week 1-2 | Phase 1 |
| Phase 3 | Week 2 | Phase 1 |
| Phase 4 | Week 3 | Phase 1, 2 |
| Phase 5 | Week 3-4 | Phase 1-4 |
| Phase 6 | Week 4 | Phase 1-5 |

**Total Duration:** 4 weeks (can be parallelized)

## Next Actions

**Immediate (This Sprint):**
1. Convert existing ui/ components to folder structure
2. Extract formatFileSize and other utilities
3. Create MediaCard component

**Short-term (Next Sprint):**
4. Add EmptyState and PageHeader
5. Implement compound patterns for Card
6. Start adding tests

**Long-term (Future Sprints):**
7. Complete test coverage
8. Add Storybook
9. Create theme system

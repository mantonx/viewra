# Semantic Search Plugin Coupling Analysis

## Executive Summary

**Critical Issue**: The frontend implementation has created tight coupling between the core app and the semantic search plugin, violating the plugin architecture. The backend correctly implements loose coupling, but the frontend does not follow the same pattern.

## Backend Architecture ✅ CORRECT

The backend properly implements plugin-optional architecture:

### Core Domain Has NO Plugin Knowledge
```go
// internal/domain/search/types.go
type Response struct {
    Results  []Result
    Total    int
    Fallback bool  // Indicates if fallback used
}
```
- Core domain defines generic search types
- NO mention of semantic search, intent chips, or plugin concepts
- Pure business logic

### Application Layer Defines Interface
```go
// internal/application/movies/semantic_search.go
type SemanticSearchProvider interface {
    Search(ctx, query, entityTypes, limit) ([]SemanticSearchResult, error)
    IsAvailable() bool
}
```
- Application defines **abstract interface**
- Core app code depends on interface, not implementation
- Plugin provides implementation

### Infrastructure Detects and Routes
```go
// internal/api/handlers/search.go
func (h *SearchHandler) Search(c *gin.Context) {
    // Check if semantic search plugin is available
    if mapping := h.capabilityRegistry.Resolve("semantic_search"); mapping != nil {
        // Proxy to plugin
    } else {
        // Use fallback search
    }
}
```
- Handler checks plugin availability at runtime
- Routes to plugin OR fallback transparently
- Caller doesn't know which was used

**Result**: Backend can run with or without the plugin. Zero coupling.

---

## Frontend Architecture ❌ INCORRECT

The frontend directly imports and depends on plugin code:

### Problem 1: Core Components Import Plugin Types

```typescript
// web/src/components/common/MediaBrowsePage/MediaBrowsePage.types.ts
import type { IntentChip as IntentChipType } from '@/components/home/widgets'

export interface MediaBrowsePageProps {
  intentChips?: IntentChipType[]  // Plugin-specific type!
  onRemoveIntentChip?: (chipId: string) => void
  onRefineIntentChip?: (chipId: string, refinement: string) => void
}
```

**Issue**: `MediaBrowsePage` is a **core component** used by movies, TV shows, music. It now has a hard dependency on a plugin-specific type.

**Impact**:
- Type error if plugin files missing
- Violates separation of concerns
- Makes component less reusable

### Problem 2: Routes Directly Call Plugin Hooks

```typescript
// web/src/routes/_layout/movies.index.tsx
import { useSemanticSearch, useSemanticSearchAvailable } from '@/lib/hooks'

const { data: semanticData, ... } = useSemanticSearch({
  query: debouncedSearch,
  entity_types: ['movie'],
  limit: 100,
})
```

**Issue**: Movie route directly imports and calls plugin hooks.

**Impact**:
- Route depends on plugin code existing
- No abstraction layer
- Impossible to lazy-load plugin
- Violates dependency inversion principle

### Problem 3: Plugin Components in Wrong Location

```
web/src/components/home/widgets/IntentChip.tsx  ← Wrong!
```

**Issue**: Intent chips are NOT home widgets. They're search features.

**Impact**:
- Confusing import paths
- Breaks mental model of codebase
- Implies intent chips are core, not plugin features

### Problem 4: Type Duplication

**Backend**:
```go
// plugins/semantic-search/internal/types.go
type IntentChip struct {
    ID          string
    Type        string
    Value       string
    Display     string
    Removable   bool
    Refinements []string
    Role        string
}
```

**Frontend**:
```typescript
// web/src/components/home/widgets/IntentChip.tsx
export interface IntentChip {
  id: string
  type: string
  value: string
  display: string
  removable: boolean
  refinements?: string[]
  role?: string
}
```

**Issue**: Type defined in TWO places. Will drift over time.

**Correct approach**: Generate frontend types from backend (or use shared schema).

### Problem 5: Core Hooks Export Plugin Hooks

```typescript
// web/src/lib/hooks/index.ts (CORE hooks barrel)
export { useSemanticSearch, useSemanticSearchAvailable, useSimilarItems } from './useSemanticSearch'
```

**Issue**: The main hooks barrel exports plugin-specific hooks as if they're core features.

**Impact**:
- Blurs line between core and plugin
- Suggests semantic search is always available
- Breaks tree-shaking (plugin code always bundled)

---

## Coupling Score

| Layer | Backend | Frontend |
|-------|---------|----------|
| Domain | ✅ 0% coupled | ❌ N/A (no domain layer) |
| Application | ✅ Interface-based | ❌ Direct imports |
| Infrastructure | ✅ Runtime detection | ❌ Build-time dependency |
| UI/Routes | ✅ N/A | ❌ 100% coupled |

**Overall**: Backend = 0% coupling, Frontend = ~80% coupling

---

## Why This Matters

### 1. **Plugin Can't Be Optional**
```bash
# If you delete the plugin files:
rm -rf web/src/lib/api/semanticSearch.ts
rm -rf web/src/lib/hooks/useSemanticSearch.ts

# Frontend build fails:
Error: Cannot find module '@/lib/hooks/useSemanticSearch'
  in web/src/routes/_layout/movies.index.tsx
```

The app **won't compile** without the plugin code.

### 2. **Can't Distribute Plugin Separately**
If semantic search was a separate npm package:
```json
{
  "optionalDependencies": {
    "@viewra/plugin-semantic-search": "^1.0.0"
  }
}
```

The current code would fail because core components import from it directly.

### 3. **Violates Open/Closed Principle**
Adding plugin features requires modifying core components:
- Modified `MediaBrowsePage` types
- Modified movies route logic
- Modified common components barrel exports

Core should be **closed for modification**, extended via plugins.

### 4. **Testing Nightmare**
To test `MediaBrowsePage`, you now need:
- Mock `IntentChip` type
- Mock semantic search hooks
- Handle plugin availability states

Before, it was a simple component with no external dependencies.

---

## Correct Architecture (How Backend Does It)

### Backend Pattern (Dependency Inversion):

```
┌────────────────────────────┐
│  Core Application          │
│  - Depends on interface    │
│  - SemanticSearchProvider  │
└──────────┬─────────────────┘
           │ depends on
           ↓
┌────────────────────────────┐
│  Abstract Interface        │
│  type SemanticSearch..{}   │
└──────────┬─────────────────┘
           ↑ implements
           │
┌────────────────────────────┐
│  Plugin (optional)         │
│  Provides implementation   │
└────────────────────────────┘
```

Core depends on abstraction, plugin implements it. Perfect.

### Frontend Anti-Pattern (Direct Dependency):

```
┌────────────────────────────┐
│  Core Components           │
│  import { useSemanticS...} │
└──────────┬─────────────────┘
           │ directly imports
           ↓
┌────────────────────────────┐
│  Plugin Code               │
│  useSemanticSearch()       │
└────────────────────────────┘
```

Core directly depends on plugin. Coupled.

---

## How to Fix

### Option A: Follow Backend Pattern

Create abstract interface:

```typescript
// web/src/lib/search/types.ts (CORE)
export interface SearchEnhancement {
  chips?: React.ReactNode
  reasons?: React.ReactNode
  recovery?: React.ReactNode
}

export interface SearchProvider {
  search(query: string, options: SearchOptions): Promise<{
    results: SearchResult[]
    enhancement?: SearchEnhancement
  }>
  isAvailable(): boolean
}
```

Core components accept `SearchEnhancement`:

```typescript
// MediaBrowsePage.types.ts
export interface MediaBrowsePageProps {
  searchEnhancement?: React.ReactNode  // Opaque!
}
```

Plugin provides implementation:

```typescript
// features/semantic-search/provider.ts
export class SemanticSearchProvider implements SearchProvider {
  async search(query, options) {
    const data = await api.search(...)
    return {
      results: data.results,
      enhancement: <IntentChipsBar chips={data.intent_chips} />
    }
  }
}
```

Movies page uses abstraction:

```typescript
const provider = getSearchProvider() // Returns available provider
const { results, enhancement } = await provider.search(query)

<MediaBrowsePage
  data={results}
  searchEnhancement={enhancement}
/>
```

### Option B: Feature Detection at Runtime

```typescript
// web/src/lib/features/index.ts
export function useFeatures() {
  const semanticSearch = useMemo(() => {
    try {
      return require('@/features/semantic-search')
    } catch {
      return null
    }
  }, [])

  return { semanticSearch }
}
```

Components conditionally use features:

```typescript
const { semanticSearch } = useFeatures()

{semanticSearch?.available && (
  <semanticSearch.IntentChips chips={...} />
)}
```

### Option C: Monorepo with Optional Packages

Move plugin to separate package:

```
packages/
  core/          - Main app
  plugins/
    semantic-search/  - Optional plugin
```

Core never imports from plugins. Plugins extend core via exported hooks/slots.

---

## Recommended Solution

**Implement True Plugin Agnosticism** - Core code has ZERO knowledge of any specific plugins:

### Principle: Core Never Names Plugins

Just like the backend, the core should never mention "semantic search" or any specific plugin name. It only knows:
- "There might be a search plugin"
- "Here's a slot for plugin UI"

### Implementation Steps

1. **Create plugin-agnostic search abstraction**
   ```typescript
   // web/src/lib/search/types.ts - NO PLUGIN NAMES
   export interface SearchProvider {
     search(query: string, options: SearchOptions): Promise<SearchResult>
   }

   export interface SearchResult {
     items: any[]
     enhancement?: ReactNode  // Opaque slot for ANY plugin UI
   }
   ```

2. **Plugin detection via registry (NOT hardcoded names)**
   ```typescript
   // web/src/lib/search/registry.ts
   export function getSearchProvider(): SearchProvider {
     // Dynamically detect available plugins
     const plugins = window.__VIEWRA_PLUGINS__ || {}

     // Use first available search provider
     if (plugins.search) {
       return plugins.search
     }

     // Fallback to built-in
     return builtinSearchProvider
   }
   ```

3. **MediaBrowsePage accepts generic slot**
   ```typescript
   // MediaBrowsePage.types.ts - NO PLUGIN TYPES
   export interface MediaBrowsePageProps {
     searchEnhancement?: ReactNode  // Generic slot
     // NO IntentChip, NO onRemoveIntentChip, NO plugin-specific props
   }
   ```

4. **Movies page uses abstraction**
   ```typescript
   // movies.index.tsx - NO PLUGIN IMPORTS
   import { useSearch } from '@/lib/search'

   const { results, enhancement } = useSearch(query)

   <MediaBrowsePage
     data={results}
     searchEnhancement={enhancement}  // Whatever plugin returned
   />
   ```

5. **Plugin registers itself**
   ```typescript
   // features/semantic-search/index.ts (plugin code, not imported by core)
   import { registerSearchProvider } from '@/lib/search/registry'

   registerSearchProvider({
     search: async (query, options) => {
       const data = await api.search(query)
       return {
         items: data.results,
         enhancement: <IntentChipsBar chips={data.intent_chips} />
       }
     }
   })
   ```

### Result

**Core code knows nothing about:**
- ❌ Semantic search
- ❌ Intent chips
- ❌ Any plugin names
- ❌ Plugin-specific types

**Core code only knows:**
- ✅ "There's a search provider"
- ✅ "It might return enhancement UI"
- ✅ "Display enhancement in this slot"

This exactly mirrors how the backend works.

---

## Migration Checklist

### Phase 1: Create Plugin-Agnostic Abstractions

- [ ] Create `web/src/lib/search/types.ts` (generic SearchProvider interface)
- [ ] Create `web/src/lib/search/registry.ts` (plugin detection, NO hardcoded plugin names)
- [ ] Create `web/src/lib/search/hooks.ts` (useSearch hook using registry)
- [ ] Create `web/src/lib/search/builtin.ts` (fallback search provider)

### Phase 2: Move Plugin to Isolated Feature

- [ ] Create `web/src/features/semantic-search/` directory structure
- [ ] Move `lib/api/semanticSearch.ts` → `features/semantic-search/api/client.ts`
- [ ] Move `lib/hooks/useSemanticSearch.ts` → `features/semantic-search/hooks/`
- [ ] Move `components/home/widgets/IntentChip.tsx` → `features/semantic-search/components/`
- [ ] Move `components/common/IntentChipsBar.tsx` → `features/semantic-search/components/`
- [ ] Create `features/semantic-search/provider.ts` (implements SearchProvider)
- [ ] Create `features/semantic-search/index.ts` (registers with core)

### Phase 3: Remove Plugin Dependencies from Core

- [ ] Update `MediaBrowsePage.types.ts`: Remove IntentChip import
- [ ] Update `MediaBrowsePage.types.ts`: Change to `searchEnhancement?: ReactNode`
- [ ] Update `MediaBrowsePage.tsx`: Remove IntentChipsBar import
- [ ] Update `MediaBrowsePage.tsx`: Render `searchEnhancement` prop
- [ ] Update `components/common/index.ts`: Remove IntentChipsBar export
- [ ] Update `lib/hooks/index.ts`: Remove semantic search exports
- [ ] Update `movies.index.tsx`: Remove ALL semantic search imports
- [ ] Update `movies.index.tsx`: Use `useSearch()` from abstraction layer

### Phase 4: Verification

- [ ] Delete `features/semantic-search/` directory temporarily
- [ ] Run `npx tsc --noEmit` - should compile successfully
- [ ] Restore `features/semantic-search/`
- [ ] Verify app works with plugin
- [ ] Verify app works without plugin
- [ ] Update documentation

### Success Criteria

✅ **Grep test**: Running `grep -r "semantic" web/src/lib web/src/components/common web/src/routes` returns ZERO results (excluding node_modules)

✅ **Grep test**: Running `grep -r "IntentChip" web/src/lib web/src/components/common web/src/routes` returns ZERO results

✅ **Build test**: Can delete `web/src/features/semantic-search` and build succeeds

✅ **Runtime test**: App shows enhanced search when plugin present, basic search when plugin absent

---

## Conclusion

**Backend**: ✅ Exemplary plugin architecture
**Frontend**: ❌ Tightly coupled implementation

The frontend needs refactoring to match the backend's architecture. The current implementation violates the plugin philosophy and makes semantic search a hard dependency instead of an optional enhancement.

**Estimated refactoring effort**: 2-3 hours
**Risk**: Low (isolated to search-related files)
**Benefit**: Proper plugin architecture, matches backend pattern

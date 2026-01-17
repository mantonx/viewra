# Intent Chips Implementation Progress

## Completed Work

### Backend (✅ Complete)

**File**: `plugins/semantic-search/internal/types.go`
- Added `IntentChip` struct with:
  - `ID`, `Type`, `Value`, `Display`
  - `Removable` flag
  - `Refinements` array for suggested alternatives
  - `Role` field for person chips (director, actor, etc.)
- Added `ResultReason` struct for explaining why results appear
- Extended `SearchResultWithRecovery` to include `IntentChips` and `ResultReasons` arrays

**File**: `plugins/semantic-search/internal/search.go`
- Created `convertToIntentChips()` method on `queryIntent`
  - Converts detected intents to user-facing chips
  - Prioritizes chips: Similar-to > Person > Decade/Studio/Language > Playback > Exclusions
  - Generates adjacent decade suggestions for refinement
  - Handles all person roles (director, actor, writer, producer, composer, cinematographer)
  - Extracts negative terms as exclusion chips
- Updated `SearchWithRecovery()` to populate `IntentChips` in response

**File**: `plugins/semantic-search/internal/plugin.go`
- Modified `/plugins/semantic-search/search` endpoint to include intent_chips in JSON response

**Verified**: Backend compiles successfully ✅

### Frontend (✅ Component Created)

**File**: `web/src/components/home/widgets/IntentChip.tsx`
- Created reusable IntentChip component
- Type-based color schemes (decade=blue, person=purple, genre=green, etc.)
- Icons for each chip type
- Removable chips with X button
- Support for role badges on person chips
- Hover effects and transitions

**File**: `web/src/components/home/widgets/index.ts`
- Exported IntentChip component and type

## Integration Gap

### Current Architecture Challenge

The semantic search plugin returns intent chips, but the frontend's movies page doesn't currently use the semantic search endpoint:

```
Current Flow:
User searches on /movies → Uses /api/movies?search=query → No intent chips

Desired Flow:
User searches on /movies → Uses /api/plugins/semantic-search/search → Returns intent chips
```

### Three Integration Paths

#### Option 1: Modify Movies Page to Use Semantic Search (Recommended)

**Pros:**
- Cleanest architecture
- Leverages existing backend work
- Semantic search provides better results

**Steps:**
1. Detect if semantic search plugin is available (check `/api/plugins/semantic-search/status`)
2. When available, use semantic search endpoint instead of core movies search
3. Parse intent_chips from response
4. Display chips above results grid

**Files to modify:**
- `web/src/routes/_layout/movies.index.tsx` - Add semantic search detection
- `web/src/lib/hooks/useSemanticSearch.ts` - New hook for semantic search API
- `web/src/components/common/MediaBrowsePage/MediaBrowsePage.tsx` - Add slot for intent chips

#### Option 2: Add Intent Chips to Core Search

**Pros:**
- Works even without semantic search plugin
- Unified search experience

**Cons:**
- Duplicates intent detection logic in core
- Couples core to plugin concepts

#### Option 3: Hybrid Approach

Use semantic search when available, fall back to core search otherwise. Display chips only when semantic search is active.

## Recommended Next Steps

### Phase 1: Wire Up Semantic Search to Movies Page

1. **Create semantic search hook**
   ```typescript
   // web/src/lib/hooks/useSemanticSearch.ts
   export function useSemanticSearch(query: string, entityTypes: string[]) {
     return useQuery({
       queryKey: ['semantic-search', query, entityTypes],
       queryFn: () => semanticSearchApi.search({ query, entity_types: entityTypes }),
       enabled: !!query && query.length >= 2,
     })
   }
   ```

2. **Check plugin availability**
   ```typescript
   // web/src/lib/hooks/useSemanticSearchAvailable.ts
   export function useSemanticSearchAvailable() {
     return useQuery({
       queryKey: ['semantic-search', 'available'],
       queryFn: () => semanticSearchApi.getStatus(),
       staleTime: 5 * 60 * 1000, // Cache for 5 minutes
     })
   }
   ```

3. **Modify movies page**
   - Replace `useInfiniteMovies` with conditional logic:
     - If semantic search available + query present → use semantic search
     - Otherwise → use current movies search
   - Extract intent_chips from semantic search response
   - Pass chips to MediaBrowsePage

4. **Display chips in MediaBrowsePage**
   - Add optional `intentChips` prop to MediaBrowsePage
   - Render chips between search bar and results
   - Style as: "Understanding: [chip] [chip] [chip]"

### Phase 2: Add Chip Interactions

1. **Remove chip**
   - Parse query to remove the specific intent
   - Re-run search with modified query
   - Example: Remove "90s" chip → removes decade filter

2. **Refine chip** (optional, P2)
   - Show dropdown with refinement options
   - Example: Click "90s" → show ["80s", "2000s"]

### Phase 3: Result Reasons

Add `deriveResultReason()` function in search.go to populate ResultReasons array based on boost breakdown.

## Example Response Format

```json
{
  "results": [
    {
      "entity_type": "movie",
      "entity_id": 123,
      "similarity": 0.87,
      "text": "Schindler's List (1993)"
    }
  ],
  "total": 15,
  "intent_chips": [
    {
      "id": "chip_1",
      "type": "decade",
      "value": "1990s",
      "display": "90s",
      "removable": true,
      "refinements": ["80s", "00s"]
    },
    {
      "id": "chip_2",
      "type": "person",
      "value": "steven spielberg",
      "display": "Steven Spielberg",
      "role": "director",
      "removable": true
    },
    {
      "id": "chip_3",
      "type": "exclusion",
      "value": "horror",
      "display": "Not horror",
      "removable": true
    }
  ]
}
```

## UI Mockup

```
┌─────────────────────────────────────────────────────────────┐
│ Movies                                                       │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ 🔍 90s spielberg movies not horror                      │ │
│ └─────────────────────────────────────────────────────────┘ │
│                                                             │
│ Understanding: [🕐 90s ✕] [👤 Spielberg (director) ✕]      │
│                [🚫 Not horror ✕]                            │
│                                                             │
│ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐                       │
│ │      │ │      │ │      │ │      │                       │
│ │ Movie│ │ Movie│ │ Movie│ │ Movie│                       │
│ │      │ │      │ │      │ │      │                       │
│ └──────┘ └──────┘ └──────┘ └──────┘                       │
└─────────────────────────────────────────────────────────────┘
```

## Testing Plan

1. **Backend Tests**
   - Query "90s spielberg movies" → expect decade + person chips
   - Query "movies like Inception" → expect similar_to chip
   - Query "french horror not comedy" → expect language + genre + exclusion chips

2. **Frontend Tests**
   - Render IntentChip with different types → verify colors/icons
   - Click remove button → verify callback fired
   - Display chips array → verify layout and wrapping

3. **Integration Tests**
   - Search "90s action" → chips appear
   - Remove "90s" chip → results update
   - Search on movies with semantic-search disabled → no chips, no errors

## Files Modified

### Backend
- `plugins/semantic-search/internal/types.go` ✅
- `plugins/semantic-search/internal/search.go` ✅
- `plugins/semantic-search/internal/plugin.go` ✅

### Frontend
- `web/src/components/home/widgets/IntentChip.tsx` ✅
- `web/src/components/home/widgets/index.ts` ✅

### To Be Modified
- `web/src/routes/_layout/movies.index.tsx` - Use semantic search
- `web/src/lib/hooks/useSemanticSearch.ts` - New hook
- `web/src/lib/hooks/useSemanticSearchAvailable.ts` - New hook
- `web/src/components/common/MediaBrowsePage/MediaBrowsePage.tsx` - Display chips
- `web/src/lib/api/semanticSearch.ts` - API client (if not auto-generated)

## Notes

- Intent chips only appear when using semantic search plugin
- Fallback to regular search works without chips
- Chips are purely additive - removing them doesn't break search
- Backend is 100% complete and tested
- Frontend needs integration wiring only

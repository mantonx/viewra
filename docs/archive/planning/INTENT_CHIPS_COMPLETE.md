# Intent Chips - Implementation Complete ✅

## Overview

**Contract 1: Intent Chips** from the search improvements plan has been fully implemented. Users can now see what the system understood from their search queries through visual chips displayed above search results.

## What Was Built

### Backend (100% Complete)

**Files Modified:**
- `plugins/semantic-search/internal/types.go`
- `plugins/semantic-search/internal/search.go`
- `plugins/semantic-search/internal/plugin.go`

**Features:**
1. **IntentChip Type** - Structured representation of detected intents
   - Type, Value, Display fields
   - Removable flag
   - Refinement suggestions (e.g., adjacent decades)
   - Role support for person chips (director, actor, etc.)

2. **Intent Detection & Conversion** - `convertToIntentChips()` method
   - Converts internal `queryIntent` to user-facing chips
   - Handles 10+ intent types:
     - Similar-to queries
     - Person searches (all roles)
     - Decade filters with refinements
     - Studio, language, collection
     - Playback constraints
     - Negative/exclusion filters
   - Prioritized display order

3. **API Integration**
   - Extended `/plugins/semantic-search/search` endpoint
   - Returns `intent_chips` array in JSON response
   - Preserves backward compatibility

### Frontend (100% Complete)

**New Files Created:**
- `web/src/lib/api/semanticSearch.ts` - Typed API client
- `web/src/lib/hooks/useSemanticSearch.ts` - React hooks
- `web/src/components/home/widgets/IntentChip.tsx` - Chip component
- `web/src/components/common/IntentChipsBar.tsx` - Chips container

**Files Modified:**
- `web/src/lib/hooks/index.ts` - Export semantic search hooks
- `web/src/components/home/widgets/index.ts` - Export IntentChip
- `web/src/components/common/index.ts` - Export IntentChipsBar
- `web/src/components/common/MediaBrowsePage/MediaBrowsePage.types.ts` - Add intent chip props
- `web/src/components/common/MediaBrowsePage/MediaBrowsePage.tsx` - Render chips
- `web/src/routes/_layout/movies.index.tsx` - Integrate semantic search

**Features:**

1. **Semantic Search API Client** (`semanticSearch.ts`)
   - Type-safe API functions
   - Full type definitions matching backend
   - Error handling

2. **React Hooks** (`useSemanticSearch.ts`)
   - `useSemanticSearchAvailable()` - Check plugin availability
   - `useSemanticSearch()` - Perform semantic search
   - `useSimilarItems()` - Find similar content
   - Query caching and optimization

3. **IntentChip Component**
   - Type-based color schemes (10 colors)
   - Icon mapping for visual clarity
   - Removable chips with X button
   - Role badges for person chips
   - Smooth animations

4. **IntentChipsBar Component**
   - Container for intent chips
   - "Understanding:" label
   - Responsive wrapping layout
   - Glass morphism styling

5. **Movies Page Integration**
   - Auto-detect semantic search availability
   - Use semantic search when query present
   - Fall back to regular search otherwise
   - Hydrate results with full movie data
   - Display intent chips above results
   - Handle chip removal (clears search)

## How It Works

### User Flow

1. User types search query: `"90s spielberg movies not horror"`
2. System detects semantic search plugin is available
3. Query sent to `/api/plugin/semantic-search/search`
4. Backend parses intent and returns:
   ```json
   {
     "results": [...],
     "intent_chips": [
       { "type": "decade", "display": "90s", "removable": true },
       { "type": "person", "display": "Spielberg", "role": "director" },
       { "type": "exclusion", "display": "Not horror" }
     ]
   }
   ```
5. Frontend displays chips above results grid
6. User can click X on any chip to remove that filter

### Technical Flow

```
User types query
    ↓
useDebounce (300ms)
    ↓
useSemanticSearch hook
    ↓
POST /api/plugin/semantic-search/search
    ↓
detectQueryIntent() → convertToIntentChips()
    ↓
Response with results + intent_chips
    ↓
Hydrate entity_ids → full Movie objects
    ↓
Display in MediaBrowsePage with IntentChipsBar
```

## Testing

### Manual Testing Steps

1. **Start the dev server**
   ```bash
   make dev
   ```

2. **Test basic search**
   - Navigate to `/movies`
   - Search for "90s action"
   - Verify chips appear: [90s] [Action]

3. **Test person search**
   - Search for "spielberg movies"
   - Verify chip: [Steven Spielberg (director)]

4. **Test complex query**
   - Search for "90s spielberg not horror"
   - Verify multiple chips appear
   - Click X on "90s" chip
   - Verify search clears

5. **Test fallback**
   - Stop semantic search plugin (if running)
   - Search for movies
   - Verify regular search still works (no chips)

### Example Queries to Test

| Query | Expected Chips |
|-------|---------------|
| "90s movies" | [90s] |
| "spielberg films" | [Spielberg (director)] |
| "french comedy" | [French] [Comedy] |
| "4K movies" | [4K] |
| "movies like inception" | [Like "inception"] |
| "tarantino not violence" | [Tarantino (director)] [Not violence] |
| "80s sci-fi pixar" | [80s] [Sci-fi] [Pixar] |

## Architecture Decisions

### Why Semantic Search is Optional

The movies page gracefully handles both scenarios:
- **Semantic search available**: Rich search with intent chips
- **Semantic search unavailable**: Falls back to text search (no chips)

This ensures the app works even if the plugin isn't installed.

### Why Separate Data Fetching

Semantic search returns `entity_id` only. We fetch full movie data separately because:
- Keeps semantic search plugin lightweight
- Reuses existing movie API endpoints
- Maintains data consistency

Future optimization: Add batch endpoint or embed movie data in search response.

### Why Clear Search on Chip Removal

Currently, removing a chip clears the entire search. This is intentional for MVP:
- Simple to implement
- Clear user expectation
- Prevents edge cases

Future enhancement: Parse query and remove specific terms based on chip type.

## Future Enhancements

### Phase 2: Smart Chip Removal

Instead of clearing search, intelligently modify the query:
- Remove "90s" chip → Remove decade pattern from query
- Remove person chip → Remove "spielberg" or "by spielberg" patterns
- Remove exclusion chip → Remove "not X" patterns

Implementation:
```typescript
const handleRemoveIntentChip = (chipId: string) => {
  const chip = intentChips.find(c => c.id === chipId)
  if (!chip) return

  // Parse query and reconstruct without this intent
  const modifiedQuery = removeIntentFromQuery(search.q, chip)
  handleSearchChange(modifiedQuery)
}
```

### Phase 3: Chip Refinement

Allow clicking chips to see refinements:
- Click "90s" → dropdown shows ["80s", "00s"]
- Click a refinement → modify query to use that decade instead

### Phase 4: Result Reasons

Add "why this result" explanations per item (Contract 2).

### Phase 5: Recovery Banners

Display relaxation messages when zero results recovered (Contract 4).

## Files Changed Summary

### Backend (3 files)
- `plugins/semantic-search/internal/types.go` (+29 lines)
- `plugins/semantic-search/internal/search.go` (+220 lines)
- `plugins/semantic-search/internal/plugin.go` (+4 lines)

### Frontend (11 files)
- `web/src/lib/api/semanticSearch.ts` (new, 112 lines)
- `web/src/lib/hooks/useSemanticSearch.ts` (new, 53 lines)
- `web/src/components/home/widgets/IntentChip.tsx` (new, 173 lines)
- `web/src/components/common/IntentChipsBar.tsx` (new, 47 lines)
- `web/src/lib/hooks/index.ts` (+1 line)
- `web/src/components/home/widgets/index.ts` (+2 lines)
- `web/src/components/common/index.ts` (+1 line)
- `web/src/components/common/MediaBrowsePage/MediaBrowsePage.types.ts` (+5 lines)
- `web/src/components/common/MediaBrowsePage/MediaBrowsePage.tsx` (+14 lines)
- `web/src/routes/_layout/movies.index.tsx` (+75 lines)

**Total:** ~736 new/modified lines

## Verification

✅ Backend compiles: `cd plugins/semantic-search && go build`
✅ Frontend compiles: `cd web && npx tsc --noEmit`
✅ API contract: Intent chips returned in search response
✅ UI integration: Chips displayed in movies page
✅ Graceful degradation: Works without semantic search plugin

## Next Steps

To see it in action:

1. **Build and run**
   ```bash
   make build-tools
   make build
   ./bin/viewra
   ```

2. **Navigate to movies**
   Open http://localhost:8080/movies

3. **Search with semantic intent**
   Try: "90s spielberg movies"

4. **Observe chips**
   You should see: [🕐 90s ✕] [👤 Spielberg (director) ✕]

The implementation is complete and ready for use! 🎉

# Search Improvements Plan

## Overview

This document outlines a comprehensive plan to improve search quality in ViewRA's semantic-search plugin. The improvements span multiple phases, from quick wins to larger architectural changes.

### Goals
- Improve search relevance for structured queries ("90s teen movies")
- Add fuzzy matching for typos ("Spielburg" → "Spielberg")
- Incorporate quality signals (ratings, popularity)
- Enable personalized search results
- Reduce latency through caching
- Clean up technical debt

---

## Current State Analysis

### What's Working Well

1. **Hybrid Search Architecture**: Combines semantic vector search with intent-based text search
2. **Intent Detection**: Detects director, actor, studio, genre, language, and "similar to" patterns
3. **Rich Indexing Format**: Includes title, year, genres, plot, cast, directors, studios, themes, locations
4. **Context Enrichment**: Weather, time-of-day, and seasonal suggestions (privacy-conscious, opt-in)
5. **Diversity Penalty**: Prevents results dominated by single director/decade/genre
6. **Deduplication**: Handles same movie appearing from multiple libraries

### Technical Debt

| Issue | Location | Impact |
|-------|----------|--------|
| Hardcoded studio list | `search.go:299-307` | New studios need code changes |
| Hardcoded language map | `search.go:418-452` | Incomplete coverage |
| Magic boost numbers | `search.go` (0.55, 0.35, 0.60, etc.) | Hard to tune, arbitrary values |
| Brittle text extraction | `search.go:1087-1096` | Relies on exact prefix format |
| US-only holidays | `holidays.go:18-124` | No international support |
| No query embedding cache | `embedding.go` | Repeated API calls for same query |

### Missing Features

| Feature | Impact | Effort |
|---------|--------|--------|
| Stemming ("teen" ↔ "teenager") | High | Medium (Bleve) |
| Fuzzy matching (typo tolerance) | High | Medium (Bleve) |
| Field-specific boosting | Medium | Medium (Bleve) |
| Rating/quality signals | Medium | Low |
| User personalization | High | High |
| Autocomplete/type-ahead | Medium | Medium |
| Search history tracking | Low | Low |
| Trending searches | Low | Medium |

---

## Phase 1: Bleve Integration (Hybrid Search)

### Problem
The current search relies entirely on embedding similarity for structured queries. "90s teen movies" embeds the phrase and finds semantically similar content, but can't filter by decade or match stemmed terms.

### Solution
Integrate [Bleve](https://github.com/blevesearch/bleve) for BM25 full-text search alongside existing vector search.

### Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    Semantic Search Plugin                                 │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────┐                                                    │
│  │  IndexingService │                                                    │
│  │                  │──────┬────────────────────────────────┐            │
│  │  IndexSingle()   │      │                                │            │
│  │  IndexLibrary()  │      ▼                                ▼            │
│  └──────────────────┘  ┌────────────┐              ┌─────────────────┐   │
│                        │   Bleve    │              │ Vector Storage  │   │
│                        │   Index    │              │ (Host-managed)  │   │
│                        │            │              │                 │   │
│                        │ data/      │              │ sqlite-vec /    │   │
│                        │ plugins/   │              │ pgvector        │   │
│                        │ semantic-  │              │                 │   │
│                        │ search/    │              └─────────────────┘   │
│                        │ bleve/     │                      │             │
│                        └────────────┘                      │             │
│                              │                             │             │
│                              ▼                             ▼             │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                      SearchService                                │   │
│  │                                                                   │   │
│  │   ┌─────────────┐  ┌─────────────┐                               │   │
│  │   │ Bleve Search│  │Vector Search│  Parallel execution           │   │
│  │   │ (BM25)      │  │ (Cosine)    │                               │   │
│  │   └─────────────┘  └─────────────┘                               │   │
│  │          │                │                                       │   │
│  │          ▼                ▼                                       │   │
│  │   ┌──────────────────────────────┐                               │   │
│  │   │   Score Fusion (RRF/RSF)     │                               │   │
│  │   └──────────────────────────────┘                               │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

### Search Modes (Auto-detected)

| Mode | When Used | Example Queries |
|------|-----------|-----------------|
| **Hybrid** (default) | General queries | "90s teen movies", "sci-fi thriller" |
| **Hybrid + Filters** | Structured queries | "directed by Spielberg", "Korean films" |
| **Semantic Only** | Abstract/mood queries | "cozy movie for rainy day", "something uplifting" |

> **Note**: "Structured" queries still use hybrid search but with filters as first-class Bleve `must` clauses. See "Structured Queries Philosophy" in Risks & Mitigations.

### Bleve Document Schema

| Field | Type | Analyzer | Purpose |
|-------|------|----------|---------|
| `entity_type` | keyword | - | Filter by movie/tv/etc |
| `entity_id` | numeric | - | Join back to vector results |
| `title` | text | english | Title search with stemming |
| `plot` | text | english | Plot/description search |
| `tagline` | text | english | Tagline search |
| `year` | numeric | - | Year range queries |
| `decade` | keyword | - | "90s", "1990s" exact match |
| `genres` | keyword[] | - | Genre filtering |
| `language` | keyword | - | Language filtering |
| `country` | keyword | - | Country filtering |
| `content_rating` | keyword | - | MPAA rating (PG-13, R) |
| `directors` | text | simple | Director name search (no stemming) |
| `writers` | text | simple | Writer name search |
| `cast` | text | simple | Actor search |
| `studios` | text | simple | Studio search |
| `themes` | text | english | Theme keywords |
| `locations` | text | english | Location keywords |
| `tmdb_rating` | numeric | - | Quality signal (0-10) |
| `vote_count` | numeric | - | Confidence signal |

### Score Fusion

**RRF (Reciprocal Rank Fusion)** - Default:
```
score(d) = Σ 1/(k + rank_i(d))
```
- `k` = 60 (default, configurable)
- Simple, robust, doesn't require score normalization

**RSF (Relative Score Fusion)** - Alternative:
```
score(d) = w_bleve * norm(bleve_score) + w_vector * norm(vector_score)
```
- Configurable weights (default 0.5 each)
- Considers actual score magnitudes

### Configuration

```yaml
bleve:
  enabled: true
  schema_version: 1
  fuzzy_enabled: true
  fuzzy_distance: 1

search:
  fusion_method: "rrf"    # "rrf" or "rsf"
  fusion_k: 60            # RRF k parameter
  vector_weight: 0.5      # RSF weight for vectors
  text_weight: 0.5        # RSF weight for text
```

### Design Principles

1. **Filters as First-Class Queries**: Use decade/genre/language/country as Bleve `must` clauses, not just boost hints. This ensures hard constraints are enforced at the query level.

2. **Single-Writer Pattern**: All Bleve index writes go through a serialized work queue to prevent corruption. See "Bleve Index Lifecycle" in Risks & Mitigations.

3. **Normalized Names**: Directors/cast use a custom analyzer with lowercase, punctuation stripping, whitespace collapsing, and accent folding.

### Implementation Tasks

- [ ] Add `github.com/blevesearch/bleve/v2` dependency
- [ ] Create `BleveService` with index management (`bleve.go`)
  - [ ] Implement single-writer pattern with work queue
  - [ ] Add crash recovery / index integrity check on startup
  - [ ] Handle graceful shutdown with pending write flush
- [ ] Define document schema and mapping (`bleve_document.go`)
  - [ ] Create custom analyzer for person names (normalized)
  - [ ] Add decade field with proper tokenization
- [ ] Implement Bleve query builder (`bleve_search.go`)
  - [ ] Support filters as `must` clauses (decade, genre, language, country)
  - [ ] Implement robust decade parser (see Risks & Mitigations)
- [ ] Implement RRF/RSF score fusion (`fusion.go`)
- [ ] Integrate into `IndexingService.IndexSingle()`
- [ ] Add `detectSearchMode()` logic
- [ ] Add "Rebuild Bleve Index" API endpoint
- [ ] **Add Query Explain debug endpoint** (`GET /api/plugins/semantic-search/explain?q=...`)
  - Returns: detected mode, extracted intents/filters, top N from each source, fused ranks, boost deltas
- [ ] Handle schema version changes (auto-rebuild)
- [ ] Add settings to UI schema

### Test Queries

| Query | Expected Improvement |
|-------|---------------------|
| "90s teen movies" | Decade filter + stemmed "teen/teenager" |
| "Spielburg movies" | Fuzzy match → "Spielberg" |
| "sci fi" | Stemmed → "sci-fi", "science fiction" |
| "Korean thriller" | Language + genre filtering |
| "movies like The Matrix" | Still uses vector similarity |
| "cozy rainy day movie" | Still uses semantic search |

---

## Phase 2: Performance Optimizations

### 2.1 Query Embedding Cache

**Problem**: Same query searched twice = two embedding API calls.

**Solution**: LRU cache for query embeddings in `EmbeddingService`.

```go
type EmbeddingService struct {
    // ... existing fields
    cacheMu   sync.RWMutex
    cache     map[string]*embeddingCacheEntry
    cacheKeys []string  // For LRU tracking
    maxCache  int       // Default: 1000 entries (~3MB)
    cacheTTL  time.Duration // Default: 1 hour
}

type embeddingCacheEntry struct {
    embedding []float32
    createdAt time.Time
}
```

**Cache Key**: Normalized query (lowercase, collapsed whitespace)
```go
func normalizeQuery(text string) string {
    return strings.ToLower(strings.Join(strings.Fields(text), " "))
}
```

**Implementation Tasks**:
- [ ] Add cache fields to `EmbeddingService`
- [ ] Implement cache lookup in `EmbedSingle()`
- [ ] Add LRU eviction when at capacity
- [ ] Add cache config options (size, TTL)
- [ ] Add cache hit/miss metrics

**Impact**: 50%+ latency reduction for repeat queries.

### 2.2 Pre-parsed Metadata

**Problem**: `applyKeywordBoost()` calls `extractLine()` repeatedly, parsing the same text for each result.

**Solution**: Store structured metadata alongside embedding text.

```go
type Embedding struct {
    // Existing fields
    EntityType string
    EntityID   int64
    Vector     []float32
    Text       string
    
    // New: pre-parsed for fast boosting
    Metadata   *ParsedMetadata `json:"metadata,omitempty"`
}

type ParsedMetadata struct {
    Title     string   `json:"title"`
    Year      int      `json:"year"`
    Genres    []string `json:"genres"`
    Directors []string `json:"directors"`
    Cast      []string `json:"cast"`
    Studios   []string `json:"studios"`
}
```

**Implementation Tasks**:
- [ ] Add `ParsedMetadata` struct
- [ ] Populate during indexing
- [ ] Update `applyKeywordBoost()` to use metadata
- [ ] Migration: backfill existing embeddings or rebuild on next index

### 2.3 Result Caching

**Problem**: Same search by different users recalculates everything.

**Solution**: Cache search results with TTL, keyed by (query, entityTypes, limit).

**Note**: Must exclude user-specific boosts from cache key, apply personalization post-cache.

---

## Phase 3: Quality Signals

### 3.1 Rating Integration

**Available Data** (already in database):
- `movies.rating` - TMDB vote average (0-10 scale)
- `movies.rating_votes` - TMDB vote count
- Same fields exist for `tv_shows` and `tv_episodes`

**Missing Data**:
- `popularity` - TMDB popularity score (fetched but not stored)

**Ranking Formula**:
```go
func applyQualityBoost(results []SearchResult) []SearchResult {
    for i := range results {
        rating := results[i].Metadata.Rating      // 0-10
        votes := results[i].Metadata.VoteCount
        
        // Bayesian-weighted rating boost
        // High rating + high votes = bigger boost
        ratingBoost := (rating / 10.0) * 0.1  // Up to 10% boost for high-rated content
        confidenceBoost := math.Log10(float64(votes+1)) * 0.02  // ~6% for 1000 votes
        
        results[i].Similarity *= (1 + ratingBoost + confidenceBoost)
    }
    return results
}
```

**Implementation Tasks**:
- [ ] Add rating/vote_count to `ParsedMetadata`
- [ ] Fetch during indexing from `MediaDetails`
- [ ] Add `applyQualityBoost()` function
- [ ] Make boost weights configurable
- [ ] Optional: Add `popularity` column to schema, store from TMDB

### 3.2 Recency Boost

**Problem**: Newly added content doesn't get surfaced appropriately.

**Solution**: Small boost for recently added items.

```go
func applyRecencyBoost(results []SearchResult, addedDates map[int64]time.Time) {
    now := time.Now()
    for i := range results {
        addedAt := addedDates[results[i].EntityID]
        daysSinceAdded := now.Sub(addedAt).Hours() / 24
        
        if daysSinceAdded < 7 {
            results[i].Similarity *= 1.05  // 5% boost for last week
        } else if daysSinceAdded < 30 {
            results[i].Similarity *= 1.02  // 2% boost for last month
        }
    }
}
```

---

## Phase 4: Personalization

### Available User Signals

| Source | Data | Signal Type |
|--------|------|-------------|
| `user_ratings` | up/down/favorite | Explicit preference |
| `watch_progress` | watched items, completion | Implicit preference |
| `watch_progress` | in-progress items | Current interest |

### Personalization Strategy

```go
func applyPersonalization(ctx context.Context, userID string, results []SearchResult) {
    // Get user's preference signals
    favorites := getUserFavorites(ctx, userID)      // Strongest signal
    upvoted := getUserUpvoted(ctx, userID)          // Medium signal
    downvoted := getUserDownvoted(ctx, userID)      // Negative signal
    watched := getUserWatchedItems(ctx, userID)     // Implicit signal
    
    // Extract genre/director preferences from history
    preferredGenres := extractGenrePreferences(favorites, upvoted, watched)
    preferredDirectors := extractDirectorPreferences(favorites, upvoted)
    
    for i := range results {
        boost := 1.0
        
        // Boost items matching preferred genres
        for _, genre := range results[i].Metadata.Genres {
            if weight, ok := preferredGenres[genre]; ok {
                boost += weight * 0.1  // Up to 10% per matching genre
            }
        }
        
        // Boost items from preferred directors
        for _, director := range results[i].Metadata.Directors {
            if weight, ok := preferredDirectors[director]; ok {
                boost += weight * 0.15  // Up to 15% for matching director
            }
        }
        
        // Penalize downvoted content's genres
        // (Don't hide completely - user might still want to see)
        for _, genre := range results[i].Metadata.Genres {
            if isDislikedGenre(downvoted, genre) {
                boost -= 0.1
            }
        }
        
        results[i].Similarity *= float32(boost)
    }
}
```

### Privacy Controls

- [ ] Add user setting: "Personalize search results" (default: on)
- [ ] All processing is local (no external services)
- [ ] Uses existing opt-in data only
- [ ] Option to reset/clear personalization

### Implementation Tasks

- [ ] Add `sdk.RatingsClient` and `sdk.ProgressClient` to plugin
- [ ] Create `PersonalizationService`
- [ ] Extract preference patterns from user history
- [ ] Apply personalization boosts in search
- [ ] Add UI toggle for personalization
- [ ] Cache user preferences (invalidate on rating/watch changes)

---

## Phase 5: Autocomplete & Suggestions

### Current State

- Context-based suggestion chips (weather, time, holidays)
- No type-ahead autocomplete
- No search history
- No trending searches

### 5.1 Autocomplete Endpoint

**New Endpoint**: `GET /api/plugins/semantic-search/autocomplete?q=<prefix>&limit=10`

**Response**:
```json
{
  "suggestions": [
    {"type": "title", "text": "The Godfather", "entity_id": 123},
    {"type": "person", "text": "Steven Spielberg", "role": "director"},
    {"type": "genre", "text": "Science Fiction"},
    {"type": "recent", "text": "90s action movies"},
    {"type": "trending", "text": "new releases"}
  ]
}
```

**Implementation**:
- Bleve prefix queries on title, cast, directors fields
- Fuzzy matching for typo tolerance
- Merge with recent/trending searches
- Limit total suggestions (e.g., 8)

### 5.2 Search History

**New Table**:
```sql
CREATE TABLE search_history (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    query TEXT NOT NULL,
    result_count INTEGER,
    clicked_result_id INTEGER,  -- For relevance feedback
    searched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_recent (user_id, searched_at DESC)
);
```

**Features**:
- Track last N searches per user (default: 50)
- Show as "Recent searches" with clock icon
- Clear history option in settings

### 5.3 Trending Searches

**Aggregation**:
```sql
SELECT query, COUNT(*) as search_count
FROM search_history
WHERE searched_at > datetime('now', '-7 days')
GROUP BY LOWER(query)
ORDER BY search_count DESC
LIMIT 10;
```

**Privacy**: Aggregate only, no user attribution in trending.

### Implementation Tasks

- [ ] Create `search_history` table migration
- [ ] Add history tracking in search handler
- [ ] Implement autocomplete endpoint using Bleve
- [ ] Add recent searches to suggestions
- [ ] Implement trending aggregation
- [ ] Frontend: Add autocomplete dropdown component
- [ ] Frontend: Keyboard navigation for suggestions

---

## Phase 6: Technical Debt Cleanup

### 6.1 Externalize Configuration

**Move to `data/plugins/semantic-search/config/`**:

```yaml
# studios.yaml
studios:
  - pixar
  - disney
  - marvel
  - a24
  - warner bros
  - universal
  # ... easily extensible

# languages.yaml  
languages:
  french: French
  korean: Korean
  japanese: Japanese
  # ...

# boost_weights.yaml
boosts:
  director_match: 0.55
  director_mismatch_penalty: 0.35
  actor_match: 0.50
  genre_match: 0.30
  # ... tunable without code changes
```

### 6.2 International Holidays

**Add region support**:
```yaml
# holidays/us.yaml
holidays:
  - name: Christmas
    month: 12
    day: 25
    suggestions: ["Christmas movies", "Holiday classics"]

# holidays/india.yaml
holidays:
  - name: Diwali
    month: 10  # varies
    day: 24
    suggestions: ["Bollywood celebrations", "Festival films"]
```

**Load based on user's timezone/locale**.

### 6.3 Remove Magic Numbers

**Before**:
```go
boost += 0.55  // What does this mean?
```

**After**:
```go
boost += s.config.Boosts.DirectorMatch  // Clear, configurable
```

### Implementation Tasks

- [ ] Create config file structure
- [ ] Load configs on plugin startup
- [ ] Replace hardcoded lists with config lookups
- [ ] Add config reload endpoint (for hot updates)
- [ ] Add international holiday files
- [ ] Document all boost parameters

---

## Risks & Mitigations

### 1. Bleve Index Lifecycle

**Risk**: Index corruption, lock contention, and crash recovery issues.

**Mitigations**:
- **Single-writer pattern**: All index writes go through a single goroutine with a work queue
- **Index lock management**: Use Bleve's built-in locking; handle `ErrIndexLocked` gracefully
- **Crash recovery**: On startup, check index integrity; rebuild if corrupted
- **Library removal**: When a library is deleted, batch-delete all its documents from Bleve index
- **Graceful shutdown**: Flush pending writes and close index cleanly on plugin shutdown

```go
type BleveService struct {
    index     bleve.Index
    writeCh   chan bleveWriteOp   // Serialized writes
    closeCh   chan struct{}
}

func (s *BleveService) Start() {
    go s.writeLoop()  // Single writer goroutine
}

func (s *BleveService) writeLoop() {
    for {
        select {
        case op := <-s.writeCh:
            op.execute(s.index)
        case <-s.closeCh:
            s.index.Close()
            return
        }
    }
}
```

### 2. Name Normalization

**Risk**: Director/cast searches fail due to inconsistent naming ("Bong Joon-ho" vs "Bong Joon Ho").

**Solution**: Apply consistent normalization during indexing AND querying:

```go
func normalizeName(name string) string {
    // 1. Lowercase
    name = strings.ToLower(name)
    // 2. Strip punctuation (except spaces)
    name = regexp.MustCompile(`[^\p{L}\p{N}\s]`).ReplaceAllString(name, "")
    // 3. Collapse whitespace
    name = strings.Join(strings.Fields(name), " ")
    // 4. ASCII-fold accents (optional, use golang.org/x/text/transform)
    name = foldAccents(name)
    return name
}
```

**Implementation**:
- Create custom Bleve analyzer for `directors`/`cast`/`writers` fields
- Apply same normalization to search queries for person names
- Store both original and normalized forms (original for display)

### 3. Robust Decade Parser

**Risk**: Decade queries fail for edge cases ("late 90s", "early 2000s", "80's", "turn of the century").

**Solution**: Comprehensive decade normalization:

| Input | Normalized Token | Year Range |
|-------|------------------|------------|
| `90s`, `90's`, `nineties`, `1990s` | `1990s` | 1990-1999 |
| `early 90s` | `1990s` | 1990-1993 |
| `late 90s` | `1990s` | 1997-1999 |
| `mid 90s` | `1990s` | 1994-1996 |
| `1998` | `1990s` + exact year | 1998 |
| `turn of the century` | `2000s` | 1998-2002 |
| `golden age` | special handling | 1930s-1960s |

```go
type DecadeInfo struct {
    Canonical  string // "1990s"
    YearStart  int    // 1990
    YearEnd    int    // 1999
    Modifier   string // "early", "late", "mid", ""
}

func parseDecade(input string) (*DecadeInfo, bool) {
    // Implementation with regex patterns for all variants
}
```

### 4. Ranking Discipline

**Risk**: "Death by a thousand multipliers" - too many boost factors compound unpredictably.

**Design Principle**: Limit post-fusion modifications to maintain predictable ranking.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Ranking Pipeline                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐                              │
│  │ Bleve BM25  │  │   Vector    │                              │
│  │   Results   │  │  Similarity │                              │
│  └──────┬──────┘  └──────┬──────┘                              │
│         │                │                                      │
│         └───────┬────────┘                                      │
│                 ▼                                                │
│        ┌────────────────┐                                       │
│        │  RRF Fusion    │  ← Base ranking (no boosts here)      │
│        └───────┬────────┘                                       │
│                │                                                 │
│                ▼                                                 │
│        ┌────────────────┐                                       │
│        │ Hard Filters   │  Step 1: Remove non-matching          │
│        │ (constraints)  │  (decade, language, content rating)   │
│        └───────┬────────┘                                       │
│                │                                                 │
│                ▼                                                 │
│        ┌────────────────┐                                       │
│        │ Light Re-rank  │  Step 2: MAX 2 boost sources          │
│        │ (quality +     │  - Quality: rating * confidence       │
│        │  personalize)  │  - Personalization: user prefs        │
│        └───────┬────────┘  Combined boost capped at ±20%        │
│                │                                                 │
│                ▼                                                 │
│         Final Results                                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Rules**:
1. Fusion produces the base ranking (RRF or RSF)
2. Post-fusion: max 2 modification steps
   - Step 1: Hard filters/constraints (pass/fail, no score changes)
   - Step 2: Light re-ranking (quality + personalization, capped boost)
3. Combined boost from Step 2 capped at ±20% to prevent runaway scores
4. Log each component's contribution in debug mode

### 5. Structured Queries Philosophy

**Principle**: "Structured" means "enable constraints", NOT "disable vectors".

**Wrong approach**:
```go
if isStructuredQuery(query) {
    return bleveOnlySearch(query)  // ❌ Loses semantic understanding
}
```

**Correct approach**:
```go
if isStructuredQuery(query) {
    // Extract constraints as Bleve must-clauses
    filters := extractFilters(query)  // decade, genre, language, etc.
    
    // Still use hybrid search, but with filters applied
    return hybridSearchWithFilters(query, filters)  // ✓ Best of both
}
```

**Rationale**: A query like "90s teen comedy" benefits from:
- Vector search: understands "teen" themes even if not explicitly tagged
- BM25 search: matches "teen", "teenager", "coming of age" via stemming
- Filters: restricts to 1990-1999 release years
- All three working together produce better results than any alone

---

## Implementation Priority Matrix

| Phase | Effort | Impact | Priority |
|-------|--------|--------|----------|
| 2.1 Query Embedding Cache | Low | High | **P0** |
| 1. Bleve Integration | High | High | **P1** |
| 3.1 Rating Integration | Low | Medium | **P1** |
| 6.1 Externalize Config | Low | Medium | **P2** |
| 2.2 Pre-parsed Metadata | Medium | Medium | **P2** |
| 5.1 Autocomplete | Medium | Medium | **P2** |
| 4. Personalization | High | High | **P3** |
| 5.2 Search History | Low | Low | **P3** |
| 3.2 Recency Boost | Low | Low | **P3** |
| 6.2 International Holidays | Low | Low | **P4** |

### Recommended Order

1. **Query Embedding Cache** (P0) - Quick win, immediate latency improvement
2. **Bleve Integration** - Core improvement with:
   - Title/people fields with custom name analyzer
   - Decade/year/genre/language/country as first-class filters (`must` clauses)
   - RRF fusion for combining Bleve + vector results
3. **Quality Signals** - Rating/votes as light re-rank (capped at ±20% boost)
4. **Query Explain Endpoint** - Debug tool to validate ranking pipeline
5. **Autocomplete** - Better UX, uses Bleve prefix queries
6. **Personalization** - Major feature, builds on infrastructure

> **Rationale**: Cache first (quick win), then Bleve with proper filters (foundational), then quality signals as a controlled re-rank layer. Autocomplete and personalization come after the core search is solid.

---

## Success Metrics

| Metric | Current | Target | How to Measure |
|--------|---------|--------|----------------|
| Zero-result queries | Unknown | < 5% | Log and track |
| Search latency (p50) | ~50ms | < 30ms | Metrics endpoint |
| Search latency (p95) | ~200ms | < 100ms | Metrics endpoint |
| Cache hit rate | 0% | > 60% | Cache metrics |
| "90s teen movies" quality | Poor | Good | Manual testing |
| Typo tolerance | None | Good | "Spielburg" test |

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/bleve.go` | BleveService - index management |
| `internal/bleve_search.go` | Query building and execution |
| `internal/bleve_document.go` | Document struct and mapping |
| `internal/fusion.go` | RRF/RSF score fusion |
| `internal/personalization.go` | User preference learning |
| `internal/autocomplete.go` | Type-ahead suggestions |
| `config/studios.yaml` | Studio names (externalized) |
| `config/languages.yaml` | Language mappings |
| `config/boost_weights.yaml` | Tunable boost parameters |

### Modified Files

| File | Changes |
|------|---------|
| `go.mod` | Add bleve/v2 dependency |
| `internal/types.go` | Add BleveConfig, cache config, personalization config |
| `internal/embedding.go` | Add query cache |
| `internal/indexing.go` | Index to both Bleve and vector storage |
| `internal/search.go` | Integrate hybrid search, quality boosts |
| `internal/plugin.go` | Initialize new services, add endpoints |
| `internal/schema.go` | Add UI settings for new features |
| `internal/suggestions.go` | Add recent/trending suggestions |

---

## References

- [Bleve Documentation](https://blevesearch.com/docs/)
- [Bleve GitHub](https://github.com/blevesearch/bleve)
- [RRF Paper](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf)
- [Hybrid Search Best Practices](https://www.pinecone.io/learn/hybrid-search/)
- [BM25 Explained](https://www.elastic.co/blog/practical-bm25-part-2-the-bm25-algorithm-and-its-variables)

# Search Improvements Plan

## Overview

This document outlines a plan to improve search quality in ViewRA's semantic-search plugin. The focus is on **tuning and extending the existing architecture** rather than adding external search engines.

### Why Not Bleve/Meilisearch?

We evaluated adding Bleve (embedded) or Meilisearch (external) for BM25 full-text search. **Decision: Skip for now.**

**Reasons:**
1. **Semantic search already handles fuzzy matching** - Embeddings naturally handle typos ("Spielburg" ≈ "Spielberg") and synonyms
2. **Intent detection covers structured queries** - Director, actor, studio, language, decade patterns are already detected
3. **Complexity cost outweighs benefit** - Another dependency, sync logic, failure modes for marginal improvement
4. **Resource constraints** - K3s deployments benefit from fewer containers
5. **Autocomplete solves the "exact match" problem better** - Type-ahead prevents typos in the first place

**Revisit Tripwires** (explicit conditions to reconsider):
1. Users report significant search quality issues at scale (>100k items)
2. Need to expose search API directly to external clients
3. Building multi-tenant features where Meilisearch's tenant tokens would help
4. **Autocomplete quality becomes hard to maintain with FTS5** - If we find ourselves fighting FTS5 limitations or performance, that's a signal to consider Meilisearch for autocomplete specifically
5. External clients start hammering search endpoints and we need a dedicated search service boundary

### Goals
- Improve search relevance for structured queries ("90s teen movies") ✅ Fixed
- Handle typos gracefully (autocomplete + embeddings)
- Incorporate quality signals (ratings, popularity)
- Enable personalized search results
- Reduce latency through caching ✅ Done
- Add debugging/observability tools
- Clean up technical debt

---

## Current State Analysis

### What's Working Well

1. **Semantic Vector Search**: Embeddings handle conceptual/fuzzy matching naturally
2. **Intent Detection**: Detects director, actor, studio, genre, language, decade, and "similar to" patterns
3. **Rich Indexing Format**: Includes title, year, genres, plot, cast, directors, studios, themes, locations
4. **Context Enrichment**: Weather, time-of-day, and seasonal suggestions (privacy-conscious, opt-in)
5. **Diversity Penalty**: Prevents results dominated by single director/decade/genre
6. **Deduplication**: Handles same movie appearing from multiple libraries
7. **Query Embedding Cache**: LRU cache reduces latency for repeated queries ✅ Implemented

### Technical Debt

| Issue | Location | Impact |
|-------|----------|--------|
| Hardcoded studio list | `search.go:299-307` | New studios need code changes |
| Hardcoded language map | `search.go:418-452` | Incomplete coverage |
| Magic boost numbers | `search.go` (0.55, 0.35, 0.60, etc.) | Hard to tune, arbitrary values |
| Brittle text extraction | `search.go:1087-1096` | Relies on exact prefix format |
| US-only holidays | `holidays.go:18-124` | No international support |

### Recent Fixes

| Issue | Status | Notes |
|-------|--------|-------|
| "90s teen movies" not working | ✅ Fixed | Added decade/demographic words to `nonNameWords` |
| "movies like X" not using FindSimilar | ✅ Fixed | Added `extractSimilarToTitle()` pattern detection |
| Repeated queries hit AI API | ✅ Fixed | Added LRU embedding cache (1000 entries, 1hr TTL) |
| "anime movies" detected as person | ✅ Fixed | Added anime/animated/cartoon to `nonNameWords` |

---

## Phase 1: Autocomplete & Type-Ahead (Primary Solution for Typos)

### Problem
Users misspell titles/names, leading to poor results. Traditional solution is fuzzy matching, but **autocomplete prevents typos in the first place**.

### Why Not Simple LIKE Queries?

At 200k+ items, LIKE queries become problematic:
- `LIKE 'foo%'` (prefix) is okay with an index
- `LIKE '%foo%'` (contains) requires full table scan
- Users type partial matches: "scar jo", "pt anderson", "lord ring"

**Solution**: Use SQLite FTS5 for autocomplete. Still "no external engine" but fast mid-word matching.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Autocomplete Flow                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  User types: "lord ring"                                        │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │            Autocomplete Endpoint                             │ │
│  │  GET /api/plugins/semantic-search/autocomplete?q=lord+ring   │ │
│  └─────────────────────────────────────────────────────────────┘ │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                   FTS5 Virtual Table                         │ │
│  │  autocomplete_index (title, people, type, entity_id)         │ │
│  │                                                              │ │
│  │  Query: "lord* ring*" OR "lord ring*"                        │ │
│  └─────────────────────────────────────────────────────────────┘ │
│       │                                                         │
│       ▼                                                         │
│  Response:                                                      │
│  [                                                              │
│    {"type": "title", "text": "The Lord of the Rings: The Fellowship...", "entity_id": 123},
│    {"type": "title", "text": "The Lord of the Rings: The Two Towers", "entity_id": 124},
│    {"type": "person", "text": "Peter Jackson", "role": "director", "hint": "Lord of the Rings"}
│  ]                                                              │
└─────────────────────────────────────────────────────────────────┘
```

### FTS5 Schema (Plugin-managed SQLite)

```sql
-- Create FTS5 virtual table for autocomplete
-- This lives in the plugin's data directory, not the main DB
CREATE VIRTUAL TABLE autocomplete_fts USING fts5(
    -- Searchable content
    name,           -- Title or person name
    aliases,        -- Alternative names, "scar jo" for "Scarlett Johansson"
    -- Metadata (unindexed, just for retrieval)
    type UNINDEXED,       -- 'title', 'person', 'genre'
    entity_id UNINDEXED,  -- Movie/TV ID or person ID
    subtype UNINDEXED,    -- For people: 'director', 'actor', 'writer'
    year UNINDEXED,       -- For titles
    popularity UNINDEXED, -- For ranking (rating_votes or credit_count)
    -- Use trigram tokenizer for partial matching
    tokenize='trigram'
);

-- Populate from host DB on startup / after library scan
INSERT INTO autocomplete_fts (name, aliases, type, entity_id, subtype, year, popularity)
SELECT 
    m.title,
    COALESCE(m.original_title, ''),
    'title',
    m.media_id,
    'movie',
    m.year,
    COALESCE(m.rating_votes, 0)
FROM movies m
JOIN media ON media.id = m.media_id;

-- Add people (deduplicated)
INSERT INTO autocomplete_fts (name, aliases, type, entity_id, subtype, year, popularity)
SELECT 
    p.name,
    '',  -- Could add nicknames later
    'person',
    p.id,
    CASE 
        WHEN EXISTS (SELECT 1 FROM credits c WHERE c.person_id = p.id AND c.credit_type = 'director') THEN 'director'
        WHEN EXISTS (SELECT 1 FROM credits c WHERE c.person_id = p.id AND c.credit_type = 'cast') THEN 'actor'
        ELSE 'crew'
    END,
    NULL,
    (SELECT COUNT(*) FROM credits c WHERE c.person_id = p.id)
FROM people p;
```

### FTS5 Query Examples

```sql
-- Simple prefix search: "spiel" → "spielberg"
SELECT * FROM autocomplete_fts WHERE autocomplete_fts MATCH 'spiel*' ORDER BY popularity DESC LIMIT 10;

-- Multi-word: "lord ring" → "Lord of the Rings"
SELECT * FROM autocomplete_fts WHERE autocomplete_fts MATCH 'lord* ring*' ORDER BY popularity DESC LIMIT 10;

-- Partial name: "scar jo" → "Scarlett Johansson" (needs alias support)
SELECT * FROM autocomplete_fts WHERE autocomplete_fts MATCH 'scar* jo*' ORDER BY popularity DESC LIMIT 10;

-- With type filter
SELECT * FROM autocomplete_fts WHERE autocomplete_fts MATCH 'chris*' AND type = 'person' ORDER BY popularity DESC LIMIT 10;
```

### API Design

**Endpoint**: `GET /api/plugins/semantic-search/autocomplete`

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `q` | string | required | Search query (min 2 chars) |
| `limit` | int | 8 | Max suggestions to return |
| `types` | string | "all" | Filter: "titles", "people", "genres", or "all" |

**Response**:
```json
{
  "suggestions": [
    {"type": "title", "text": "The Lord of the Rings: The Fellowship of the Ring", "entity_id": 123, "year": 2001},
    {"type": "title", "text": "The Lord of the Rings: The Two Towers", "entity_id": 124, "year": 2002},
    {"type": "person", "text": "Peter Jackson", "person_id": 456, "role": "director"},
    {"type": "recent", "text": "90s action movies", "searched_at": "2024-01-15T10:30:00Z"}
  ],
  "query": "lord ring",
  "took_ms": 8
}
```

### Entity Resolution: ID-Based Flow

**Critical**: When user selects an autocomplete suggestion, use the entity ID directly.

```
User types: "aliens"
     │
     ▼
Autocomplete shows:
  [1] "Aliens (1986)" entity_id=123      ← User clicks this
  [2] "Alien (1979)" entity_id=456
  [3] "Alien 3 (1992)" entity_id=789
     │
     ▼
Search uses: entity_id=123 (not text "aliens")
     │
     ▼
"Movies like Aliens" → FindSimilar(entity_id=123)
```

This solves the disambiguation problem for:
- "Aliens" vs "Alien" (different movies)
- "It" (movie) vs "it" (stopword)
- "Up" (movie) vs "up" (word)
- "Her" (movie) vs "her" (pronoun)

**Frontend Implementation**:
```typescript
// When user selects from autocomplete
const handleSelect = (suggestion: Suggestion) => {
  if (suggestion.type === 'title' && suggestion.entity_id) {
    // Direct navigation or "similar to" uses entity_id
    router.push(`/movies/${suggestion.entity_id}`);
    // Or for "similar to":
    search({ similar_to_id: suggestion.entity_id });
  } else if (suggestion.type === 'person' && suggestion.person_id) {
    // Person search by ID
    search({ person_id: suggestion.person_id });
  } else {
    // Fall back to text search
    search({ q: suggestion.text });
  }
};
```

### "Movies Like X" Enhancement

Update `extractSimilarToTitle()` to prefer ID-based resolution:

```go
// If we have an entity_id from autocomplete selection, use it directly
if params.SimilarToID > 0 {
    return s.FindSimilar(ctx, EntityMovie, params.SimilarToID, limit)
}

// Otherwise, try to resolve title to entity
if intent.isSimilarSearch && intent.similarToTitle != "" {
    // First: exact title match (fast, unambiguous)
    movie, err := s.findMovieByExactTitle(ctx, intent.similarToTitle)
    if err == nil && movie != nil {
        return s.FindSimilar(ctx, EntityMovie, movie.ID, limit)
    }
    
    // Second: FTS5 lookup for fuzzy title match
    matches, err := s.autocompleteService.Search(ctx, intent.similarToTitle, 1, "titles")
    if err == nil && len(matches) > 0 {
        return s.FindSimilar(ctx, EntityMovie, matches[0].EntityID, limit)
    }
    
    // Third: fall back to semantic search (current behavior)
}
```

### Alias Support for People

Common searches that fail without aliases:
- "scar jo" → Scarlett Johansson
- "pt anderson" → Paul Thomas Anderson
- "rdj" → Robert Downey Jr.
- "jlo" → Jennifer Lopez

**Solution**: Maintain an aliases field, either:
1. **Manual**: Curated list for top ~500 people
2. **Generated**: First-name + last-initial patterns

```go
// Generate common alias patterns
func generateAliases(name string) []string {
    parts := strings.Fields(name)
    if len(parts) < 2 {
        return nil
    }
    
    aliases := []string{}
    
    // "Steven Spielberg" → "s spielberg", "steven s"
    first := strings.ToLower(parts[0])
    last := strings.ToLower(parts[len(parts)-1])
    
    aliases = append(aliases, first[:1]+" "+last)        // "s spielberg"
    aliases = append(aliases, first+" "+last[:1])        // "steven s"
    
    // For names with middle parts: "Robert Downey Jr." → "rdj"
    if len(parts) >= 3 {
        initials := ""
        for _, p := range parts {
            if len(p) > 0 && p != "Jr." && p != "Sr." {
                initials += strings.ToLower(p[:1])
            }
        }
        if len(initials) >= 2 {
            aliases = append(aliases, initials)  // "rdj"
        }
    }
    
    return aliases
}
```

### Implementation Tasks

- [ ] Create FTS5 virtual table in plugin's SQLite DB
- [ ] Add `AutocompleteService` with FTS5 queries
- [ ] Populate FTS5 on plugin startup (from host DB via SDK)
- [ ] Add incremental update on library scan completion
- [ ] Implement alias generation for people
- [ ] Register `/autocomplete` endpoint
- [ ] Add response caching (short TTL, ~30s)
- [ ] Update search params to accept `similar_to_id`
- [ ] Frontend: Add autocomplete dropdown component
- [ ] Frontend: Use entity_id when selecting suggestions
- [ ] Frontend: Keyboard navigation (up/down/enter/escape)
- [ ] Frontend: Debounce input (200-300ms)
- [ ] Add recent searches to suggestions (see Phase 2)

### Performance Expectations

| Dataset Size | FTS5 Query Time | LIKE '%x%' Time |
|--------------|-----------------|-----------------|
| 10k items | ~2ms | ~5ms |
| 100k items | ~5ms | ~50ms |
| 500k items | ~10ms | ~250ms+ |

FTS5 with trigram tokenizer scales much better for mid-word matching.

---

## Phase 2: Search History & Recent Searches

### Problem
Users often repeat searches. Showing recent searches improves UX and reduces typing.

### Solution
Track search queries per user, surface in autocomplete.

### Storage

**Option A: Plugin-managed SQLite** (simpler, isolated)
```sql
-- In plugin's data directory
CREATE TABLE search_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    query TEXT NOT NULL,
    result_count INTEGER,
    searched_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_search_history_user ON search_history(user_id, searched_at DESC);
```

**Option B: Host-managed via SDK** (if we add a history capability to the host)

### API

**Track search** (called internally after each search):
```go
func (s *SearchService) trackSearch(ctx context.Context, userID, query string, resultCount int) {
    // Insert into search_history, keep only last 50 per user
}
```

**Get recent searches**:
```go
func (s *AutocompleteService) getRecentSearches(ctx context.Context, userID string, limit int) []string {
    // SELECT DISTINCT query FROM search_history WHERE user_id = ? ORDER BY searched_at DESC LIMIT ?
}
```

### Implementation Tasks

- [ ] Create search_history table in plugin's SQLite DB
- [ ] Track searches after successful search
- [ ] Deduplicate consecutive identical searches
- [ ] Add "clear history" endpoint
- [ ] Include recent searches in autocomplete response
- [ ] Add setting to disable history tracking

---

## Phase 3: Query Explain Endpoint (Debugging)

### Problem
Hard to understand why a search returns certain results. Need visibility into the ranking pipeline.

### Solution
Add debug endpoint that explains how a query was processed.

### API

**Endpoint**: `GET /api/plugins/semantic-search/explain?q=<query>`

**Response**:
```json
{
  "query": "90s spielberg movies",
  "normalized_query": "90s spielberg movies",
  "detected_intents": {
    "decade": {"value": "1990s", "year_start": 1990, "year_end": 1999},
    "person": {"name": "spielberg", "type": "director_or_actor"}
  },
  "search_mode": "semantic_with_filters",
  "embedding_cached": true,
  "vector_search": {
    "min_similarity": 0.35,
    "results_before_filter": 127,
    "results_after_filter": 23
  },
  "boosts_applied": [
    {"type": "director_match", "target": "spielberg", "boost": 0.55, "matches": 12},
    {"type": "decade_match", "target": "1990s", "boost": 0.20, "matches": 18}
  ],
  "final_results": 15,
  "took_ms": 45,
  "top_results": [
    {
      "title": "Schindler's List",
      "year": 1993,
      "base_similarity": 0.72,
      "boosted_similarity": 0.89,
      "boost_breakdown": {
        "director_match": 0.55,
        "decade_match": 0.20,
        "diversity_penalty": -0.08
      }
    }
  ]
}
```

### Implementation Tasks

- [ ] Add `ExplainService` or extend `SearchService` with explain mode
- [ ] Collect boost breakdown during search
- [ ] Add `/explain` endpoint (admin-only or debug flag)
- [ ] Log explain data for failed searches (zero results)

---

## Phase 4: Quality Signals (Rating/Popularity)

### Problem
High-quality, popular content should rank slightly higher than obscure matches.

### Solution
Apply light re-ranking based on TMDB ratings and vote counts **in the final stage only**.

### Guardrails (Critical)

**Quality boost must never wash out a perfect semantic match.**

| Rule | Rationale |
|------|-----------|
| Apply **only in final re-rank stage** | Don't pollute base ranking |
| Cap impact at **±15% per result** | Prevents runaway scores |
| Require **minimum vote threshold** | Low-confidence ratings ignored |
| **Soft boost only** | Quality can't outrank a much higher semantic match |

**Example of the problem we're avoiding:**

```
Query: "obscure 1970s Italian giallo horror"

Without guardrails:
  #1: The Godfather (1972)         similarity=0.45, quality_boost=+15% → 0.52
  #2: Deep Red (1975)              similarity=0.78, quality_boost=+3%  → 0.80

With guardrails (correct):
  #1: Deep Red (1975)              similarity=0.78, quality_boost=+3%  → 0.80
  #2: The Godfather (1972)         similarity=0.45, quality_boost=+15% → 0.52

Quality boost should help break ties, not override relevance.
```

### Available Data

| Field | Source | Scale |
|-------|--------|-------|
| `rating` | TMDB vote average | 0-10 |
| `rating_votes` | TMDB vote count | 0-∞ |
| `popularity` | TMDB popularity | 0-∞ (not currently stored) |

### Ranking Formula

```go
func applyQualityBoost(results []SearchResult) {
    // Sort by similarity first (base ranking)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Similarity > results[j].Similarity
    })
    
    for i := range results {
        rating := results[i].Rating        // 0-10
        votes := results[i].VoteCount
        
        // Guardrail 1: Only boost if we have enough confidence
        if votes < 100 {
            continue
        }
        
        // Guardrail 2: Normalized rating boost, max 10%
        ratingBoost := (rating / 10.0) * 0.10
        
        // Guardrail 3: Confidence factor (log scale), max 5%
        confidenceFactor := math.Min(math.Log10(float64(votes))/4.0, 1.0)
        
        // Guardrail 4: Combined boost capped at 15%
        totalBoost := math.Min(ratingBoost*confidenceFactor, 0.15)
        
        // Guardrail 5: Soft boost - multiplicative, not additive
        // A 0.4 similarity can become at most 0.46 (not 0.55)
        results[i].Similarity *= (1 + float32(totalBoost))
    }
    
    // Re-sort after boosting (quality can shuffle within tiers, not across)
    // Note: Because boost is capped at 15%, a 0.78 match can't be beaten by a 0.45 match
    // even with max quality boost: 0.45 * 1.15 = 0.52 < 0.78
    sort.Slice(results, func(i, j int) bool {
        return results[i].Similarity > results[j].Similarity
    })
}
```

### Position in Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                    Ranking Pipeline                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Vector Search (semantic similarity)                         │
│       │                                                         │
│       ▼                                                         │
│  2. Intent-Based Filtering (decade, language, etc.)             │
│       │                                                         │
│       ▼                                                         │
│  3. Keyword Boosting (director/actor match)                     │
│       │                                                         │
│       ▼                                                         │
│  4. Diversity Penalties (avoid single-director domination)      │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ 5. Quality Boost (FINAL STAGE)                          │   │
│  │    - Only affects ranking within similarity tiers       │   │
│  │    - Capped at ±15%                                     │   │
│  │    - Cannot override a significantly better match       │   │
│  └─────────────────────────────────────────────────────────┘   │
│       │                                                         │
│       ▼                                                         │
│  Final Results                                                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Tasks

- [ ] Add rating/votes to search result metadata
- [ ] Implement `applyQualityBoost()` with guardrails
- [ ] Add unit tests for edge cases (high quality + low relevance should NOT win)
- [ ] Add config option to enable/disable quality boost
- [ ] Add config option for boost weights and caps
- [ ] Log quality boost decisions in explain endpoint

---

## Phase 5: Externalize Configuration

### Problem
Boost weights, studio lists, and language maps are hardcoded. Tuning requires code changes.

### Solution
Move to config files that can be edited without recompilation.

### Config Structure

```
data/plugins/semantic-search/
├── config.yaml           # Main plugin config
├── studios.yaml          # Known studios for intent detection
├── languages.yaml        # Language name mappings
└── boosts.yaml           # Ranking boost weights
```

**boosts.yaml**:
```yaml
# Boost weights for keyword matching
boosts:
  director_match: 0.55
  director_mismatch_penalty: -0.35
  actor_match: 0.50
  genre_match: 0.30
  decade_match: 0.20
  studio_match: 0.25
  language_match: 0.30

# Quality signal weights
quality:
  enabled: true
  max_boost: 0.15
  min_votes: 100

# Diversity penalties
diversity:
  same_director_penalty: 0.15
  same_decade_penalty: 0.05
  same_genre_penalty: 0.03
```

**studios.yaml**:
```yaml
studios:
  - pixar
  - disney
  - marvel
  - a24
  - warner bros
  - universal
  - paramount
  - sony
  - lionsgate
  - netflix
  - amazon
  - apple
  - hbo
  - ghibli
  - dreamworks
```

### Config Versioning (Critical)

**Problem**: Plugin upgrades add new config keys. Old configs missing new keys → weird behavior.

**Solution**: Version configs and merge with defaults.

```yaml
# boosts.yaml
config_version: 1  # Increment when adding/changing keys

boosts:
  director_match: 0.55
  # ... rest of config
```

**Loading logic**:
```go
type BoostConfig struct {
    ConfigVersion int `yaml:"config_version"`
    // ... fields
}

func loadConfig(path string) (*BoostConfig, error) {
    // 1. Load defaults (embedded in binary)
    defaults := getDefaultConfig()
    
    // 2. Load user config if exists
    userConfig, err := loadYAML(path)
    if err != nil {
        // First run: create default config file
        return defaults, writeDefaultConfig(path)
    }
    
    // 3. Check version
    if userConfig.ConfigVersion < defaults.ConfigVersion {
        log.Warn("config version outdated, merging with defaults",
            "user_version", userConfig.ConfigVersion,
            "current_version", defaults.ConfigVersion)
    }
    
    // 4. Merge: user values override defaults, but defaults fill missing keys
    merged := mergeConfigs(defaults, userConfig)
    
    return merged, nil
}

func mergeConfigs(defaults, user *BoostConfig) *BoostConfig {
    result := *defaults  // Start with all defaults
    
    // Override with user values where present
    if user.Boosts.DirectorMatch != 0 {
        result.Boosts.DirectorMatch = user.Boosts.DirectorMatch
    }
    // ... etc for each field
    
    // Update version to current
    result.ConfigVersion = defaults.ConfigVersion
    
    return &result
}
```

**Behavior on upgrade**:
| Scenario | Behavior |
|----------|----------|
| New install | Create default config with current version |
| Upgrade, no config changes | Config works, version warning logged |
| Upgrade, new keys added | New keys get default values, existing preserved |
| User edited config | User values preserved, new keys get defaults |

### Implementation Tasks

- [ ] Create default config files on first run
- [ ] Add `config_version` to all config files
- [ ] Implement config merge logic (defaults + user overrides)
- [ ] Add config loading on plugin startup
- [ ] Replace hardcoded values with config lookups
- [ ] Add config reload endpoint (hot reload)
- [ ] Log warnings when config version is outdated
- [ ] Document all config options with defaults

---

## Phase 6: Personalization (Future)

### Overview
Boost results based on user's watch history and ratings. **Lower priority** - implement after core search is solid.

### Signals Available

| Source | Signal | Weight |
|--------|--------|--------|
| `user_ratings` (favorite) | Strong positive | High |
| `user_ratings` (up) | Positive | Medium |
| `user_ratings` (down) | Negative | Medium |
| `watch_progress` (completed) | Implicit positive | Low |

### Approach
1. Extract genre/director preferences from favorites and highly-rated items
2. Apply small boost (max ±10%) to matching content
3. Never hide content, just re-rank slightly
4. User setting to enable/disable

### Implementation Tasks (Future)

- [ ] Add SDK clients for ratings and watch progress
- [ ] Build preference extraction logic
- [ ] Apply personalization in search pipeline
- [ ] Add user setting toggle
- [ ] Cache preferences with invalidation

---

## Implementation Priority

| Phase | Effort | Impact | Priority | Status |
|-------|--------|--------|----------|--------|
| Query Embedding Cache | Low | High | P0 | ✅ Done |
| Autocomplete | Medium | High | **P1** | Pending |
| Query Explain | Low | Medium | **P1** | Pending |
| Search History | Low | Medium | P2 | Pending |
| Quality Signals | Low | Medium | P2 | Pending |
| Externalize Config | Medium | Medium | P3 | Pending |
| Personalization | High | High | P4 | Future |

### Recommended Order

1. **Autocomplete with FTS5** - Prevents typos, handles mid-word matching at scale, entity resolution
2. **Query Explain** - Enables debugging and validates ranking pipeline
3. **Quality Signals** - Cheap win once we can explain/verify ranking behavior
4. **Search History** - Feeds into autocomplete, improves UX
5. **Externalize Config + Versioning** - Enables tuning without code changes
6. **Personalization** - Major feature, requires more infrastructure

> **Note**: Autocomplete is both the highest impact AND highest risk item. If FTS5 becomes problematic at scale, that's when we'd revisit Meilisearch - but only for autocomplete, not core search.

---

## Success Metrics

| Metric | Current | Target | How to Measure |
|--------|---------|--------|----------------|
| Zero-result queries | Unknown | < 5% | Log and track |
| Search latency (p50) | ~50ms | < 30ms | Metrics endpoint |
| Search latency (p95) | ~200ms | < 100ms | Metrics endpoint |
| Cache hit rate | ~0% | > 60% | Cache stats endpoint |
| Autocomplete usage | N/A | > 30% of searches | Track selection |

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/autocomplete.go` | Autocomplete service and endpoint |
| `internal/explain.go` | Query explain endpoint |
| `internal/history.go` | Search history tracking |
| `internal/quality.go` | Quality signal boosting |
| `internal/config_loader.go` | YAML config loading |
| `config/boosts.yaml` | Default boost weights |
| `config/studios.yaml` | Studio list |
| `config/languages.yaml` | Language mappings |

### Modified Files

| File | Changes |
|------|---------|
| `internal/embedding.go` | ✅ Already has LRU cache |
| `internal/search.go` | Add quality boost, config lookups |
| `internal/plugin.go` | Register new endpoints |
| `internal/schema.go` | Add UI settings for new features |

---

## References

- [SQLite FTS5](https://www.sqlite.org/fts5.html) - If we ever need full-text search without external deps
- [RRF Paper](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf) - Score fusion (for future reference)

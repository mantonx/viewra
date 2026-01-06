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

**Revisit if:**
- Users report significant search quality issues at scale
- Need to expose search API directly to external clients
- Building multi-tenant features where Meilisearch's tenant tokens would help

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

### Solution
Add type-ahead autocomplete that suggests titles, people, and genres as the user types.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Autocomplete Flow                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  User types: "spiel"                                            │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │            Autocomplete Endpoint                         │   │
│  │  GET /api/plugins/semantic-search/autocomplete?q=spiel   │   │
│  └─────────────────────────────────────────────────────────┘   │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │   Titles    │  │   People    │  │   Genres    │            │
│  │   (SQL      │  │   (SQL      │  │   (Static   │            │
│  │   LIKE)     │  │   LIKE)     │  │   list)     │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│       │                │                │                       │
│       └────────────────┼────────────────┘                       │
│                        ▼                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  Merge & Rank (titles first, then people, then genres)   │   │
│  └─────────────────────────────────────────────────────────┘   │
│       │                                                         │
│       ▼                                                         │
│  Response:                                                      │
│  [                                                              │
│    {"type": "person", "text": "Steven Spielberg", "role": "director"},
│    {"type": "title", "text": "Spielberg (2017)", "entity_id": 456},
│    {"type": "title", "text": "The Post", "entity_id": 789, "hint": "Spielberg"}
│  ]                                                              │
└─────────────────────────────────────────────────────────────────┘
```

### API Design

**Endpoint**: `GET /api/plugins/semantic-search/autocomplete`

**Parameters**:
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `q` | string | required | Search prefix (min 2 chars) |
| `limit` | int | 8 | Max suggestions to return |
| `types` | string | "all" | Filter: "titles", "people", "genres", or "all" |

**Response**:
```json
{
  "suggestions": [
    {"type": "title", "text": "The Godfather", "entity_id": 123, "year": 1972},
    {"type": "person", "text": "Steven Spielberg", "role": "director", "movie_count": 34},
    {"type": "genre", "text": "Science Fiction"},
    {"type": "recent", "text": "90s action movies", "searched_at": "2024-01-15T10:30:00Z"}
  ],
  "query": "spiel",
  "took_ms": 12
}
```

### Implementation

**Title Search** (SQL):
```sql
-- SQLite
SELECT id, title, year FROM movies 
WHERE title LIKE ? || '%' COLLATE NOCASE
ORDER BY rating_votes DESC
LIMIT 5;

-- With trigram for mid-word matching (optional enhancement)
SELECT id, title, year FROM movies
WHERE title LIKE '%' || ? || '%' COLLATE NOCASE
ORDER BY 
  CASE WHEN title LIKE ? || '%' COLLATE NOCASE THEN 0 ELSE 1 END,
  rating_votes DESC
LIMIT 5;
```

**People Search** (from indexed embeddings metadata or separate table):
```sql
SELECT DISTINCT director as name, 'director' as role, COUNT(*) as movie_count
FROM movies 
WHERE director LIKE ? || '%' COLLATE NOCASE
GROUP BY director
ORDER BY movie_count DESC
LIMIT 3;

-- Combined with cast
UNION ALL

SELECT DISTINCT name, 'actor' as role, COUNT(*) as movie_count
FROM movie_cast
WHERE name LIKE ? || '%' COLLATE NOCASE
GROUP BY name
ORDER BY movie_count DESC
LIMIT 3;
```

**Genre Suggestions** (static list with prefix filter):
```go
var genres = []string{
    "Action", "Adventure", "Animation", "Comedy", "Crime",
    "Documentary", "Drama", "Family", "Fantasy", "History",
    "Horror", "Music", "Mystery", "Romance", "Science Fiction",
    "Thriller", "War", "Western",
}

func suggestGenres(prefix string) []string {
    prefix = strings.ToLower(prefix)
    var matches []string
    for _, g := range genres {
        if strings.HasPrefix(strings.ToLower(g), prefix) {
            matches = append(matches, g)
        }
    }
    return matches
}
```

### Implementation Tasks

- [ ] Add `AutocompleteService` in `internal/autocomplete.go`
- [ ] Add SQL queries for title/people prefix search
- [ ] Register `/autocomplete` endpoint in plugin
- [ ] Add response caching (short TTL, ~30s)
- [ ] Frontend: Add autocomplete dropdown component
- [ ] Frontend: Keyboard navigation (up/down/enter/escape)
- [ ] Frontend: Debounce input (200-300ms)
- [ ] Add recent searches to suggestions (see Phase 2)

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
Apply light re-ranking based on TMDB ratings and vote counts.

### Available Data

| Field | Source | Scale |
|-------|--------|-------|
| `rating` | TMDB vote average | 0-10 |
| `rating_votes` | TMDB vote count | 0-∞ |
| `popularity` | TMDB popularity | 0-∞ (not currently stored) |

### Ranking Formula

```go
func applyQualityBoost(results []SearchResult) {
    for i := range results {
        rating := results[i].Rating        // 0-10
        votes := results[i].VoteCount
        
        // Only boost if we have enough confidence (votes > 100)
        if votes < 100 {
            continue
        }
        
        // Normalized rating boost: max 10% for perfect 10.0 rating
        ratingBoost := (rating / 10.0) * 0.10
        
        // Confidence factor: log scale, ~5% boost at 1000 votes
        confidenceFactor := math.Min(math.Log10(float64(votes))/4.0, 1.0)
        
        // Combined boost, capped at 15%
        totalBoost := math.Min(ratingBoost*confidenceFactor, 0.15)
        
        results[i].Similarity *= (1 + float32(totalBoost))
    }
}
```

### Implementation Tasks

- [ ] Add rating/votes to search result metadata
- [ ] Implement `applyQualityBoost()` function
- [ ] Add config option to enable/disable quality boost
- [ ] Add config option for boost weights
- [ ] Test that obscure but relevant results aren't buried

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

### Implementation Tasks

- [ ] Create default config files on first run
- [ ] Add config loading on plugin startup
- [ ] Replace hardcoded values with config lookups
- [ ] Add config reload endpoint (hot reload)
- [ ] Document all config options

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

1. **Autocomplete** - Prevents typos, improves UX significantly
2. **Query Explain** - Enables debugging and tuning
3. **Search History** - Quick win, enhances autocomplete
4. **Quality Signals** - Light re-ranking based on ratings
5. **Externalize Config** - Enables tuning without code changes
6. **Personalization** - Major feature, requires more infrastructure

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

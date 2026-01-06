# Search Improvements Plan

## Overview

This document outlines a plan to improve search quality in ViewRA's **semantic-search plugin**. The focus is on **tuning and extending the existing architecture** rather than adding external search engines.

> **Important**: The semantic-search plugin is **optional**. When disabled, ViewRA falls back to basic text search in the core application. This document covers plugin improvements only.

### Core vs Plugin Search

| Feature | Core (Fallback) | Semantic-Search Plugin |
|---------|-----------------|------------------------|
| Title search | ✅ LIKE/ILIKE | ✅ Vector + intent |
| Typo tolerance | ❌ | ✅ Embeddings |
| "Movies like X" | ❌ | ✅ FindSimilar |
| Vibe/mood queries | ❌ | ✅ Semantic |
| Director/actor search | ✅ Basic LIKE | ✅ Intent detection |
| Decade/genre filters | ❌ | ✅ Intent detection |
| Context (weather/time) | ❌ | ✅ Optional enrichment |

**Core search location**: `internal/api/handlers/search.go:47-87`

When semantic-search is unavailable, `/api/search` falls back to `searchService.Search()` which performs LIKE queries across movies and TV shows. The response includes `fallback: true` to indicate fallback mode.

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

## Search Scenario Taxonomy

Comprehensive coverage of what users actually try to do in a media server.

### Coverage Matrix

| # | Scenario | Status | Implementation |
|---|----------|--------|----------------|
| 1 | **Exact/Navigational** | ✅ Planned | Autocomplete → entity_id |
| 2 | **Search-as-You-Type** | ✅ Planned | FTS5 trigram |
| 3 | **Fuzzy/Typo Tolerance** | ✅ Covered | Embeddings + autocomplete prevention |
| 4 | **Semantic/Vibe Discovery** | ✅ Covered | Core strength (embeddings + mood) |
| 5 | **Similarity ("More Like This")** | ✅ Covered | FindSimilar with entity_id |
| 6 | **Structured/Faceted** | ✅ Covered | Intent detection (decade, genre, language) |
| 7 | **Mixed NL + Structure** | ✅ Covered | Intent + semantic + boosts |
| 8 | **Disambiguation** | ✅ Planned | Entity ID flow ("It", "Up", "Her") |
| 9 | **Negative/Exclusion** | ✅ Covered | `extractNegativeTerms()`, mood-implied |
| 10 | **Playback Constraints** | ❌ Missing | Codec, HDR, subtitle filters |
| 11 | **Collections/Franchises** | ⚠️ Partial | Data exists, not exposed |
| 12 | **Role-Qualified People** | ⚠️ Partial | Writer/producer yes, composer no |
| 13 | **Language/Title Variants** | ✅ Planned | original_title in FTS5 |
| 14 | **Zero-Result Recovery** | ❌ Missing | Need relaxation + explanation |
| 15 | **Session Refinement** | ❌ Missing | Incremental constraints |
| 16 | **Quality Ranking** | ✅ Planned | With guardrails |
| 17 | **Personalization** | ✅ Planned | Future phase |

### Scenario Details

#### 1. Exact/Navigational Search ✅
User knows what they want.

**Examples**: "Alien", "Aliens 1986", "The Godfather Part II", "Spielberg"

**Implementation**: Autocomplete with entity_id resolution. Highest-frequency path.

#### 2. Search-as-You-Type ✅
User explores or avoids typos.

**Examples**: "spiel", "scar jo", "lord ring", "pt anderson", "rdj"

**Implementation**: FTS5 trigram + alias expansion + tiered ranking.

#### 3. Fuzzy/Typo Tolerance ✅
User makes mistakes.

**Examples**: "Spielburg", "Scorsesee", "Shindlers List"

**Implementation**: Semantic embeddings handle this naturally. Autocomplete prevents most typos.

#### 4. Semantic/Vibe Discovery ✅
User describes a feeling or mood.

**Examples**: "movies for a rainy day", "comfort movies", "slow atmospheric sci-fi"

**Implementation**: Embedding similarity + mood tags + context enrichment (weather, time, season).
This is the flagship differentiator.

#### 5. Similarity Search ✅
User likes something and wants more.

**Examples**: "movies like Cocktail", "films similar to Heat", "more like Parasite"

**Implementation**: `extractSimilarToTitle()` → entity_id → `FindSimilar()`.
String fallback only if ID resolution fails.

#### 6. Structured/Faceted Queries ✅
User specifies explicit constraints.

**Examples**: "90s teen movies", "French horror", "A24 sci-fi", "Korean thrillers"

**Implementation**: Intent detection extracts decade, genre, language, studio → filters + semantic ranking.

#### 7. Mixed Natural Language + Structure ✅
User blends vibes with constraints.

**Examples**: "90s Spielberg movies", "funny but not stupid action movies"

**Implementation**: Intent extraction + semantic candidates + keyword boosts + diversity penalties.
This is the hardest and most valuable class.

#### 8. Disambiguation ✅
Short words with overloaded meaning.

**Examples**: "It", "Up", "Her", "Cars"

**Implementation**: Autocomplete → entity_id. Most search systems fail here; we solve it explicitly.

#### 9. Negative/Exclusion Queries ✅
User knows what they don't want.

**Examples**: "rainy day movies not horror", "comedies without subtitles"

**Implementation**: `extractNegativeTerms()` detects "no", "without", "not", "avoid", "excluding".
`extractMoodImpliedNegatives()` handles implicit exclusions ("cozy" → no horror).

**Current patterns** (from code):
```go
{"no ", ""},           // "no horror"
{"not ", ""},          // "not scary"
{"without ", ""},      // "without violence"
{"non-", ""},          // "non-violent"
{"avoid ", ""},        // "avoid gore"
{"excluding ", ""},    // "excluding thrillers"
```

#### 10. Playback/Availability Constraints ❌ MISSING
User wants something they can actually watch.

**Examples**: "4K Dolby Vision", "Direct Play on this device", "has subtitles"

**Should filter by**: codec, container, HDR format, audio format, subtitle presence, transcode vs direct play.

**Status**: Not implemented. High importance for media server UX.

**Implementation plan**:
- Add `SearchParams.PlaybackConstraints` struct
- Query media table for codec/resolution/HDR info
- Apply as hard filters before ranking

#### 11. Collections/Franchises ⚠️ PARTIAL
User wants grouped content.

**Examples**: "all Mission: Impossible movies", "Harry Potter in order", "Pixar movies"

**Status**: TMDb data exists in DB, studio detection works ("Pixar movies"), but franchise/collection queries not explicit.

**Implementation plan**:
- Add collection/franchise to autocomplete suggestions
- Add `isCollectionSearch` intent

#### 12. Role-Qualified People Queries ⚠️ PARTIAL
User searches by specific contribution.

**Examples**: "movies written by Aaron Sorkin", "music by Hans Zimmer", "cinematography by Roger Deakins"

**Currently supported** (from code):
- ✅ Director: "directed by X", "X movies" 
- ✅ Actor: "starring X", "with X", "featuring X"
- ✅ Writer: "written by X", "screenplay by X"
- ✅ Producer: "produced by X", "from producer X"

**Missing**:
- ❌ Composer: "music by X", "score by X"
- ❌ Cinematographer: "cinematography by X", "shot by X"

**Implementation plan**: Add patterns to `detectQueryIntent()`:
```go
composerPatterns := []string{"music by ", "score by ", "composed by ", "composer "}
cinematographerPatterns := []string{"cinematography by ", "shot by ", "filmed by ", "dp "}
```

DB already has `credit_type = 'composer'` in credits table.

#### 13. Language/Title Variants ✅
User uses alternate titles or languages.

**Examples**: Original-language titles, translated titles, regional names

**Implementation**: Include `original_title` in FTS5 autocomplete index.

#### 14. Zero-Result Recovery ❌ MISSING
System helps when nothing matches.

**Examples**: Over-constrained queries, rare combinations

**Should do**:
1. Detect zero results
2. Progressively relax filters (decade → ±5 years, genre → parent genre)
3. Lower similarity threshold slightly
4. Return results with explanation of what changed

**Implementation plan**:
- Add to `SearchService.Search()` after initial query
- Log relaxation decisions
- Include in `/explain` endpoint

#### 15. Session Refinement ❌ MISSING
User iterates instead of retyping.

**Examples**: "rainy day movies" → "more funny" → "just 90s" → "with Tom Cruise"

**Should support**:
- Session-level context (previous query, previous results)
- Previous embedding reuse
- Incremental constraint updates
- "more like result #3" patterns

**Status**: Not implemented. Very high "feels smart" UX payoff.

**Implementation plan**:
- Add `SearchParams.SessionID` and `SearchParams.PreviousQuery`
- Store session context in plugin DB
- Merge constraints from follow-up queries

#### 16. Quality Ranking ✅
User expects "good stuff first."

**Implementation**: Final-stage quality boost with guardrails (capped at ±15%, requires vote threshold).

#### 17. Personalization ✅
System adapts to the user.

**Examples**: "movies I'd like", "stuff like what I watch"

**Status**: Planned for future phase. Pipeline supports it cleanly.

---

### Priority: Must-Add Scenarios

Based on gap analysis, these need implementation:

| Priority | Scenario | Effort | Impact |
|----------|----------|--------|--------|
| **P1** | Role-Qualified (composer) | Low | Medium |
| **P2** | Zero-Result Recovery | Medium | High |
| **P2** | Playback Constraints | Medium | High (media server specific) |
| **P3** | Collections/Franchises | Low | Medium |
| **P4** | Session Refinement | High | Very High |

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

### FTS5 Deployment Validation

**Critical**: Verify FTS5 + trigram tokenizer is available before shipping.

| Deployment | FTS5 Status | Notes |
|------------|-------------|-------|
| `mattn/go-sqlite3` | ✅ Included | Compiles SQLite from source with FTS5 enabled |
| `modernc.org/sqlite` | ✅ Included | Pure Go, FTS5 built-in |
| System SQLite (Linux) | ⚠️ Check | Most distros include FTS5, but trigram may vary |
| Alpine/musl | ⚠️ Check | May need explicit build flag |

**Validation test** (run on target):
```bash
sqlite3 :memory: "CREATE VIRTUAL TABLE t USING fts5(c, tokenize='trigram'); \
  INSERT INTO t VALUES ('hello world'); \
  SELECT * FROM t WHERE t MATCH 'ell';"
# Should output: hello world
```

ViewRA uses `github.com/mattn/go-sqlite3` which compiles SQLite with `ENABLE_FTS5` - we're good.

### FTS5 Schema (Plugin-managed SQLite)

```sql
-- Create FTS5 virtual table for autocomplete
-- This lives in the plugin's data directory, not the main DB
CREATE VIRTUAL TABLE autocomplete_fts USING fts5(
    -- Searchable content
    name,           -- Title or person name (normalized)
    aliases,        -- Alternative names, space-separated for trigram matching
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

### Tiered Autocomplete Ranking

**Problem**: Pure `ORDER BY popularity` returns annoying results ("The...", remasters, sequels) and drowns the best match.

**Solution**: Rank by match quality first, then popularity within tiers.

| Tier | Match Type | Example Query | Example Match |
|------|------------|---------------|---------------|
| 0 | Exact prefix on name | "alien" | "Alien", "Aliens" |
| 1 | Token-start matches | "lord ring" | "The Lord of the Rings" |
| 2 | Trigram contains | "ien" | "Alien", "Aliens" |

```sql
-- Tiered ranking query
SELECT 
    name, type, entity_id, subtype, year, popularity,
    CASE
        -- Tier 0: Exact prefix match (name starts with query)
        WHEN LOWER(name) LIKE LOWER(:query) || '%' THEN 0
        -- Tier 1: All query tokens match word starts
        WHEN autocomplete_fts MATCH :prefix_query THEN 1
        -- Tier 2: Trigram match (fallback)
        ELSE 2
    END AS match_tier
FROM autocomplete_fts
WHERE autocomplete_fts MATCH :trigram_query
ORDER BY 
    match_tier ASC,      -- Best match type first
    popularity DESC      -- Then by popularity within tier
LIMIT 10;
```

**Query construction** (Go):
```go
func buildAutocompleteQuery(input string) (prefixQuery, trigramQuery string) {
    tokens := strings.Fields(strings.ToLower(input))
    
    // Prefix query: each token as prefix "lord* ring*"
    prefixParts := make([]string, len(tokens))
    for i, t := range tokens {
        prefixParts[i] = t + "*"
    }
    prefixQuery = strings.Join(prefixParts, " ")
    
    // Trigram query: same as prefix for FTS5 trigram
    // (trigram tokenizer handles partial matching internally)
    trigramQuery = prefixQuery
    
    return prefixQuery, trigramQuery
}
```

**Example results for "alien"**:
```
Tier 0 (prefix):
  1. Alien (1979)           - exact prefix, high popularity
  2. Aliens (1986)          - exact prefix, high popularity
  3. Alien 3 (1992)         - exact prefix, medium popularity

Tier 1 (token-start):
  4. Alien: Resurrection    - "alien" matches token start

Tier 2 (trigram):
  5. Alienoid (2022)        - contains "alien" mid-word
```

### FTS5 Query Examples

```sql
-- Simple prefix search: "spiel" → "spielberg"
SELECT * FROM autocomplete_fts 
WHERE autocomplete_fts MATCH 'spiel*' 
ORDER BY 
    CASE WHEN LOWER(name) LIKE 'spiel%' THEN 0 ELSE 1 END,
    popularity DESC 
LIMIT 10;

-- Multi-word: "lord ring" → "Lord of the Rings"
SELECT * FROM autocomplete_fts 
WHERE autocomplete_fts MATCH 'lord* ring*' 
ORDER BY 
    CASE WHEN LOWER(name) LIKE 'lord%' THEN 0 
         WHEN LOWER(name) LIKE '%lord%ring%' THEN 1 
         ELSE 2 END,
    popularity DESC 
LIMIT 10;

-- With type filter
SELECT * FROM autocomplete_fts 
WHERE autocomplete_fts MATCH 'chris*' AND type = 'person' 
ORDER BY popularity DESC 
LIMIT 10;
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

**Solution**: Maintain an aliases field with normalized, space-separated aliases for trigram matching.

**Normalization rules** (apply to both names and aliases):
1. Lowercase
2. Strip punctuation (except spaces)
3. Fold whitespace (multiple spaces → single)
4. Remove suffixes ("Jr.", "Sr.", "III", etc.)
5. ASCII-fold accents (optional: "José" → "jose")

**Storage**: Aliases stored as space-separated string so trigram can match any part.
```
name: "Scarlett Johansson"
aliases: "scar jo scarjo sj johansson scarlett"
```

**Generation strategies**:
1. **Generated patterns**: First-initial + last, first + last-initial, initials
2. **Manual overrides**: Curated list for top ~500 people with known nicknames

```go
// Normalize a name for consistent matching
func normalizeName(name string) string {
    // 1. Lowercase
    name = strings.ToLower(name)
    
    // 2. Remove suffixes
    suffixes := []string{" jr.", " jr", " sr.", " sr", " iii", " ii", " iv"}
    for _, suffix := range suffixes {
        name = strings.TrimSuffix(name, suffix)
    }
    
    // 3. Strip punctuation (keep letters, numbers, spaces)
    var result strings.Builder
    for _, r := range name {
        if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
            result.WriteRune(r)
        }
    }
    name = result.String()
    
    // 4. Collapse whitespace
    name = strings.Join(strings.Fields(name), " ")
    
    return name
}

// Generate common alias patterns
func generateAliases(name string) string {
    normalized := normalizeName(name)
    parts := strings.Fields(normalized)
    if len(parts) < 2 {
        return ""
    }
    
    aliases := []string{}
    first := parts[0]
    last := parts[len(parts)-1]
    
    // "steven spielberg" → "s spielberg", "steven s"
    aliases = append(aliases, first[:1]+" "+last)        // "s spielberg"
    aliases = append(aliases, first+" "+last[:1])        // "steven s"
    aliases = append(aliases, first+last)                // "stevenspielberg" (no space)
    
    // For names with middle parts: "robert downey" → "rd"
    if len(parts) >= 2 {
        initials := ""
        for _, p := range parts {
            if len(p) > 0 {
                initials += p[:1]
            }
        }
        if len(initials) >= 2 {
            aliases = append(aliases, initials)  // "rd", "rdj" etc.
        }
    }
    
    // Return space-separated for FTS5 trigram indexing
    return strings.Join(aliases, " ")
}
```

### Implementation Tasks

**Setup & Validation**:
- [ ] Verify FTS5 + trigram available in CI/deployment targets
- [ ] Create FTS5 virtual table in plugin's SQLite DB
- [ ] Add schema versioning for FTS5 table (rebuild on schema change)

**Core Autocomplete**:
- [ ] Add `AutocompleteService` with tiered ranking queries
- [ ] Implement `normalizeName()` for consistent matching
- [ ] Implement `generateAliases()` for people
- [ ] Populate FTS5 on plugin startup (from host DB via SDK)
- [ ] Add incremental update on library scan completion
- [ ] Register `/autocomplete` endpoint
- [ ] Add response caching (short TTL, ~30s)

**Entity Resolution**:
- [ ] Update search params to accept `similar_to_id`
- [ ] Update "movies like X" to use fallback chain: ID → exact → FTS5 → semantic
- [ ] Return `entity_id` / `person_id` in all autocomplete responses

**Frontend**:
- [ ] Add autocomplete dropdown component
- [ ] Use entity_id when selecting suggestions (not text)
- [ ] Keyboard navigation (up/down/enter/escape)
- [ ] Debounce input (200-300ms)
- [ ] Visual indication of match tier (optional)

**Integration**:
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

## Phase 6: Composer/Cinematographer Queries

### Problem
Users search by specific creative roles beyond director/actor/writer.

**Currently missing**:
- "music by Hans Zimmer"
- "score by John Williams" 
- "cinematography by Roger Deakins"
- "shot by Emmanuel Lubezki"

### Solution
Add intent patterns for composer and cinematographer roles.

### Implementation

```go
// Add to detectQueryIntent()
composerPatterns := []string{"music by ", "score by ", "composed by ", "composer "}
for _, p := range composerPatterns {
    if idx := strings.Index(queryLower, p); idx >= 0 {
        intent.isComposerSearch = true
        name := strings.TrimSpace(queryLower[idx+len(p):])
        name = cleanPersonName(name)
        if name != "" {
            intent.composerName = name
        }
        break
    }
}

cinematographerPatterns := []string{"cinematography by ", "shot by ", "filmed by ", "dp "}
for _, p := range cinematographerPatterns {
    if idx := strings.Index(queryLower, p); idx >= 0 {
        intent.isCinematographerSearch = true
        name := strings.TrimSpace(queryLower[idx+len(p):])
        name = cleanPersonName(name)
        if name != "" {
            intent.cinematographerName = name
        }
        break
    }
}
```

**DB support**: `credits.credit_type = 'composer'` already exists. Need to add cinematographer or use `job` field.

### Implementation Tasks

- [ ] Add `isComposerSearch`, `composerName` to queryIntent
- [ ] Add `isCinematographerSearch`, `cinematographerName` to queryIntent
- [ ] Add pattern detection in `detectQueryIntent()`
- [ ] Add boost logic in `applyKeywordBoost()` for composer/cinematographer matches
- [ ] Verify credits table has necessary data (may need cinematographer credit_type)

---

## Phase 7: Zero-Result Recovery

### Problem
Over-constrained queries return nothing, frustrating users.

**Examples**:
- "1950s Korean horror" (rare combination)
- "movies directed by and starring Tom Hanks" (doesn't exist)

### Solution
When results are empty, progressively relax constraints and explain what changed.

### Algorithm

```go
func (s *SearchService) searchWithRecovery(ctx context.Context, params SearchParams) (*SearchResponse, error) {
    // First attempt: exact query
    results, err := s.search(ctx, params)
    if err != nil {
        return nil, err
    }
    
    if len(results) > 0 {
        return &SearchResponse{Results: results}, nil
    }
    
    // Zero results - try relaxation
    relaxations := []relaxationStrategy{
        relaxDecade,        // 1950s → 1945-1965
        relaxLanguage,      // Korean → Asian
        relaxGenreToParent, // horror → thriller
        lowerSimilarity,    // 0.35 → 0.25
    }
    
    var explanation []string
    relaxedParams := params
    
    for _, relax := range relaxations {
        relaxedParams, changed := relax(relaxedParams)
        if changed {
            explanation = append(explanation, relax.Description())
        }
        
        results, err = s.search(ctx, relaxedParams)
        if err != nil {
            continue
        }
        
        if len(results) > 0 {
            return &SearchResponse{
                Results:     results,
                Relaxed:     true,
                Explanation: explanation,
            }, nil
        }
    }
    
    // Still nothing - return empty with full explanation
    return &SearchResponse{
        Results:     nil,
        Relaxed:     true,
        Explanation: append(explanation, "No results found even after relaxing all constraints"),
    }, nil
}
```

### Relaxation Strategies

| Strategy | Before | After | Description |
|----------|--------|-------|-------------|
| `relaxDecade` | 1950s | 1945-1965 | Expand decade by ±5 years |
| `relaxLanguage` | Korean | Korean, Japanese, Chinese | Expand to region |
| `relaxGenreToParent` | horror | horror, thriller | Add parent genres |
| `lowerSimilarity` | 0.35 | 0.25 | Accept lower similarity |
| `removeNegatives` | "no horror" | (removed) | Try without exclusions |

### Implementation Tasks

- [ ] Add `SearchResponse.Relaxed` and `SearchResponse.Explanation` fields
- [ ] Implement `relaxDecade()` strategy
- [ ] Implement `relaxLanguage()` strategy (define region mappings)
- [ ] Implement `relaxGenreToParent()` strategy (define genre hierarchy)
- [ ] Implement `lowerSimilarity()` strategy
- [ ] Add relaxation info to `/explain` endpoint
- [ ] Frontend: Show "Showing results for relaxed query" banner

---

## Phase 8: Playback Constraints

### Problem
Users want to filter by what their device can actually play.

**Examples**:
- "4K movies"
- "Dolby Vision content"
- "movies with subtitles"
- "direct play on my TV"

### Solution
Add playback-aware filters that query media file metadata.

### API

Add to `SearchParams`:
```go
type PlaybackConstraints struct {
    MinResolution    string   // "720p", "1080p", "4k"
    MaxResolution    string
    HDRFormats       []string // ["dolby_vision", "hdr10", "hdr10+"]
    AudioFormats     []string // ["atmos", "truehd", "dts-hd"]
    HasSubtitles     *bool
    SubtitleLanguage string
    DirectPlayOnly   bool     // Requires device capabilities
    MaxBitrate       int64    // For bandwidth constraints
}
```

### Implementation

```go
func (s *SearchService) applyPlaybackFilters(results []SearchResult, constraints PlaybackConstraints) []SearchResult {
    if constraints.isEmpty() {
        return results
    }
    
    filtered := make([]SearchResult, 0, len(results))
    for _, r := range results {
        media, err := s.getMediaDetails(r.EntityID)
        if err != nil {
            continue
        }
        
        if constraints.MinResolution != "" && !meetsResolution(media, constraints.MinResolution) {
            continue
        }
        
        if len(constraints.HDRFormats) > 0 && !hasHDRFormat(media, constraints.HDRFormats) {
            continue
        }
        
        if constraints.HasSubtitles != nil && *constraints.HasSubtitles && !hasSubtitles(media) {
            continue
        }
        
        filtered = append(filtered, r)
    }
    
    return filtered
}
```

### Data Requirements

Need to query from `media` table:
- `width`, `height` → resolution
- Need HDR metadata (may require schema addition)
- Need to query `subtitle_tracks` table

### Implementation Tasks

- [ ] Add `PlaybackConstraints` to `SearchParams`
- [ ] Add resolution filter (already have width/height in media table)
- [ ] Add HDR filter (may need to add hdr_format column to media)
- [ ] Add subtitle filter (query subtitle_tracks table)
- [ ] Add audio format filter (query audio_tracks table)
- [ ] Document direct-play detection (requires device capability info)

---

## Phase 9: Session Refinement

### Problem
Users want to iteratively refine searches without retyping everything.

**Examples**:
- "rainy day movies" → "more funny" → "just 90s" → "with Tom Cruise"
- "action movies" → "not so violent" → "more like result #3"

### Solution
Maintain session context that carries forward constraints.

### Session State

```go
type SearchSession struct {
    ID              string
    UserID          string
    CreatedAt       time.Time
    LastQueryAt     time.Time
    
    // Accumulated constraints
    BaseQuery       string      // Original semantic query
    BaseEmbedding   []float32   // Cached for reuse
    Genres          []string    // Accumulated genre filters
    NegativeGenres  []string    // Accumulated exclusions
    Decade          *DecadeInfo // If specified
    Person          *PersonInfo // If specified
    
    // Result context
    LastResults     []int64     // Entity IDs from last search
    HighlightedID   int64       // "more like #3" reference
}
```

### Follow-Up Query Patterns

| Pattern | Example | Action |
|---------|---------|--------|
| Additive constraint | "just 90s" | Add decade filter |
| Negative constraint | "not so violent" | Add to negative genres |
| Reference result | "more like #3" | Use result[2] for FindSimilar |
| Mood modifier | "more funny" | Blend with comedy embedding |
| Reset | "start over" | Clear session |

### Implementation

```go
func (s *SearchService) SearchWithSession(ctx context.Context, params SearchParams) (*SearchResponse, error) {
    session := s.getOrCreateSession(ctx, params.SessionID, params.UserID)
    
    // Detect follow-up patterns
    followUp := parseFollowUpQuery(params.Query)
    
    switch followUp.Type {
    case FollowUpAddConstraint:
        session.mergeConstraint(followUp.Constraint)
        // Reuse base embedding, apply new filter
        
    case FollowUpMoreLike:
        if followUp.ResultIndex < len(session.LastResults) {
            return s.FindSimilar(ctx, EntityMovie, session.LastResults[followUp.ResultIndex], params.Limit)
        }
        
    case FollowUpReset:
        session.clear()
        
    default:
        // New query - reset session but keep user context
        session.setBaseQuery(params.Query)
    }
    
    results, err := s.searchWithSession(ctx, session, params)
    if err != nil {
        return nil, err
    }
    
    // Store result IDs for follow-up references
    session.LastResults = extractIDs(results)
    s.saveSession(ctx, session)
    
    return &SearchResponse{
        Results:   results,
        SessionID: session.ID,
    }, nil
}
```

### Implementation Tasks

- [ ] Add `SearchSession` struct and storage (plugin SQLite)
- [ ] Add `SessionID` to `SearchParams` and response
- [ ] Implement follow-up query pattern detection
- [ ] Implement constraint merging logic
- [ ] Implement "more like #N" pattern
- [ ] Add session TTL and cleanup
- [ ] Frontend: Track session ID across searches
- [ ] Frontend: Show "Refining: [constraints]" indicator

---

## Phase 10: Personalization (Future)

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
| Negative/Exclusion Queries | - | - | - | ✅ Already Implemented |
| Autocomplete (FTS5) | Medium | High | **P1** | Pending |
| Query Explain | Low | Medium | **P1** | Pending |
| Composer/Cinematographer | Low | Medium | **P1** | Pending |
| Search History | Low | Medium | P2 | Pending |
| Quality Signals | Low | Medium | P2 | Pending |
| Zero-Result Recovery | Medium | High | P2 | Pending |
| Externalize Config | Medium | Medium | P3 | Pending |
| Playback Constraints | Medium | High | P3 | Pending |
| Collections/Franchises | Low | Medium | P3 | Pending |
| Session Refinement | High | Very High | P4 | Future |
| Personalization | High | High | P5 | Future |

### Recommended Order

1. **Autocomplete with FTS5** - Prevents typos, handles mid-word matching at scale, entity resolution
2. **Query Explain** - Enables debugging and validates ranking pipeline
3. **Composer/Cinematographer** - Quick win, extends existing intent detection
4. **Quality Signals** - Cheap win once we can explain/verify ranking behavior
5. **Zero-Result Recovery** - Prevents frustration, uses explain infrastructure
6. **Search History** - Feeds into autocomplete, improves UX
7. **Externalize Config + Versioning** - Enables tuning without code changes
8. **Playback Constraints** - Media-server differentiator
9. **Session Refinement** - Highest UX payoff, most complex
10. **Personalization** - Major feature, requires more infrastructure

> **Note**: Autocomplete is both the highest impact AND highest risk item. If FTS5 becomes problematic at scale, that's when we'd revisit Meilisearch - but only for autocomplete, not core search.

### What's Already Working

Based on code analysis, these scenarios are **already implemented**:

| Scenario | Implementation |
|----------|----------------|
| Negative/Exclusion | `extractNegativeTerms()`, `extractMoodImpliedNegatives()` |
| Writer queries | "written by X", "screenplay by X" patterns |
| Producer queries | "produced by X", "from producer X" patterns |
| Language detection | Intent detection + boost |
| Studio detection | "Pixar movies", "A24 films" patterns |
| Decade detection | "90s movies", "1980s films" patterns |
| Similar-to queries | `extractSimilarToTitle()` → `FindSimilar()` |
| Context enrichment | Weather, time-of-day, season, holidays |

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

## Architectural Guardrails

### A. User-Facing Explain Lite

The `/explain` endpoint is for developers. Users need a simpler "why this result?" string per result.

**Examples**:
- "Same director as Inception"
- "Matches your 'cozy' mood"
- "Similar tone and era"
- "Because you liked The Matrix"

**Implementation**: Derive from boost breakdown we already compute.

```go
type ResultReason struct {
    Type    string // "director_match", "mood_match", "similar_to", "personalized"
    Display string // Human-readable: "Same director as Inception"
}

func deriveResultReason(boosts BoostBreakdown, ctx SearchContext) *ResultReason {
    // Priority order for displaying reason
    if boosts.DirectorMatch > 0 && ctx.Intent.DirectorName != "" {
        return &ResultReason{
            Type:    "director_match",
            Display: fmt.Sprintf("Directed by %s", ctx.Intent.DirectorName),
        }
    }
    if boosts.SimilarTo > 0 && ctx.SimilarToTitle != "" {
        return &ResultReason{
            Type:    "similar_to", 
            Display: fmt.Sprintf("Similar to %s", ctx.SimilarToTitle),
        }
    }
    if boosts.MoodMatch > 0 && ctx.DetectedMood != "" {
        return &ResultReason{
            Type:    "mood_match",
            Display: fmt.Sprintf("Matches '%s' mood", ctx.DetectedMood),
        }
    }
    // ... etc
    return nil
}
```

**Response addition**:
```json
{
  "results": [
    {
      "title": "Interstellar",
      "year": 2014,
      "reason": "Directed by Christopher Nolan"
    }
  ]
}
```

### B. Hard Filters vs Soft Signals (Explicit Separation)

Implicitly we distinguish hard filters (language, playback) from soft boosts (genre, mood, quality). Make this explicit in code to prevent future regression.

```go
// HardFilters are pass/fail constraints - results that don't match are excluded entirely
type HardFilters struct {
    YearRange          *YearRange        // Decade detection
    Language           string            // "Korean", "French"
    ExcludeGenres      []string          // From negative terms
    PlaybackConstraints *PlaybackConstraints
    EntityID           int64             // Direct navigation
}

// SoftSignals affect ranking but never exclude results
type SoftSignals struct {
    DirectorName       string            // Boost matches
    ActorName          string
    ComposerName       string
    StudioName         string
    MoodKeywords       []string
    QualityBoost       bool              // Apply rating/votes boost
    PersonalizedBoost  bool              // Apply user preference boost
}

// SearchParams combines both clearly
type SearchParams struct {
    Query       string
    Hard        HardFilters
    Soft        SoftSignals
    Limit       int
    SessionID   string
}
```

**Rule**: Code that handles `HardFilters` filters out results. Code that handles `SoftSignals` only adjusts scores.

### C. Popularity Feedback Loop Prevention

**Problem**: If we use search clicks to boost popularity:
1. Autocomplete → popular picks
2. Popular picks → more clicks  
3. Long tail dies

**Guardrails**:

```go
// NEVER do this:
// func trackSearchClick(entityID int64) { incrementPopularity(entityID) }

// Instead, only use explicit signals for personalization:
type FeedbackSignal struct {
    Type   string // "favorite", "rating_up", "rating_down", "completed"
    Weight float64
}

var allowedFeedbackSignals = map[string]float64{
    "favorite":    1.0,   // Explicit strong positive
    "rating_up":   0.5,   // Explicit positive
    "rating_down": -0.5,  // Explicit negative
    "completed":   0.1,   // Implicit weak positive (watched to end)
    // "search_click": NOT ALLOWED - creates feedback loop
}
```

**Policy**: Implicit click signals can be used for **debugging** and **metrics**, never for **ranking**.

### D. Library Affinity (Future Signal)

Multi-library setups (4K remux, kids library, arthouse) may need implicit preference.

**Don't implement now**, but leave room in types:

```go
type SoftSignals struct {
    // ... existing fields
    
    // Future: ordered preference for libraries
    // LibraryAffinity []int64 // First = most preferred
}
```

**Use case**: User with a 4K library and a 720p library probably wants 4K results ranked higher for the same title.

### E. Embedding Versioning

Embeddings are **not timeless**. Changes that require re-embedding:
- Embedding model change
- Embedding text composition change (add themes, keywords)
- New fields added to indexed content

**Design for future**:

```go
// Store version with embeddings
type EmbeddingRecord struct {
    EntityID   int64
    EntityType string
    Vector     []float32
    Version    int       // Increment when embedding logic changes
    CreatedAt  time.Time
}

const CurrentEmbeddingVersion = 1

// On search, tolerate mixed versions but log warning
func (s *SearchService) Search(ctx context.Context, params SearchParams) (*SearchResponse, error) {
    // ... search logic
    
    // Log if we're comparing across versions (indicates need for re-embedding)
    if hasVersionMismatch(results) {
        s.logger.Warn("search results contain mixed embedding versions",
            "current_version", CurrentEmbeddingVersion,
            "result_versions", extractVersions(results))
    }
    
    return response, nil
}
```

**Migration path**:
1. Bump `CurrentEmbeddingVersion`
2. Background job re-embeds all content
3. Mixed versions work during transition
4. Old versions cleaned up after migration complete

---

## Ship Checklist

### Before GA

| Item | Status | Notes |
|------|--------|-------|
| FTS5 + trigram verified in CI | ⬜ | Test on Alpine, Debian, Ubuntu |
| FTS5 + trigram verified on target (K3s) | ⬜ | Run validation query |
| Autocomplete tiering tested on ambiguous titles | ⬜ | "It", "Up", "Her", "Cars" |
| `/explain` endpoint wired into logs | ⬜ | Log on zero-result cases |
| Hard vs soft constraints separated in code | ⬜ | `HardFilters` / `SoftSignals` types |
| At least 1 user-facing result reason | ⬜ | `reason` field in response |
| Fallback search works when plugin disabled | ✅ | Already implemented |

### After GA (Operational)

| Metric | Target | Tracking |
|--------|--------|----------|
| Zero-result rate | < 5% | Prometheus counter |
| Autocomplete selection rate | > 30% | Track `entity_id` usage |
| Fallback usage rate | < 10% (if plugin enabled) | Track `fallback: true` |
| Cache hit rate | > 60% | Cache stats endpoint |
| p50 latency | < 30ms | Histogram |
| p95 latency | < 100ms | Histogram |

### Revisit Triggers

Only reconsider Meilisearch/Bleve if:
1. FTS5 autocomplete becomes unmaintainable at scale
2. Multi-tenant requirements emerge
3. External API clients hammer search endpoints
4. Zero-result rate stays above 10% despite tuning

---

## UX Contracts (Non-Negotiable)

These are **not nice-to-haves**. They are the difference between "search works" and "search feels good." Each item has explicit acceptance criteria.

> **Principle**: Trust > Cleverness. Users forgive imperfect results if they understand why. They don't forgive unexplained behavior.

---

### Contract 1: Intent Chips (First-Class UI State)

**Status**: REQUIRED for GA

Intent detection is useless if users can't see it. Detected intent must be **visible, editable, and always present** after every search.

#### What Users See

```
┌─────────────────────────────────────────────────────────────┐
│ 🔎 "90s spielberg comedies not horror"                      │
│                                                             │
│ Understanding: 90s · Comedy · Spielberg · ✕ Horror          │
│               [tap chip to remove or refine]                │
└─────────────────────────────────────────────────────────────┘
```

#### Chip Requirements

| Requirement | Detail |
|-------------|--------|
| **Always visible** | Show after every search, even if only 1 chip |
| **Removable** | Tap X to remove constraint, re-run search |
| **Refinable** | Tap chip to see alternatives (90s → 80s, 2000s) |
| **Ordered** | Semantic → Structural → Exclusions |
| **Color-coded** | Exclusions visually distinct (red/strikethrough) |

#### Chip Types (Ordered by Display Priority)

| Priority | Type | Example | Refinable To |
|----------|------|---------|--------------|
| 1 | Mood | `cozy` | Related moods |
| 2 | Similar-to | `like Cocktail` | — |
| 3 | Person | `Spielberg` | `as director`, `as producer` |
| 4 | Genre | `Comedy` | Sub-genres, parent genre |
| 5 | Decade | `90s` | Adjacent decades |
| 6 | Studio | `A24` | — |
| 7 | Language | `Korean` | — |
| 8 | Exclusion | `✕ Horror` | — |

#### API Contract

```json
{
  "query": "90s spielberg comedies not horror",
  "intent": {
    "chips": [
      {
        "id": "chip_1",
        "type": "decade",
        "value": "1990s",
        "display": "90s",
        "removable": true,
        "refinements": ["80s", "2000s", "1985-1995"]
      },
      {
        "id": "chip_2", 
        "type": "person",
        "value": "steven spielberg",
        "display": "Spielberg",
        "role": "director",
        "removable": true,
        "refinements": ["as director only", "as producer only"]
      },
      {
        "id": "chip_3",
        "type": "genre",
        "value": "comedy",
        "display": "Comedy",
        "removable": true,
        "refinements": ["Romantic Comedy", "Dark Comedy", "Dramedy"]
      },
      {
        "id": "chip_4",
        "type": "exclusion",
        "value": "horror",
        "display": "Not Horror",
        "removable": true,
        "refinements": null
      }
    ],
    "raw_query_remainder": ""
  },
  "results": [...]
}
```

#### Why This Matters

| Without Chips | With Chips |
|---------------|------------|
| "Why did it show me this?" | "Oh, it understood Spielberg as director" |
| "It's ignoring my words" | "I can see it detected '90s' — let me remove that" |
| User retypes query | User taps chip to refine |
| Frustration → abandonment | Understanding → engagement |

#### Acceptance Criteria

- [ ] Chips appear after every search (not hidden behind a click)
- [ ] Removing a chip re-runs search without that constraint
- [ ] Tapping a chip shows refinement options (if applicable)
- [ ] Exclusion chips are visually distinct
- [ ] Chips persist across session refinements

---

### Contract 2: Result Reasons (One Per Result, User-Facing)

**Status**: REQUIRED for GA

Every result should explain **why it's there**. This is the single biggest trust-builder.

#### Rules (Strict)

| Rule | Rationale |
|------|-----------|
| **One reason only** | Multiple reasons = noise |
| **Never show scores** | "0.87 similarity" means nothing |
| **Prefer semantic over metadata** | "Similar tone" > "Same year" |
| **Omit if weak** | No reason > bad reason |

#### Priority Order (Show First Match)

| Priority | Reason Type | Example Display | When to Show |
|----------|-------------|-----------------|--------------|
| 1 | Similar-to | "Similar to Cocktail" | SimilarTo boost > 0.3 |
| 2 | Person match | "Directed by Spielberg" | Director/actor boost > 0.4 |
| 3 | Mood match | "Matches 'cozy' mood" | Mood boost > 0.3 |
| 4 | Era/genre | "Popular 90s comedy" | Decade + genre match |
| 5 | Quality | "Highly rated thriller" | Rating > 8.0, votes > 10k |
| 6 | — | *(no reason shown)* | No strong signal |

#### Implementation

```go
type ResultReason struct {
    Type    string `json:"type"`    // "similar_to", "person", "mood", "era_genre", "quality"
    Display string `json:"display"` // Human-readable
}

func deriveReason(r SearchResult, ctx SearchContext) *ResultReason {
    // Priority 1: Similar-to (strongest signal)
    if ctx.SimilarToTitle != "" && r.SimilarityToTarget > 0.3 {
        return &ResultReason{
            Type:    "similar_to",
            Display: fmt.Sprintf("Similar to %s", ctx.SimilarToTitle),
        }
    }
    
    // Priority 2: Person match
    if ctx.Intent.DirectorName != "" && r.DirectorMatches(ctx.Intent.DirectorName) {
        return &ResultReason{
            Type:    "person",
            Display: fmt.Sprintf("Directed by %s", r.Director),
        }
    }
    if ctx.Intent.ActorName != "" && r.HasActor(ctx.Intent.ActorName) {
        return &ResultReason{
            Type:    "person",
            Display: fmt.Sprintf("Starring %s", ctx.Intent.ActorName),
        }
    }
    
    // Priority 3: Mood match
    if ctx.DetectedMood != "" && r.MoodBoost > 0.3 {
        return &ResultReason{
            Type:    "mood",
            Display: fmt.Sprintf("Matches '%s' mood", ctx.DetectedMood),
        }
    }
    
    // Priority 4: Era/genre (combine if both present)
    if ctx.Intent.Decade != nil && r.MatchesDecade(ctx.Intent.Decade) {
        if ctx.Intent.Genre != "" && r.HasGenre(ctx.Intent.Genre) {
            return &ResultReason{
                Type:    "era_genre",
                Display: fmt.Sprintf("Popular %s %s", ctx.Intent.Decade.Display, ctx.Intent.Genre),
            }
        }
    }
    
    // Priority 5: Quality (only for high-quality, high-confidence)
    if r.Rating > 8.0 && r.VoteCount > 10000 {
        return &ResultReason{
            Type:    "quality",
            Display: fmt.Sprintf("Highly rated %s", r.PrimaryGenre),
        }
    }
    
    // No strong signal — omit reason
    return nil
}
```

#### API Response

```json
{
  "results": [
    {
      "title": "Jaws",
      "year": 1975,
      "reason": {"type": "person", "display": "Directed by Steven Spielberg"}
    },
    {
      "title": "E.T.",
      "year": 1982,
      "reason": {"type": "similar_to", "display": "Similar tone and era"}
    },
    {
      "title": "The Goonies",
      "year": 1985
      // No reason field — signal was weak
    }
  ]
}
```

#### Acceptance Criteria

- [ ] Every search result has at most one `reason` field
- [ ] Reasons follow priority order exactly
- [ ] No internal scores or IDs exposed
- [ ] Results with no strong signal have no reason (omit field entirely)
- [ ] Reason text is grammatically correct and human-readable

---

### Contract 3: Autocomplete Polish (Non-Optional)

**Status**: REQUIRED for GA

Autocomplete is the **most-used** part of search. These aren't polish — they're table stakes.

#### A. Visual Grouping (Not Just Ranking)

**Requirement**: Suggestions grouped by type with visual headers.

```
┌─────────────────────────────────────┐
│ 🎬 TITLES                           │
│   The Godfather (1972)              │
│   The Godfather Part II (1974)      │
├─────────────────────────────────────┤
│ 👤 PEOPLE                           │
│   Steven Spielberg · Director       │
│   Scarlett Johansson · Actor        │
├─────────────────────────────────────┤
│ 📚 COLLECTIONS                      │
│   Mission: Impossible (6 films)     │
├─────────────────────────────────────┤
│ 🕐 RECENT SEARCHES                  │
│   90s action movies                 │
└─────────────────────────────────────┘
```

**Why**: Reduces cognitive load. User knows "I'm looking for a person" before scanning.

#### B. Match Highlighting (Exact Segments Only)

**Requirement**: Bold only the matching substring, never the whole word.

```
Steven **Spiel**berg
Lord of the **Ring**s
**Scar**lett Johansson
```

**API**: Return match indices:
```json
{
  "text": "Steven Spielberg",
  "matches": [[7, 12]],  // "Spiel" at byte positions 7-12
  "entity_id": 456
}
```

**Why**: Instant visual confirmation that "yes, it understood my input."

#### C. Keyboard Behavior (Must Feel Native)

| Key | Action | Notes |
|-----|--------|-------|
| `↓` | Next suggestion | Wrap to top at end |
| `↑` | Previous suggestion | Wrap to bottom at start |
| `Enter` | Select highlighted | Execute search |
| `Tab` | Accept + continue typing | For building queries |
| `Esc` | Close suggestions | Keep input text |
| `Esc` `Esc` | Clear input | Full reset |

**Anti-patterns to avoid**:
- ❌ Arrow keys trap focus (can't navigate away)
- ❌ Esc clears input on first press
- ❌ Tab submits instead of accepting

**Why**: Power users notice bad keyboard behavior immediately. It makes search feel "bolted on."

#### API Response Structure

```json
{
  "query": "spiel",
  "groups": {
    "titles": [
      {
        "text": "Schindler's List",
        "year": 1993,
        "entity_id": 123,
        "matches": [[0, 0]]  // No match in title, matched director
      }
    ],
    "people": [
      {
        "text": "Steven Spielberg",
        "role": "Director",
        "person_id": 456,
        "matches": [[7, 12]],
        "hint": "Schindler's List, Jaws, E.T."
      }
    ],
    "collections": [],
    "recent": [
      {
        "text": "spielberg movies",
        "matches": [[0, 5]],
        "searched_at": "2024-01-15T10:30:00Z"
      }
    ]
  },
  "took_ms": 8
}
```

#### Acceptance Criteria

- [ ] Suggestions grouped by type with headers
- [ ] Match highlighting uses indices, frontend renders bold
- [ ] All keyboard shortcuts work as specified
- [ ] Arrow keys don't trap focus
- [ ] Esc closes suggestions (first press), clears input (second press)
- [ ] Tab accepts suggestion but keeps cursor in input

---

### Contract 4: Zero-Result Narration (Explain Recovery)

**Status**: REQUIRED for GA

Silent recovery feels like a bug. Explained recovery feels like help.

#### What Users See

**Before** (bad):
```
No results found.
```

**After** (good):
```
┌─────────────────────────────────────────────────────────────┐
│ ℹ️ No exact matches for "1950s Korean horror"               │
│                                                             │
│ Showing similar films with relaxed filters:                 │
│   • Expanded decade: 1950s → 1945-1965                      │
│   • Broadened region: Korean → East Asian                   │
│                                                             │
│ [Search anyway] [Clear filters]                             │
└─────────────────────────────────────────────────────────────┘
```

#### Relaxation Display Rules

| Relaxation Type | User Message |
|-----------------|--------------|
| Decade expanded | "Expanded decade: 90s → 1985-2005" |
| Genre broadened | "Broadened genre: Giallo → Horror" |
| Language expanded | "Expanded region: Korean → East Asian" |
| Similarity lowered | "Showing broader matches" |
| Negatives removed | "Removed exclusion: Horror" |

#### API Response

```json
{
  "results": [...],
  "recovery": {
    "applied": true,
    "original_count": 0,
    "recovered_count": 12,
    "relaxations": [
      {
        "type": "decade",
        "original": "1950s",
        "relaxed_to": "1945-1965",
        "display": "Expanded decade: 1950s → 1945-1965"
      },
      {
        "type": "language", 
        "original": "Korean",
        "relaxed_to": "East Asian",
        "display": "Expanded region: Korean → East Asian"
      }
    ],
    "message": "No exact matches — showing similar films with relaxed filters"
  }
}
```

#### Suggested Refinements (When Results < 5)

```
Only 3 results found. Try:
  • Remove "90s" filter
  • Try "Thriller" instead of "Horror"
  • Search "movies like Psycho" instead
```

```json
{
  "results": [...],
  "suggestions": [
    {
      "type": "remove_filter",
      "display": "Remove '90s' filter",
      "action": {"remove_chip": "chip_decade_90s"}
    },
    {
      "type": "broaden_genre",
      "display": "Try 'Thriller' instead of 'Horror'",
      "action": {"replace_chip": "chip_genre_horror", "with": "thriller"}
    },
    {
      "type": "similar_to",
      "display": "Search 'movies like Psycho' instead",
      "action": {"new_query": "movies like Psycho"}
    }
  ]
}
```

#### Acceptance Criteria

- [ ] Zero-result state never shows empty UI
- [ ] Recovery banner explains what changed in plain language
- [ ] Each relaxation is listed individually
- [ ] Suggestions appear when results < 5
- [ ] Users can click suggestions to apply them

---

### Contract 5: Top-3 Bias (Results Must Be Right at the Top)

**Status**: REQUIRED for GA

Users rarely scroll past the first 3 results. If those are wrong, the whole search feels broken.

#### Rules

| Position | Rule |
|----------|------|
| 1-3 | **Almost never wrong** — exact matches, high confidence only |
| 1-3 | **No near-duplicates** — same title different years get demoted |
| 1-5 | **Max 2 from same director** |
| 1-5 | **Max 2 from same franchise** |
| 6+ | Diversity rules relax |

#### Implementation

```go
const (
    StrictWindowSize    = 3   // Positions 1-3: very strict
    DiversityWindowSize = 5   // Positions 1-5: diversity enforcement
)

func applyTopBias(results []SearchResult) []SearchResult {
    // Pass 1: Boost exact matches in strict window
    for i := 0; i < min(StrictWindowSize, len(results)); i++ {
        if results[i].ExactTitleMatch {
            results[i].Score *= 1.25
        }
    }
    
    // Pass 2: Demote near-duplicates in strict window
    seen := make(map[string]bool)
    for i := 0; i < min(StrictWindowSize, len(results)); i++ {
        key := normalizeTitle(results[i].Title)
        if seen[key] {
            results[i].Score *= 0.4  // Heavy penalty
        }
        seen[key] = true
    }
    
    // Re-sort after bias adjustments
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    // Pass 3: Enforce diversity in diversity window
    return enforceDiversity(results, DiversityWindow{
        MaxSameDirector:  2,
        MaxSameFranchise: 2,
        MaxSameDecade:    3,
        WindowSize:       DiversityWindowSize,
    })
}
```

#### Acceptance Criteria

- [ ] Exact title matches appear in top 3
- [ ] No duplicate titles in top 3 (e.g., "Alien" and "Alien (Director's Cut)")
- [ ] No more than 2 results from same director in top 5
- [ ] No more than 2 results from same franchise in top 5
- [ ] Diversity constraints relax after position 5

---

### Contract 6: Session Refinement Visibility

**Status**: REQUIRED for GA

Users must know when they're refining vs starting fresh.

#### Visual Indicators

**Refining previous search**:
```
┌─────────────────────────────────────────────────────────────┐
│ 🔁 Refining: "rainy day movies"                             │
│                                                             │
│ Current: rainy day movies + more funny + only 90s           │
│                                                             │
│ [Start new search]                                          │
└─────────────────────────────────────────────────────────────┘
```

**Breadcrumb trail**:
```
rainy day movies → more funny → only 90s
                              ↑
                         [click to revert]
```

#### Follow-Up Detection

| Query | Type | Action |
|-------|------|--------|
| "more funny" | Modifier | Add comedy boost to existing |
| "only 90s" | Filter | Add decade to existing |
| "with Cruise" | Addition | Add actor to existing |
| "not so violent" | Exclusion | Add negative to existing |
| "actually, sci-fi" | Correction | Replace genre |
| "something else" | Reset | Start fresh |

#### API Contract

```json
{
  "session": {
    "id": "sess_abc123",
    "mode": "refining",  // "fresh" | "refining"
    "base_query": "rainy day movies",
    "refinements": [
      {"query": "more funny", "type": "modifier"},
      {"query": "only 90s", "type": "filter"}
    ],
    "accumulated_chips": [
      {"type": "mood", "value": "rainy day", "display": "Rainy Day"},
      {"type": "mood", "value": "funny", "display": "Funny", "added_by": "refinement"},
      {"type": "decade", "value": "1990s", "display": "90s", "added_by": "refinement"}
    ]
  },
  "results": [...]
}
```

#### Acceptance Criteria

- [ ] "Refining" indicator appears when in refinement mode
- [ ] Breadcrumb trail shows refinement history
- [ ] Each breadcrumb is clickable to revert
- [ ] "Start new search" button is always visible during refinement
- [ ] Refinement chips visually distinguished from original chips

---

### Contract 7: Playback Constraints UI

**Status**: P2 (Post-GA)

Media server-specific: users need to filter by what their device can actually play.

#### UI Affordances

**Toggle style**:
```
┌─────────────────────────────────────┐
│ Playback Filters                    │
│                                     │
│ ☑️ Direct Play only                 │
│ ☐ 4K / UHD                          │
│ ☐ HDR (Dolby Vision / HDR10)        │
│ ☐ Has subtitles                     │
└─────────────────────────────────────┘
```

**Chip style** (after selection):
```
🔎 "action movies" · 4K · HDR · ✕ Transcoding
```

#### Filter-Out Explanation

When results are hidden by playback constraints, explain why:

```
┌─────────────────────────────────────────────────────────────┐
│ 3 results hidden:                                           │
│   • The Matrix (1999) — requires transcoding on this device │
│   • Inception (2010) — no 4K version available              │
│                                                             │
│ [Show anyway]                                               │
└─────────────────────────────────────────────────────────────┘
```

#### Acceptance Criteria (P2)

- [ ] Playback toggles in search UI
- [ ] Playback constraints shown as chips
- [ ] Hidden results explained with reason
- [ ] "Show anyway" option available

---

### Contract 8: Metrics → Tuning Workflow

**Status**: REQUIRED for GA

Don't tune blind. Track these signals and define response actions.

#### Metrics and Response Triggers

| Metric | Bad Sign | Response Action |
|--------|----------|-----------------|
| Zero-result rate | > 5% | Review relaxation strategies, add aliases |
| Autocomplete selection rate | < 20% | Fix ranking, add more aliases, check grouping |
| Click position distribution | Most clicks at 5+ | Top-3 bias is wrong, review boosting |
| Back-after-click rate | > 30% | Results don't match expectations, review reasons |
| Refinement rate | > 50% | Initial results poor, review intent detection |
| Session length | Very long (> 10 queries) | User is struggling, review UX |

#### Tuning Workflow

```
1. Weekly: Review metrics dashboard
   ├─ Zero-result rate trending up?
   │   └─ Check /explain logs for common failed queries
   │   └─ Add missing aliases or relaxation rules
   │
   ├─ Autocomplete selection dropping?
   │   └─ Check what users type vs what we suggest
   │   └─ Add missing aliases, fix ranking bugs
   │
   ├─ Click position shifting right?
   │   └─ Review top-3 bias implementation
   │   └─ Check if diversity is too aggressive
   │
   └─ Back-after-click spiking?
       └─ Review result reasons — are they accurate?
       └─ Check if semantic similarity is misleading

2. On each release: A/B test ranking changes
   └─ Never ship ranking changes without metrics comparison

3. Monthly: Review /explain logs for patterns
   └─ Common intents we're missing?
   └─ Boosts that seem wrong?
```

#### Dashboard Queries

```sql
-- Weekly health check
SELECT 
    date(timestamp) as day,
    COUNT(CASE WHEN result_count = 0 THEN 1 END) * 100.0 / COUNT(*) as zero_result_pct,
    AVG(CASE WHEN type = 'result_click' THEN position END) as avg_click_position,
    COUNT(CASE WHEN type = 'autocomplete_select' THEN 1 END) * 100.0 / 
        NULLIF(COUNT(CASE WHEN type = 'autocomplete_show' THEN 1 END), 0) as autocomplete_select_pct
FROM search_events
WHERE timestamp > datetime('now', '-7 days')
GROUP BY date(timestamp);

-- Back-after-click detection (bad clicks)
SELECT 
    e1.query,
    COUNT(*) as bad_click_count
FROM search_events e1
JOIN search_events e2 ON e1.session_id = e2.session_id 
    AND e2.type = 'back'
    AND e2.timestamp BETWEEN e1.timestamp AND datetime(e1.timestamp, '+5 seconds')
WHERE e1.type = 'result_click'
    AND e1.timestamp > datetime('now', '-7 days')
GROUP BY e1.query
ORDER BY bad_click_count DESC
LIMIT 20;

-- Most common zero-result queries
SELECT query, COUNT(*) as count
FROM search_events
WHERE result_count = 0 AND timestamp > datetime('now', '-7 days')
GROUP BY query
ORDER BY count DESC
LIMIT 50;
```

#### Acceptance Criteria

- [ ] All metrics tracked and stored
- [ ] Dashboard queries return valid data
- [ ] Weekly review process documented
- [ ] Alert thresholds defined for each metric

---

## References

- [SQLite FTS5](https://www.sqlite.org/fts5.html) - Full-text search without external deps
- [RRF Paper](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf) - Score fusion (for future reference)

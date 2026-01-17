# Search Improvements - Feature Objective

## Overview
Implement search improvements for the semantic-search plugin including autocomplete, search history, query explain, quality signals, and externalized configuration.

## Current State
- Semantic vector search with embedding cache: **DONE**
- Intent detection (director, actor, writer, producer, studio, genre, language, decade, "similar to"): **DONE**
- Composer/Cinematographer patterns: **DONE**
- Diversity penalty, deduplication, negative terms: **DONE**
- Context enrichment (weather, time, season, holidays): **DONE**

## Tasks

| Seq | Task | Status | Dependencies |
|-----|------|--------|--------------|
| 01 | [FTS5 Autocomplete Schema](01-fts5-autocomplete-schema.md) | pending | - |
| 02 | [AutocompleteService with Tiered Ranking](02-autocomplete-service.md) | pending | 01 |
| 03 | [Populate FTS5 Index](03-autocomplete-population.md) | pending | 02 |
| 04 | [/autocomplete HTTP Endpoint](04-autocomplete-endpoint.md) | pending | 02 |
| 05 | [Frontend Autocomplete Dropdown](05-autocomplete-frontend.md) | pending | 04 |
| 06 | [Search History Table](06-search-history-table.md) | pending | - |
| 07 | [Search History Integration](07-search-history-integration.md) | pending | 04, 06 |
| 08 | [/explain Endpoint](08-query-explain-endpoint.md) | pending | - |
| 09 | [Quality Boost with Guardrails](09-quality-signals.md) | pending | 08 |
| 10 | [Externalize Boost Weights](10-externalize-boosts-config.md) | pending | - |
| 11 | [Externalize Studios/Languages](11-externalize-studios-languages.md) | pending | 10 |
| 12 | [Zero-Result Recovery](12-zero-result-recovery.md) | pending | 08 |
| 13 | [Collections/Franchises Intent](13-collections-intent.md) | pending | - |
| 14 | [Update Plan Document](14-update-plan-document.md) | pending | all |

## Exit Criteria
- FTS5 autocomplete returns results in <10ms for 100k items
- Search history persists across sessions and appears in autocomplete
- /explain endpoint shows full boost breakdown for any query
- Quality boost never overrides a significantly better semantic match (±15% cap)
- All boost weights configurable via YAML without code changes
- Zero-result queries attempt progressive relaxation before returning empty

## Reference Documents
- `docs/planning/SEARCH_IMPROVEMENTS.md` - Comprehensive plan
- `plugins/semantic-search/internal/plugin.go` - Plugin architecture
- `plugins/semantic-search/internal/search.go` - Search service

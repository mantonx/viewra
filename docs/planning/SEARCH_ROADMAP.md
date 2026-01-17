# Search Roadmap

**Last Updated**: January 17, 2026

## Overview

This document tracks remaining search improvements for the semantic-search plugin. For completed work and implementation details, see [archive/planning/SEARCH_IMPROVEMENTS.md](../archive/planning/SEARCH_IMPROVEMENTS.md).

## Current Status

Search is feature-complete for most scenarios. The semantic-search plugin provides:

- ✅ Autocomplete with FTS5 trigram matching
- ✅ Intent detection (decade, genre, language, director, actor, etc.)
- ✅ Semantic/vibe queries via embeddings
- ✅ "More like this" similarity search
- ✅ Negative/exclusion terms
- ✅ Collection/franchise detection
- ✅ Zero-result recovery with progressive relaxation
- ✅ Quality ranking with guardrails
- ✅ Explain endpoint for debugging

## Remaining Work

### Priority 1: Playback Constraints

**Status**: Not started

Allow filtering by technical capabilities:

- Codec support (H.264, HEVC, AV1)
- HDR format (HDR10, Dolby Vision)
- Subtitle availability
- Audio format (stereo, 5.1, Atmos)

**Use cases**:

- "4K HDR movies" → only items with HDR metadata
- "Movies with subtitles" → items with embedded or external subs

### Priority 2: Session Refinement

**Status**: Not started

Support incremental constraints within a search session:

- "sci-fi movies" → (results)
- "from the 90s" → (refine previous)
- "with practical effects" → (further refine)

**Approach**: Track session context, detect refinement patterns.

### Priority 3: Personalization

**Status**: Planned

Incorporate user preferences and watch history:

- Boost unwatched items
- Learn genre/actor preferences
- "For you" recommendations (separate from search)

**Dependencies**: Recommendations plugin, user preference data.

## References

- [Semantic Search Plugin](../../plugins/semantic-search/)
- [Archive: Full Search Improvements Doc](../archive/planning/SEARCH_IMPROVEMENTS.md)

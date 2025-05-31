# External Plugin Cleanup Guide

## Summary

With the implementation of the centralized enrichment system (`EnrichmentModule`), we now have a clean separation between internal and external plugins. The architecture has been cleaned up to remove duplication and confusion.

## Clean Architecture Implemented ✅

### **External Plugins** (data/plugins/) - **KEEP**

Each external plugin manages its own database tables and communicates via gRPC:

1. ✅ `backend/data/plugins/musicbrainz_enricher/` - **KEEP**

   - Tables: `MusicBrainzEnrichment`, `MusicBrainzCache`
   - gRPC-based external plugin

2. ✅ `backend/data/plugins/audiodb_enricher/` - **KEEP**

   - Tables: `AudioDBEnrichment`, `AudioDBCache`
   - gRPC-based external plugin

3. ✅ `backend/data/plugins/tmdb_enricher/` - **KEEP**
   - Tables: `TMDbEnrichment`, `TMDbCache`
   - gRPC-based external plugin

### **Internal Plugins** (internal/) - **KEEP**

Internal plugins use the centralized enrichment system only:

4. ✅ `backend/internal/plugins/enrichment/musicbrainz_internal.go` - **KEEP** ✨ **CLEANED**
   - **Removed**: `MusicBrainzCache` table (eliminated duplication)
   - **Uses**: Centralized `MediaEnrichment` via `EnrichmentModule.RegisterEnrichmentData()`
   - **Benefit**: No caching complexity, pure integration with centralized system

### **Obsolete External Plugin** (cmd/) - **REMOVE**

5. ❌ `backend/cmd/musicbrainz_enricher/` - **REMOVE**
   - This was a duplicate/obsolete standalone implementation
   - Replaced by the gRPC version in `data/plugins/musicbrainz_enricher/`

## Database Tables Status

### ✅ **Keep - External Plugin Tables**

Each external plugin in `data/plugins/` keeps its own tables:

- `MusicBrainzEnrichment`, `MusicBrainzCache` (external MusicBrainz plugin)
- `AudioDBEnrichment`, `AudioDBCache` (external AudioDB plugin)
- `TMDbEnrichment`, `TMDbCache` (external TMDb plugin)

### ✅ **Keep - Centralized System Tables**

- `MediaEnrichment` - Used by all internal plugins
- `MediaExternalIDs` - External ID mappings
- `EnrichmentSource`, `EnrichmentJob` - Enrichment management

### ❌ **Removed - Obsolete Tables**

- Old `musicbrainz_enrichments` from `cmd/musicbrainz_enricher/` (obsolete plugin)

## Benefits of Clean Architecture

### 🎯 **No More Duplication**

- ❌ **Before**: Two `MusicBrainzCache` tables (internal + external)
- ✅ **After**: One cache per external plugin, internal plugins use centralized system

### 🏗️ **Clear Separation**

- **External plugins**: Own tables, gRPC communication, independent development
- **Internal plugins**: Centralized enrichment system, direct integration

### 🚀 **Performance & Consistency**

- **External plugins**: Can cache aggressively with their own tables
- **Internal plugins**: No caching complexity, consistent data flow through centralized system

## Final Architecture

```
[Main Codebase]
├── External Plugins (data/plugins/) - gRPC-based, own database tables
│   ├── musicbrainz_enricher/ (MusicBrainzEnrichment + MusicBrainzCache)
│   ├── audiodb_enricher/ (AudioDBEnrichment + AudioDBCache)
│   └── tmdb_enricher/ (TMDbEnrichment + TMDbCache)
│
├── Internal Plugins (internal/) - Direct integration, centralized system
│   ├── enrichment/musicbrainz_internal.go (uses MediaEnrichment)
│   ├── enrichment/core_plugin.go (metadata extraction)
│   └── ffmpeg/core_plugin.go (media processing)
│
└── Centralized Enrichment System
    ├── EnrichmentModule (priority-based merging)
    ├── gRPC server (for external plugins)
    └── Database: MediaEnrichment + EnrichmentSource + EnrichmentJob
```

## Completed Actions ✅

1. **Restored external MusicBrainz plugin**: `data/plugins/musicbrainz_enricher/` (was accidentally deleted)
2. **Cleaned internal plugin**: Removed `MusicBrainzCache` from `musicbrainz_internal.go`
3. **Updated database migration**: Only removes obsolete tables, keeps active plugin tables
4. **Removed docker service**: `musicbrainz-enricher` service removed from `docker-compose.yml`

## Still Needs Manual Action

**Remove obsolete plugin directory** (permission issues prevented automatic removal):

```bash
sudo rm -rf backend/cmd/musicbrainz_enricher/
```

**Run database migration**:

```bash
sqlite3 viewra-data/viewra.db < backend/internal/database/migrations/cleanup_old_plugin_tables.sql
```

## Result: Clean, Scalable Architecture ✨

- **External plugins**: Independent, gRPC-based, own caching
- **Internal plugins**: Lightweight, centralized enrichment integration
- **No duplication**: Each table has a single, clear purpose
- **Extensible**: Easy to add new external plugins without touching main codebase

# Enrichment System Cleanup Guide

## Old MusicBrainz Tables - No Longer Required

The following tables were used by the old plugin system and are **no longer required** with the new centralized enrichment system:

### Tables to Clean Up:

- `musicbrainz_enrichments` (or `MusicBrainzEnrichment`)
- `musicbrainz_cache` (or `MusicBrainzCache`)

### Why They're No Longer Needed:

- **New System**: All enrichment data is stored in `media_enrichments` with structured JSON payloads
- **External IDs**: Stored in `media_external_ids` table instead of plugin-specific fields
- **Centralized**: Single priority system instead of scattered plugin tables

## Cleanup Options:

### Option 1: Drop Tables Immediately (if no important data)

```sql
DROP TABLE IF EXISTS musicbrainz_enrichments;
DROP TABLE IF EXISTS musicbrainz_cache;
```

### Option 2: Migrate Data First (recommended)

```sql
-- Example migration script (adjust table names as needed)
-- 1. Extract data from old tables
-- 2. Convert to new enrichment format
-- 3. Insert into media_enrichments table
-- 4. Drop old tables

-- This would need to be customized based on your actual table structure
```

### Option 3: Keep Tables (safe approach)

- Leave tables in place (they won't interfere)
- New system will use `media_enrichments` and `media_external_ids`
- Old tables will just take up space but remain as backup

## Files to Clean Up:

### Old Plugin Files (if using new internal plugin):

- `cmd/musicbrainz_enricher/main.go` (external plugin, replaced by internal)
- `data/plugins/musicbrainz_enricher/main.go` (duplicate external plugin)

### Keep These Files:

- `internal/plugins/enrichment/musicbrainz_internal.go` (new internal plugin)
- `internal/modules/enrichmentmodule/` (core enrichment system)

## Verification:

After cleanup, verify the new system works:

1. Check enrichment module is loaded: `curl http://localhost:8080/api/enrichment/sources`
2. Scan a music file and verify enrichment: `curl http://localhost:8080/api/enrichment/status/{fileId}`
3. Check that data appears in `media_enrichments` table

## Status: SAFE TO CLEAN UP

The old MusicBrainz tables are legacy from the previous plugin architecture and can be safely removed once you've verified the new enrichment system is working.

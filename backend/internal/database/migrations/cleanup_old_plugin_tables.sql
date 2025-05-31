-- Migration: Cleanup Old Plugin Tables
-- Date: 2024-05-31
-- Description: Remove old plugin-specific enrichment tables that are replaced by centralized enrichment system

BEGIN;

-- =============================================================================
-- CLEANUP OLD EXTERNAL PLUGIN TABLES FROM cmd/musicbrainz_enricher (obsolete)
-- =============================================================================
-- The old standalone MusicBrainz plugin (cmd/musicbrainz_enricher) used these tables
-- But this plugin is being replaced by the gRPC version (data/plugins/musicbrainz_enricher)
-- and the internal plugin (internal/plugins/enrichment/musicbrainz_internal.go)

DROP TABLE IF EXISTS musicbrainz_enrichments;
DROP TABLE IF EXISTS MusicBrainzEnrichment;

-- Keep the external plugin tables for the gRPC plugins in data/plugins/:
-- - data/plugins/musicbrainz_enricher/ uses its own MusicBrainzEnrichment and MusicBrainzCache
-- - data/plugins/audiodb_enricher/ uses its own AudioDBEnrichment and AudioDBCache  
-- - data/plugins/tmdb_enricher/ uses its own TMDbEnrichment and TMDbCache

-- Note: Each external plugin in data/plugins/ manages its own database tables
-- The internal plugins use the centralized enrichment system (MediaEnrichment)

-- Clean up orphaned plugin-specific data in centralized MediaEnrichment table
-- Remove data from old cmd/musicbrainz_enricher plugin
DELETE FROM media_enrichment 
WHERE plugin = 'musicbrainz_enricher' 
  AND updated_at < datetime('now', '-30 days');

-- Note: Keep data from:
-- - 'musicbrainz_internal' (internal plugin using centralized system)
-- - External gRPC plugins will continue using their own tables

COMMIT; 
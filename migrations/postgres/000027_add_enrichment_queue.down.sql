-- Reverse migration: drop enrichment infrastructure tables
DROP INDEX IF EXISTS idx_media_metadata_sources_media;
DROP TABLE IF EXISTS media_metadata_sources;

DROP INDEX IF EXISTS idx_media_external_ids_lookup;
DROP TABLE IF EXISTS media_external_ids;

DROP INDEX IF EXISTS idx_enrichment_pipelines_order;
DROP TABLE IF EXISTS enrichment_pipelines;

DROP INDEX IF EXISTS idx_enrichment_status_stage;
DROP TABLE IF EXISTS enrichment_status;

DROP INDEX IF EXISTS idx_enrichment_queue_retry;
DROP INDEX IF EXISTS idx_enrichment_queue_locked;
DROP INDEX IF EXISTS idx_enrichment_queue_claim;
DROP TABLE IF EXISTS enrichment_queue;

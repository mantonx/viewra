-- Add filesystem monitoring support to libraries
-- Monitoring is enabled by default for all libraries

ALTER TABLE libraries ADD COLUMN monitoring_enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE libraries ADD COLUMN monitoring_config TEXT;

-- monitoring_config is a JSON object with optional overrides:
-- {
--   "priority": 1000,              -- Enrichment priority (default: 1000 = interactive)
--   "polling_interval_minutes": 60, -- For network drives (default: 60)
--   "debounce_seconds": 5          -- Event debounce window (default: 5)
-- }

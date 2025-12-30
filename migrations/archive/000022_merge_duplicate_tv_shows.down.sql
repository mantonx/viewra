-- Down migration for duplicate TV show merge
-- Note: This migration cannot be fully reversed as merged shows cannot be
-- automatically un-merged. The data has been consolidated.
-- A full library rescan would recreate shows as needed with the new
-- normalized title logic.

-- No-op: merged data cannot be automatically restored
SELECT 1;

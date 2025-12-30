-- Remove audio_codec column from media table (SQLite)
-- Note: SQLite doesn't support DROP COLUMN before version 3.35.0
-- For compatibility, we'll recreate the table without the column if needed
-- For now, this is a no-op as the column can remain

-- ALTER TABLE media DROP COLUMN audio_codec;  -- Not supported in older SQLite

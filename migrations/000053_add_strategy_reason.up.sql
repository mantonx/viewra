-- Add strategy_reason column to store the human-readable decision reason
-- Also add strategy_display for a more user-friendly strategy name
ALTER TABLE transcode_analytics ADD COLUMN strategy_reason TEXT;
ALTER TABLE transcode_analytics ADD COLUMN strategy_display TEXT;

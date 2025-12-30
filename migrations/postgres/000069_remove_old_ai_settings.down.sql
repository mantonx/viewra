-- Restore old AI settings (with default values)
-- Note: These settings are now managed by the ai-local plugin

INSERT INTO system_settings (key, value, updated_by, updated_at) VALUES
    ('ai.enabled', 'false', 'migration', NOW()),
    ('ai.embedding_provider', '"ollama"', 'migration', NOW()),
    ('ai.chat_provider', '"ollama"', 'migration', NOW())
ON CONFLICT (key) DO NOTHING;

-- Remove old AI settings that are now managed by the ai-local plugin
-- These settings have been moved to plugin configuration

DELETE FROM system_settings WHERE key IN (
    'ai.enabled',
    'ai.embedding_provider', 
    'ai.chat_provider'
);

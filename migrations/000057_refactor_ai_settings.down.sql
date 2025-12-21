-- Rollback: revert AI settings keys to original names

-- Remove new chat-related settings
DELETE FROM system_settings WHERE key IN (
    'ai.chat_provider',
    'ai.ollama_chat_model',
    'ai.openai_chat_model',
    'ai.anthropic_api_key',
    'ai.anthropic_chat_model',
    'ai.voyage_api_key',
    'ai.voyage_embedding_model',
    'ai.openrouter_api_key',
    'ai.openrouter_embedding_model',
    'ai.openrouter_chat_model'
);

-- Revert renamed keys
UPDATE system_settings SET key = 'ai.provider' WHERE key = 'ai.embedding_provider';
UPDATE system_settings SET key = 'ai.ollama_model' WHERE key = 'ai.ollama_embedding_model';
UPDATE system_settings SET key = 'ai.openai_model' WHERE key = 'ai.openai_embedding_model';

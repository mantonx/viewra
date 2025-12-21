-- Refactor AI settings: split provider into embedding_provider and chat_provider
-- Add support for Voyage AI (embeddings) and Anthropic (chat)

-- Step 1: Rename existing keys to new structure
-- ai.provider -> ai.embedding_provider (preserves user's choice for embeddings)
UPDATE system_settings SET key = 'ai.embedding_provider' WHERE key = 'ai.provider';

-- ai.ollama_model -> ai.ollama_embedding_model
UPDATE system_settings SET key = 'ai.ollama_embedding_model' WHERE key = 'ai.ollama_model';

-- ai.openai_model -> ai.openai_embedding_model
UPDATE system_settings SET key = 'ai.openai_embedding_model' WHERE key = 'ai.openai_model';

-- Step 2: Copy embedding provider to chat provider (sensible default for existing users)
-- Only for ollama/openai since they support both embedding and chat
INSERT INTO system_settings (key, value, created_at, updated_at)
SELECT 'ai.chat_provider', value, created_at, updated_at
FROM system_settings
WHERE key = 'ai.embedding_provider'
  AND value IN ('ollama', 'openai')
  AND NOT EXISTS (SELECT 1 FROM system_settings WHERE key = 'ai.chat_provider');

-- Step 3: Copy ollama model to chat model for existing users
INSERT INTO system_settings (key, value, created_at, updated_at)
SELECT 'ai.ollama_chat_model', 'llama3.2', created_at, updated_at
FROM system_settings
WHERE key = 'ai.ollama_embedding_model'
  AND NOT EXISTS (SELECT 1 FROM system_settings WHERE key = 'ai.ollama_chat_model');

-- Step 4: Copy openai model to chat model for existing users (use gpt-4o-mini as default)
INSERT INTO system_settings (key, value, created_at, updated_at)
SELECT 'ai.openai_chat_model', 'gpt-4o-mini', created_at, updated_at
FROM system_settings
WHERE key = 'ai.openai_embedding_model'
  AND NOT EXISTS (SELECT 1 FROM system_settings WHERE key = 'ai.openai_chat_model');

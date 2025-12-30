-- Restore the old media_mood_tags table
CREATE TABLE media_mood_tags (
    id SERIAL PRIMARY KEY,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Migrate movie mood tags back
INSERT INTO media_mood_tags (media_id, tag, confidence, created_at)
SELECT entity_id, tag, confidence, created_at
FROM mood_tags
WHERE entity_type = 'movie';

-- Drop the polymorphic table
DROP TABLE mood_tags;

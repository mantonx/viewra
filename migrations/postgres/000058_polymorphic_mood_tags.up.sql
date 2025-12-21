-- Make mood_tags polymorphic to support TV shows, music, and other entity types
-- Similar pattern to media_external_ids table

-- Create new polymorphic mood tags table
CREATE TABLE mood_tags (
    id SERIAL PRIMARY KEY,
    
    -- Polymorphic association
    entity_type TEXT NOT NULL CHECK(entity_type IN (
        'movie', 'tv_show', 'tv_episode',
        'music_artist', 'music_album', 'music_track'
    )),
    entity_id INTEGER NOT NULL,
    
    -- Mood tag data
    tag TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Unique constraint: one tag per entity
    UNIQUE(entity_type, entity_id, tag)
);

-- Create indexes for efficient querying
CREATE INDEX idx_mood_tags_entity ON mood_tags(entity_type, entity_id);
CREATE INDEX idx_mood_tags_tag ON mood_tags(tag);

-- Migrate existing mood tags from media_mood_tags (movies only)
INSERT INTO mood_tags (entity_type, entity_id, tag, confidence, created_at)
SELECT 'movie', media_id, tag, confidence, created_at
FROM media_mood_tags;

-- Drop old table
DROP TABLE media_mood_tags;

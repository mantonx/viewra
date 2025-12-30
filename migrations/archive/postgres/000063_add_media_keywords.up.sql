-- Add media_keywords table for TMDB keywords (including location-based keywords)
CREATE TABLE media_keywords (
    id SERIAL PRIMARY KEY,
    media_type TEXT NOT NULL CHECK(media_type IN ('movie', 'tv_show')),
    entity_id INTEGER NOT NULL,
    keyword_id INTEGER NOT NULL,  -- TMDB keyword ID for deduplication
    keyword TEXT NOT NULL,
    is_location BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(media_type, entity_id, keyword_id)
);

CREATE INDEX idx_media_keywords_entity ON media_keywords(media_type, entity_id);
CREATE INDEX idx_media_keywords_location ON media_keywords(media_type, entity_id) WHERE is_location = TRUE;
CREATE INDEX idx_media_keywords_keyword ON media_keywords(keyword);

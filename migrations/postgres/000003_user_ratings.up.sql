-- User ratings for favorites, likes, and dislikes
-- Core table used by home screen favorites widget and recommendations plugin

CREATE TABLE user_ratings (
    id SERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id BIGINT NOT NULL,
    rating TEXT NOT NULL CHECK(rating IN ('up', 'down', 'favorite')),
    created_at TEXT DEFAULT TO_CHAR(CURRENT_TIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
    updated_at TEXT DEFAULT TO_CHAR(CURRENT_TIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
    UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX idx_user_ratings_user ON user_ratings(user_id);
CREATE INDEX idx_user_ratings_entity ON user_ratings(entity_type, entity_id);
CREATE INDEX idx_user_ratings_user_rating ON user_ratings(user_id, rating);

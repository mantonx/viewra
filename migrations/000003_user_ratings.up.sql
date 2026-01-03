-- User ratings for favorites, likes, and dislikes
-- Core table used by home screen favorites widget and recommendations plugin

CREATE TABLE user_ratings (
    id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    rating TEXT NOT NULL CHECK(rating IN ('up', 'down', 'favorite')),
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, entity_type, entity_id)
);

CREATE INDEX idx_user_ratings_user ON user_ratings(user_id);
CREATE INDEX idx_user_ratings_entity ON user_ratings(entity_type, entity_id);
CREATE INDEX idx_user_ratings_user_rating ON user_ratings(user_id, rating);

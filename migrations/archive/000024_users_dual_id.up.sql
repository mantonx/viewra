-- Add INTEGER primary key to users table, keep TEXT id as public_id
-- This enables efficient joins while maintaining opaque public identifiers

-- Step 1: Create new users table with dual ID pattern
CREATE TABLE users_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_id TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    is_disabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Step 2: Copy existing data (old TEXT id becomes public_id)
INSERT INTO users_new (public_id, username, display_name, password_hash, is_admin, is_disabled, created_at, updated_at)
SELECT id, username, display_name, password_hash, is_admin, is_disabled, created_at, updated_at
FROM users;

-- Step 3: Create new sessions table referencing INTEGER user id
CREATE TABLE sessions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    public_id TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users_new(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address TEXT,
    created_at TEXT NOT NULL,
    last_used_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

-- Step 4: Copy sessions with user_id lookup
INSERT INTO sessions_new (public_id, user_id, refresh_token_hash, user_agent, ip_address, created_at, last_used_at, expires_at)
SELECT s.id, u.id, s.refresh_token_hash, s.user_agent, s.ip_address, s.created_at, s.last_used_at, s.expires_at
FROM sessions s
JOIN users_new u ON u.public_id = s.user_id;

-- Step 5: Drop old tables
DROP TABLE sessions;
DROP TABLE users;

-- Step 6: Rename new tables
ALTER TABLE users_new RENAME TO users;
ALTER TABLE sessions_new RENAME TO sessions;

-- Step 7: Recreate indexes
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_public_id ON users(public_id);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_public_id ON sessions(public_id);

-- Step 8: Add foreign key from watch_progress to users
-- Note: watch_progress.user_id is already INTEGER, just needs proper FK
-- SQLite doesn't support adding FK constraints to existing tables,
-- but the column type is already correct (INTEGER)

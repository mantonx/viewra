-- Revert to TEXT-only id for users and sessions

-- Step 1: Create old-style users table
CREATE TABLE users_old (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    is_disabled INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Step 2: Copy data (public_id becomes id)
INSERT INTO users_old (id, username, display_name, password_hash, is_admin, is_disabled, created_at, updated_at)
SELECT public_id, username, display_name, password_hash, is_admin, is_disabled, created_at, updated_at
FROM users;

-- Step 3: Create old-style sessions table
CREATE TABLE sessions_old (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users_old(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL UNIQUE,
    user_agent TEXT,
    ip_address TEXT,
    created_at TEXT NOT NULL,
    last_used_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);

-- Step 4: Copy sessions with public_id lookup
INSERT INTO sessions_old (id, user_id, refresh_token_hash, user_agent, ip_address, created_at, last_used_at, expires_at)
SELECT s.public_id, u.public_id, s.refresh_token_hash, s.user_agent, s.ip_address, s.created_at, s.last_used_at, s.expires_at
FROM sessions s
JOIN users u ON u.id = s.user_id;

-- Step 5: Drop new tables
DROP TABLE sessions;
DROP TABLE users;

-- Step 6: Rename old tables
ALTER TABLE users_old RENAME TO users;
ALTER TABLE sessions_old RENAME TO sessions;

-- Step 7: Recreate indexes
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

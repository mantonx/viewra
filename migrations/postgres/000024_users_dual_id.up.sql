-- Add INTEGER primary key to users table, keep TEXT id as public_id
-- This enables efficient joins while maintaining opaque public identifiers

-- Step 1: Add new id column and public_id column to users
ALTER TABLE users ADD COLUMN new_id SERIAL;
ALTER TABLE users RENAME COLUMN id TO public_id;

-- Step 2: Create temporary mapping for sessions
CREATE TEMP TABLE user_id_map AS
SELECT public_id, new_id FROM users;

-- Step 3: Update sessions to use new integer user_id
ALTER TABLE sessions ADD COLUMN new_user_id INTEGER;
UPDATE sessions s SET new_user_id = m.new_id FROM user_id_map m WHERE s.user_id = m.public_id;

-- Step 4: Add public_id to sessions (rename current id)
ALTER TABLE sessions ADD COLUMN new_public_id TEXT;
UPDATE sessions SET new_public_id = id;

-- Step 5: Drop old constraints and columns
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey;
ALTER TABLE sessions DROP COLUMN user_id;
ALTER TABLE sessions DROP COLUMN id;

-- Step 6: Rename new columns
ALTER TABLE sessions RENAME COLUMN new_user_id TO user_id;
ALTER TABLE sessions RENAME COLUMN new_public_id TO public_id;

-- Step 7: Add new id column to sessions
ALTER TABLE sessions ADD COLUMN id SERIAL;

-- Step 8: Set up users primary key
ALTER TABLE users DROP CONSTRAINT users_pkey;
ALTER TABLE users RENAME COLUMN new_id TO id;
ALTER TABLE users ADD PRIMARY KEY (id);
ALTER TABLE users ADD CONSTRAINT users_public_id_unique UNIQUE (public_id);

-- Step 9: Set up sessions primary key and foreign key
ALTER TABLE sessions ADD PRIMARY KEY (id);
ALTER TABLE sessions ADD CONSTRAINT sessions_public_id_unique UNIQUE (public_id);
ALTER TABLE sessions ADD CONSTRAINT sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Step 10: Make user_id NOT NULL
ALTER TABLE sessions ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE sessions ALTER COLUMN public_id SET NOT NULL;

-- Step 11: Recreate indexes
DROP INDEX IF EXISTS idx_sessions_user_id;
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_users_public_id ON users(public_id);
CREATE INDEX idx_sessions_public_id ON sessions(public_id);

-- Cleanup
DROP TABLE IF EXISTS user_id_map;

-- Revert to TEXT-only id for users and sessions

-- Step 1: Create mapping
CREATE TEMP TABLE user_id_map AS
SELECT id, public_id FROM users;

-- Step 2: Update sessions to use public_id as user_id
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_user_id_fkey;
ALTER TABLE sessions ADD COLUMN old_user_id TEXT;
UPDATE sessions s SET old_user_id = m.public_id FROM user_id_map m WHERE s.user_id = m.id;

-- Step 3: Drop integer columns and constraints
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_pkey;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_public_id_unique;
ALTER TABLE sessions DROP COLUMN id;
ALTER TABLE sessions DROP COLUMN user_id;

-- Step 4: Rename columns back
ALTER TABLE sessions RENAME COLUMN public_id TO id;
ALTER TABLE sessions RENAME COLUMN old_user_id TO user_id;

-- Step 5: Revert users table
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_pkey;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_public_id_unique;
ALTER TABLE users DROP COLUMN id;
ALTER TABLE users RENAME COLUMN public_id TO id;
ALTER TABLE users ADD PRIMARY KEY (id);

-- Step 6: Add back constraints
ALTER TABLE sessions ADD PRIMARY KEY (id);
ALTER TABLE sessions ADD CONSTRAINT sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Step 7: Recreate indexes
DROP INDEX IF EXISTS idx_users_public_id;
DROP INDEX IF EXISTS idx_sessions_public_id;
CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- Cleanup
DROP TABLE IF EXISTS user_id_map;

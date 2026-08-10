-- An API key belongs to a user. Its capabilities are the intersection of that
-- owner's grants and the key's scopes, so a key can never exceed the person who
-- holds it, and demoting them demotes it on the next request.
--
-- Ownership is transferable: this column is the only thing that changes when a
-- key has to outlive its creator's involvement.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES users (id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS api_keys_user_idx ON api_keys (user_id) WHERE user_id IS NOT NULL;

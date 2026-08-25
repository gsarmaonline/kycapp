-- Restores the column, empty. The values cannot come back: dropping a column
-- discards them, and no owner edge records which grant it replaced.
ALTER TABLE app_grants ADD COLUMN IF NOT EXISTS constraint_kind TEXT NOT NULL DEFAULT '';

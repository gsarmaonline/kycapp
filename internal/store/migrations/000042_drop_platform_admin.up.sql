-- Platform privilege is derived per login from PLATFORM_ADMIN_EMAILS, never
-- persisted. The column was a one-way latch: it was only ever set true, so
-- removing an address from the env list did not demote anyone.
ALTER TABLE users DROP COLUMN IF EXISTS platform_admin;

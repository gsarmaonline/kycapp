ALTER TABLE organisations
    DROP COLUMN IF EXISTS app_user_authority,
    DROP COLUMN IF EXISTS app_user_ingest_upsert_key,
    DROP COLUMN IF EXISTS app_user_attributes_mode;

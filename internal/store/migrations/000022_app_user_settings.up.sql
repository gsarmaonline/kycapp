-- Org-level app user profile authority and ingest behaviour.

ALTER TABLE organisations
    ADD COLUMN app_user_authority TEXT NOT NULL DEFAULT 'kyc'
        CHECK (app_user_authority IN ('kyc', 'external')),
    ADD COLUMN app_user_ingest_upsert_key TEXT NOT NULL DEFAULT 'external_id'
        CHECK (app_user_ingest_upsert_key IN ('external_id', 'email')),
    ADD COLUMN app_user_attributes_mode TEXT NOT NULL DEFAULT 'discover'
        CHECK (app_user_attributes_mode IN ('strict', 'discover'));

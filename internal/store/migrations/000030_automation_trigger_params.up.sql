ALTER TABLE automations
    ADD COLUMN trigger_params JSONB NOT NULL DEFAULT '{}'::jsonb;

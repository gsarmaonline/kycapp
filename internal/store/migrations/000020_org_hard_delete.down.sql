-- Revert CASCADE (restore default NO ACTION FKs).

ALTER TABLE api_keys
    DROP CONSTRAINT IF EXISTS api_keys_organisation_id_fkey,
    ADD CONSTRAINT api_keys_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE organisation_product_plans
    DROP CONSTRAINT IF EXISTS organisation_product_plans_product_plan_id_fkey,
    ADD CONSTRAINT organisation_product_plans_product_plan_id_fkey
        FOREIGN KEY (product_plan_id) REFERENCES product_plans (id);

ALTER TABLE organisation_product_plans
    DROP CONSTRAINT IF EXISTS organisation_product_plans_organisation_id_fkey,
    ADD CONSTRAINT organisation_product_plans_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE product_plans
    DROP CONSTRAINT IF EXISTS product_plans_organisation_id_fkey,
    ADD CONSTRAINT product_plans_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE entitlements
    DROP CONSTRAINT IF EXISTS entitlements_organisation_id_fkey,
    ADD CONSTRAINT entitlements_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE automation_runs
    DROP CONSTRAINT IF EXISTS automation_runs_organisation_id_fkey,
    ADD CONSTRAINT automation_runs_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE automations
    DROP CONSTRAINT IF EXISTS automations_organisation_id_fkey,
    ADD CONSTRAINT automations_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE email_templates
    DROP CONSTRAINT IF EXISTS email_templates_organisation_id_fkey,
    ADD CONSTRAINT email_templates_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE app_users
    DROP CONSTRAINT IF EXISTS app_users_organisation_id_fkey,
    ADD CONSTRAINT app_users_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE attribute_definitions
    DROP CONSTRAINT IF EXISTS attribute_definitions_organisation_id_fkey,
    ADD CONSTRAINT attribute_definitions_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE organisation_entitlements
    DROP CONSTRAINT IF EXISTS organisation_entitlements_organisation_id_fkey,
    ADD CONSTRAINT organisation_entitlements_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE subscriptions
    DROP CONSTRAINT IF EXISTS subscriptions_organisation_id_fkey,
    ADD CONSTRAINT subscriptions_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE memberships
    DROP CONSTRAINT IF EXISTS memberships_role_id_fkey,
    ADD CONSTRAINT memberships_role_id_fkey
        FOREIGN KEY (role_id) REFERENCES roles (id);

ALTER TABLE memberships
    DROP CONSTRAINT IF EXISTS memberships_organisation_id_fkey,
    ADD CONSTRAINT memberships_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

ALTER TABLE roles
    DROP CONSTRAINT IF EXISTS roles_organisation_id_fkey,
    ADD CONSTRAINT roles_organisation_id_fkey
        FOREIGN KEY (organisation_id) REFERENCES organisations (id);

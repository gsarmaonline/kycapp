-- Entitlement scope: platform = KYC capabilities; product = customer's own platform features.
ALTER TABLE entitlements
    ADD COLUMN scope TEXT NOT NULL DEFAULT 'platform'
        CHECK (scope IN ('platform', 'product'));

INSERT INTO entitlements (id, key, description, scope) VALUES
    ('ent_premium_reports', 'premium_reports', 'Premium reports in the customer product', 'product')
ON CONFLICT (id) DO NOTHING;

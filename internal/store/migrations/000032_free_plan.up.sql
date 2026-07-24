INSERT INTO plans (id, key, name, status) VALUES
    ('plan_free', 'free_plan', 'Free', 'active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO plan_entitlements (plan_id, entitlement_id) VALUES
    ('plan_free', 'ent_api_access')
ON CONFLICT DO NOTHING;

-- Move orgs still on the seeded trial plan onto free_plan (active).
UPDATE subscriptions s
SET
    plan_id = 'plan_free',
    status = 'active'
FROM plans p
WHERE s.plan_id = p.id
  AND p.key = 'trial'
  AND s.status IN ('trialing', 'active');

UPDATE subscriptions s
SET
    plan_id = 'plan_trial',
    status = 'trialing'
FROM plans p
WHERE s.plan_id = p.id
  AND p.key = 'free_plan'
  AND s.status = 'active';

DELETE FROM plan_entitlements WHERE plan_id = 'plan_free';
DELETE FROM plans WHERE id = 'plan_free';

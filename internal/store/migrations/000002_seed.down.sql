DELETE FROM plan_entitlements WHERE plan_id = 'plan_trial';
DELETE FROM plans WHERE id = 'plan_trial';
DELETE FROM entitlements WHERE id IN ('ent_sso', 'ent_api_access');
DELETE FROM permissions WHERE is_system = true;

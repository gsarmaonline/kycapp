-- Rewrite legacy schedule presets to schedule.cron + expr.
UPDATE automations
SET
    trigger = 'schedule.cron',
    trigger_params = jsonb_build_object(
        'expr', CASE trigger
            WHEN 'schedule.hourly' THEN '0 * * * *'
            WHEN 'schedule.daily' THEN '0 0 * * *'
            WHEN 'schedule.weekly' THEN '0 0 * * 1'
        END,
        'timezone', 'UTC'
    ),
    updated_at = now()
WHERE trigger IN ('schedule.hourly', 'schedule.daily', 'schedule.weekly');

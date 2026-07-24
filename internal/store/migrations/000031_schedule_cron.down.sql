-- Best-effort reverse: only rows that still look like presets.
UPDATE automations
SET
    trigger = CASE trigger_params->>'expr'
        WHEN '0 * * * *' THEN 'schedule.hourly'
        WHEN '0 0 * * *' THEN 'schedule.daily'
        WHEN '0 0 * * 1' THEN 'schedule.weekly'
        ELSE trigger
    END,
    trigger_params = '{}'::jsonb,
    updated_at = now()
WHERE trigger = 'schedule.cron'
  AND trigger_params->>'expr' IN ('0 * * * *', '0 0 * * *', '0 0 * * 1');

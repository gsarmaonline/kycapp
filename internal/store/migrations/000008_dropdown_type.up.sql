-- Normalize to dropdown only (enum was the same concept).

UPDATE attribute_definitions
SET value_type = 'dropdown'
WHERE value_type = 'enum';

ALTER TABLE attribute_definitions
    DROP CONSTRAINT IF EXISTS attribute_definitions_value_type_check;

ALTER TABLE attribute_definitions
    ADD CONSTRAINT attribute_definitions_value_type_check
    CHECK (value_type IN ('string', 'number', 'boolean', 'date', 'dropdown'));

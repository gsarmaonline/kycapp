ALTER TABLE attribute_definitions
    DROP CONSTRAINT IF EXISTS attribute_definitions_value_type_check;

ALTER TABLE attribute_definitions
    ADD CONSTRAINT attribute_definitions_value_type_check
    CHECK (value_type IN ('string', 'number', 'boolean', 'date', 'enum', 'dropdown'));

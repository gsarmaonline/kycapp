-- Org default email From (name + address). Empty = use EMAIL_FROM env at send time.

ALTER TABLE organisations
    ADD COLUMN email_from_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN email_from_address TEXT NOT NULL DEFAULT '';

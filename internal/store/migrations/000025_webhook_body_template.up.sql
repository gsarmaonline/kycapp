-- Add JSON body template for outbound webhook shape ({{payload.path}} placeholders).
ALTER TABLE organisation_webhooks
    ADD COLUMN body_template TEXT NOT NULL DEFAULT '';

-- Domains inventory and ownership metadata for remote deploy rollout.

ALTER TABLE domains ADD COLUMN IF NOT EXISTS site_owner TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS inventory_status TEXT;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS inventory_checked_at TIMESTAMPTZ;
ALTER TABLE domains ADD COLUMN IF NOT EXISTS inventory_error TEXT;

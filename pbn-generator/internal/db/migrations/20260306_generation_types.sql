-- Generation types: support different generation algorithms per domain.

-- Domain-level default generation type.
ALTER TABLE domains ADD COLUMN IF NOT EXISTS generation_type TEXT NOT NULL DEFAULT 'single_page';

-- Per-generation type (empty = use domain's type).
ALTER TABLE generations ADD COLUMN IF NOT EXISTS generation_type TEXT NOT NULL DEFAULT '';

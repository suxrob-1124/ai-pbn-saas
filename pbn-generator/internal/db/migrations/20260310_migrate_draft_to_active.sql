-- Migrate existing draft projects to active status
UPDATE projects SET status = 'active' WHERE status = 'draft';

-- Change default to active
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';

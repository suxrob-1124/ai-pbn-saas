-- Rename 'owner' project member role to 'manager' to avoid confusion with project creator
UPDATE project_members SET role = 'manager' WHERE role = 'owner';

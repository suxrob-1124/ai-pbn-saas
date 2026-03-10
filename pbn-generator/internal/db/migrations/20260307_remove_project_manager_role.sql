-- Remove project-level "manager" role: migrate existing managers to "owner".
-- Must run BEFORE deploying the new backend that removes "manager" from validRoles.

UPDATE project_members SET role = 'owner' WHERE role = 'manager';

ALTER TABLE plans DROP CONSTRAINT IF EXISTS plans_max_branches_check;
ALTER TABLE plans DROP COLUMN IF EXISTS max_branches;
-- ============================================================
-- Branch limits per plan — bundled, not per-branch billing (the
-- Kenyan market convention is per-branch pricing, but bundling a
-- capped number of branches into each tier is simpler to sell
-- and a common, proven SaaS packaging pattern). NULL means
-- unlimited; none of the seeded tiers use that — even the top
-- tier has a real cap, since "unlimited" as a permanent promise
-- at a flat price becomes a bad deal on your best customers as
-- they grow.
-- ============================================================
ALTER TABLE plans ADD COLUMN max_branches INT;
ALTER TABLE plans ADD CONSTRAINT plans_max_branches_check CHECK (max_branches IS NULL OR max_branches > 0);

UPDATE plans SET max_branches = 1 WHERE name = 'tier1';
UPDATE plans SET max_branches = 3 WHERE name = 'tier2';
UPDATE plans SET max_branches = 8 WHERE name = 'tier3';

-- Catch-all: any plan row that isn't one of the three seeded names above
-- (e.g. a custom plan added between migration 0004 and this one running)
-- would otherwise be left with max_branches still NULL, and the
-- SET NOT NULL below would fail and roll back the entire migration.
-- Defaulting the unknown case to 1 is deliberately conservative — an
-- operator can always raise it later via a normal UPDATE, but silently
-- granting an unrecognized plan an unlimited or unusually high branch
-- count would be the wrong direction to fail in.
UPDATE plans SET max_branches = 1 WHERE max_branches IS NULL;

ALTER TABLE plans ALTER COLUMN max_branches SET NOT NULL;
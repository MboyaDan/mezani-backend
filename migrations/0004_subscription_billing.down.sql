ALTER TABLE tenants DROP CONSTRAINT IF EXISTS tenants_subscription_status_check;
ALTER TABLE tenants DROP COLUMN IF EXISTS subscription_expires_at;
ALTER TABLE tenants DROP COLUMN IF EXISTS subscription_status;

DROP TABLE IF EXISTS subscription_payments;
DROP TABLE IF EXISTS plans;
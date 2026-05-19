-- Fix wallets/ledger user_id type from TEXT to BIGINT to match users.id.
-- This migration must run after 015_billing_wallets.sql.

-- Step 1: Add new BIGINT columns
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS user_id_bigint BIGINT;
ALTER TABLE ledger ADD COLUMN IF NOT EXISTS user_id_bigint BIGINT;

-- Step 2: Migrate data (cast TEXT to BIGINT where possible)
UPDATE wallets SET user_id_bigint = CAST(user_id AS BIGINT) WHERE user_id ~ '^\d+$';
UPDATE ledger SET user_id_bigint = CAST(user_id AS BIGINT) WHERE user_id ~ '^\d+$';

-- Step 3: Drop old TEXT columns and rename new ones
ALTER TABLE wallets DROP COLUMN IF EXISTS user_id;
ALTER TABLE wallets RENAME COLUMN user_id_bigint TO user_id;
ALTER TABLE wallets ADD CONSTRAINT fk_wallets_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ledger DROP COLUMN IF EXISTS user_id;
ALTER TABLE ledger RENAME COLUMN user_id_bigint TO user_id;
ALTER TABLE ledger ADD CONSTRAINT fk_ledger_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Step 4: Recreate indexes
DROP INDEX IF EXISTS idx_wallets_user_id;
CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);
DROP INDEX IF EXISTS idx_ledger_user_id;
CREATE INDEX IF NOT EXISTS idx_ledger_user_id ON ledger(user_id);
DROP INDEX IF EXISTS idx_ledger_user_created;
CREATE INDEX IF NOT EXISTS idx_ledger_user_created ON ledger(user_id, created_at DESC);

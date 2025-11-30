-- Restore UNIQUE constraint on email column
ALTER TABLE iam_users ADD CONSTRAINT iam_users_email_key UNIQUE (email);
-- Remove UNIQUE constraint from email column in iam_users table
ALTER TABLE iam_users DROP CONSTRAINT IF EXISTS iam_users_email_key;

-- Recreate the index without UNIQUE constraint (keep it for performance)
DROP INDEX IF EXISTS idx_iam_users_email;
CREATE INDEX IF NOT EXISTS idx_iam_users_email ON iam_users(email);


-- Drop indexes
DROP INDEX IF EXISTS idx_iam_users_role;
DROP INDEX IF EXISTS idx_iam_users_name;
DROP INDEX IF EXISTS idx_iam_users_email;
DROP INDEX IF EXISTS idx_iam_users_access_key;

-- Drop table
DROP TABLE IF EXISTS iam_users;

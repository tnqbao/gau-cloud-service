-- Drop indexes
DROP INDEX IF EXISTS idx_iam_users_role;
DROP INDEX IF EXISTS idx_iam_users_name;
DROP INDEX IF EXISTS idx_iam_users_email;
DROP INDEX IF EXISTS idx_iam_users_access_key;

-- Drop table
DROP TABLE IF EXISTS iam_users;

-- Drop UUID extension (optional - only if no other tables use it)
-- DROP EXTENSION IF EXISTS "uuid-ossp";

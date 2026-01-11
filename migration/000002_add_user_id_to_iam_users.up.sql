-- Add user_id column to iam_users table
ALTER TABLE iam_users ADD COLUMN IF NOT EXISTS user_id UUID NOT NULL DEFAULT uuid_generate_v4();

-- Create index for user_id for faster queries
CREATE INDEX IF NOT EXISTS idx_iam_users_user_id ON iam_users(user_id);


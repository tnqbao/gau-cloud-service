-- Create IAM users table
CREATE TABLE IF NOT EXISTS iam_users (
    id SERIAL PRIMARY KEY,
    access_key VARCHAR(64) UNIQUE NOT NULL,
    secret_key VARCHAR(128) NOT NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    role VARCHAR(50) NOT NULL
);

-- Create indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_iam_users_access_key ON iam_users(access_key);
CREATE INDEX IF NOT EXISTS idx_iam_users_email ON iam_users(email);
CREATE INDEX IF NOT EXISTS idx_iam_users_name ON iam_users(name);
CREATE INDEX IF NOT EXISTS idx_iam_users_role ON iam_users(role);

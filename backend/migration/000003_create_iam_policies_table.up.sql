-- Create IAM policies table
CREATE TABLE IF NOT EXISTS iam_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    iam_id UUID NOT NULL REFERENCES iam_users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    policy JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for faster queries
CREATE INDEX IF NOT EXISTS idx_iam_policies_iam_id ON iam_policies(iam_id);
CREATE INDEX IF NOT EXISTS idx_iam_policies_type ON iam_policies(type);
CREATE INDEX IF NOT EXISTS idx_iam_policies_policy ON iam_policies USING GIN(policy);


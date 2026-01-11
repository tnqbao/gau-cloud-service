-- Drop indexes first
DROP INDEX IF EXISTS idx_buckets_owner_id;
DROP INDEX IF EXISTS idx_buckets_name;

-- Drop buckets table
DROP TABLE IF EXISTS buckets;
-- Create buckets table
CREATE TABLE IF NOT EXISTS buckets (
    id UUID PRIMARY KEY,
    name VARCHAR(63) UNIQUE NOT NULL,
    region VARCHAR(255) NOT NULL,
    created_at VARCHAR(255) NOT NULL,
    owner_id UUID NOT NULL,
    created_at_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on owner_id for faster queries
CREATE INDEX idx_buckets_owner_id ON buckets(owner_id);

-- Create index on name for faster lookups
CREATE INDEX idx_buckets_name ON buckets(name);

-- Add comment to table
COMMENT ON TABLE buckets IS 'Stores bucket information for MinIO S3-compatible storage';
COMMENT ON COLUMN buckets.id IS 'Unique identifier for the bucket';
COMMENT ON COLUMN buckets.name IS 'Bucket name (must be globally unique, 3-63 characters)';
COMMENT ON COLUMN buckets.region IS 'Region where the bucket is located';
COMMENT ON COLUMN buckets.created_at IS 'RFC3339 formatted creation timestamp';
COMMENT ON COLUMN buckets.owner_id IS 'UUID of the user who owns this bucket';


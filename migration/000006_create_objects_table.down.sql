-- Drop indexes first
DROP INDEX IF EXISTS idx_bucket_key;
DROP INDEX IF EXISTS idx_objects_bucket_id;

-- Drop objects table
DROP TABLE IF EXISTS objects;
-- Create objects table with improved schema
CREATE TABLE IF NOT EXISTS objects (
    id UUID PRIMARY KEY,
    bucket_id UUID NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    key VARCHAR(1024) NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_type VARCHAR(255),
    etag VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create composite unique index on bucket_id and key
-- This ensures that within a bucket, each key is unique
CREATE UNIQUE INDEX IF NOT EXISTS idx_bucket_key ON objects(bucket_id, key);

-- Create index on bucket_id for faster queries
CREATE INDEX IF NOT EXISTS idx_objects_bucket_id ON objects(bucket_id);

-- Add comments for documentation
COMMENT ON TABLE objects IS 'Stores metadata for objects in MinIO S3 buckets';
COMMENT ON COLUMN objects.id IS 'Unique identifier for the object';
COMMENT ON COLUMN objects.bucket_id IS 'Reference to the bucket containing this object';
COMMENT ON COLUMN objects.key IS 'Object key/path within the bucket (max 1024 chars)';
COMMENT ON COLUMN objects.size_bytes IS 'Object size in bytes';
COMMENT ON COLUMN objects.content_type IS 'MIME type of the object (e.g., image/png, application/pdf)';
COMMENT ON COLUMN objects.etag IS 'ETag (MD5 hash) for object integrity verification';
COMMENT ON COLUMN objects.created_at IS 'Timestamp when object was created';
COMMENT ON COLUMN objects.updated_at IS 'Timestamp when object was last updated';


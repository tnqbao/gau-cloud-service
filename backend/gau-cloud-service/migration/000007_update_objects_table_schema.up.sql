-- Update objects table schema to match entity
-- Add new columns
ALTER TABLE objects ADD COLUMN IF NOT EXISTS origin_name VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE objects ADD COLUMN IF NOT EXISTS parent_path VARCHAR(1024);
ALTER TABLE objects ADD COLUMN IF NOT EXISTS url VARCHAR(1024) NOT NULL DEFAULT '';
ALTER TABLE objects ADD COLUMN IF NOT EXISTS file_hash VARCHAR(255);

-- Rename updated_at to last_modified
ALTER TABLE objects RENAME COLUMN updated_at TO last_modified;

-- Rename size_bytes to size
ALTER TABLE objects RENAME COLUMN size_bytes TO size;

-- Drop unused columns
ALTER TABLE objects DROP COLUMN IF EXISTS key;
ALTER TABLE objects DROP COLUMN IF EXISTS etag;

-- Drop old index that referenced 'key' column
DROP INDEX IF EXISTS idx_bucket_key;

-- Create new index on file_hash
CREATE INDEX IF NOT EXISTS idx_objects_file_hash ON objects(file_hash);

-- Update comments
COMMENT ON COLUMN objects.origin_name IS 'Original file name when uploaded';
COMMENT ON COLUMN objects.parent_path IS 'Parent directory path within the bucket';
COMMENT ON COLUMN objects.url IS 'URL path in format hash.ext';
COMMENT ON COLUMN objects.file_hash IS 'Hash of the file content for deduplication';
COMMENT ON COLUMN objects.last_modified IS 'Timestamp when object was last modified';
COMMENT ON COLUMN objects.size IS 'Object size in bytes';

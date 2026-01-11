-- Revert objects table schema changes
-- Drop new index
DROP INDEX IF EXISTS idx_objects_file_hash;

-- Recreate old index (will fail if key column doesn't exist, but that's expected in rollback)
-- CREATE UNIQUE INDEX IF NOT EXISTS idx_bucket_key ON objects(bucket_id, key);

-- Add back dropped columns
ALTER TABLE objects ADD COLUMN IF NOT EXISTS key VARCHAR(1024);
ALTER TABLE objects ADD COLUMN IF NOT EXISTS etag VARCHAR(255);

-- Rename columns back
ALTER TABLE objects RENAME COLUMN last_modified TO updated_at;
ALTER TABLE objects RENAME COLUMN size TO size_bytes;

-- Drop new columns
ALTER TABLE objects DROP COLUMN IF EXISTS origin_name;
ALTER TABLE objects DROP COLUMN IF EXISTS parent_path;
ALTER TABLE objects DROP COLUMN IF EXISTS url;
ALTER TABLE objects DROP COLUMN IF EXISTS file_hash;

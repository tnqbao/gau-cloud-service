-- Create upload_sessions table for chunked upload tracking
CREATE TABLE IF NOT EXISTS upload_sessions (
    id UUID PRIMARY KEY,
    bucket_id UUID NOT NULL REFERENCES buckets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    file_name VARCHAR(512) NOT NULL,
    file_size BIGINT NOT NULL,
    content_type VARCHAR(255),
    custom_path VARCHAR(1024),
    chunk_size BIGINT NOT NULL,
    total_chunks INT NOT NULL,
    uploaded_chunks INT DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'INIT',
    temp_bucket VARCHAR(255) NOT NULL,
    temp_prefix VARCHAR(512) NOT NULL,
    file_hash VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

-- Create indexes for efficient querying
CREATE INDEX idx_upload_sessions_bucket_id ON upload_sessions(bucket_id);
CREATE INDEX idx_upload_sessions_user_id ON upload_sessions(user_id);
CREATE INDEX idx_upload_sessions_status ON upload_sessions(status);
CREATE INDEX idx_upload_sessions_expires_at ON upload_sessions(expires_at);

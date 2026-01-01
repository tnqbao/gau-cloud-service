package dto

// InitUploadRequest represents the request to initialize a chunked upload
type InitUploadRequest struct {
	FileName    string `json:"file_name" binding:"required"`
	FileSize    int64  `json:"file_size" binding:"required,gt=0"`
	ContentType string `json:"content_type"`
	Path        string `json:"path"` // Optional custom path
}

// InitUploadResponse represents the response after initializing a chunked upload
type InitUploadResponse struct {
	UploadID    string `json:"upload_id"`
	ChunkSize   int64  `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	TempPrefix  string `json:"temp_prefix"`
	ExpiresAt   string `json:"expires_at"`
}

// UploadChunkRequest represents the request to upload a chunk (from query/header params)
type UploadChunkRequest struct {
	UploadID    string `form:"upload_id" binding:"required"`
	ChunkIndex  int    `form:"chunk_index" binding:"min=0"`
	TotalChunks int    `form:"total_chunks" binding:"required,gt=0"`
}

// UploadChunkResponse represents the response after uploading a chunk
type UploadChunkResponse struct {
	ChunkIndex     int    `json:"chunk_index"`
	UploadedChunks int    `json:"uploaded_chunks"`
	TotalChunks    int    `json:"total_chunks"`
	Status         string `json:"status"`
}

// CompleteUploadRequest represents the request to complete a chunked upload
type CompleteUploadRequest struct {
	UploadID string `json:"upload_id" binding:"required"`
}

// CompleteUploadResponse represents the response after completing a chunked upload
type CompleteUploadResponse struct {
	Message      string      `json:"message"`
	Object       interface{} `json:"object"`
	Status       string      `json:"status"`
	FileHash     string      `json:"file_hash"`
	TargetFolder string      `json:"target_folder"`
}

// UploadProgressResponse represents the current upload progress
type UploadProgressResponse struct {
	UploadID       string  `json:"upload_id"`
	UploadedChunks int     `json:"uploaded_chunks"`
	TotalChunks    int     `json:"total_chunks"`
	Status         string  `json:"status"`
	Progress       float64 `json:"progress"` // Percentage 0-100
}

// AbortUploadRequest represents the request to abort a chunked upload
type AbortUploadRequest struct {
	UploadID string `json:"upload_id" binding:"required"`
}

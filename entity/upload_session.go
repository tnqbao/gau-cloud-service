package entity

import (
	"time"

	"github.com/google/uuid"
)

// UploadStatus represents the status of an upload session
type UploadStatus string

const (
	UploadStatusInit       UploadStatus = "INIT"
	UploadStatusUploading  UploadStatus = "UPLOADING"
	UploadStatusCompleted  UploadStatus = "COMPLETED"
	UploadStatusProcessing UploadStatus = "PROCESSING"
	UploadStatusFailed     UploadStatus = "FAILED"
	UploadStatusExpired    UploadStatus = "EXPIRED"
)

// UploadSession represents a chunked upload session
type UploadSession struct {
	ID             uuid.UUID    `json:"id" gorm:"type:uuid;primaryKey"`
	BucketID       uuid.UUID    `json:"bucket_id" gorm:"type:uuid;not null;index"`
	UserID         uuid.UUID    `json:"user_id" gorm:"type:uuid;not null;index"`
	FileName       string       `json:"file_name" gorm:"type:varchar(512);not null"`
	FileSize       int64        `json:"file_size" gorm:"not null"`
	ContentType    string       `json:"content_type" gorm:"type:varchar(255)"`
	CustomPath     string       `json:"custom_path" gorm:"type:varchar(1024)"`
	ChunkSize      int64        `json:"chunk_size" gorm:"not null"`
	TotalChunks    int          `json:"total_chunks" gorm:"not null"`
	UploadedChunks int          `json:"uploaded_chunks" gorm:"default:0"`
	Status         UploadStatus `json:"status" gorm:"type:varchar(32);not null;default:'INIT'"`
	TempBucket     string       `json:"temp_bucket" gorm:"type:varchar(255);not null"`
	TempPrefix     string       `json:"temp_prefix" gorm:"type:varchar(512);not null"`
	FileHash       string       `json:"file_hash" gorm:"type:varchar(255)"`
	CreatedAt      time.Time    `json:"created_at" gorm:"not null;autoCreateTime"`
	UpdatedAt      time.Time    `json:"updated_at" gorm:"autoUpdateTime"`
	ExpiresAt      time.Time    `json:"expires_at" gorm:"not null;index"`

	Bucket *Bucket `json:"bucket,omitempty" gorm:"foreignKey:BucketID;constraint:OnDelete:CASCADE"`
}

// ChunkInfo represents information about a single chunk
type ChunkInfo struct {
	Index      int       `json:"index"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

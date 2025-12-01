package entity

import (
	"time"

	"github.com/google/uuid"
)

type Object struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	BucketID    uuid.UUID `json:"bucket_id" gorm:"type:uuid;not null;index"`
	Key         string    `json:"key" gorm:"type:varchar(1024);not null;index:idx_bucket_key"`
	SizeBytes   int64     `json:"size_bytes" gorm:"not null"`
	ContentType string    `json:"content_type" gorm:"type:varchar(255)"`
	ETag        string    `json:"etag" gorm:"type:varchar(255)"`
	CreatedAt   time.Time `json:"created_at" gorm:"not null;autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`

	// Composite unique constraint (bucket_id + key)
	_ struct{} `gorm:"uniqueIndex:idx_bucket_key"`

	Bucket *Bucket `json:"bucket,omitempty" gorm:"foreignKey:BucketID;constraint:OnDelete:CASCADE"`
}

package entity

import (
	"time"

	"github.com/google/uuid"
)

type Object struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	BucketID     uuid.UUID `json:"bucket_id" gorm:"type:uuid;not null;index"`
	ContentType  string    `json:"content_type" gorm:"type:varchar(255)"`
	OriginName   string    `json:"origin_name" gorm:"type:varchar(512);not null"`
	ParentPath   string    `json:"parent_path" gorm:"type:varchar(1024)"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null;autoCreateTime"`
	LastModified time.Time `json:"last_modified" gorm:"autoUpdateTime"`
	Size         int64     `json:"size" gorm:"not null"`
	URL          string    `json:"url" gorm:"type:varchar(1024);not null"` // hash.ext format
	FileHash     string    `json:"file_hash" gorm:"type:varchar(255);index"`

	Bucket *Bucket `json:"bucket,omitempty" gorm:"foreignKey:BucketID;constraint:OnDelete:CASCADE"`
}

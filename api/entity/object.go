package entity

import "github.com/google/uuid"

type Object struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	BucketID  uuid.UUID `json:"bucket_id" binding:"required" gorm:"type:uuid;not null;index"`
	Key       string    `json:"key" binding:"required,min=1,max=1024" gorm:"not null"`
	SizeKB    int       `json:"size_kb" binding:"required,min=0" gorm:"not null"`
	CreatedAt string    `json:"created_at" gorm:"not null"`
	Bucket    *Bucket   `json:"bucket,omitempty" gorm:"foreignKey:BucketID;constraint:OnDelete:CASCADE"`
}

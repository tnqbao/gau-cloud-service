package entity

import "github.com/google/uuid"

type Bucket struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string    `json:"name" binding:"required,min=3,max=63" gorm:"uniqueIndex;not null"`
	Region    string    `json:"region" binding:"required" gorm:"not null"`
	CreatedAt string    `json:"created_at" gorm:"not null"`
	OwnerID   uuid.UUID `json:"owner_id" binding:"required" gorm:"type:uuid;not null;index"`
	Objects   []Object  `json:"objects,omitempty" gorm:"foreignKey:BucketID;constraint:OnDelete:CASCADE"`
}

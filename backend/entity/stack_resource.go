package entity

import "github.com/google/uuid"

type StackResource struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	StackID    uuid.UUID `json:"stack_id" binding:"required" gorm:"type:uuid;not null;index"`
	Type       string    `json:"type" binding:"required" gorm:"not null"` // e.g., "EC2", "S3", "Lambda"
	ResourceID string    `json:"resource_id" binding:"required" gorm:"not null"`
	Status     string    `json:"status" binding:"required,oneof=creating active updating deleting deleted failed" gorm:"not null;index"`
	Stack      *Stack    `json:"stack,omitempty" gorm:"foreignKey:StackID;constraint:OnDelete:CASCADE"`
}

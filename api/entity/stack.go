package entity

import "github.com/google/uuid"

type Stack struct {
	ID        uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string          `json:"name" binding:"required,min=1,max=128" gorm:"uniqueIndex;not null"`
	Template  string          `json:"template" binding:"required" gorm:"type:text;not null"`
	Status    string          `json:"status" binding:"required,oneof=creating active updating deleting deleted failed" gorm:"not null;index"`
	CreatedAt string          `json:"created_at" gorm:"not null"`
	UpdatedAt string          `json:"updated_at" gorm:"not null"`
	OwnerID   string          `json:"owner_id" binding:"required" gorm:"not null;index"`
	Resources []StackResource `json:"resources,omitempty" gorm:"foreignKey:StackID;constraint:OnDelete:CASCADE"`
}

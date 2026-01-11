package entity

import "github.com/google/uuid"

type Function struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string    `json:"name" binding:"required,min=1,max=64" gorm:"uniqueIndex;not null"`
	Runtime   string    `json:"runtime" binding:"required" gorm:"not null"`
	Handler   string    `json:"handler" binding:"required" gorm:"not null"`
	Status    string    `json:"status" binding:"required,oneof=active inactive pending error" gorm:"not null;index"`
	CreatedAt string    `json:"created_at" gorm:"not null"`
	UpdatedAt string    `json:"updated_at" gorm:"not null"`
}

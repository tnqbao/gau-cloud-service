package entity

import "github.com/google/uuid"

type Job struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Type        string    `json:"type" binding:"required" gorm:"not null;index"`
	Status      string    `json:"status" binding:"required,oneof=pending running completed failed cancelled" gorm:"not null;index"`
	TargetID    uuid.UUID `json:"target_id" binding:"required" gorm:"type:uuid;not null;index"` // ID of the target entity (e.g., Object, Function, Stack)
	StartAt     string    `json:"start_at"`
	FinishAt    string    `json:"finish_at"`
	Message     string    `json:"message" gorm:"type:text"`
	InitiatorID string    `json:"initiator_id" binding:"required" gorm:"not null;index"`
}

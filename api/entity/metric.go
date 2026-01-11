package entity

import "github.com/google/uuid"

type Metric struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	EntityType string    `json:"entity_type" binding:"required" gorm:"not null;index:idx_entity"` // e.g., "Function", "Stack", "StackResource", "Object"
	EntityID   uuid.UUID `json:"entity_id" binding:"required" gorm:"type:uuid;not null;index:idx_entity"`
	MetricType string    `json:"metric_type" binding:"required" gorm:"not null;index"` // e.g., "CPUUtilization", "MemoryUsage", "RequestCount"
	Value      float64   `json:"value" binding:"required,min=0" gorm:"not null"`
	Timestamp  string    `json:"timestamp" binding:"required" gorm:"not null;index"`
}

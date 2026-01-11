package entity

import "github.com/google/uuid"

type VM struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string    `json:"name" binding:"required,min=1,max=255" gorm:"uniqueIndex;not null"`
	CPU       int       `json:"cpu" binding:"required,min=1,max=128" gorm:"not null"`
	MemoryGB  int       `json:"memory_gb" binding:"required,min=0" gorm:"not null"`
	DiskGB    int       `json:"disk_gb" binding:"required,min=1" gorm:"not null"`
	Status    string    `json:"status" binding:"required,oneof=creating running stopped stopping starting deleting deleted error" gorm:"not null;index"`
	IP        string    `json:"ip" binding:"omitempty,ip"`
	HostNode  string    `json:"host_node"`
	CreatedAt string    `json:"created_at" gorm:"not null"`
	UpdatedAt string    `json:"updated_at" gorm:"not null"`
}

package entity

import (
	"github.com/google/uuid"
)

type IAMUser struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;" json:"id"`
	UserId    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	AccessKey string    `gorm:"uniqueIndex;size:64" json:"access_key"`
	SecretKey string    `gorm:"size:128" json:"secret_key"`
	Name      string    `gorm:"size:255" json:"name"`
	Email     string    `gorm:"size:255;uniqueIndex" json:"email"`
	Role      string    `gorm:"size:50" json:"role"`
}

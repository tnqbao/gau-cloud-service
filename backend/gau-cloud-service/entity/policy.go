package entity

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type IAMPolicy struct {
	ID     uuid.UUID      `gorm:"type:uuid;primaryKey;" json:"id"`
	IAMID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"iam_id"`
	Type   string         `gorm:"size:50;not null" json:"type"`
	Policy datatypes.JSON `gorm:"type:jsonb;not null" json:"policy"`
}

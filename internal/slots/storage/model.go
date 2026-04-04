package storage

import "time"

// Slot is the GORM model for slots (009).
type Slot struct {
	ID        string    `gorm:"column:id;type:uuid;primaryKey"`
	Name      string    `gorm:"column:name;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null;autoCreateTime"`
}

// TableName is the SQL table name.
func (Slot) TableName() string {
	return "slots"
}

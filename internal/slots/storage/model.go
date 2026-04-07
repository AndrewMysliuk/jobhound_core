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

// SlotIdempotency maps POST /slots Idempotency-Key header values to created slot rows.
type SlotIdempotency struct {
	IdempotencyKey string `gorm:"column:idempotency_key;primaryKey"`
	SlotID         string `gorm:"column:slot_id;not null"`
}

// TableName is the SQL table name.
func (SlotIdempotency) TableName() string {
	return "slot_idempotency_keys"
}

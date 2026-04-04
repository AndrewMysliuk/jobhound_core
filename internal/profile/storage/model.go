package storage

import "time"

// UserProfile is the singleton row (id = 1) for global stage-3 text (009).
type UserProfile struct {
	ID        int       `gorm:"column:id;primaryKey"`
	Text      string    `gorm:"column:text;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName is the SQL table name.
func (UserProfile) TableName() string {
	return "user_profile"
}

package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string    `gorm:"type:varchar(100);not null" json:"name"`
	Email        string    `gorm:"type:varchar(150);uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"type:varchar(255);not null" json:"-"` // json:"-" -> API'de asla görünmez!
	AvatarURL    string    `gorm:"type:varchar(255)" json:"avatar_url,omitempty"`
	CreatedAt    time.Time `gorm:"not null;default:now()" json:"created_at"`
}

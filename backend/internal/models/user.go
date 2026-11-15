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
	// AvatarVersion profil fotoğrafının sürümüdür (user_avatars.etag); fotoğraf yoksa boştur.
	// Görsel GET /users/:id/avatar ile alınır. Eski, hiç kullanılmamış avatar_url sütunu
	// veritabanında durur (AutoMigrate sütun silmez); modelde yer almaz.
	AvatarVersion string    `gorm:"type:varchar(64);not null;default:''" json:"avatar_version,omitempty"`
	CreatedAt     time.Time `gorm:"not null;default:now()" json:"created_at"`
}

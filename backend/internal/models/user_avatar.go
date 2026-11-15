package models

import (
	"time"

	"github.com/google/uuid"
)

// UserAvatar bir kullanıcının profil fotoğrafıdır: sunucuda işlenmiş (kare, en çok 512x512,
// metadata'sız) JPEG baytları. Kullanıcı başına en fazla bir satır vardır. Baytlar bilerek
// users tablosundan ayrıdır; kullanıcı ve liste sorguları bu tabloya dokunmaz.
type UserAvatar struct {
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Data        []byte    `gorm:"type:bytea;not null" json:"-"`
	ContentType string    `gorm:"type:varchar(50);not null" json:"-"`
	// ETag içeriğin özetidir; users.avatar_version ile aynıdır.
	ETag      string    `gorm:"column:etag;type:varchar(64);not null" json:"-"`
	Width     int       `gorm:"not null" json:"-"`
	Height    int       `gorm:"not null" json:"-"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"-"`

	// Kullanıcı silinince fotoğrafı da silinir.
	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

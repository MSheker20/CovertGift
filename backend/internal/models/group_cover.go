package models

import (
	"time"

	"github.com/google/uuid"
)

// GroupCover bir grubun kapak fotoğrafıdır: sunucuda işlenmiş (4:3, en çok 1600x1200,
// metadata'sız) JPEG baytları. Grup başına en fazla bir satır vardır. Baytlar bilerek
// groups tablosundan ayrıdır; grup ve liste sorguları bu tabloya dokunmaz.
type GroupCover struct {
	GroupID     uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Data        []byte    `gorm:"type:bytea;not null" json:"-"`
	ContentType string    `gorm:"type:varchar(50);not null" json:"-"`
	// ETag içeriğin özetidir; groups.cover_version ile aynıdır.
	ETag      string     `gorm:"column:etag;type:varchar(64);not null" json:"-"`
	Width     int        `gorm:"not null" json:"-"`
	Height    int        `gorm:"not null" json:"-"`
	UpdatedAt time.Time  `gorm:"not null;default:now()" json:"-"`
	UpdatedBy *uuid.UUID `gorm:"type:uuid" json:"-"`

	// Grup silinince kapak da silinir; yükleyen kullanıcı silinirse kapak kalır.
	Group   Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	Updater *User `gorm:"foreignKey:UpdatedBy;constraint:OnDelete:SET NULL" json:"-"`
}

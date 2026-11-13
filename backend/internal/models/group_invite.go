package models

import (
	"time"

	"github.com/google/uuid"
)

// GroupInvite bir grubun paylaşılabilir davet linkidir. Grup başına aynı anda en fazla bir
// aktif davet vardır; yenisi oluşturulunca öncekiler iptal edilir. Ham token hiçbir yerde
// saklanmaz: yalnızca SHA-256 özeti (hex) tutulur, ham değer oluşturma yanıtında bir kez döner.
// "Aktif" = iptal edilmemiş ve süresi dolmamış.
type GroupInvite struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	GroupID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	TokenHash string     `gorm:"type:varchar(64);not null;uniqueIndex" json:"-"`
	CreatedBy uuid.UUID  `gorm:"type:uuid;not null" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"-"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `gorm:"not null;default:now()" json:"-"`

	Group   Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	Creator User  `gorm:"foreignKey:CreatedBy;constraint:OnDelete:CASCADE" json:"-"`
}

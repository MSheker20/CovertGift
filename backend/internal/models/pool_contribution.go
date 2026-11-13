package models

import (
	"time"

	"github.com/google/uuid"
)

// PoolContribution bir üyenin bir havuza katkı verip vermediğidir. Satırın olmaması "katkı
// veriyor" demektir; tüm sorgular COALESCE(contributing, true) kullanır. Satırlar yine de havuz
// kurulurken ve her katılma yolunda (üye ekleme, davet linki) hemen yazılır.
type PoolContribution struct {
	PoolID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	UserID       uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Contributing bool      `gorm:"not null;default:true" json:"-"`
	UpdatedAt    time.Time `gorm:"not null;default:now()" json:"-"`

	Pool GiftPool `gorm:"foreignKey:PoolID;constraint:OnDelete:CASCADE" json:"-"`
	User User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

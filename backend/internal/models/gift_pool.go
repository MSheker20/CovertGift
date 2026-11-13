package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// GiftPool MULTI bir grupta tek bir alıcı için yürütülen hediye havuzudur: kendi fikirleri,
// oyları, bütçesi, ilgi alanları, tarihi, durumu ve kazananları vardır.
//
// Anti-Spoiler: havuzun alıcısı (RecipientUserID) havuzun hiçbir verisini göremez. Bu yapı
// hiçbir JSON yanıtında olduğu gibi dönmez (tüm alanlar json:"-"); yanıtlar services
// paketindeki LockedPool / OpenPool / PoolDetail tipleriyle üretilir.
//
// Etkin yönetici: alıcı grubun yöneticisi (admin) ise ManagerUserID (NULL ise kimse), değilse
// grubun yöneticisi.
type GiftPool struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"-"`
	GroupID         uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_gift_pools_group_recipient,priority:1" json:"-"`
	RecipientUserID uuid.UUID       `gorm:"type:uuid;not null;uniqueIndex:idx_gift_pools_group_recipient,priority:2;index" json:"-"`
	ManagerUserID   *uuid.UUID      `gorm:"type:uuid" json:"-"`
	TotalBudget     decimal.Decimal `gorm:"type:numeric(10,2);not null;default:0.00" json:"-"`
	// Interests satır sonuyla ayrılmış ilgi alanlarıdır (ParseInterests ile okunur).
	Interests   string     `gorm:"type:text;not null;default:''" json:"-"`
	EventDate   *Date      `gorm:"type:date" json:"-"`
	Status      string     `gorm:"type:varchar(20);not null;default:'ACTIVE'" json:"-"`
	CompletedAt *time.Time `json:"-"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"-"`

	Group     Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	Recipient User  `gorm:"foreignKey:RecipientUserID;constraint:OnDelete:CASCADE" json:"-"`
	Manager   *User `gorm:"foreignKey:ManagerUserID;constraint:OnDelete:SET NULL" json:"-"`
}

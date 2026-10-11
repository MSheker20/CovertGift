package models

import (
	"time"

	"github.com/google/uuid"
)

// GroupMember bir kullanıcının gruptaki üyeliğini ve rolünü temsil eder
type GroupMember struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GroupID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_group_user" json:"group_id"`
	UserID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_group_user" json:"user_id"`
	Role     string    `gorm:"type:varchar(20);not null;default:'member'" json:"role"` // 'admin' veya 'member'
	JoinedAt time.Time `gorm:"not null;default:now()" json:"joined_at"`

	// İlişkiler ve Cascade kuralları
	Group Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	User  User  `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
}

package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Grup durumları
const (
	GroupStatusActive    = "ACTIVE"
	GroupStatusCompleted = "COMPLETED"
)

// Group hediye gruplarını temsil eder
type Group struct {
	ID               uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title            string          `gorm:"type:varchar(150);not null" json:"title"`
	Description      string          `gorm:"type:text" json:"description,omitempty"`
	TargetPersonName string          `gorm:"type:varchar(100);not null" json:"target_person_name"`
	TargetInterests  string          `gorm:"type:text" json:"target_interests,omitempty"`     // Gemini AI için ilgi alanları ve hobiler
	TargetUserID     *uuid.UUID      `gorm:"type:uuid;index" json:"target_user_id,omitempty"` // Anti-Spoiler için kritik alan (NULL olabilir)
	TotalBudget      decimal.Decimal `gorm:"type:numeric(10,2);not null;default:0.00" json:"total_budget"`
	CreatorID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"creator_id"`
	CreatedAt        time.Time       `gorm:"not null;default:now()" json:"created_at"`
	// Hediyenin verileceği gün (doğum günü vb.). İsteğe bağlı, tek seferlik.
	EventDate *Date `gorm:"type:date" json:"event_date,omitempty"`
	// ACTIVE: oylama sürüyor. COMPLETED: admin kazananı seçti; fikir ve oy işlemleri kapalı.
	Status      string     `gorm:"type:varchar(20);not null;default:'ACTIVE'" json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// İlişkiler (Foreign Key)
	Creator    *User `gorm:"foreignKey:CreatorID;constraint:OnDelete:RESTRICT" json:"creator,omitempty"`
	TargetUser *User `gorm:"foreignKey:TargetUserID;constraint:OnDelete:SET NULL" json:"target_user,omitempty"`
}

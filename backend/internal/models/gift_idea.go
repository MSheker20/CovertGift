package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// GiftIdea gruptaki hediye önerilerini temsil eder
type GiftIdea struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GroupID        uuid.UUID       `gorm:"type:uuid;not null;index" json:"group_id"`
	Title          string          `gorm:"type:varchar(200);not null" json:"title"`
	Description    string          `gorm:"type:text" json:"description,omitempty"`
	EstimatedPrice decimal.Decimal `gorm:"type:numeric(10,2);not null;default:0.00" json:"estimated_price"`
	SourceType     string          `gorm:"type:varchar(20);not null;default:'AI'" json:"source_type"` // 'AI' veya 'MANUAL'
	Status         string          `gorm:"type:varchar(20);not null;default:'ACTIVE'" json:"status"`   // 'ACTIVE', 'REJECTED', 'SELECTED'
	SearchQuery    string          `gorm:"type:varchar(255);not null" json:"search_query"`            // Pazaryerleri için arama terimi
	ImageURL       string          `gorm:"type:varchar(500)" json:"image_url,omitempty"`
	ProductURL     string          `gorm:"type:varchar(1000)" json:"product_url,omitempty"` // Sadece MANUAL eklemelerde dolu
	CreatedBy      uuid.UUID       `gorm:"type:uuid;not null;index" json:"created_by"`
	CreatedAt      time.Time       `gorm:"not null;default:now()" json:"created_at"`

	// İlişkiler
	Group   Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	Creator User  `gorm:"foreignKey:CreatedBy;constraint:OnDelete:RESTRICT" json:"creator,omitempty"`
}

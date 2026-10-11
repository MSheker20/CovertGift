package models

import (
	"time"

	"github.com/google/uuid"
)

// Vote bir hediye fikrine verilen oyu (+1 Upvote veya -1 Downvote) temsil eder
type Vote struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GiftIdeaID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_idea_user" json:"gift_idea_id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_idea_user" json:"user_id"`
	VoteType   int8      `gorm:"type:smallint;not null;check:vote_type IN (-1, 1)" json:"vote_type"`
	CreatedAt  time.Time `gorm:"not null;default:now()" json:"created_at"`

	// İlişkiler
	GiftIdea GiftIdea `gorm:"foreignKey:GiftIdeaID;constraint:OnDelete:CASCADE" json:"-"`
	User     User     `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

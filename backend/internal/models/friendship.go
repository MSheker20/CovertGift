package models

import (
	"time"

	"github.com/google/uuid"
)

// Arkadaşlık durumları.
const (
	FriendshipPending  = "PENDING"
	FriendshipAccepted = "ACCEPTED"
)

// Friendship iki kullanıcı arasındaki tek arkadaşlık satırıdır (istek ya da kabul edilmiş).
//
// Çift başına tek satır olsun diye kullanıcılar sıralı tutulur: UserLowID < UserHighID (UUID
// metin sırası). (user_low_id, user_high_id) benzersizdir; böylece A→B ve B→A istekleri aynı anda
// var olamaz. RequesterID isteği kimin gönderdiğini söyler.
type Friendship struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserLowID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_friendship_pair,priority:1" json:"-"`
	UserHighID  uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_friendship_pair,priority:2;index" json:"-"`
	RequesterID uuid.UUID  `gorm:"type:uuid;not null" json:"-"`
	Status      string     `gorm:"type:varchar(20);not null" json:"status"`
	CreatedAt   time.Time  `gorm:"not null;default:now()" json:"created_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`

	UserLow  User `gorm:"foreignKey:UserLowID;constraint:OnDelete:CASCADE" json:"-"`
	UserHigh User `gorm:"foreignKey:UserHighID;constraint:OnDelete:CASCADE" json:"-"`
}

// FriendPair iki kullanıcı kimliğini (low, high) sırasına koyar.
func FriendPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if a.String() < b.String() {
		return a, b
	}
	return b, a
}

// FriendLink kullanıcının kişisel arkadaşlık linkidir. Linki açan kişi onaylayınca doğrudan
// arkadaş olur. Kod sahibine tekrar gösterilebilmesi için düz saklanır; sıfırlanınca eskisi
// geçersiz olur. Yol parametresi olarak taşındığı için loglarda yalnızca rota kalıbı yazılır.
type FriendLink struct {
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"-"`
	Code      string    `gorm:"type:varchar(32);not null;uniqueIndex" json:"-"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"-"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// GroupMemberInvite bir arkadaşın gruba davetidir; kabul edilince üyeliğe dönüşür ve silinir.
// Süresi dolmaz; admin iptal edebilir, davetli reddedebilir. Grubun hedef kişisi davet edilemez;
// hedef sonradan bağlanırsa daveti aynı transaction'da silinir (Anti-Spoiler).
type GroupMemberInvite struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GroupID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_member_invite,priority:1" json:"group_id"`
	InviteeID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_member_invite,priority:2;index" json:"invitee_id"`
	InviterID uuid.UUID `gorm:"type:uuid;not null" json:"inviter_id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	Group   Group `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	Invitee User  `gorm:"foreignKey:InviteeID;constraint:OnDelete:CASCADE" json:"-"`
	Inviter User  `gorm:"foreignKey:InviterID;constraint:OnDelete:CASCADE" json:"-"`
}

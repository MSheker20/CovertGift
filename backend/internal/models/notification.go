package models

import (
	"time"

	"github.com/google/uuid"
)

// Bildirim türleri. Metin istemcide (i18n) üretilir; sunucu yalnızca tür ve parametre saklar.
const (
	NotificationIdeaAdded        = "idea_added"         // params: actor_name, idea_title
	NotificationAIIdeasAdded     = "ai_ideas_added"     // params: actor_name, count
	NotificationAddedToGroup     = "added_to_group"     // params: actor_name
	NotificationMemberJoined     = "member_joined"      // params: actor_name, member_name
	NotificationWinnerSelected   = "winner_selected"    // params: actor_name, count
	NotificationGroupReopened    = "group_reopened"     // params: actor_name
	NotificationEventDateChanged = "event_date_changed" // params: actor_name, event_date ("YYYY-MM-DD" | null)
	NotificationBudgetChanged    = "budget_changed"     // params: actor_name, total_budget
	NotificationEventReminder    = "event_reminder"     // params: days_left (0–3), event_date
	NotificationWinnerOverdue    = "winner_overdue"     // params: event_date
	// MULTI gruplar: havuz olayları pool_id taşır ve params içinde recipient_name bulunur. Havuzun
	// alıcısına hiçbir zaman gönderilmez.
	NotificationPoolAdded           = "pool_added"            // params: actor_name, recipient_name
	NotificationPoolManagerAssigned = "pool_manager_assigned" // params: actor_name, recipient_name
	// Grup daveti: henüz üye olmayan davetliye gider (grubun hedef kişisine asla).
	NotificationGroupInvite = "group_invite" // params: actor_name
)

// NotificationParams bildirim metnini oluşturmak için gereken küçük, türe özel veridir.
type NotificationParams map[string]any

// Notification bir kullanıcıya, bir grupla ilgili gösterilen kalıcı bildirimdir.
//
// Anti-Spoiler: bir grubun hedef kişisine o grup için satır üretilmez; hedef sonradan
// bağlanırsa satırları silinir ve listeleme ayrıca groups.target_user_id ile süzülür.
type Notification struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID  uuid.UUID `gorm:"type:uuid;not null;index:idx_notifications_user_created,priority:1;uniqueIndex:idx_notifications_user_dedupe,priority:1" json:"-"`
	GroupID uuid.UUID `gorm:"type:uuid;not null;index" json:"group_id"`
	// PoolID olay bir havuza aitse doludur (MULTI grup). Havuz silinince bildirim de silinir;
	// havuzun alıcısı bu satırları hiçbir zaman göremez.
	PoolID *uuid.UUID         `gorm:"type:uuid;index" json:"pool_id,omitempty"`
	Type   string             `gorm:"type:varchar(40);not null" json:"type"`
	Params NotificationParams `gorm:"type:jsonb;serializer:json;not null" json:"params"`
	// DedupeKey hatırlatmaların kullanıcı başına bir kez üretilmesini sağlar (kullanıcı + anahtar
	// benzersiz). Anlık olay bildirimlerinde NULL'dır; PostgreSQL NULL'ları birbirinden farklı sayar.
	DedupeKey *string    `gorm:"type:varchar(200);uniqueIndex:idx_notifications_user_dedupe,priority:2" json:"-"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `gorm:"not null;default:now();index:idx_notifications_user_created,priority:2" json:"created_at"`

	// Kullanıcı veya grup silinince bildirimleri de silinir.
	User  User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Group Group     `gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE" json:"-"`
	Pool  *GiftPool `gorm:"foreignKey:PoolID;constraint:OnDelete:CASCADE" json:"-"`
}

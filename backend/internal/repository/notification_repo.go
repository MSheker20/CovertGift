package repository

import (
	"encoding/json"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationItem listelemede dönen tek bildirimdir. Grup adı listeleme anındaki güncel addır.
type NotificationItem struct {
	ID         uuid.UUID       `json:"id"`
	GroupID    uuid.UUID       `json:"group_id"`
	GroupTitle string          `json:"group_title" example:"Ahmet'in 30. Yaş Sürprizi"`
	Type       string          `json:"type" example:"idea_added"`
	Params     json.RawMessage `json:"params" swaggertype:"object"`
	ReadAt     *time.Time      `json:"read_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ReminderGroup hatırlatma zamanlayıcısının baktığı grup özetidir.
type ReminderGroup struct {
	ID           uuid.UUID
	EventDate    models.Date
	Status       string
	TargetUserID *uuid.UUID
}

// ReminderMember bir grubun üyesi ve rolüdür.
type ReminderMember struct {
	GroupID uuid.UUID
	UserID  uuid.UUID
	Role    string
}

// NotificationRepository bildirim tablosu işlemlerini tanımlar.
//
// Okuma ve işaretleme sorgularının hepsi visibleToUser ile süzülür: kullanıcı bir grubun
// hedef kişisiyse o gruba ait satırlar yok sayılır (Anti-Spoiler, savunma derinliği).
type NotificationRepository interface {
	// Insert satırı ekler. Aynı kullanıcı + dedupe_key zaten varsa eklemez ve false döner.
	Insert(n *models.Notification) (bool, error)
	ListForUser(userID uuid.UUID, limit int) ([]NotificationItem, error)
	CountUnread(userID uuid.UUID) (int64, error)
	// MarkRead kullanıcının kendi bildirimini okundu yapar; satır yoksa (ya da başkasınınsa) false döner.
	MarkRead(userID, id uuid.UUID, at time.Time) (bool, error)
	MarkAllRead(userID uuid.UUID, at time.Time) (int64, error)
	// ReminderGroups hediye tarihi until veya öncesinde olan grupları döner: bugün ve sonrası
	// için tüm gruplar, geçmiş tarihler için yalnızca hâlâ ACTIVE olanlar.
	ReminderGroups(today, until models.Date) ([]ReminderGroup, error)
	MembersOfGroups(groupIDs []uuid.UUID) ([]ReminderMember, error)
}

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository yeni bir NotificationRepository oluşturur.
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{db: db}
}

// visibleToUser kullanıcının hedef kişisi olduğu gruplara ait satırları dışlar.
const visibleToUser = "notifications.user_id = ? AND notifications.group_id NOT IN (SELECT id FROM groups WHERE target_user_id = ?)"

func (r *notificationRepository) Insert(n *models.Notification) (bool, error) {
	res := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "dedupe_key"}},
		DoNothing: true,
	}).Create(n)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *notificationRepository) ListForUser(userID uuid.UUID, limit int) ([]NotificationItem, error) {
	items := []NotificationItem{}
	err := r.db.Table("notifications").
		Select("notifications.id, notifications.group_id, groups.title AS group_title, notifications.type, notifications.params, notifications.read_at, notifications.created_at").
		Joins("JOIN groups ON groups.id = notifications.group_id").
		Where(visibleToUser, userID, userID).
		Order("notifications.created_at DESC, notifications.id DESC").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *notificationRepository) CountUnread(userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Notification{}).
		Where(visibleToUser, userID, userID).
		Where("notifications.read_at IS NULL").
		Count(&count).Error
	return count, err
}

func (r *notificationRepository) MarkRead(userID, id uuid.UUID, at time.Time) (bool, error) {
	// COALESCE: ikinci kez okundu yapmak ilk okunma anını değiştirmez ama yine başarılıdır.
	res := r.db.Model(&models.Notification{}).
		Where("notifications.id = ?", id).
		Where(visibleToUser, userID, userID).
		Update("read_at", gorm.Expr("COALESCE(read_at, ?)", at))
	return res.RowsAffected > 0, res.Error
}

func (r *notificationRepository) MarkAllRead(userID uuid.UUID, at time.Time) (int64, error) {
	res := r.db.Model(&models.Notification{}).
		Where(visibleToUser, userID, userID).
		Where("notifications.read_at IS NULL").
		Update("read_at", at)
	return res.RowsAffected, res.Error
}

func (r *notificationRepository) ReminderGroups(today, until models.Date) ([]ReminderGroup, error) {
	var groups []ReminderGroup
	err := r.db.Model(&models.Group{}).
		Select("id", "event_date", "status", "target_user_id").
		Where("event_date IS NOT NULL AND event_date <= ?", until).
		Where("event_date >= ? OR status = ?", today, models.GroupStatusActive).
		Scan(&groups).Error
	return groups, err
}

func (r *notificationRepository) MembersOfGroups(groupIDs []uuid.UUID) ([]ReminderMember, error) {
	var members []ReminderMember
	if len(groupIDs) == 0 {
		return members, nil
	}
	err := r.db.Model(&models.GroupMember{}).
		Select("group_id", "user_id", "role").
		Where("group_id IN ?", groupIDs).
		Scan(&members).Error
	return members, err
}

package services

import (
	"errors"
	"log/slog"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

// ErrNotificationNotFound bildirim yoksa ya da çağıran kullanıcıya ait değilse döner.
// Başkasının bildirimi için de aynı hata döner; varlığı sızdırılmaz.
var ErrNotificationNotFound = errors.New("bildirim bulunamadı")

// NotificationListLimit zil listesinde dönen en fazla bildirim sayısıdır (yeniden eskiye).
const NotificationListLimit = 50

// UserNotifier bir kullanıcının açık bildirim akışlarına "değişti" sinyali gönderir
// (realtime.Hub bunu karşılar). Sinyal veri taşımaz.
type UserNotifier interface {
	Publish(userID uuid.UUID)
}

// NotificationEvent bir istekte gerçekleşen, bildirime dönüşecek olaydır. Handler olayı
// yalnızca bildirir; alıcı seçimi ve Anti-Spoiler kuralı Dispatch içinde tek yerde uygulanır.
type NotificationEvent struct {
	GroupID uuid.UUID
	Type    string
	Params  models.NotificationParams
	// Only doluysa bildirim yalnızca bu kullanıcılara gider (yine üye olmalı; işlemi yapan
	// ve hedef kişi yine hariç). Boşsa tüm üyelere.
	Only []uuid.UUID
	// Exclude bu kullanıcıları alıcılardan çıkarır.
	Exclude []uuid.UUID
	// RefreshOnly true ise satır üretilmez; Only'deki kullanıcıların açık akışlarına yalnızca
	// yeniden çekme sinyali gider (ör. hedef bağlanınca bildirimleri silinen kullanıcı).
	RefreshOnly bool
}

// NotificationList GET /notifications yanıtıdır.
type NotificationList struct {
	Items       []repository.NotificationItem `json:"items"`
	UnreadCount int64                         `json:"unread_count" example:"3"`
}

// NotificationService bildirim üretimi ve okuma iş mantığıdır.
type NotificationService interface {
	// Dispatch olayları alıcılara kalıcı bildirim olarak yazar ve açık akışlarına sinyal gönderir.
	// Alıcılar: grubun üyeleri; işlemi yapan (actorID) ve grubun hedef kişisi HARİÇ.
	// Hatalar loglanır; isteğin sonucu çoktan yazıldığı için çağırana dönülmez.
	Dispatch(actorID uuid.UUID, events []NotificationEvent)
	List(userID uuid.UUID) (*NotificationList, error)
	MarkRead(userID, id uuid.UUID) error
	MarkAllRead(userID uuid.UUID) error
}

type notificationService struct {
	repo      repository.NotificationRepository
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
	notifier  UserNotifier
	now       func() time.Time
}

// NewNotificationService yeni bir NotificationService oluşturur.
func NewNotificationService(repo repository.NotificationRepository, groupRepo repository.GroupRepository, userRepo repository.UserRepository, notifier UserNotifier) NotificationService {
	return &notificationService{repo: repo, groupRepo: groupRepo, userRepo: userRepo, notifier: notifier, now: time.Now}
}

func (s *notificationService) Dispatch(actorID uuid.UUID, events []NotificationEvent) {
	actorName := ""
	for _, event := range events {
		if event.RefreshOnly {
			for _, userID := range event.Only {
				s.notifier.Publish(userID)
			}
			continue
		}
		if actorName == "" {
			actor, err := s.userRepo.FindByID(actorID)
			if err != nil {
				slog.Error("bildirim: işlemi yapan kullanıcı okunamadı", "error", err, "group_id", event.GroupID)
				return
			}
			if actor != nil {
				actorName = actor.Name
			}
		}
		if err := s.dispatchOne(actorID, actorName, event); err != nil {
			slog.Error("bildirim üretilemedi", "error", err, "type", event.Type, "group_id", event.GroupID)
		}
	}
}

func (s *notificationService) dispatchOne(actorID uuid.UUID, actorName string, event NotificationEvent) error {
	// Grup her olayda yeniden okunur: hedef kişi kararı isteğin başındaki değil, en güncel
	// target_user_id ile verilir.
	group, err := s.groupRepo.GetGroupByID(event.GroupID)
	if err != nil {
		return err
	}
	if group == nil {
		return nil // grup bu arada silinmiş
	}
	members, err := s.groupRepo.GetMembers(event.GroupID)
	if err != nil {
		return err
	}

	only := idSet(event.Only)
	skip := idSet(event.Exclude)
	skip[actorID] = true
	if group.TargetUserID != nil {
		skip[*group.TargetUserID] = true // Anti-Spoiler: hedef kişiye o gruptan bildirim gitmez
	}

	params := models.NotificationParams{"actor_name": actorName}
	for k, v := range event.Params {
		params[k] = v
	}
	for _, m := range members {
		if skip[m.UserID] || (len(only) > 0 && !only[m.UserID]) {
			continue
		}
		created, err := s.repo.Insert(&models.Notification{
			UserID: m.UserID, GroupID: event.GroupID, Type: event.Type, Params: params, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		if created {
			s.notifier.Publish(m.UserID)
		}
	}
	return nil
}

func idSet(ids []uuid.UUID) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(ids)+2)
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func (s *notificationService) List(userID uuid.UUID) (*NotificationList, error) {
	items, err := s.repo.ListForUser(userID, NotificationListLimit)
	if err != nil {
		return nil, err
	}
	unread, err := s.repo.CountUnread(userID)
	if err != nil {
		return nil, err
	}
	return &NotificationList{Items: items, UnreadCount: unread}, nil
}

func (s *notificationService) MarkRead(userID, id uuid.UUID) error {
	found, err := s.repo.MarkRead(userID, id, s.now())
	if err != nil {
		return err
	}
	if !found {
		return ErrNotificationNotFound
	}
	// Kullanıcının diğer sekmelerindeki sayaç da güncellensin.
	s.notifier.Publish(userID)
	return nil
}

func (s *notificationService) MarkAllRead(userID uuid.UUID) error {
	changed, err := s.repo.MarkAllRead(userID, s.now())
	if err != nil {
		return err
	}
	if changed > 0 {
		s.notifier.Publish(userID)
	}
	return nil
}

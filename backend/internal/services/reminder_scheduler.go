package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

// ReminderScheduler hediye tarihine bağlı hatırlatma bildirimlerini üretir:
//
//   - 3 gün kala ve 1 gün kala: grubun tüm üyelerine (grup tamamlanmış olsa da; hediye yine verilecek).
//   - Tarih geçti ve grup hâlâ ACTIVE (kazanan seçilmedi): yalnızca yöneticilere, bir kez.
//
// MULTI gruplarda aynı kurallar her havuz için ayrı işler (anahtar havuz kimliğini taşır,
// bildirim pool_id ve recipient_name içerir): hatırlatmalar alıcı HARİÇ tüm üyelere, "süre doldu"
// yalnızca havuzun etkin yöneticisine (yoksa kimseye) gider. Havuzun alıcısına hiçbir şey gitmez.
//
// Her hatırlatma kullanıcı + dedupe_key benzersizliğiyle bir kez üretilir; anahtar tarihi de
// içerir, böylece tarih değişirse yeni tarih için yeniden hatırlatılır. Zamanlayıcı gün
// sayısına "tam 3" diye değil aralıkla bakar: API kapalıyken (ör. uykudayken) kaçan bir
// hatırlatma, API açıldığında kalan gün sayısıyla yine üretilir.
type ReminderScheduler struct {
	repo     repository.NotificationRepository
	notifier UserNotifier
	// Now saat kaynağıdır; testler sabit bir an enjekte eder.
	Now func() time.Time
	// Interval çalışmalar arasındaki süredir.
	Interval time.Duration
}

// NewReminderScheduler saatlik çalışan, gerçek saati kullanan bir zamanlayıcı oluşturur.
func NewReminderScheduler(repo repository.NotificationRepository, notifier UserNotifier) *ReminderScheduler {
	return &ReminderScheduler{repo: repo, notifier: notifier, Now: time.Now, Interval: time.Hour}
}

// Run başlangıçta bir kez, sonra her Interval'da RunOnce çağırır; ctx iptal edilince döner.
func (s *ReminderScheduler) Run(ctx context.Context) {
	s.runLogged()
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runLogged()
		}
	}
}

func (s *ReminderScheduler) runLogged() {
	created, err := s.RunOnce()
	if err != nil {
		slog.Error("hatırlatma zamanlayıcısı başarısız", "error", err)
		return
	}
	if created > 0 {
		slog.Info("hatırlatma bildirimleri üretildi", "count", created)
	}
}

// reminderWindowDays en erken hatırlatmanın etkinlikten kaç gün önce başladığıdır.
const reminderWindowDays = 3

// RunOnce o an gereken hatırlatmaları üretir ve yeni üretilen bildirim sayısını döner.
// Tekrar çalıştırmak güvenlidir (idempotent).
func (s *ReminderScheduler) RunOnce() (int, error) {
	// "Bugün" hediye tarihi doğrulamasıyla aynı saat dilimindedir (UTC+3).
	n := s.Now().In(turkeyTZ)
	today := models.Date{Time: time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)}
	until := models.Date{Time: today.AddDate(0, 0, reminderWindowDays)}

	groups, err := s.repo.ReminderGroups(today, until)
	if err != nil {
		return 0, err
	}
	pools, err := s.repo.ReminderPools(today, until)
	if err != nil {
		return 0, err
	}
	if len(groups) == 0 && len(pools) == 0 {
		return 0, nil
	}
	ids := make([]uuid.UUID, 0, len(groups)+len(pools))
	for _, g := range groups {
		ids = append(ids, g.ID)
	}
	for _, p := range pools {
		ids = append(ids, p.GroupID)
	}
	members, err := s.repo.MembersOfGroups(ids)
	if err != nil {
		return 0, err
	}
	byGroup := map[uuid.UUID][]repository.ReminderMember{}
	for _, m := range members {
		byGroup[m.GroupID] = append(byGroup[m.GroupID], m)
	}

	created := 0
	for _, g := range groups {
		reminder, ok := reminderFor(g.ID, g.EventDate, g.Status, today)
		if !ok {
			continue
		}
		for _, m := range byGroup[g.ID] {
			if reminder.adminsOnly && m.Role != "admin" {
				continue
			}
			// Anti-Spoiler: hedef kişi üye olamaz; yine de burada da dışlanır.
			if g.TargetUserID != nil && *g.TargetUserID == m.UserID {
				continue
			}
			key := reminder.dedupeKey
			inserted, err := s.repo.Insert(&models.Notification{
				UserID: m.UserID, GroupID: g.ID, Type: reminder.kind, Params: reminder.params,
				DedupeKey: &key, CreatedAt: s.Now(),
			})
			if err != nil {
				return created, err
			}
			if inserted {
				created++
				s.notifier.Publish(m.UserID)
			}
		}
	}
	for _, p := range pools {
		n, err := s.remindPool(p, byGroup[p.GroupID], today)
		created += n
		if err != nil {
			return created, err
		}
	}
	return created, nil
}

// remindPool bir havuzun hatırlatmalarını üretir. Alıcı hiçbir zaman alıcılar arasında değildir;
// "süre doldu" yalnızca etkin yöneticiye gider.
func (s *ReminderScheduler) remindPool(p repository.ReminderPool, members []repository.ReminderMember, today models.Date) (int, error) {
	reminder, ok := reminderFor(p.ID, p.EventDate, p.Status, today)
	if !ok {
		return 0, nil
	}
	adminID := uuid.Nil
	for _, m := range members {
		if m.Role == "admin" {
			adminID = m.UserID
			break
		}
	}
	manager := effectiveManagerID(&models.GiftPool{RecipientUserID: p.RecipientUserID, ManagerUserID: p.ManagerUserID}, adminID)
	params := models.NotificationParams{"recipient_name": p.RecipientName}
	for k, v := range reminder.params {
		params[k] = v
	}
	poolID := p.ID
	created := 0
	for _, m := range members {
		if m.UserID == p.RecipientUserID {
			continue // Anti-Spoiler: havuzun alıcısına o havuzdan hatırlatma gitmez
		}
		if reminder.adminsOnly && (manager == nil || *manager != m.UserID) {
			continue
		}
		key := reminder.dedupeKey
		inserted, err := s.repo.Insert(&models.Notification{
			UserID: m.UserID, GroupID: p.GroupID, PoolID: &poolID, Type: reminder.kind, Params: params,
			DedupeKey: &key, CreatedAt: s.Now(),
		})
		if err != nil {
			return created, err
		}
		if inserted {
			created++
			s.notifier.Publish(m.UserID)
		}
	}
	return created, nil
}

type reminder struct {
	kind       string
	dedupeKey  string
	params     models.NotificationParams
	adminsOnly bool
}

// reminderFor bir grubun ya da havuzun (key) bugün için gereken hatırlatmasını belirler. Anahtar
// "<tür>:<key>:<tarih>" biçimindedir; havuzda key havuz kimliğidir.
func reminderFor(key uuid.UUID, eventDate models.Date, status string, today models.Date) (reminder, bool) {
	date := eventDate.String()
	days := int(eventDate.Sub(today.Time).Hours() / 24)
	suffix := ":" + key.String() + ":" + date
	switch {
	case days < 0:
		if status != models.GroupStatusActive {
			return reminder{}, false
		}
		return reminder{
			kind: models.NotificationWinnerOverdue, dedupeKey: "overdue" + suffix,
			params: models.NotificationParams{"event_date": date}, adminsOnly: true,
		}, true
	case days <= 1:
		// 1 gün kala; o gün kaçtıysa etkinlik günü (days = 0) yine bir kez hatırlatılır.
		return reminder{
			kind: models.NotificationEventReminder, dedupeKey: "reminder:1d" + suffix,
			params: models.NotificationParams{"days_left": days, "event_date": date},
		}, true
	case days <= reminderWindowDays:
		// 3 gün kala; o gün kaçtıysa 2 gün kala yine bir kez hatırlatılır.
		return reminder{
			kind: models.NotificationEventReminder, dedupeKey: "reminder:3d" + suffix,
			params: models.NotificationParams{"days_left": days, "event_date": date},
		}, true
	}
	return reminder{}, false
}

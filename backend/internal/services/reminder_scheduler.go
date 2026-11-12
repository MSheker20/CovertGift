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
	if len(groups) == 0 {
		return 0, nil
	}
	ids := make([]uuid.UUID, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
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
		reminder, ok := reminderFor(g, today)
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
	return created, nil
}

type reminder struct {
	kind       string
	dedupeKey  string
	params     models.NotificationParams
	adminsOnly bool
}

// reminderFor grubun bugün için gereken hatırlatmasını belirler.
func reminderFor(g repository.ReminderGroup, today models.Date) (reminder, bool) {
	date := g.EventDate.String()
	days := int(g.EventDate.Sub(today.Time).Hours() / 24)
	suffix := ":" + g.ID.String() + ":" + date
	switch {
	case days < 0:
		if g.Status != models.GroupStatusActive {
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

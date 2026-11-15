package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// showcaseGroupTitle vitrin senaryosunun işaret grubudur; varsa vitrin bir daha eklenmez.
const showcaseGroupTitle = "Selin'in Mezuniyeti"

// pastGroupTitle kazananı seçilmiş, tarihi geçmiş (Geçmiş sürprizler) demo grubudur.
const pastGroupTitle = "Elif'in Doğum Günü"

// noopNotifier gerçek zamanlı yayın olmayan seed sürecinde bildirim sinyalini yutar.
type noopNotifier struct{}

func (noopNotifier) Publish(uuid.UUID) {}

// seedShowcase yeni özellikleri gösteren ek demo grupları ekler. Yalnızca ekler; işaret grubu
// varsa errAlreadySeeded döner. Üç senaryo:
//   - "Selin'in Mezuniyeti": hedef hesabı henüz bağlı değil, açıklama + kapak, davet linki, 2 gün sonra
//     hediye tarihi (hatırlatma bildirimi) ve bildirim akışı.
//   - "Elif'in Doğum Günü": kazananları seçilmiş, tarihi geçmiş grup (Geçmiş sürprizler).
func seedShowcase(tx *gorm.DB, users map[string]*models.User) error {
	var count int64
	if err := tx.Model(&models.Group{}).Where("title = ?", showcaseGroupTitle).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errAlreadySeeded
	}

	userRepo := repository.NewUserRepository(tx)
	groupRepo := repository.NewGroupRepository(tx)
	ideaRepo := repository.NewIdeaRepository(tx)
	poolRepo := repository.NewPoolRepository(tx)
	notifRepo := repository.NewNotificationRepository(tx)
	groupSvc := services.NewGroupService(groupRepo, userRepo, poolRepo)
	ideaSvc := services.NewIdeaService(ideaRepo, groupRepo, poolRepo, nil)
	inviteSvc := services.NewInviteService(repository.NewInviteRepository(tx), groupRepo, userRepo)
	notifSvc := services.NewNotificationService(notifRepo, groupRepo, poolRepo, userRepo, noopNotifier{})

	admin, member, friend, ahmet := users[demoAdmin.Email], users[demoMember.Email], users[demoFriend.Email], users[demoTarget.Email]
	now := time.Now()
	day := func(offset int) *models.Date {
		return &models.Date{Time: time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, time.UTC)}
	}

	// 1) Hedef hesabı bağlı olmayan, kapaklı, yaklaşan tarihli grup.
	group, err := groupSvc.CreateGroup(admin.ID, showcaseGroupTitle,
		"Selin bu ay mezun oluyor. Hesabı yok; davet linkiyle herkesi toplayıp sürprizi birlikte seçiyoruz.",
		"Selin", "Fotoğrafçılık\nSeyahat\nKitap", nil, "", decimal.RequireFromString("4000"), day(2))
	if err != nil {
		return fmt.Errorf("vitrin grubu: %w", err)
	}
	for _, email := range []string{demoMember.Email, demoFriend.Email, demoTarget.Email} {
		if _, err := groupSvc.AddMemberByEmail(group.ID, admin.ID, email); err != nil {
			return fmt.Errorf("vitrin %s üyeliği: %w", email, err)
		}
	}
	cover, err := coverPNG()
	if err != nil {
		return err
	}
	if _, err := groupSvc.SetCover(group.ID, admin.ID, cover); err != nil {
		return fmt.Errorf("kapak: %w", err)
	}
	invite, err := inviteSvc.Create(group.ID, admin.ID, 7)
	if err != nil {
		return fmt.Errorf("davet linki: %w", err)
	}

	ideas := []struct {
		by                 *models.User
		title, desc, price string
		query              string
	}{
		{member, "Instax Mini 12 Anlık Fotoğraf Makinesi", "Anı biriktirmeyi sever", "3200", "instax mini 12"},
		{friend, "Kapadokya Hediye Balon Turu", "Mezuniyet için unutulmaz bir anı", "5500", "kapadokya balon turu hediye"},
		{ahmet, "Gezgin Not Defteri Seti", "Seyahat günlüğü için", "650", "seyahat not defteri seti"},
	}
	created := make([]*models.GiftIdea, 0, len(ideas))
	for _, it := range ideas {
		idea, err := ideaSvc.AddManualIdea(repository.GroupScope(group.ID), it.by.ID, it.title, it.desc, it.query, "", "", decimal.RequireFromString(it.price))
		if err != nil {
			return fmt.Errorf("vitrin fikri %q: %w", it.title, err)
		}
		created = append(created, idea)
	}
	for _, v := range []struct {
		idea *models.GiftIdea
		by   *models.User
		v    int8
	}{{created[0], admin, 1}, {created[0], friend, 1}, {created[0], ahmet, 1}, {created[1], member, 1}, {created[2], admin, -1}} {
		if err := ideaSvc.VoteIdea(v.idea.ID, v.by.ID, v.v); err != nil {
			return fmt.Errorf("vitrin oyu: %w", err)
		}
	}
	// Bildirim akışı: Can'a (yönetici) ve diğer üyelere gelen örnek olaylar.
	notifSvc.Dispatch(member.ID, []services.NotificationEvent{
		{GroupID: group.ID, Type: models.NotificationIdeaAdded, Params: models.NotificationParams{"idea_title": created[0].Title}},
	})
	notifSvc.Dispatch(friend.ID, []services.NotificationEvent{
		{GroupID: group.ID, Type: models.NotificationAIIdeasAdded, Params: models.NotificationParams{"count": 4}},
		{GroupID: group.ID, Type: models.NotificationBudgetChanged, Params: models.NotificationParams{"total_budget": "4000"}},
	})
	notifSvc.Dispatch(ahmet.ID, []services.NotificationEvent{
		{GroupID: group.ID, Type: models.NotificationMemberJoined, Params: models.NotificationParams{"member_name": ahmet.Name}},
	})

	// 2) Kazananları seçilmiş, tarihi geçmiş grup (hedef: Elif, bağlı hesap; Elif grubu göremez).
	past, err := groupSvc.CreateGroup(admin.ID, pastGroupTitle, "Geçen haftaki sürpriz; hediye verildi.",
		"Elif", "Yoga\nBitkiler", nil, demoFriend.Email, decimal.RequireFromString("2000"), day(-5))
	if err != nil {
		return fmt.Errorf("geçmiş grup: %w", err)
	}
	for _, email := range []string{demoMember.Email, demoTarget.Email} {
		if _, err := groupSvc.AddMemberByEmail(past.ID, admin.ID, email); err != nil {
			return fmt.Errorf("geçmiş grup %s üyeliği: %w", email, err)
		}
	}
	win, err := ideaSvc.AddManualIdea(repository.GroupScope(past.ID), member.ID, "Yoga Matı ve Blok Seti", "Evde pratik için", "yoga matı blok seti", "", "", decimal.RequireFromString("1300"))
	if err != nil {
		return fmt.Errorf("geçmiş grup fikri: %w", err)
	}
	if err := groupSvc.SelectWinners(past.ID, admin.ID, []uuid.UUID{win.ID}); err != nil {
		return fmt.Errorf("kazanan seçimi: %w", err)
	}

	// Hatırlatma zamanlayıcısını bir kez çalıştırır: 2 gün kalan grup için "event_reminder" üretir.
	if _, err := services.NewReminderScheduler(notifRepo, noopNotifier{}).RunOnce(); err != nil {
		return fmt.Errorf("hatırlatmalar: %w", err)
	}

	fmt.Printf("\n\"%s\": hedef hesabı bağlı değil, kapak + davet linki + 2 gün kala hatırlatma + bildirimler.\n", showcaseGroupTitle)
	fmt.Printf("  davet linki (7 gün geçerli): /invite/%s\n", invite.Token)
	fmt.Printf("\"%s\": kazanan seçilmiş, Geçmiş sürprizler'de görünür.\n", pastGroupTitle)
	return nil
}

// coverPNG demo kapağı için 800x600'lük degrade bir görsel üretir.
func coverPNG() ([]byte, error) {
	const w, h = 800, 600
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(47 + x*120/w), G: uint8(91 + y*100/h), B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

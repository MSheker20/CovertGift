package main

import (
	"fmt"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// visualShowcaseTitle görsel vitrin senaryosunun işaret grubudur; varsa senaryo bir daha eklenmez.
const visualShowcaseTitle = "Mert'in Veda Hediyesi"

const (
	houseGroupTitle = "Zeynep'in Yeni Evi"
	emptyGroupTitle = "Deniz'in Doğum Günü"
)

// seedVisualShowcase arayüzün yeni görünümlerini demo verisinde görünür kılar:
//   - "Mert'in Veda Hediyesi": Zeynep yönetir. Can'ı ekler, kazananı seçer, grubu yeniden açıp
//     tekrar tamamlar. Can'ın panelinde yeşil (eklendin), altın (kazanan) ve nötr (yeniden açıldı)
//     bildirimler; hepsi 1–3 gün önceye tarihli ve okunmuş. Grup Geçmiş sürprizler'de görünür.
//   - "Zeynep'in Yeni Evi": Can yönetir, kapaklı, tarihi yarın (Sıradaki sürpriz'de "Yarın" ve
//     nabız noktası). Hedef Zeynep'in hesabı bağlı; Zeynep grubu ve bildirimlerini göremez.
//   - "Deniz'in Doğum Günü": fikri ve tarihi yok (havuzun boş durum çizimi, "tarih ekle" ipucu).
//
// Bildirimler gerçek akıştan geçer (NotificationService.Dispatch, ReminderScheduler); seed
// Anti-Spoiler kuralını ayrıca yazmaz. Yalnızca bu adımın ürettiği satırların tarihi değiştirilir.
func seedVisualShowcase(tx *gorm.DB, users map[string]*models.User) error {
	var count int64
	if err := tx.Model(&models.Group{}).Where("title = ?", visualShowcaseTitle).Count(&count).Error; err != nil {
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
	notifSvc := services.NewNotificationService(notifRepo, groupRepo, poolRepo, userRepo, noopNotifier{})

	admin, member, friend, ahmet := users[demoAdmin.Email], users[demoMember.Email], users[demoFriend.Email], users[demoTarget.Email]
	now := time.Now()
	day := func(offset int) *models.Date {
		return &models.Date{Time: time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, time.UTC)}
	}
	// backdate bu adımda üretilmiş, henüz tarihlenmemiş bildirimleri (since'ten sonra yazılmış)
	// geçmişe taşır; read true ise bir saat sonra okunmuş sayar.
	since := now.Add(-time.Second)
	backdate := func(groupID uuid.UUID, ago time.Duration, read bool) error {
		at := now.Add(-ago)
		updates := map[string]any{"created_at": at}
		if read {
			updates["read_at"] = at.Add(time.Hour)
		}
		res := tx.Model(&models.Notification{}).Where("group_id = ? AND created_at >= ?", groupID, since).Updates(updates)
		return res.Error
	}

	// 1) Zeynep'in yönettiği, tamamlanmış grup.
	farewell, err := groupSvc.CreateGroup(member.ID, visualShowcaseTitle,
		"Mert başka şehre taşınıyor; ofisten küçük bir veda hediyesi.", "Mert", "Bisiklet\nKahve",
		nil, "", decimal.RequireFromString("1800"), day(-3))
	if err != nil {
		return fmt.Errorf("veda grubu: %w", err)
	}
	for _, u := range []*models.User{admin, friend} {
		if _, err := groupSvc.AddMemberByEmail(farewell.ID, member.ID, u.Email); err != nil {
			return fmt.Errorf("veda grubu %s üyeliği: %w", u.Email, err)
		}
		notifSvc.Dispatch(member.ID, []services.NotificationEvent{
			{GroupID: farewell.ID, Type: models.NotificationAddedToGroup, Only: []uuid.UUID{u.ID}},
		})
	}
	if err := backdate(farewell.ID, 72*time.Hour, true); err != nil {
		return err
	}
	var ideas []*models.GiftIdea
	for _, it := range []struct {
		by           *models.User
		title, price string
		query        string
	}{
		{admin, "Bisiklet Gidon Çantası", "750", "bisiklet gidon çantası"},
		{friend, "Seyahat Tipi Kahve Değirmeni", "1350", "manuel kahve değirmeni seyahat"},
	} {
		idea, err := ideaSvc.AddManualIdea(repository.GroupScope(farewell.ID), it.by.ID, it.title, "", it.query, "", "", decimal.RequireFromString(it.price))
		if err != nil {
			return fmt.Errorf("veda fikri %q: %w", it.title, err)
		}
		ideas = append(ideas, idea)
	}
	if err := ideaSvc.VoteIdea(ideas[1].ID, admin.ID, 1); err != nil {
		return fmt.Errorf("veda oyu: %w", err)
	}
	// Kazanan seçilir, grup yeniden açılır, sonra yeniden tamamlanır.
	if err := groupSvc.SelectWinners(farewell.ID, member.ID, []uuid.UUID{ideas[0].ID}); err != nil {
		return fmt.Errorf("veda kazananı: %w", err)
	}
	notifSvc.Dispatch(member.ID, []services.NotificationEvent{
		{GroupID: farewell.ID, Type: models.NotificationWinnerSelected, Params: models.NotificationParams{"count": 1}},
	})
	if err := backdate(farewell.ID, 50*time.Hour, true); err != nil {
		return err
	}
	if err := groupSvc.ReopenGroup(farewell.ID, member.ID); err != nil {
		return fmt.Errorf("veda grubu yeniden açma: %w", err)
	}
	notifSvc.Dispatch(member.ID, []services.NotificationEvent{
		{GroupID: farewell.ID, Type: models.NotificationGroupReopened},
	})
	if err := backdate(farewell.ID, 48*time.Hour, true); err != nil {
		return err
	}
	if err := groupSvc.SelectWinners(farewell.ID, member.ID, []uuid.UUID{ideas[1].ID}); err != nil {
		return fmt.Errorf("veda kazananı (ikinci): %w", err)
	}
	notifSvc.Dispatch(member.ID, []services.NotificationEvent{
		{GroupID: farewell.ID, Type: models.NotificationWinnerSelected, Params: models.NotificationParams{"count": 1}},
	})
	if err := backdate(farewell.ID, 26*time.Hour, true); err != nil {
		return err
	}
	// Tamamlanma tarihi son kazanan bildirimiyle aynı ana (dün) çekilir; Geçmiş sürprizler'de tutarlı görünsün.
	if err := tx.Model(&models.Group{}).Where("id = ?", farewell.ID).Update("completed_at", now.Add(-26*time.Hour)).Error; err != nil {
		return fmt.Errorf("veda grubu tamamlanma tarihi: %w", err)
	}

	// 2) Yarınki sürpriz: kapaklı, hedefi bağlı (Zeynep göremez).
	house, err := groupSvc.CreateGroup(admin.ID, houseGroupTitle,
		"Zeynep yeni evine taşındı; eve dair işe yarar bir hediye arıyoruz.", "Zeynep", "Bitkiler\nKahve\nMinimal dekor",
		nil, demoMember.Email, decimal.RequireFromString("2400"), day(1))
	if err != nil {
		return fmt.Errorf("yeni ev grubu: %w", err)
	}
	for _, email := range []string{demoFriend.Email, demoTarget.Email} {
		if _, err := groupSvc.AddMemberByEmail(house.ID, admin.ID, email); err != nil {
			return fmt.Errorf("yeni ev %s üyeliği: %w", email, err)
		}
	}
	cover, err := coverPNG()
	if err != nil {
		return err
	}
	if _, err := groupSvc.SetCover(house.ID, admin.ID, cover); err != nil {
		return fmt.Errorf("yeni ev kapağı: %w", err)
	}
	houseIdeas := []struct {
		by           *models.User
		title, price string
		query        string
	}{
		{friend, "Kendinden Sulamalı Saksı Seti", "890", "kendinden sulamalı saksı seti"},
		{ahmet, "Filtre Kahve Makinesi", "2100", "filtre kahve makinesi"},
	}
	for _, it := range houseIdeas {
		idea, err := ideaSvc.AddManualIdea(repository.GroupScope(house.ID), it.by.ID, it.title, "", it.query, "", "", decimal.RequireFromString(it.price))
		if err != nil {
			return fmt.Errorf("yeni ev fikri %q: %w", it.title, err)
		}
		notifSvc.Dispatch(it.by.ID, []services.NotificationEvent{
			{GroupID: house.ID, Type: models.NotificationIdeaAdded, Params: models.NotificationParams{"idea_title": idea.Title}},
		})
	}
	// Fikir bildirimleri okunmamış kalır, birkaç saat öncesine tarihlenir.
	if err := backdate(house.ID, 3*time.Hour, false); err != nil {
		return err
	}

	// 3) Boş grup: fikir ve tarih yok.
	empty, err := groupSvc.CreateGroup(admin.ID, emptyGroupTitle, "", "Deniz", "",
		nil, "", decimal.RequireFromString("1500"), nil)
	if err != nil {
		return fmt.Errorf("boş grup: %w", err)
	}
	if _, err := groupSvc.AddMemberByEmail(empty.ID, admin.ID, demoMember.Email); err != nil {
		return fmt.Errorf("boş grup üyeliği: %w", err)
	}

	// Yarınki grup için "hediye günü yarın" hatırlatmasını üretir (dedupe_key ile bir kez).
	if _, err := services.NewReminderScheduler(notifRepo, noopNotifier{}).RunOnce(); err != nil {
		return fmt.Errorf("hatırlatmalar: %w", err)
	}

	fmt.Printf("\"%s\": Zeynep yönetir; eklendin, kazanan, yeniden açıldı bildirimleri (okunmuş, 1–3 gün önce).\n", visualShowcaseTitle)
	fmt.Printf("\"%s\": tarihi yarın, kapaklı; Sıradaki sürpriz olur. \"%s\": fikri ve tarihi yok.\n", houseGroupTitle, emptyGroupTitle)
	return nil
}

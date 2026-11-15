package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// poolShowcaseTitle havuz bildirimleri vitrininin işaret grubudur; varsa vitrin bir daha eklenmez.
const poolShowcaseTitle = "Ekip Doğum Günleri"

// seedExtras temel demo verisinin üstüne vitrin senaryolarını ekler. Her senaryo kendi işaret
// grubuyla korunur; ikisi de zaten varsa errAlreadySeeded döner.
func seedExtras(tx *gorm.DB, users map[string]*models.User) error {
	shown := seedShowcase(tx, users)
	if shown != nil && !errors.Is(shown, errAlreadySeeded) {
		return shown
	}
	pooled := seedPoolNotifications(tx, users)
	if pooled != nil && !errors.Is(pooled, errAlreadySeeded) {
		return pooled
	}
	if shown != nil && pooled != nil {
		return errAlreadySeeded
	}
	return nil
}

// seedPoolNotifications MULTI havuz bildirimlerini gösterir. Bildirimler gerçek akıştan geçer
// (NotificationService.Dispatch ve ReminderScheduler): havuzun alıcısı alıcılardan çıkarılır ve
// recipient_name eklenir (Anti-Spoiler), seed ayrıca kural yazmaz.
//   - "Ofis Yılbaşı": kişi eklendi, sorumlu atandı, fikir eklendi, tarih değişti.
//   - "Ekip Doğum Günleri": Ahmet'in havuzu 2 gün sonra (hatırlatma), Elif'in havuzunun tarihi
//     geçmiş ve kazanan seçilmemiş ("süre doldu", yalnızca havuzun sorumlusu Can'a).
//
// Sorumlu yalnızca grup yöneticisinin kendi havuzuna atanabildiği için iki havuzu da Can yönetir.
func seedPoolNotifications(tx *gorm.DB, users map[string]*models.User) error {
	var count int64
	if err := tx.Model(&models.Group{}).Where("title = ?", poolShowcaseTitle).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errAlreadySeeded
	}

	userRepo := repository.NewUserRepository(tx)
	groupRepo := repository.NewGroupRepository(tx)
	poolRepo := repository.NewPoolRepository(tx)
	notifRepo := repository.NewNotificationRepository(tx)
	groupSvc := services.NewGroupService(groupRepo, userRepo, poolRepo)
	poolSvc := services.NewPoolService(groupRepo, poolRepo)
	ideaSvc := services.NewIdeaService(repository.NewIdeaRepository(tx), groupRepo, poolRepo, nil)
	notifSvc := services.NewNotificationService(notifRepo, groupRepo, poolRepo, userRepo, noopNotifier{})
	admin, member, friend, ahmet := users[demoAdmin.Email], users[demoMember.Email], users[demoFriend.Email], users[demoTarget.Email]

	// 1) "Ofis Yılbaşı" olayları. Havuzlar alıcıya göre bulunur (önce Zeynep, sonra Can).
	var office models.Group
	if err := tx.Where("title = ? AND mode = ?", multiGroupTitle, models.GroupModeMulti).First(&office).Error; err != nil {
		return fmt.Errorf("Ofis Yılbaşı: %w", err)
	}
	pools, err := poolRepo.ListPools(office.ID)
	if err != nil {
		return err
	}
	var zeynepPool, canPool *models.GiftPool
	for i := range pools {
		switch pools[i].RecipientUserID {
		case member.ID:
			zeynepPool = &pools[i]
		case admin.ID:
			canPool = &pools[i]
		}
	}
	if zeynepPool == nil || canPool == nil {
		return fmt.Errorf("Ofis Yılbaşı havuzları eksik")
	}
	notifSvc.Dispatch(admin.ID, []services.NotificationEvent{
		{GroupID: office.ID, PoolID: &zeynepPool.ID, Type: models.NotificationPoolAdded},
		{GroupID: office.ID, PoolID: &canPool.ID, Type: models.NotificationPoolManagerAssigned, Only: []uuid.UUID{friend.ID}},
	})
	notifSvc.Dispatch(ahmet.ID, []services.NotificationEvent{
		{GroupID: office.ID, PoolID: &zeynepPool.ID, Type: models.NotificationIdeaAdded, Params: models.NotificationParams{"idea_title": "Seramik Kupa Atölyesi"}},
	})
	notifSvc.Dispatch(member.ID, []services.NotificationEvent{
		{GroupID: office.ID, PoolID: &canPool.ID, Type: models.NotificationIdeaAdded, Params: models.NotificationParams{"idea_title": "Bluetooth Kulaklık"}},
	})
	if canPool.EventDate != nil {
		notifSvc.Dispatch(friend.ID, []services.NotificationEvent{
			{GroupID: office.ID, PoolID: &canPool.ID, Type: models.NotificationEventDateChanged, Params: models.NotificationParams{"event_date": canPool.EventDate.String()}},
		})
	}

	// 2) Hatırlatma ve "süre doldu" için yakın ve geçmiş tarihli ikinci MULTI grup.
	group, err := groupSvc.CreateMultiGroup(admin.ID, poolShowcaseTitle, "Ekipteki doğum günleri: her kişi için ayrı havuz")
	if err != nil {
		return fmt.Errorf("%s: %w", poolShowcaseTitle, err)
	}
	for _, email := range []string{demoMember.Email, demoFriend.Email, demoTarget.Email} {
		if _, err := groupSvc.AddMemberByEmail(group.ID, admin.ID, email); err != nil {
			return fmt.Errorf("%s %s üyeliği: %w", poolShowcaseTitle, email, err)
		}
	}
	now := time.Now()
	day := func(offset int) *models.Date {
		return &models.Date{Time: time.Date(now.Year(), now.Month(), now.Day()+offset, 0, 0, 0, 0, time.UTC)}
	}
	_, ahmetPool, err := poolSvc.AddRecipient(group.ID, admin.ID, ahmet.ID, nil)
	if err != nil {
		return fmt.Errorf("Ahmet havuzu: %w", err)
	}
	_, elifPool, err := poolSvc.AddRecipient(group.ID, admin.ID, friend.ID, nil)
	if err != nil {
		return fmt.Errorf("Elif havuzu: %w", err)
	}
	for _, p := range []struct {
		pool    *models.GiftPool
		manager *models.User
		budget  string
		date    *models.Date
	}{{ahmetPool, admin, "1200", day(2)}, {elifPool, admin, "900", day(-3)}} {
		if err := poolSvc.UpdateBudget(group.ID, p.pool.ID, p.manager.ID, decimal.RequireFromString(p.budget)); err != nil {
			return fmt.Errorf("havuz bütçesi: %w", err)
		}
		if err := poolSvc.SetEventDate(group.ID, p.pool.ID, p.manager.ID, p.date); err != nil {
			return fmt.Errorf("havuz tarihi: %w", err)
		}
	}
	if _, err := ideaSvc.AddManualIdea(repository.PoolScope(group.ID, ahmetPool.ID), member.ID, "Analog Fotoğraf Makinesi",
		"Eski usul fotoğraf çekmeyi seviyor", "analog fotoğraf makinesi", "", "", decimal.RequireFromString("1100")); err != nil {
		return fmt.Errorf("Ahmet havuzu fikri: %w", err)
	}
	if _, err := ideaSvc.AddManualIdea(repository.PoolScope(group.ID, elifPool.ID), admin.ID, "Yoga Kampı Hafta Sonu",
		"Doğada iki günlük kamp", "yoga kampı hafta sonu", "", "", decimal.RequireFromString("850")); err != nil {
		return fmt.Errorf("Elif havuzu fikri: %w", err)
	}

	// Ahmet'in havuzu için event_reminder (Can, Zeynep, Elif), Elif'in havuzu için winner_overdue (Can).
	if _, err := services.NewReminderScheduler(notifRepo, noopNotifier{}).RunOnce(); err != nil {
		return fmt.Errorf("havuz hatırlatmaları: %w", err)
	}

	fmt.Printf("\"%s\": Ahmet'in havuzu 2 gün sonra (hatırlatma), Elif'in havuzunun tarihi geçti (süre doldu, sorumlu %s).\n",
		poolShowcaseTitle, demoAdmin.Name)
	return nil
}

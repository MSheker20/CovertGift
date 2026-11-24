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

// friendsShowcaseTitle arkadaşlık senaryosunun işaret grubudur; varsa senaryo bir daha eklenmez.
const friendsShowcaseTitle = "Elif'in Mezuniyeti"

// seedFriends arkadaşlık ve hızlı grup akışını demoya hazırlar (Can ile giriş yapılır):
//   - Can, Zeynep ve Elif birbirinin arkadaşı. Ahmet → Can isteği bekliyor: Can'ın Arkadaşlar
//     rozetinde 1 gelen istek. Kabul edilince Ahmet "Kime hediye?" listesinden seçilebilir.
//   - "Elif'in Mezuniyeti": Zeynep kurar, hedef Elif (hesabı bağlı; grubu ve daveti hiç görmez),
//     Can'ı davet eder. Can'ın ana sayfasında grup daveti, zilinde "seni davet etti" bildirimi.
//
// Arkadaşlıklar ve davet gerçek repository/servislerden geçer; Anti-Spoiler ayrıca yazılmaz.
func seedFriends(tx *gorm.DB, users map[string]*models.User) error {
	var count int64
	if err := tx.Model(&models.Group{}).Where("title = ?", friendsShowcaseTitle).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errAlreadySeeded
	}

	userRepo := repository.NewUserRepository(tx)
	groupRepo := repository.NewGroupRepository(tx)
	poolRepo := repository.NewPoolRepository(tx)
	friendRepo := repository.NewFriendRepository(tx)
	friendSvc := services.NewFriendService(friendRepo, repository.NewMemberInviteRepository(tx), groupRepo, userRepo)
	groupSvc := services.NewGroupService(groupRepo, userRepo, poolRepo)
	notifSvc := services.NewNotificationService(repository.NewNotificationRepository(tx), groupRepo, poolRepo, userRepo, noopNotifier{})

	can, zeynep, elif, ahmet := users[demoAdmin.Email], users[demoMember.Email], users[demoFriend.Email], users[demoTarget.Email]
	now := time.Now()
	for _, pair := range [][2]*models.User{{can, zeynep}, {can, elif}, {zeynep, elif}} {
		if _, err := friendRepo.Befriend(pair[0].ID, pair[1].ID, now); err != nil {
			return fmt.Errorf("arkadaşlık %s–%s: %w", pair[0].Name, pair[1].Name, err)
		}
	}
	// Bekleyen istek: zaten arkadaşlarsa (demo sırasında kabul edilmişse) dokunulmaz.
	if _, err := friendSvc.Request(ahmet.ID, can.Email); err != nil && !errors.Is(err, services.ErrFriendshipExists) {
		return fmt.Errorf("Ahmet → Can isteği: %w", err)
	}

	group, err := groupSvc.CreateGroup(zeynep.ID, friendsShowcaseTitle,
		"Elif mezun oluyor! Fikirleri cuma akşamına kadar ekleyelim.",
		"Elif", "Fotoğrafçılık, seyahat, vintage plaklar", nil, elif.Email,
		decimal.RequireFromString("2500"), nil)
	if err != nil {
		return fmt.Errorf("%s grubu: %w", friendsShowcaseTitle, err)
	}
	invited, err := friendSvc.InviteCreatedGroup(group.ID, zeynep.ID, []uuid.UUID{can.ID})
	if err != nil {
		return fmt.Errorf("Can daveti: %w", err)
	}
	notifSvc.Dispatch(zeynep.ID, []services.NotificationEvent{{
		GroupID: group.ID, Type: models.NotificationGroupInvite, Only: invited, NonMembers: true,
	}})
	return nil
}

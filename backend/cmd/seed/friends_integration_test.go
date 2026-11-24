package main

import (
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
)

func TestSeedFriends(t *testing.T) {
	db := openTestDB(t)
	if err := seed(db, "Demo1234!"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	users := map[string]models.User{}
	for _, u := range demoUsers {
		var found models.User
		if err := db.Where("email = ?", u.Email).First(&found).Error; err != nil {
			t.Fatal(err)
		}
		users[u.Email] = found
	}
	can, elif := users[demoAdmin.Email], users[demoFriend.Email]
	friends := repository.NewFriendRepository(db)

	rows, err := friends.List(can.ID)
	if err != nil {
		t.Fatal(err)
	}
	accepted, incoming := 0, 0
	for _, r := range rows {
		switch {
		case r.Status == models.FriendshipAccepted:
			accepted++
		case r.RequesterID != can.ID:
			incoming++
		}
	}
	if accepted != 2 || incoming != 1 {
		t.Fatalf("Can: %d arkadaş, %d gelen istek; 2 ve 1 bekleniyordu", accepted, incoming)
	}

	invites := repository.NewMemberInviteRepository(db)
	canInvites, err := invites.ListForUser(can.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(canInvites) != 1 || canInvites[0].GroupTitle != friendsShowcaseTitle {
		t.Fatalf("Can'in davetleri: %+v", canInvites)
	}
	var notif int64
	db.Model(&models.Notification{}).Where("user_id = ? AND group_id = ? AND type = ?", can.ID, canInvites[0].GroupID, models.NotificationGroupInvite).Count(&notif)
	if notif != 1 {
		t.Fatalf("Can'in davet bildirimi %d", notif)
	}

	// Anti-Spoiler: hedef Elif grubu, daveti ve bildirimi hiç görmez.
	elifInvites, err := invites.ListForUser(elif.ID)
	if err != nil {
		t.Fatal(err)
	}
	var elifNotif int64
	db.Model(&models.Notification{}).Where("user_id = ? AND group_id = ?", elif.ID, canInvites[0].GroupID).Count(&elifNotif)
	if len(elifInvites) != 0 || elifNotif != 0 {
		t.Fatalf("hedef Elif'e sızıntı: %d davet, %d bildirim", len(elifInvites), elifNotif)
	}

	// İkinci çalıştırma bir şey eklemez.
	if err := seed(db, "Demo1234!"); err != errAlreadySeeded {
		t.Fatalf("ikinci seed: %v", err)
	}
}

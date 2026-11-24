package repository

import (
	"errors"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrMemberInviteNotFound davet yoksa, başkasınınsa ya da grup silinmişse döner.
var ErrMemberInviteNotFound = errors.New("member invite not found")

// PendingInviteRow grubun bekleyen davetlerinden biridir (grup sayfası).
type PendingInviteRow struct {
	UserID        uuid.UUID `json:"user_id" format:"uuid"`
	Name          string    `json:"name" example:"Can"`
	AvatarVersion string    `json:"avatar_version,omitempty"`
	InviterName   string    `json:"inviter_name" example:"Zeynep"`
	CreatedAt     time.Time `json:"created_at"`
}

// MyInviteRow kullanıcıya gelen bir grup davetidir.
type MyInviteRow struct {
	ID          uuid.UUID `json:"id" format:"uuid"`
	GroupID     uuid.UUID `json:"group_id" format:"uuid"`
	GroupTitle  string    `json:"group_title" example:"Ahmet'in 30. Yaş Sürprizi"`
	GroupMode   string    `json:"group_mode" example:"SINGLE"`
	InviterName string    `json:"inviter_name" example:"Zeynep"`
	MemberCount int64     `json:"member_count" example:"3"`
	CreatedAt   time.Time `json:"created_at"`
}

// MemberInviteResult davet kabul/ret işleminin sonucudur.
type MemberInviteResult struct {
	GroupID uuid.UUID
	// Joined kabulde yeni üye satırı oluştuysa true.
	Joined bool
}

// MemberInviteRepository arkadaşların gruba davetlerini yönetir. Yazan her işlem önce grup
// satırını kilitler (groups → group_member_invites → group_members), hedef bağlama ve üye ekleme
// ile aynı sırada.
type MemberInviteRepository interface {
	// Invite davetleri yazar ve gerçekten yeni davet edilen kullanıcıları döner. Hedef kişi
	// ErrMemberIsGroupTarget; zaten üye ya da zaten davetli olanlar sessizce atlanır.
	Invite(groupID, inviterID uuid.UUID, inviteeIDs []uuid.UUID) ([]uuid.UUID, error)
	ListForGroup(groupID uuid.UUID) ([]PendingInviteRow, error)
	// Cancel admin iptali; yoksa ErrMemberInviteNotFound.
	Cancel(groupID, inviteeID uuid.UUID) error
	// ListForUser kullanıcının davetleri. Hedef kişisi olduğu grupların davetleri hiç dönmez.
	ListForUser(userID uuid.UUID) ([]MyInviteRow, error)
	// Accept daveti üyeliğe çevirir. Grup kilitliyken hedef kontrolü tekrarlanır
	// (ErrMemberIsGroupTarget). Davetin zil bildirimi de silinir.
	Accept(inviteID, userID uuid.UUID) (*MemberInviteResult, error)
	// Decline daveti ve zil bildirimini siler.
	Decline(inviteID, userID uuid.UUID) (*MemberInviteResult, error)
}

type memberInviteRepository struct {
	db *gorm.DB
}

// NewMemberInviteRepository yeni bir MemberInviteRepository oluşturur.
func NewMemberInviteRepository(db *gorm.DB) MemberInviteRepository {
	return &memberInviteRepository{db: db}
}

// insertInvites grup satırı kilitliyken çağrılmalıdır. Hedef kişiyi reddeder; üye ve zaten davetli
// olanları atlar.
func insertInvites(tx *gorm.DB, groupID, inviterID uuid.UUID, inviteeIDs []uuid.UUID) ([]uuid.UUID, error) {
	invited := []uuid.UUID{}
	if len(inviteeIDs) == 0 {
		return invited, nil
	}
	var group models.Group
	if err := tx.Select("id", "target_user_id").First(&group, "id = ?", groupID).Error; err != nil {
		return nil, err
	}
	var skip []uuid.UUID
	if err := tx.Model(&models.GroupMember{}).Where("group_id = ? AND user_id IN ?", groupID, inviteeIDs).
		Pluck("user_id", &skip).Error; err != nil {
		return nil, err
	}
	skipSet := map[uuid.UUID]bool{}
	for _, id := range skip {
		skipSet[id] = true
	}
	for _, id := range inviteeIDs {
		if group.TargetUserID != nil && *group.TargetUserID == id {
			return nil, ErrMemberIsGroupTarget
		}
		if skipSet[id] || id == inviterID {
			continue
		}
		skipSet[id] = true
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "group_id"}, {Name: "invitee_id"}},
			DoNothing: true,
		}).Create(&models.GroupMemberInvite{GroupID: groupID, InviteeID: id, InviterID: inviterID})
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected > 0 {
			invited = append(invited, id)
		}
	}
	return invited, nil
}

func (r *memberInviteRepository) Invite(groupID, inviterID uuid.UUID, inviteeIDs []uuid.UUID) ([]uuid.UUID, error) {
	var invited []uuid.UUID
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockGroup(tx, groupID); err != nil {
			return err
		}
		var err error
		invited, err = insertInvites(tx, groupID, inviterID, inviteeIDs)
		return err
	})
	return invited, err
}

func (r *memberInviteRepository) ListForGroup(groupID uuid.UUID) ([]PendingInviteRow, error) {
	rows := []PendingInviteRow{}
	err := r.db.Raw(`SELECT u.id AS user_id, u.name, u.avatar_version, inv.name AS inviter_name, i.created_at
		FROM group_member_invites i
		JOIN users u ON u.id = i.invitee_id
		JOIN users inv ON inv.id = i.inviter_id
		WHERE i.group_id = ?
		ORDER BY i.created_at, u.id`, groupID).Scan(&rows).Error
	return rows, err
}

func (r *memberInviteRepository) Cancel(groupID, inviteeID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockGroup(tx, groupID); err != nil {
			return err
		}
		res := tx.Where("group_id = ? AND invitee_id = ?", groupID, inviteeID).Delete(&models.GroupMemberInvite{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrMemberInviteNotFound
		}
		return deleteInviteNotifications(tx, groupID, inviteeID)
	})
}

func deleteInviteNotifications(tx *gorm.DB, groupID, userID uuid.UUID) error {
	return tx.Where("group_id = ? AND user_id = ? AND type = ?", groupID, userID, models.NotificationGroupInvite).
		Delete(&models.Notification{}).Error
}

func (r *memberInviteRepository) ListForUser(userID uuid.UUID) ([]MyInviteRow, error) {
	rows := []MyInviteRow{}
	err := r.db.Raw(`SELECT i.id, g.id AS group_id, g.title AS group_title, g.mode AS group_mode,
			inv.name AS inviter_name,
			(SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
			i.created_at
		FROM group_member_invites i
		JOIN groups g ON g.id = i.group_id
		JOIN users inv ON inv.id = i.inviter_id
		WHERE i.invitee_id = ? AND (g.target_user_id IS NULL OR g.target_user_id <> ?)
		ORDER BY i.created_at DESC, i.id`, userID, userID).Scan(&rows).Error
	return rows, err
}

// loadOwnInvite daveti okur ve grup satırını kilitler; davet çağıranın değilse ErrMemberInviteNotFound.
func loadOwnInvite(tx *gorm.DB, inviteID, userID uuid.UUID) (*models.GroupMemberInvite, *models.Group, error) {
	var invite models.GroupMemberInvite
	if err := tx.First(&invite, "id = ? AND invitee_id = ?", inviteID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrMemberInviteNotFound
		}
		return nil, nil, err
	}
	group, err := lockGroup(tx, invite.GroupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrMemberInviteNotFound
		}
		return nil, nil, err
	}
	// Kilitten sonra davet yeniden okunur: bu arada iptal edilmiş olabilir.
	var count int64
	if err := tx.Model(&models.GroupMemberInvite{}).Where("id = ?", inviteID).Count(&count).Error; err != nil {
		return nil, nil, err
	}
	if count == 0 {
		return nil, nil, ErrMemberInviteNotFound
	}
	return &invite, group, nil
}

func (r *memberInviteRepository) Accept(inviteID, userID uuid.UUID) (*MemberInviteResult, error) {
	var result *MemberInviteResult
	err := r.db.Transaction(func(tx *gorm.DB) error {
		invite, group, err := loadOwnInvite(tx, inviteID, userID)
		if err != nil {
			return err
		}
		if group.TargetUserID != nil && *group.TargetUserID == userID {
			return ErrMemberIsGroupTarget
		}
		if err := tx.Delete(&models.GroupMemberInvite{}, "id = ?", invite.ID).Error; err != nil {
			return err
		}
		if err := deleteInviteNotifications(tx, invite.GroupID, userID); err != nil {
			return err
		}
		result = &MemberInviteResult{GroupID: invite.GroupID}
		var memberCount int64
		if err := tx.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", invite.GroupID, userID).
			Count(&memberCount).Error; err != nil {
			return err
		}
		if memberCount > 0 {
			return nil
		}
		if err := tx.Create(&models.GroupMember{GroupID: invite.GroupID, UserID: userID, Role: "member"}).Error; err != nil {
			return err
		}
		if err := AddContributionsForMember(tx, invite.GroupID, userID); err != nil {
			return err
		}
		result.Joined = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *memberInviteRepository) Decline(inviteID, userID uuid.UUID) (*MemberInviteResult, error) {
	var result *MemberInviteResult
	err := r.db.Transaction(func(tx *gorm.DB) error {
		invite, _, err := loadOwnInvite(tx, inviteID, userID)
		if err != nil {
			return err
		}
		if err := tx.Delete(&models.GroupMemberInvite{}, "id = ?", invite.ID).Error; err != nil {
			return err
		}
		result = &MemberInviteResult{GroupID: invite.GroupID}
		return deleteInviteNotifications(tx, invite.GroupID, userID)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

package repository

import (
	"errors"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrInviteInactive davet bulunamadığında, iptal edildiğinde ya da süresi dolduğunda döner.
var ErrInviteInactive = errors.New("invite is not active")

// InviteJoinResult davetle katılma işleminin sonucudur.
type InviteJoinResult struct {
	GroupID uuid.UUID
	// Joined yeni üye satırı oluştuysa true; zaten üyeyse false (idempotent).
	Joined bool
}

// InviteRepository davet linki veritabanı işlemlerini tanımlar. Davet oluşturma, iptal ve
// katılma grup satırını kilitler; hedef bağlama ve üye ekleme ile aynı kilidi kullanır.
type InviteRepository interface {
	GetActive(groupID uuid.UUID, now time.Time) (*models.GroupInvite, error)
	// Replace grubun aktif davetlerini aynı transaction'da iptal edip yenisini kaydeder.
	Replace(invite *models.GroupInvite, now time.Time) error
	// RevokeActive grubun aktif davetlerini iptal eder; aktif yoksa da hata vermez.
	RevokeActive(groupID uuid.UUID, now time.Time) error
	FindActiveByHash(hash string, now time.Time) (*models.GroupInvite, error)
	// Join çağıranı daveti taşıyan gruba üye yapar. Grup satırı kilitliyken davetin hâlâ aktif
	// olduğu, çağıranın hedef kişi olmadığı (ErrMemberIsGroupTarget) ve üyeliğinin olmadığı yeniden denetlenir.
	Join(hash string, userID uuid.UUID, now time.Time) (*InviteJoinResult, error)
}

type inviteRepository struct {
	db *gorm.DB
}

// NewInviteRepository yeni bir InviteRepository örneği oluşturur.
func NewInviteRepository(db *gorm.DB) InviteRepository {
	return &inviteRepository{db: db}
}

func activeScope(db *gorm.DB, now time.Time) *gorm.DB {
	return db.Where("revoked_at IS NULL AND expires_at > ?", now)
}

func (r *inviteRepository) GetActive(groupID uuid.UUID, now time.Time) (*models.GroupInvite, error) {
	var invite models.GroupInvite
	err := activeScope(r.db, now).Where("group_id = ?", groupID).Order("created_at DESC").First(&invite).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &invite, nil
}

func lockGroup(tx *gorm.DB, groupID uuid.UUID) (*models.Group, error) {
	var group models.Group
	if err := tx.Select("id", "target_user_id").Clauses(clause.Locking{Strength: lockNoKeyUpdate}).
		First(&group, "id = ?", groupID).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *inviteRepository) Replace(invite *models.GroupInvite, now time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockGroup(tx, invite.GroupID); err != nil {
			return err
		}
		if err := activeScope(tx.Model(&models.GroupInvite{}), now).Where("group_id = ?", invite.GroupID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Create(invite).Error
	})
}

func (r *inviteRepository) RevokeActive(groupID uuid.UUID, now time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := lockGroup(tx, groupID); err != nil {
			return err
		}
		return activeScope(tx.Model(&models.GroupInvite{}), now).Where("group_id = ?", groupID).
			Update("revoked_at", now).Error
	})
}

func (r *inviteRepository) FindActiveByHash(hash string, now time.Time) (*models.GroupInvite, error) {
	var invite models.GroupInvite
	err := activeScope(r.db, now).Where("token_hash = ?", hash).First(&invite).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &invite, nil
}

func (r *inviteRepository) Join(hash string, userID uuid.UUID, now time.Time) (*InviteJoinResult, error) {
	var result *InviteJoinResult
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var invite models.GroupInvite
		if err := tx.Where("token_hash = ?", hash).First(&invite).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteInactive
			}
			return err
		}
		group, err := lockGroup(tx, invite.GroupID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteInactive
			}
			return err
		}
		// Kilit alındıktan sonra davet yeniden okunur: bu arada iptal/yenileme olmuş olabilir.
		var current models.GroupInvite
		if err := activeScope(tx, now).Where("id = ?", invite.ID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInviteInactive
			}
			return err
		}
		if group.TargetUserID != nil && *group.TargetUserID == userID {
			return ErrMemberIsGroupTarget
		}
		var memberCount int64
		if err := tx.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", invite.GroupID, userID).
			Count(&memberCount).Error; err != nil {
			return err
		}
		result = &InviteJoinResult{GroupID: invite.GroupID}
		// Linkle katılan kişinin bekleyen e-posta/arkadaş daveti varsa artık gereksizdir.
		if err := tx.Where("group_id = ? AND invitee_id = ?", invite.GroupID, userID).Delete(&models.GroupMemberInvite{}).Error; err != nil {
			return err
		}
		if err := deleteInviteNotifications(tx, invite.GroupID, userID); err != nil {
			return err
		}
		if memberCount > 0 {
			return nil
		}
		if err := tx.Create(&models.GroupMember{GroupID: invite.GroupID, UserID: userID, Role: "member"}).Error; err != nil {
			return err
		}
		// MULTI grup: yeni üye, alıcısı kendisi olmayan her havuza varsayılan olarak katkı verir.
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

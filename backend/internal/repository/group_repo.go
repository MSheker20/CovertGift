package repository

import (
	"errors"
	"strings"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GroupRepository grup ve üyelik veritabanı işlemlerini tanımlayan arayüzdür
type GroupRepository interface {
	CreateGroupWithAdmin(group *models.Group, adminUserID uuid.UUID) error
	GetUserGroups(userID uuid.UUID) ([]models.Group, error)
	GetGroupByID(groupID uuid.UUID) (*models.Group, error)
	GetMember(groupID, userID uuid.UUID) (*models.GroupMember, error)
	GetMemberCount(groupID uuid.UUID) (int64, error)
	GetMembers(groupID uuid.UUID) ([]models.GroupMember, error)
	AddMember(member *models.GroupMember) error
	MutateInterests(groupID uuid.UUID, change func([]string) ([]string, error)) ([]string, error)
	UpdateBudget(groupID uuid.UUID, budget decimal.Decimal) error
	DeleteGroup(groupID uuid.UUID) error
}

type groupRepository struct {
	db *gorm.DB
}

// NewGroupRepository yeni bir GroupRepository örneği oluşturur
func NewGroupRepository(db *gorm.DB) GroupRepository {
	return &groupRepository{db: db}
}

// CreateGroupWithAdmin transaction kullanarak grubu açar ve oluşturanı admin üye yapar
func (r *groupRepository) CreateGroupWithAdmin(group *models.Group, adminUserID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Grubu oluştur
		if err := tx.Create(group).Error; err != nil {
			return err
		}

		// 2. Oluşturan kişiyi otomatik 'admin' rolüyle üye yap
		member := &models.GroupMember{
			GroupID: group.ID,
			UserID:  adminUserID,
			Role:    "admin",
		}

		if err := tx.Create(member).Error; err != nil {
			return err // Hata olursa grup kaydı da geri alınır (Rollback)!
		}

		// 3. Grubu oluşturan kullanıcının profil bilgilerini Creator alanına yükle
		var creator models.User
		if err := tx.First(&creator, "id = ?", adminUserID).Error; err == nil {
			group.Creator = &creator
		}

		return nil
	})
}

// GetUserGroups kullanıcının üye olduğu grupları getirir (Anti-Spoiler: hedef kişi olduğu gruplar filtrelenir)
func (r *groupRepository) GetUserGroups(userID uuid.UUID) ([]models.Group, error) {
	var groups []models.Group

	// group_members üzerinden kullanıcının üyesi olduğu grupları buluyoruz
	// Anti-Spoiler: target_user_id bu kullanıcı olan grupları hariç tutuyoruz!
	err := r.db.
		Joins("JOIN group_members ON group_members.group_id = groups.id").
		Where("group_members.user_id = ?", userID).
		Where("groups.target_user_id IS NULL OR groups.target_user_id != ?", userID).
		Preload("Creator").
		Order("groups.created_at DESC").
		Find(&groups).Error

	return groups, err
}

// GetGroupByID tek bir grubun detaylarını ilişkileriyle birlikte getirir
func (r *groupRepository) GetGroupByID(groupID uuid.UUID) (*models.Group, error) {
	var group models.Group
	err := r.db.Preload("Creator").Preload("TargetUser").First(&group, "id = ?", groupID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &group, nil
}

// GetMember kullanıcının belirli bir gruptaki üyelik kaydını (ve rolünü) getirir
func (r *groupRepository) GetMember(groupID, userID uuid.UUID) (*models.GroupMember, error) {
	var member models.GroupMember
	err := r.db.Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

// GetMemberCount gruptaki toplam aktif üye sayısını hesaplar (Kişi başı bütçe için kullanılır)
func (r *groupRepository) GetMemberCount(groupID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.GroupMember{}).Where("group_id = ?", groupID).Count(&count).Error
	return count, err
}

// GetMembers bir grubun tüm üyelerini kullanıcı profilleriyle birlikte listeler
func (r *groupRepository) GetMembers(groupID uuid.UUID) ([]models.GroupMember, error) {
	var members []models.GroupMember
	err := r.db.Where("group_id = ?", groupID).Preload("User").Find(&members).Error
	return members, err
}

// AddMember gruba yeni bir üye kaydı ekler
func (r *groupRepository) AddMember(member *models.GroupMember) error {
	return r.db.Create(member).Error
}

// MutateInterests locks the row so concurrent additions cannot overwrite each other.
func (r *groupRepository) MutateInterests(groupID uuid.UUID, change func([]string) ([]string, error)) ([]string, error) {
	var interests []string
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Select("id", "target_interests").Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&group, "id = ?", groupID).Error; err != nil {
			return err
		}
		updated, err := change(models.ParseInterests(group.TargetInterests))
		if err != nil {
			return err
		}
		if err := tx.Model(&models.Group{}).Where("id = ?", groupID).
			Update("target_interests", strings.Join(updated, "\n")).Error; err != nil {
			return err
		}
		interests = updated
		return nil
	})
	return interests, err
}

// UpdateBudget keeps the group's budget and existing gift warnings consistent.
func (r *groupRepository) UpdateBudget(groupID uuid.UUID, budget decimal.Decimal) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Group{}).Where("id = ?", groupID).Update("total_budget", budget)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var overBudget interface{} = false
		if budget.GreaterThan(decimal.Zero) {
			overBudget = gorm.Expr("estimated_price > ?", budget)
		}
		return tx.Model(&models.GiftIdea{}).Where("group_id = ?", groupID).
			Update("is_over_budget", overBudget).Error
	})
}

// DeleteGroup grubu ve ona bağlı tüm fikirleri, oyları ve üyelikleri atomik bir transaction ile siler
func (r *groupRepository) DeleteGroup(groupID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Gruba ait hediye fikirlerinin ID'lerini topla
		var ideaIDs []uuid.UUID
		if err := tx.Model(&models.GiftIdea{}).Where("group_id = ?", groupID).Pluck("id", &ideaIDs).Error; err != nil {
			return err
		}

		// 2. Fikirlere ait tüm oyları sil
		if len(ideaIDs) > 0 {
			if err := tx.Where("gift_idea_id IN ?", ideaIDs).Delete(&models.Vote{}).Error; err != nil {
				return err
			}
			// 3. Fikirleri sil
			if err := tx.Where("id IN ?", ideaIDs).Delete(&models.GiftIdea{}).Error; err != nil {
				return err
			}
		}

		// 4. Grubun üyelerini sil
		if err := tx.Where("group_id = ?", groupID).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}

		// 5. Grubu sil
		res := tx.Where("id = ?", groupID).Delete(&models.Group{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})
}

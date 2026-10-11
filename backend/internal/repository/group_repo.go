package repository

import (
	"errors"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

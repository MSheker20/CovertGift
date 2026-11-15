package repository

import (
	"errors"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AvatarRepository profil fotoğraflarının veritabanı işlemlerini tanımlar.
type AvatarRepository interface {
	// Save fotoğrafı ekler ya da değiştirir ve users.avatar_version'ı aynı transaction'da günceller.
	Save(avatar *models.UserAvatar) error
	// Delete fotoğrafı siler ve users.avatar_version'ı boşaltır. Fotoğraf yoksa hata vermez.
	Delete(userID uuid.UUID) error
	// Get fotoğrafı döner; yoksa nil.
	Get(userID uuid.UUID) (*models.UserAvatar, error)
	// SharesGroup iki kullanıcının en az bir grupta birlikte üye olduğunu söyler.
	SharesGroup(a, b uuid.UUID) (bool, error)
}

type avatarRepository struct {
	db *gorm.DB
}

// NewAvatarRepository yeni bir AvatarRepository oluşturur.
func NewAvatarRepository(db *gorm.DB) AvatarRepository {
	return &avatarRepository{db: db}
}

func (r *avatarRepository) Save(avatar *models.UserAvatar) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data", "content_type", "etag", "width", "height", "updated_at"}),
		}).Create(avatar).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id = ?", avatar.UserID).Update("avatar_version", avatar.ETag).Error
	})
}

func (r *avatarRepository) Delete(userID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserAvatar{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id = ?", userID).Update("avatar_version", "").Error
	})
}

func (r *avatarRepository) Get(userID uuid.UUID) (*models.UserAvatar, error) {
	var avatar models.UserAvatar
	err := r.db.Where("user_id = ?", userID).First(&avatar).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &avatar, nil
}

func (r *avatarRepository) SharesGroup(a, b uuid.UUID) (bool, error) {
	var shared bool
	err := r.db.Raw(`SELECT EXISTS (
		SELECT 1 FROM group_members x JOIN group_members y ON y.group_id = x.group_id
		WHERE x.user_id = ? AND y.user_id = ?)`, a, b).Scan(&shared).Error
	return shared, err
}

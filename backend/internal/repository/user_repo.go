package repository

import (
	"errors"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRepository veritabanı kullanıcı işlemlerini soyutlayan arayüzdür (Interface)
type UserRepository interface {
	Create(user *models.User) error
	FindByEmail(email string) (*models.User, error)
	FindByID(id uuid.UUID) (*models.User, error)
}

// userRepository arayüzün GORM tabanlı somut (concrete) implementasyonudur
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository dışarıya nesne üreten yapıcı fonksiyondur (Constructor)
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create yeni bir kullanıcıyı veritabanına kaydeder
func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// FindByEmail e-posta adresine göre kullanıcı arar (büyük/küçük harf duyarsız)
func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Kayıt yoksa hata patlatmak yerine nil dönüyoruz
		}
		return nil, err
	}
	return &user, nil
}

// FindByID UUID kimliğine göre kullanıcı arar
func (r *userRepository) FindByID(id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

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
	UpdateProfile(id uuid.UUID, name, email string) error
	UpdatePasswordHash(id uuid.UUID, passwordHash string) error
	Delete(id uuid.UUID) error
}

// ErrDuplicateEmail e-posta benzersizlik kısıtı ihlal edildiğinde döner (eşzamanlı güncelleme yarışı)
var ErrDuplicateEmail = errors.New("e-posta benzersizlik kısıtı ihlal edildi")

// isUniqueViolation PostgreSQL benzersizlik ihlalini (SQLSTATE 23505) tanır.
// pgconn.PgError'a doğrudan bağımlı olmamak için SQLState() arayüzü kullanılır.
func isUniqueViolation(err error) bool {
	var stateErr interface{ SQLState() string }
	return errors.As(err, &stateErr) && stateErr.SQLState() == "23505"
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

// UpdateProfile kullanıcının adını ve e-postasını günceller. Kullanıcı yoksa
// gorm.ErrRecordNotFound, e-posta başka kullanıcıdaysa ErrDuplicateEmail döner.
func (r *userRepository) UpdateProfile(id uuid.UUID, name, email string) error {
	res := r.db.Model(&models.User{}).Where("id = ?", id).Updates(map[string]any{"name": name, "email": email})
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return ErrDuplicateEmail
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdatePasswordHash kullanıcının parola hash'ini değiştirir. Kullanıcı yoksa gorm.ErrRecordNotFound döner.
func (r *userRepository) UpdatePasswordHash(id uuid.UUID, passwordHash string) error {
	res := r.db.Model(&models.User{}).Where("id = ?", id).Update("password_hash", passwordHash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete bir kullanıcıyı ve ona bağlı tüm ilişkili verileri atomik bir transaction ile siler
func (r *userRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Kullanıcı var mı kontrol et
		var user models.User
		if err := tx.First(&user, "id = ?", id).Error; err != nil {
			return err
		}

		// Kilit sırası (groups -> gift_pools -> diğerleri): bu silme işlemi kullanıcının dokunduğu
		// her grupta fikir, oy, üyelik ve havuz satırlarını değiştirir. Diğer yazma işlemleri önce
		// grup, sonra havuz satırını kilitlediği için burada da önce ilgili gruplar, sonra o
		// grupların havuzları kimlik sırasıyla kilitlenir; aksi halde eşzamanlı bir oy ya da fikir
		// eklemeyle karşılıklı kilitlenme (deadlock) oluşabilir.
		var lockedGroups []uuid.UUID
		if err := tx.Raw(`SELECT id FROM groups
			WHERE creator_id = ? OR target_user_id = ?
			   OR id IN (SELECT group_id FROM group_members WHERE user_id = ?)
			   OR id IN (SELECT group_id FROM gift_ideas WHERE created_by = ?)
			   OR id IN (SELECT group_id FROM gift_pools WHERE recipient_user_id = ? OR manager_user_id = ?)
			ORDER BY id FOR UPDATE`, id, id, id, id, id, id).Scan(&lockedGroups).Error; err != nil {
			return err
		}
		if len(lockedGroups) > 0 {
			var lockedPools []uuid.UUID
			if err := tx.Raw("SELECT id FROM gift_pools WHERE group_id IN ? ORDER BY id FOR UPDATE", lockedGroups).Scan(&lockedPools).Error; err != nil {
				return err
			}
		}

		// 2. Kullanıcının hedef kişi olduğu gruplarda target_user_id'yi NULL yap (sürpriz bozulmasın, ilişki kopsun)
		if err := tx.Model(&models.Group{}).Where("target_user_id = ?", id).Update("target_user_id", nil).Error; err != nil {
			return err
		}

		// 3. Kullanıcının verdiği tüm oyları sil
		if err := tx.Where("user_id = ?", id).Delete(&models.Vote{}).Error; err != nil {
			return err
		}

		// 4. Kullanıcının herhangi bir grupta önerdiği hediye fikirlerini ve oylarını sil
		var userIdeaIDs []uuid.UUID
		if err := tx.Model(&models.GiftIdea{}).Where("created_by = ?", id).Pluck("id", &userIdeaIDs).Error; err != nil {
			return err
		}
		if len(userIdeaIDs) > 0 {
			if err := tx.Where("gift_idea_id IN ?", userIdeaIDs).Delete(&models.Vote{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", userIdeaIDs).Delete(&models.GiftIdea{}).Error; err != nil {
				return err
			}
		}

		// 5. Kullanıcının oluşturduğu grupları, altındaki fikirleri, oyları ve üyelikleri sil
		var groupIDs []uuid.UUID
		if err := tx.Model(&models.Group{}).Where("creator_id = ?", id).Pluck("id", &groupIDs).Error; err != nil {
			return err
		}
		if len(groupIDs) > 0 {
			var groupIdeaIDs []uuid.UUID
			if err := tx.Model(&models.GiftIdea{}).Where("group_id IN ?", groupIDs).Pluck("id", &groupIdeaIDs).Error; err != nil {
				return err
			}
			if len(groupIdeaIDs) > 0 {
				if err := tx.Where("gift_idea_id IN ?", groupIdeaIDs).Delete(&models.Vote{}).Error; err != nil {
					return err
				}
				if err := tx.Where("id IN ?", groupIdeaIDs).Delete(&models.GiftIdea{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("group_id IN ?", groupIDs).Delete(&models.GroupMember{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", groupIDs).Delete(&models.Group{}).Error; err != nil {
				return err
			}
		}

		// 6. Kullanıcının diğer gruplardaki üyeliklerini sil
		if err := tx.Where("user_id = ?", id).Delete(&models.GroupMember{}).Error; err != nil {
			return err
		}

		// 7. Kullanıcıyı sil
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}

		return nil
	})
}


package repository

import (
	"errors"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PoolMembership havuz kurulurken grup satırı kilitliyken okunan üyelik durumudur.
type PoolMembership struct {
	RecipientIsMember bool
	// ManagerIsMember yönetici verilmediyse false'tur.
	ManagerIsMember bool
	// PoolExists alıcının bu grupta zaten havuzu varsa true'dur.
	PoolExists bool
}

// PoolStats bir havuzun çağırana özel sayaçlarıdır. Yalnızca çağıranın alıcısı olmadığı
// havuzlar için hesaplanır.
type PoolStats struct {
	PoolID uuid.UUID
	// IdeaCount reddedilmemiş fikir sayısıdır.
	IdeaCount int64
	// PendingVotes açık havuzdaki, çağıranın eklemediği ve henüz oy vermediği aktif fikirlerdir.
	PendingVotes int64
	// ContributorCount alıcı dışındaki, katkı veren üye sayısıdır (satırı olmayan üye katkı verir).
	ContributorCount int64
	// MyContribution çağıranın kendi katkı durumudur (satır yoksa true).
	MyContribution bool
}

// Contributor bir havuzda alıcı dışındaki bir üyenin katkı durumudur.
type Contributor struct {
	UserID       uuid.UUID `json:"user_id" format:"uuid"`
	Name         string    `json:"name" example:"Zeynep Kaya"`
	Contributing bool      `json:"contributing" example:"true"`
	// AvatarVersion profil fotoğrafının sürümüdür; fotoğraf yoksa alan yer almaz.
	AvatarVersion string `json:"avatar_version,omitempty"`
}

// PoolRepository MULTI grup havuzlarının veritabanı işlemlerini tanımlar.
//
// Kilit sırası: groups satırı, sonra gift_pools satırı, sonra diğer her şey. Havuz kurma ve
// silme, alıcı/yönetici değiştirme grup satırını kilitler (üye ekleme ve davetle katılma da
// aynı kilidi kullanır); bütçe, ilgi alanı, tarih, kazanan ve katkı yalnızca havuz satırını kilitler.
type PoolRepository interface {
	// CreatePool grup satırı kilitliyken guard'ı çağırır, havuzu ekler ve alıcı dışındaki tüm
	// üyeler için katkı satırı yazar. Dönen havuzda Recipient ve Manager yüklüdür.
	CreatePool(groupID, recipientID uuid.UUID, managerID *uuid.UUID, guard func(group *models.Group, m PoolMembership) error) (*models.GiftPool, error)
	// GetPool gruba ait havuzu getirir (Recipient ve Manager yüklü); yoksa nil.
	GetPool(groupID, poolID uuid.UUID) (*models.GiftPool, error)
	// GetPoolByID havuzu kimliğiyle getirir (Recipient yüklü); yoksa nil.
	GetPoolByID(poolID uuid.UUID) (*models.GiftPool, error)
	// ListPools grubun havuzlarını kurulma sırasıyla döner (Recipient ve Manager yüklü).
	ListPools(groupID uuid.UUID) ([]models.GiftPool, error)
	// PoolsOfGroups verilen grupların havuzlarını kurulma sırasıyla döner (Recipient yüklü).
	PoolsOfGroups(groupIDs []uuid.UUID) ([]models.GiftPool, error)
	// VisiblePoolIDs çağıranın alıcısı olmadığı havuzların kimlikleridir.
	VisiblePoolIDs(groupID, viewerID uuid.UUID) ([]uuid.UUID, error)
	// PoolStats yalnızca çağıranın alıcısı olmadığı havuzların sayaçlarını hesaplar.
	PoolStats(groupID, viewerID uuid.UUID) (map[uuid.UUID]PoolStats, error)
	// ListContributors alıcı dışındaki üyeleri katılma sırasıyla ve katkı durumlarıyla döner.
	ListContributors(poolID uuid.UUID) ([]Contributor, error)
	// DeletePool grup ve havuz satırlarını kilitler, fikir sayısıyla guard'ı çağırır ve havuzu
	// fikirleri, oyları, katkıları ve bildirimleriyle birlikte siler. Havuz yoksa gorm.ErrRecordNotFound.
	DeletePool(groupID, recipientID uuid.UUID, guard func(pool *models.GiftPool, ideaCount int64) error) error
	// SetManager grup ve havuz satırlarını kilitler, guard'ı çağırır ve manager_user_id'yi yazar.
	// Havuz yoksa gorm.ErrRecordNotFound. Dönen havuzda Recipient ve Manager yüklüdür.
	SetManager(groupID, recipientID uuid.UUID, managerID *uuid.UUID, guard func(pool *models.GiftPool, managerIsMember bool) error) (*models.GiftPool, error)
	UpdateBudget(poolID uuid.UUID, budget decimal.Decimal) error
	MutateInterests(poolID uuid.UUID, change func([]string) ([]string, error)) ([]string, error)
	SetEventDate(poolID uuid.UUID, date *models.Date) error
	// Complete havuz satırını kilitler; guard havuzun güncel halini ve havuza ait bulunan fikirleri denetler.
	Complete(poolID uuid.UUID, ideaIDs []uuid.UUID, guard func(pool *models.GiftPool, ideas []models.GiftIdea) error) error
	Reopen(poolID uuid.UUID, guard func(pool *models.GiftPool) error) error
	// SetContribution havuz satırını kilitler, guard'ı çağırır ve kullanıcının katkı satırını yazar.
	SetContribution(poolID, userID uuid.UUID, contributing bool, guard func(pool *models.GiftPool) error) error
}

type poolRepository struct {
	db *gorm.DB
}

// NewPoolRepository yeni bir PoolRepository örneği oluşturur.
func NewPoolRepository(db *gorm.DB) PoolRepository {
	return &poolRepository{db: db}
}

// AddContributionsForMember yeni katılan üye için, alıcısı kendisi olmayan grubun tüm havuzlarına
// katkı satırı yazar. Üyeliği oluşturan transaction'ın içinde (grup satırı kilitliyken) çağrılır;
// böylece eşzamanlı havuz kurma ile tutarlı kalır. Satır zaten varsa dokunmaz.
func AddContributionsForMember(tx *gorm.DB, groupID, userID uuid.UUID) error {
	return tx.Exec(`
		INSERT INTO pool_contributions (pool_id, user_id, contributing, updated_at)
		SELECT gp.id, ?, true, now() FROM gift_pools gp
		WHERE gp.group_id = ? AND gp.recipient_user_id <> ?
		ON CONFLICT (pool_id, user_id) DO NOTHING`, userID, groupID, userID).Error
}

func countRows(tx *gorm.DB, model any, query string, args ...any) (int64, error) {
	var n int64
	err := tx.Model(model).Where(query, args...).Count(&n).Error
	return n, err
}

func (r *poolRepository) CreatePool(groupID, recipientID uuid.UUID, managerID *uuid.UUID, guard func(group *models.Group, m PoolMembership) error) (*models.GiftPool, error) {
	var created *models.GiftPool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Clauses(clause.Locking{Strength: lockNoKeyUpdate}).First(&group, "id = ?", groupID).Error; err != nil {
			return err
		}
		var m PoolMembership
		n, err := countRows(tx, &models.GroupMember{}, "group_id = ? AND user_id = ?", groupID, recipientID)
		if err != nil {
			return err
		}
		m.RecipientIsMember = n > 0
		if managerID != nil {
			if n, err = countRows(tx, &models.GroupMember{}, "group_id = ? AND user_id = ?", groupID, *managerID); err != nil {
				return err
			}
			m.ManagerIsMember = n > 0
		}
		if n, err = countRows(tx, &models.GiftPool{}, "group_id = ? AND recipient_user_id = ?", groupID, recipientID); err != nil {
			return err
		}
		m.PoolExists = n > 0
		if err := guard(&group, m); err != nil {
			return err
		}
		// created_at açıkça yazılır: sütun varsayılanı now() transaction başlangıç anıdır; aynı
		// transaction'da (ör. seed) kurulan havuzlar aksi halde aynı zamanı alır. Liste sırası
		// "created_at, id" ile her durumda kararlıdır.
		pool := &models.GiftPool{
			GroupID: groupID, RecipientUserID: recipientID, ManagerUserID: managerID,
			Status: models.GroupStatusActive, CreatedAt: time.Now(),
		}
		if err := tx.Omit(clause.Associations).Create(pool).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO pool_contributions (pool_id, user_id, contributing, updated_at)
			SELECT ?, gm.user_id, true, now() FROM group_members gm
			WHERE gm.group_id = ? AND gm.user_id <> ?
			ON CONFLICT (pool_id, user_id) DO NOTHING`, pool.ID, groupID, recipientID).Error; err != nil {
			return err
		}
		loaded, err := loadPool(tx, "id = ?", pool.ID)
		if err != nil {
			return err
		}
		created = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// loadPool havuzu Recipient ve Manager yüklü getirir; yoksa nil döner.
func loadPool(db *gorm.DB, query string, args ...any) (*models.GiftPool, error) {
	var pool models.GiftPool
	err := db.Preload("Recipient").Preload("Manager").Where(query, args...).First(&pool).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &pool, nil
}

func (r *poolRepository) GetPool(groupID, poolID uuid.UUID) (*models.GiftPool, error) {
	return loadPool(r.db, "id = ? AND group_id = ?", poolID, groupID)
}

func (r *poolRepository) GetPoolByID(poolID uuid.UUID) (*models.GiftPool, error) {
	return loadPool(r.db, "id = ?", poolID)
}

func (r *poolRepository) ListPools(groupID uuid.UUID) ([]models.GiftPool, error) {
	var pools []models.GiftPool
	err := r.db.Preload("Recipient").Preload("Manager").Where("group_id = ?", groupID).
		Order("created_at, id").Find(&pools).Error
	return pools, err
}

func (r *poolRepository) PoolsOfGroups(groupIDs []uuid.UUID) ([]models.GiftPool, error) {
	var pools []models.GiftPool
	if len(groupIDs) == 0 {
		return pools, nil
	}
	err := r.db.Preload("Recipient").Where("group_id IN ?", groupIDs).Order("created_at, id").Find(&pools).Error
	return pools, err
}

func (r *poolRepository) VisiblePoolIDs(groupID, viewerID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.Model(&models.GiftPool{}).Where("group_id = ? AND recipient_user_id <> ?", groupID, viewerID).
		Order("id").Pluck("id", &ids).Error
	return ids, err
}

func (r *poolRepository) PoolStats(groupID, viewerID uuid.UUID) (map[uuid.UUID]PoolStats, error) {
	var rows []PoolStats
	err := r.db.Raw(`
		SELECT gp.id AS pool_id,
			(SELECT COUNT(*) FROM gift_ideas gi WHERE gi.pool_id = gp.id AND gi.status <> 'REJECTED') AS idea_count,
			CASE WHEN gp.status = ? THEN (
				SELECT COUNT(*) FROM gift_ideas gi
				WHERE gi.pool_id = gp.id AND gi.status = 'ACTIVE' AND gi.created_by <> ?
					AND NOT EXISTS (SELECT 1 FROM votes v WHERE v.gift_idea_id = gi.id AND v.user_id = ?)
			) ELSE 0 END AS pending_votes,
			(SELECT COUNT(*) FROM group_members gm
				LEFT JOIN pool_contributions pc ON pc.pool_id = gp.id AND pc.user_id = gm.user_id
				WHERE gm.group_id = gp.group_id AND gm.user_id <> gp.recipient_user_id
					AND COALESCE(pc.contributing, true)) AS contributor_count,
			COALESCE((SELECT pc.contributing FROM pool_contributions pc WHERE pc.pool_id = gp.id AND pc.user_id = ?), true) AS my_contribution
		FROM gift_pools gp
		WHERE gp.group_id = ? AND gp.recipient_user_id <> ?`,
		models.GroupStatusActive, viewerID, viewerID, viewerID, groupID, viewerID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]PoolStats, len(rows))
	for _, row := range rows {
		out[row.PoolID] = row
	}
	return out, nil
}

func (r *poolRepository) ListContributors(poolID uuid.UUID) ([]Contributor, error) {
	contributors := []Contributor{}
	err := r.db.Raw(`
		SELECT gm.user_id, u.name, u.avatar_version, COALESCE(pc.contributing, true) AS contributing
		FROM gift_pools gp
		JOIN group_members gm ON gm.group_id = gp.group_id AND gm.user_id <> gp.recipient_user_id
		JOIN users u ON u.id = gm.user_id
		LEFT JOIN pool_contributions pc ON pc.pool_id = gp.id AND pc.user_id = gm.user_id
		WHERE gp.id = ?
		ORDER BY gm.joined_at, gm.user_id`, poolID).Scan(&contributors).Error
	return contributors, err
}

// lockGroupRow grup satırını FOR NO KEY UPDATE kilitler (havuz silme/yönetici değişikliği). Satırın
// anahtarı değişmediği için FOR UPDATE gerekmez; NO KEY UPDATE, havuz yazan işlemlerin yabancı anahtar
// denetiminde aldığı KEY SHARE ile çakışmaz (deadlock yok), kendisiyle ve SHARE ile çakışır (sıralama korunur).
func lockGroupRow(tx *gorm.DB, groupID uuid.UUID) error {
	var group models.Group
	return tx.Select("id").Clauses(clause.Locking{Strength: lockNoKeyUpdate}).First(&group, "id = ?", groupID).Error
}

// lockPoolRow havuz satırını verilen güçte kilitler ve döner. Silme FOR UPDATE ister; diğer
// değişiklikler NO KEY UPDATE kullanır.
func lockPoolRow(tx *gorm.DB, strength string, query string, args ...any) (*models.GiftPool, error) {
	var pool models.GiftPool
	if err := tx.Clauses(clause.Locking{Strength: strength}).Where(query, args...).First(&pool).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

// lockPoolByID grup satırını KEY SHARE, sonra havuz satırını NO KEY UPDATE kilitler (kilit sırası:
// groups → gift_pools → diğerleri). Kazanan, yeniden açma ve katkı işlemleri kullanır.
func lockPoolByID(tx *gorm.DB, poolID uuid.UUID) (*models.GiftPool, error) {
	if err := lockPoolParent(tx, poolID); err != nil {
		return nil, err
	}
	return lockPoolRow(tx, lockNoKeyUpdate, "id = ?", poolID)
}

func (r *poolRepository) DeletePool(groupID, recipientID uuid.UUID, guard func(pool *models.GiftPool, ideaCount int64) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupRow(tx, groupID); err != nil {
			return err
		}
		// Havuz silineceği için FOR UPDATE.
		pool, err := lockPoolRow(tx, "UPDATE", "group_id = ? AND recipient_user_id = ?", groupID, recipientID)
		if err != nil {
			return err
		}
		ideaCount, err := countRows(tx, &models.GiftIdea{}, "pool_id = ?", pool.ID)
		if err != nil {
			return err
		}
		if err := guard(pool, ideaCount); err != nil {
			return err
		}
		// Yabancı anahtarlar CASCADE'dir; yine de açıkça silinir (eski şemada kısıt eksik kalsa da iz kalmaz).
		if err := tx.Exec("DELETE FROM votes WHERE gift_idea_id IN (SELECT id FROM gift_ideas WHERE pool_id = ?)", pool.ID).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id = ?", pool.ID).Delete(&models.GiftIdea{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id = ?", pool.ID).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		if err := tx.Where("pool_id = ?", pool.ID).Delete(&models.PoolContribution{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", pool.ID).Delete(&models.GiftPool{}).Error
	})
}

func (r *poolRepository) SetManager(groupID, recipientID uuid.UUID, managerID *uuid.UUID, guard func(pool *models.GiftPool, managerIsMember bool) error) (*models.GiftPool, error) {
	var updated *models.GiftPool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupRow(tx, groupID); err != nil {
			return err
		}
		pool, err := lockPoolRow(tx, lockNoKeyUpdate, "group_id = ? AND recipient_user_id = ?", groupID, recipientID)
		if err != nil {
			return err
		}
		isMember := false
		if managerID != nil {
			n, err := countRows(tx, &models.GroupMember{}, "group_id = ? AND user_id = ?", groupID, *managerID)
			if err != nil {
				return err
			}
			isMember = n > 0
		}
		if err := guard(pool, isMember); err != nil {
			return err
		}
		if err := tx.Model(&models.GiftPool{}).Where("id = ?", pool.ID).Update("manager_user_id", managerID).Error; err != nil {
			return err
		}
		loaded, err := loadPool(tx, "id = ?", pool.ID)
		if err != nil {
			return err
		}
		updated = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func poolScopeByID(poolID uuid.UUID) IdeaScope {
	return IdeaScope{PoolID: &poolID}
}

func (r *poolRepository) UpdateBudget(poolID uuid.UUID, budget decimal.Decimal) error {
	return updateScopeBudget(r.db, poolScopeByID(poolID), budget)
}

func (r *poolRepository) MutateInterests(poolID uuid.UUID, change func([]string) ([]string, error)) ([]string, error) {
	return mutateScopeInterests(r.db, poolScopeByID(poolID), change)
}

func (r *poolRepository) SetEventDate(poolID uuid.UUID, date *models.Date) error {
	return setScopeEventDate(r.db, poolScopeByID(poolID), date)
}

func (r *poolRepository) Complete(poolID uuid.UUID, ideaIDs []uuid.UUID, guard func(pool *models.GiftPool, ideas []models.GiftIdea) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		pool, err := lockPoolByID(tx, poolID)
		if err != nil {
			return err
		}
		scope := poolScopeByID(poolID)
		ideas, err := scopeIdeasByIDs(tx, scope, ideaIDs)
		if err != nil {
			return err
		}
		if err := guard(pool, ideas); err != nil {
			return err
		}
		return completeScope(tx, scope, ideaIDs)
	})
}

func (r *poolRepository) Reopen(poolID uuid.UUID, guard func(pool *models.GiftPool) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		pool, err := lockPoolByID(tx, poolID)
		if err != nil {
			return err
		}
		if err := guard(pool); err != nil {
			return err
		}
		return reopenScope(tx, poolScopeByID(poolID))
	})
}

func (r *poolRepository) SetContribution(poolID, userID uuid.UUID, contributing bool, guard func(pool *models.GiftPool) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		pool, err := lockPoolByID(tx, poolID)
		if err != nil {
			return err
		}
		if err := guard(pool); err != nil {
			return err
		}
		// GORM, false değerini "boş" sayıp varsayılanı (true) yazabileceği için ham upsert kullanılır.
		return tx.Exec(`
			INSERT INTO pool_contributions (pool_id, user_id, contributing, updated_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (pool_id, user_id) DO UPDATE SET contributing = EXCLUDED.contributing, updated_at = EXCLUDED.updated_at`,
			poolID, userID, contributing, time.Now()).Error
	})
}

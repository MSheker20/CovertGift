package repository

import (
	"strings"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// IdeaScope fikirlerin ve bütçe/ilgi alanı/tarih/durum alanlarının ait olduğu kapsamdır:
//   - PoolID nil: SINGLE grubun kendisi. Fikir filtresi "group_id = ? AND pool_id IS NULL"dır;
//     SINGLE grupların tüm fikirlerinde pool_id NULL olduğu için sonuçlar eskisiyle aynıdır.
//   - PoolID dolu: MULTI gruptaki bir havuz. Fikir filtresi "pool_id = ?"dir.
type IdeaScope struct {
	GroupID uuid.UUID
	PoolID  *uuid.UUID
}

// GroupScope SINGLE bir grubun kapsamıdır.
func GroupScope(groupID uuid.UUID) IdeaScope {
	return IdeaScope{GroupID: groupID}
}

// PoolScope bir havuzun kapsamıdır.
func PoolScope(groupID, poolID uuid.UUID) IdeaScope {
	return IdeaScope{GroupID: groupID, PoolID: &poolID}
}

// IsPool kapsamın bir havuz olduğunu söyler.
func (s IdeaScope) IsPool() bool { return s.PoolID != nil }

// ideas fikir sorgusunu kapsama göre süzer. prefix tablo adı ön ekidir ("" ya da "gift_ideas.").
func (s IdeaScope) ideas(db *gorm.DB, prefix string) *gorm.DB {
	if s.PoolID != nil {
		return db.Where(prefix+"pool_id = ?", *s.PoolID)
	}
	return db.Where(prefix+"group_id = ? AND "+prefix+"pool_id IS NULL", s.GroupID)
}

// row kapsamın bütçe, ilgi alanı, tarih ve durum alanlarını taşıyan satırıdır: tablo adı,
// satır kimliği ve ilgi alanı sütunu. Değerler sabittir (kullanıcı girdisi değildir).
func (s IdeaScope) row() (table string, id uuid.UUID, interestsColumn string) {
	if s.PoolID != nil {
		return "gift_pools", *s.PoolID, "interests"
	}
	return "groups", s.GroupID, "target_interests"
}

// scopeLock kilitlenen kapsam satırının durum alanlarıdır.
type scopeLock struct {
	Status string
	// Recipient havuzun alıcısıdır; SINGLE kapsamda nil.
	Recipient *uuid.UUID
}

// Satır kilidi güçleri. Grup ve havuz satırlarının anahtarı hiçbir zaman değişmez; bu yüzden
// silme dışındaki tüm yazma kilitleri FOR NO KEY UPDATE'tir. Bu kilit, başka tablolara satır
// eklerken yabancı anahtar denetiminin aldığı FOR KEY SHARE ile çakışmaz; böylece "havuzu tutan
// işlem grup satırının anahtar kilidini bekler, grup satırını tutan işlem havuzu bekler"
// döngüsü (deadlock) oluşmaz.
const (
	lockNoKeyUpdate = "NO KEY UPDATE"
	lockShare       = "SHARE"
)

// lockPoolParent bir havuz kapsamında, havuz satırından ÖNCE grup satırını FOR KEY SHARE alır.
// Kilit sırası her yerde aynıdır: groups satırı → gift_pools satırı → diğer her şey. KEY SHARE,
// grup satırındaki NO KEY UPDATE kilitleriyle (üye ekleme, alıcı/yönetici değişikliği) çakışmaz;
// yalnızca grubu silen FOR UPDATE ile sıraya girer. Havuz yoksa hiçbir satır kilitlenmez.
func lockPoolParent(tx *gorm.DB, poolID uuid.UUID) error {
	var ids []uuid.UUID
	return tx.Raw("SELECT id FROM groups WHERE id = (SELECT group_id FROM gift_pools WHERE id = ?) FOR KEY SHARE", poolID).
		Scan(&ids).Error
}

// lockScope kapsam satırını verilen güçte kilitler (lockShare / lockNoKeyUpdate) ve durumunu döner.
// Satır yoksa gorm.ErrRecordNotFound döner. Havuz kapsamında önce grup satırı KEY SHARE alınır
// (kilit sırası: groups → gift_pools → diğerleri).
func lockScope(tx *gorm.DB, s IdeaScope, strength string) (*scopeLock, error) {
	if s.PoolID != nil {
		if err := lockPoolParent(tx, *s.PoolID); err != nil {
			return nil, err
		}
	}
	table, id, _ := s.row()
	recipient := "NULL::uuid"
	if s.PoolID != nil {
		recipient = "recipient_user_id"
	}
	var rows []struct {
		Status    string
		Recipient *uuid.UUID
	}
	if err := tx.Raw("SELECT status, "+recipient+" AS recipient FROM "+table+" WHERE id = ? FOR "+strength, id).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &scopeLock{Status: rows[0].Status, Recipient: rows[0].Recipient}, nil
}

// updateScopeBudget kapsamın bütçesini ve kapsamdaki fikirlerin bütçe aşımı bayraklarını aynı
// transaction'da günceller.
func updateScopeBudget(db *gorm.DB, s IdeaScope, budget decimal.Decimal) error {
	table, id, _ := s.row()
	return db.Transaction(func(tx *gorm.DB) error {
		if s.PoolID != nil {
			if err := lockPoolParent(tx, *s.PoolID); err != nil {
				return err
			}
		}
		result := tx.Table(table).Where("id = ?", id).Update("total_budget", budget)
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
		return s.ideas(tx.Model(&models.GiftIdea{}), "").Update("is_over_budget", overBudget).Error
	})
}

// mutateScopeInterests satırı kilitler; eşzamanlı eklemeler birbirinin üzerine yazamaz.
func mutateScopeInterests(db *gorm.DB, s IdeaScope, change func([]string) ([]string, error)) ([]string, error) {
	table, id, column := s.row()
	var interests []string
	err := db.Transaction(func(tx *gorm.DB) error {
		if s.PoolID != nil {
			if err := lockPoolParent(tx, *s.PoolID); err != nil {
				return err
			}
		}
		var current []string
		if err := tx.Raw("SELECT COALESCE("+column+", '') FROM "+table+" WHERE id = ? FOR NO KEY UPDATE", id).
			Scan(&current).Error; err != nil {
			return err
		}
		if len(current) == 0 {
			return gorm.ErrRecordNotFound
		}
		updated, err := change(models.ParseInterests(current[0]))
		if err != nil {
			return err
		}
		if err := tx.Table(table).Where("id = ?", id).Update(column, strings.Join(updated, "\n")).Error; err != nil {
			return err
		}
		interests = updated
		return nil
	})
	return interests, err
}

// setScopeEventDate kapsamın hediye tarihini ayarlar; nil tarihi kaldırır.
func setScopeEventDate(db *gorm.DB, s IdeaScope, date *models.Date) error {
	table, id, _ := s.row()
	result := db.Table(table).Where("id = ?", id).Update("event_date", date)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// scopeIdeasByIDs verilen kimliklerden kapsama ait olanları döner (başka kapsamın ya da var
// olmayan kimlikler listede yer almaz).
func scopeIdeasByIDs(tx *gorm.DB, s IdeaScope, ideaIDs []uuid.UUID) ([]models.GiftIdea, error) {
	var ideas []models.GiftIdea
	err := s.ideas(tx.Where("id IN ?", ideaIDs), "").Find(&ideas).Error
	return ideas, err
}

// completeScope kazanan fikirleri SELECTED, kapsamı COMPLETED yapar. Çağıran kapsam satırını
// kilitlemiş ve kuralları denetlemiş olmalıdır.
func completeScope(tx *gorm.DB, s IdeaScope, ideaIDs []uuid.UUID) error {
	table, id, _ := s.row()
	if err := tx.Model(&models.GiftIdea{}).Where("id IN ?", ideaIDs).Update("status", "SELECTED").Error; err != nil {
		return err
	}
	return tx.Table(table).Where("id = ?", id).Updates(map[string]any{
		"status":       models.GroupStatusCompleted,
		"completed_at": time.Now(),
	}).Error
}

// reopenScope kapsamı yeniden ACTIVE yapar ve seçilmiş fikirleri ACTIVE'e döndürür. Çağıran
// kapsam satırını kilitlemiş olmalıdır.
func reopenScope(tx *gorm.DB, s IdeaScope) error {
	table, id, _ := s.row()
	if err := s.ideas(tx.Model(&models.GiftIdea{}), "").Where("status = ?", "SELECTED").
		Update("status", "ACTIVE").Error; err != nil {
		return err
	}
	return tx.Table(table).Where("id = ?", id).Updates(map[string]any{
		"status":       models.GroupStatusActive,
		"completed_at": nil,
	}).Error
}

package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrMemberIsGroupTarget eklenmek istenen kullanıcı grubun hedef kişisi olduğunda döner.
var ErrMemberIsGroupTarget = errors.New("member is the group's target")

// GroupRepository grup ve üyelik veritabanı işlemlerini tanımlayan arayüzdür
type GroupRepository interface {
	CreateGroupWithAdmin(group *models.Group, adminUserID uuid.UUID) error
	GetUserGroups(userID uuid.UUID) ([]models.Group, error)
	GetGroupListStats(userID uuid.UUID, groupIDs []uuid.UUID) (map[uuid.UUID]GroupListStats, error)
	GetGroupByID(groupID uuid.UUID) (*models.Group, error)
	GetMember(groupID, userID uuid.UUID) (*models.GroupMember, error)
	GetMemberCount(groupID uuid.UUID) (int64, error)
	GetMembers(groupID uuid.UUID) ([]models.GroupMember, error)
	AddMember(member *models.GroupMember) error
	MutateInterests(groupID uuid.UUID, change func([]string) ([]string, error)) ([]string, error)
	UpdateBudget(groupID uuid.UUID, budget decimal.Decimal) error
	SetTargetUser(groupID, targetID uuid.UUID, guard func(current *uuid.UUID, isMember bool) error) error
	DeleteGroup(groupID uuid.UUID) error
	SetEventDate(groupID uuid.UUID, date *models.Date) error
	CompleteGroup(groupID uuid.UUID, ideaIDs []uuid.UUID, guard func(group *models.Group, ideas []models.GiftIdea) error) error
	ReopenGroup(groupID uuid.UUID, guard func(group *models.Group) error) error
	GetCover(groupID uuid.UUID) (*models.GroupCover, error)
	SaveCover(cover *models.GroupCover, guard func(group *models.Group) error) error
	DeleteCover(groupID uuid.UUID, guard func(group *models.Group) error) error
	SetCoverAdminOnly(groupID uuid.UUID, adminOnly bool) error
	SetDescription(groupID uuid.UUID, description string) error
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

// GroupListStats grup listesindeki tek bir grubun sayaçlarıdır.
// PendingVotes çağıran kullanıcıya özeldir.
type GroupListStats struct {
	GroupID      uuid.UUID
	MemberCount  int64
	IdeaCount    int64
	PendingVotes int64
}

// GetGroupListStats verilen grupların üye/fikir sayılarını ve kullanıcının bekleyen
// oylarını tek sorguda hesaplar (grup başına sorgu yok).
// IdeaCount reddedilmemiş fikirleri sayar. PendingVotes yalnızca açık gruplarda, ACTIVE
// durumdaki, kullanıcının eklemediği ve henüz oy vermediği fikirleri sayar.
func (r *groupRepository) GetGroupListStats(userID uuid.UUID, groupIDs []uuid.UUID) (map[uuid.UUID]GroupListStats, error) {
	out := make(map[uuid.UUID]GroupListStats, len(groupIDs))
	if len(groupIDs) == 0 {
		return out, nil
	}

	var rows []GroupListStats
	err := r.db.Raw(`
		SELECT g.id AS group_id,
			(SELECT COUNT(*) FROM group_members gm WHERE gm.group_id = g.id) AS member_count,
			(SELECT COUNT(*) FROM gift_ideas gi WHERE gi.group_id = g.id AND gi.status <> ?) AS idea_count,
			CASE WHEN g.status = ? THEN (
				SELECT COUNT(*) FROM gift_ideas gi
				WHERE gi.group_id = g.id AND gi.status = ? AND gi.created_by <> ?
					AND NOT EXISTS (SELECT 1 FROM votes v WHERE v.gift_idea_id = gi.id AND v.user_id = ?)
			) ELSE 0 END AS pending_votes
		FROM groups g
		WHERE g.id IN ?`,
		"REJECTED", models.GroupStatusActive, "ACTIVE", userID, userID, groupIDs,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		out[row.GroupID] = row
	}
	return out, nil
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

// AddMember gruba yeni bir üye kaydı ekler. Grup satırı kilitlenir; böylece eşzamanlı
// hedef bağlama ile yarışan hedef kişi üye olarak eklenemez.
func (r *groupRepository) AddMember(member *models.GroupMember) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Select("id", "target_user_id").Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&group, "id = ?", member.GroupID).Error; err != nil {
			return err
		}
		if group.TargetUserID != nil && *group.TargetUserID == member.UserID {
			return ErrMemberIsGroupTarget
		}
		return tx.Create(member).Error
	})
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

// SetTargetUser locks the group row so the guard sees the current target and membership.
// The new target's notifications for this group are deleted in the same transaction.
func (r *groupRepository) SetTargetUser(groupID, targetID uuid.UUID, guard func(current *uuid.UUID, isMember bool) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Select("id", "target_user_id").Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&group, "id = ?", groupID).Error; err != nil {
			return err
		}
		var memberCount int64
		if err := tx.Model(&models.GroupMember{}).Where("group_id = ? AND user_id = ?", groupID, targetID).
			Count(&memberCount).Error; err != nil {
			return err
		}
		if err := guard(group.TargetUserID, memberCount > 0); err != nil {
			return err
		}
		if err := tx.Model(&models.Group{}).Where("id = ?", groupID).Update("target_user_id", targetID).Error; err != nil {
			return err
		}
		// Anti-Spoiler: hedef kişinin bu gruba ait bildirimleri aynı transaction'da silinir;
		// bağlandığı andan itibaren zilinde bu gruptan hiçbir iz kalmaz.
		return tx.Where("group_id = ? AND user_id = ?", groupID, targetID).Delete(&models.Notification{}).Error
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

// SetEventDate grubun hediye tarihini ayarlar; nil tarihi kaldırır.
func (r *groupRepository) SetEventDate(groupID uuid.UUID, date *models.Date) error {
	result := r.db.Model(&models.Group{}).Where("id = ?", groupID).Update("event_date", date)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CompleteGroup kazanan fikirleri SELECTED, grubu COMPLETED yapar. Grup satırı kilitlenir;
// guard kilit altında grubun güncel halini ve gruba ait bulunan fikirleri denetler
// (başka gruba ait ya da var olmayan kimlikler listede yer almaz).
func (r *groupRepository) CompleteGroup(groupID uuid.UUID, ideaIDs []uuid.UUID, guard func(group *models.Group, ideas []models.GiftIdea) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "id = ?", groupID).Error; err != nil {
			return err
		}
		var ideas []models.GiftIdea
		if err := tx.Where("id IN ? AND group_id = ?", ideaIDs, groupID).Find(&ideas).Error; err != nil {
			return err
		}
		if err := guard(&group, ideas); err != nil {
			return err
		}
		if err := tx.Model(&models.GiftIdea{}).Where("id IN ?", ideaIDs).Update("status", "SELECTED").Error; err != nil {
			return err
		}
		return tx.Model(&models.Group{}).Where("id = ?", groupID).Updates(map[string]any{
			"status":       models.GroupStatusCompleted,
			"completed_at": time.Now(),
		}).Error
	})
}

// ReopenGroup grubu yeniden ACTIVE yapar ve seçilmiş fikri ACTIVE'e döndürür.
func (r *groupRepository) ReopenGroup(groupID uuid.UUID, guard func(group *models.Group) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var group models.Group
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&group, "id = ?", groupID).Error; err != nil {
			return err
		}
		if err := guard(&group); err != nil {
			return err
		}
		if err := tx.Model(&models.GiftIdea{}).Where("group_id = ? AND status = ?", groupID, "SELECTED").
			Update("status", "ACTIVE").Error; err != nil {
			return err
		}
		return tx.Model(&models.Group{}).Where("id = ?", groupID).Updates(map[string]any{
			"status":       models.GroupStatusActive,
			"completed_at": nil,
		}).Error
	})
}

// GetCover grubun kapak fotoğrafını baytlarıyla getirir; yoksa nil döner. Baytları okuyan
// tek sorgudur.
func (r *groupRepository) GetCover(groupID uuid.UUID) (*models.GroupCover, error) {
	// Kapağın olmaması olağan bir durumdur; First yerine Find kullanılır ki "record not found"
	// hata olarak loglanmasın.
	var covers []models.GroupCover
	if err := r.db.Where("group_id = ?", groupID).Limit(1).Find(&covers).Error; err != nil {
		return nil, err
	}
	if len(covers) == 0 {
		return nil, nil
	}
	return &covers[0], nil
}

// lockGroupForCover grup satırını kilitler ve guard'a güncel kapak politikasını verir;
// böylece politika değişikliğiyle yarışan bir yükleme eski izinle geçemez.
func lockGroupForCover(tx *gorm.DB, groupID uuid.UUID, guard func(group *models.Group) error) error {
	var group models.Group
	if err := tx.Select("id", "cover_admin_only", "cover_version").Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&group, "id = ?", groupID).Error; err != nil {
		return err
	}
	return guard(&group)
}

// SaveCover kapağı ekler ya da değiştirir ve groups.cover_version'ı aynı transaction'da
// günceller.
func (r *groupRepository) SaveCover(cover *models.GroupCover, guard func(group *models.Group) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupForCover(tx, cover.GroupID, guard); err != nil {
			return err
		}
		cover.UpdatedAt = time.Now()
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "group_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data", "content_type", "etag", "width", "height", "updated_at", "updated_by"}),
		}).Omit("Group", "Updater").Create(cover).Error; err != nil {
			return err
		}
		return tx.Model(&models.Group{}).Where("id = ?", cover.GroupID).Update("cover_version", cover.ETag).Error
	})
}

// DeleteCover kapağı siler ve groups.cover_version'ı boşaltır. Kapak yoksa hata vermez.
func (r *groupRepository) DeleteCover(groupID uuid.UUID, guard func(group *models.Group) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockGroupForCover(tx, groupID, guard); err != nil {
			return err
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&models.GroupCover{}).Error; err != nil {
			return err
		}
		return tx.Model(&models.Group{}).Where("id = ?", groupID).Update("cover_version", "").Error
	})
}

// SetDescription grubun açıklamasını yazar; boş metin açıklamayı siler.
func (r *groupRepository) SetDescription(groupID uuid.UUID, description string) error {
	result := r.db.Model(&models.Group{}).Where("id = ?", groupID).Update("description", description)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetCoverAdminOnly kapak fotoğrafını kimin değiştirebileceğini ayarlar.
func (r *groupRepository) SetCoverAdminOnly(groupID uuid.UUID, adminOnly bool) error {
	result := r.db.Model(&models.Group{}).Where("id = ?", groupID).Update("cover_admin_only", adminOnly)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

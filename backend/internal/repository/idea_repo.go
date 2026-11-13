package repository

import (
	"errors"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IdeaWithScore hediye fikrinin toplam skoru ve kullanıcının kendi oyuyla zenginleştirilmiş halidir
type IdeaWithScore struct {
	models.GiftIdea
	Score  int64 `gorm:"column:score" json:"score"`
	MyVote int8  `gorm:"column:my_vote" json:"my_vote"` // Kullanıcının oyu: 1, -1 veya 0
}

// IdeaRepository hediye ve oy veritabanı işlemlerini tanımlayan arayüzdür. Liste ve
// başlık sorguları bir IdeaScope (SINGLE grup ya da MULTI gruptaki havuz) ile süzülür.
type IdeaRepository interface {
	// CreateIdeas kapsam satırını (grup ya da havuz) FOR SHARE kilitler, guard ile güncel durumu
	// denetler ve fikirleri aynı transaction'da ekler; böylece eşzamanlı kazanan seçimiyle yarışan
	// bir ekleme tamamlanmış kapsama yazamaz. Kapsam satırı yoksa gorm.ErrRecordNotFound döner.
	CreateIdeas(scope IdeaScope, ideas []models.GiftIdea, guard func(status string) error) error
	GetIdeasWithScores(scope IdeaScope, currentUserID uuid.UUID) ([]IdeaWithScore, error)
	GetIdeaByID(ideaID uuid.UUID) (*models.GiftIdea, error)
	// CastVote oyu tek transaction'da kaydeder: kapsam satırı FOR SHARE, fikir satırı FOR UPDATE
	// kilitlenir, guard güncel durumu denetler, oy upsert edilir. Oy -1 ise eksi oy sayısı ve
	// seçmen sayısı aynı transaction'da okunur; eksi oylar seçmenlerin yarısını aşarsa
	// (downvotes > seçmen/2) fikir REJECTED olur. Seçmenler: grubun üyeleri, havuzda alıcı hariç.
	CastVote(scope IdeaScope, vote *models.Vote, guard func(status string) error) error
	// RemoveVote, DeleteIdea ve UpdateStatus, CastVote ile aynı kilitleri alır: kapsam satırı
	// FOR SHARE (havuzda önce grup satırı KEY SHARE), guard güncel durumu denetler, sonra fikir
	// satırı kilitlenir. Böylece eşzamanlı kazanan seçimi ile yarışan bir veto, kazananı REJECTED
	// yapamaz. Fikir ya da kapsam yoksa gorm.ErrRecordNotFound döner.
	RemoveVote(scope IdeaScope, ideaID, userID uuid.UUID, guard func(status string) error) error
	DeleteIdea(scope IdeaScope, ideaID uuid.UUID, guard func(status string) error) error
	UpdateStatus(scope IdeaScope, ideaID uuid.UUID, status string, guard func(status string) error) error
	GetIdeaTitlesByStatus(scope IdeaScope) ([]string, []string, error)
}

type ideaRepository struct {
	db *gorm.DB
}

// NewIdeaRepository yeni bir IdeaRepository örneği oluşturur
func NewIdeaRepository(db *gorm.DB) IdeaRepository {
	return &ideaRepository{db: db}
}

func (r *ideaRepository) CreateIdeas(scope IdeaScope, ideas []models.GiftIdea, guard func(status string) error) error {
	if len(ideas) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		lock, err := lockScope(tx, scope, lockShare)
		if err != nil {
			return err
		}
		if err := guard(lock.Status); err != nil {
			return err
		}
		return tx.Create(&ideas).Error
	})
}

// GetIdeasWithScores kapsamdaki hediyeleri anlık oy skoru ve kullanıcının kendi oyuyla sıralı getirir
func (r *ideaRepository) GetIdeasWithScores(scope IdeaScope, currentUserID uuid.UUID) ([]IdeaWithScore, error) {
	var results []IdeaWithScore

	// Tek bir optimize SQL sorgusuyla:
	// 1. Hediyenin tüm alanlarını çek
	// 2. Oyların toplamını al (SUM)
	// 3. Giriş yapan kullanıcının kendi oyunu alt sorgu ile bul
	// 4. En yüksek puana göre sırala (score DESC)
	err := scope.ideas(r.db.Table("gift_ideas").
		Select(`
			gift_ideas.*,
			COALESCE(SUM(votes.vote_type), 0) AS score,
			COALESCE((SELECT vote_type FROM votes WHERE gift_idea_id = gift_ideas.id AND user_id = ? LIMIT 1), 0) AS my_vote
		`, currentUserID).
		Joins("LEFT JOIN votes ON votes.gift_idea_id = gift_ideas.id"), "gift_ideas.").
		Group("gift_ideas.id").
		Order("score DESC, gift_ideas.created_at DESC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// Her hediyeyi öneren kullanıcı profilini (Creator) dolduruyoruz
	for i := range results {
		var creator models.User
		if err := r.db.First(&creator, "id = ?", results[i].CreatedBy).Error; err == nil {
			results[i].Creator = creator
		}
	}

	return results, nil
}

// GetIdeaByID tek bir hediye fikrini ID'sine göre getirir
func (r *ideaRepository) GetIdeaByID(ideaID uuid.UUID) (*models.GiftIdea, error) {
	var idea models.GiftIdea
	err := r.db.Preload("Creator").First(&idea, "id = ?", ideaID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &idea, nil
}

type ideaTitleStatus struct {
	Title  string `gorm:"column:title"`
	Status string `gorm:"column:status"`
}

// GetIdeaTitlesByStatus kapsamdaki mevcut ve reddedilmiş hediye başlıklarını iki ayrı liste olarak döner
func (r *ideaRepository) GetIdeaTitlesByStatus(scope IdeaScope) ([]string, []string, error) {
	var results []ideaTitleStatus
	err := scope.ideas(r.db.Model(&models.GiftIdea{}).Select("title, status"), "").
		Scan(&results).Error
	if err != nil {
		return nil, nil, err
	}

	var existing []string
	var rejected []string
	for _, item := range results {
		if item.Status == "REJECTED" {
			rejected = append(rejected, item.Title)
		} else {
			existing = append(existing, item.Title)
		}
	}
	return existing, rejected, nil
}

func (r *ideaRepository) CastVote(scope IdeaScope, vote *models.Vote, guard func(status string) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		lock, err := lockScope(tx, scope, lockShare)
		if err != nil {
			return err
		}
		if err := guard(lock.Status); err != nil {
			return err
		}
		// Aynı fikre gelen oylar sıraya girer; eksi oy sayısı ve eşik tutarlı okunur.
		var locked []uuid.UUID
		if err := tx.Raw("SELECT id FROM gift_ideas WHERE id = ? FOR NO KEY UPDATE", vote.GiftIdeaID).Scan(&locked).Error; err != nil {
			return err
		}
		if len(locked) == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "gift_idea_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"vote_type", "created_at"}),
		}).Create(vote).Error; err != nil {
			return err
		}
		if vote.VoteType != -1 {
			return nil
		}
		var downvotes int64
		if err := tx.Model(&models.Vote{}).Where("gift_idea_id = ? AND vote_type = -1", vote.GiftIdeaID).
			Count(&downvotes).Error; err != nil {
			return err
		}
		electorate := tx.Model(&models.GroupMember{}).Where("group_id = ?", scope.GroupID)
		if lock.Recipient != nil {
			electorate = electorate.Where("user_id <> ?", *lock.Recipient)
		}
		var voters int64
		if err := electorate.Count(&voters).Error; err != nil {
			return err
		}
		// Demokrasi vetosu: eksi oylar seçmenlerin yarısını aşarsa (tam sayı bölmesi) fikir elenir.
		if voters > 0 && downvotes > voters/2 {
			return tx.Model(&models.GiftIdea{}).Where("id = ?", vote.GiftIdeaID).Update("status", "REJECTED").Error
		}
		return nil
	})
}

// withLockedIdea kapsam satırını FOR SHARE kilitler, guard'ı çağırır, sonra fikir satırını verilen
// güçte kilitler ve fn'i aynı transaction'da çalıştırır (kilit sırası: groups → gift_pools → gift_ideas).
func (r *ideaRepository) withLockedIdea(scope IdeaScope, ideaID uuid.UUID, strength string, guard func(string) error, fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		lock, err := lockScope(tx, scope, lockShare)
		if err != nil {
			return err
		}
		if err := guard(lock.Status); err != nil {
			return err
		}
		var locked []uuid.UUID
		if err := tx.Raw("SELECT id FROM gift_ideas WHERE id = ? FOR "+strength, ideaID).Scan(&locked).Error; err != nil {
			return err
		}
		if len(locked) == 0 {
			return gorm.ErrRecordNotFound
		}
		return fn(tx)
	})
}

// RemoveVote kullanıcının fikre verdiği oyu siler. Oy yoksa hata vermez (idempotent).
func (r *ideaRepository) RemoveVote(scope IdeaScope, ideaID, userID uuid.UUID, guard func(string) error) error {
	return r.withLockedIdea(scope, ideaID, lockNoKeyUpdate, guard, func(tx *gorm.DB) error {
		return tx.Where("gift_idea_id = ? AND user_id = ?", ideaID, userID).Delete(&models.Vote{}).Error
	})
}

// DeleteIdea hediye fikrini siler (ilişkili oylar CASCADE ile silinir).
func (r *ideaRepository) DeleteIdea(scope IdeaScope, ideaID uuid.UUID, guard func(string) error) error {
	return r.withLockedIdea(scope, ideaID, "UPDATE", guard, func(tx *gorm.DB) error {
		return tx.Delete(&models.GiftIdea{}, "id = ?", ideaID).Error
	})
}

// UpdateStatus hediye fikrinin durumunu günceller ('ACTIVE', 'REJECTED', 'SELECTED').
func (r *ideaRepository) UpdateStatus(scope IdeaScope, ideaID uuid.UUID, status string, guard func(string) error) error {
	return r.withLockedIdea(scope, ideaID, lockNoKeyUpdate, guard, func(tx *gorm.DB) error {
		return tx.Model(&models.GiftIdea{}).Where("id = ?", ideaID).Update("status", status).Error
	})
}

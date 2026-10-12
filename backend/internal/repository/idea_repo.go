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

// IdeaRepository hediye ve oy veritabanı işlemlerini tanımlayan arayüzdür
type IdeaRepository interface {
	CreateIdea(idea *models.GiftIdea) error
	CreateIdeasBatch(ideas []models.GiftIdea) error
	GetIdeasWithScores(groupID, currentUserID uuid.UUID) ([]IdeaWithScore, error)
	GetIdeaByID(ideaID uuid.UUID) (*models.GiftIdea, error)
	Vote(vote *models.Vote) error
	DeleteIdea(ideaID uuid.UUID) error
	UpdateStatus(ideaID uuid.UUID, status string) error
	GetGroupIdeaTitlesByStatus(groupID uuid.UUID) ([]string, []string, error)
	GetDownvoteCount(ideaID uuid.UUID) (int64, error)
}

type ideaRepository struct {
	db *gorm.DB
}

// NewIdeaRepository yeni bir IdeaRepository örneği oluşturur
func NewIdeaRepository(db *gorm.DB) IdeaRepository {
	return &ideaRepository{db: db}
}

// CreateIdea yeni hediye önerisini veritabanına kaydeder
func (r *ideaRepository) CreateIdea(idea *models.GiftIdea) error {
	return r.db.Create(idea).Error
}

// GetIdeasWithScores gruptaki hediyeleri anlık oy skoru ve kullanıcının kendi oyuyla sıralı getirir
func (r *ideaRepository) GetIdeasWithScores(groupID, currentUserID uuid.UUID) ([]IdeaWithScore, error) {
	var results []IdeaWithScore

	// Tek bir optimize SQL sorgusuyla:
	// 1. Hediyenin tüm alanlarını çek
	// 2. Oyların toplamını al (SUM)
	// 3. Giriş yapan kullanıcının kendi oyunu alt sorgu ile bul
	// 4. En yüksek puana göre sırala (score DESC)
	err := r.db.Table("gift_ideas").
		Select(`
			gift_ideas.*,
			COALESCE(SUM(votes.vote_type), 0) AS score,
			COALESCE((SELECT vote_type FROM votes WHERE gift_idea_id = gift_ideas.id AND user_id = ? LIMIT 1), 0) AS my_vote
		`, currentUserID).
		Joins("LEFT JOIN votes ON votes.gift_idea_id = gift_ideas.id").
		Where("gift_ideas.group_id = ?", groupID).
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

// Vote oyu kaydeder; aynı kullanıcı daha önce oy verdiyse günceller (Upsert)
func (r *ideaRepository) Vote(vote *models.Vote) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "gift_idea_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"vote_type", "created_at"}),
	}).Create(vote).Error
}

// DeleteIdea hediye fikrini siler (İlişkili oylar CASCADE ile otomatik silinir)
func (r *ideaRepository) DeleteIdea(ideaID uuid.UUID) error {
	return r.db.Delete(&models.GiftIdea{}, "id = ?", ideaID).Error
}

// CreateIdeasBatch birden fazla hediye önerisini tek seferde veritabanına ekler
func (r *ideaRepository) CreateIdeasBatch(ideas []models.GiftIdea) error {
	if len(ideas) == 0 {
		return nil
	}
	return r.db.Create(&ideas).Error
}

// UpdateStatus hediye fikrinin durumunu günceller ('ACTIVE', 'REJECTED', 'SELECTED')
func (r *ideaRepository) UpdateStatus(ideaID uuid.UUID, status string) error {
	return r.db.Model(&models.GiftIdea{}).Where("id = ?", ideaID).Update("status", status).Error
}

type ideaTitleStatus struct {
	Title  string `gorm:"column:title"`
	Status string `gorm:"column:status"`
}

// GetGroupIdeaTitlesByStatus gruptaki mevcut ve reddedilmiş hediye başlıklarını iki ayrı liste olarak döner
func (r *ideaRepository) GetGroupIdeaTitlesByStatus(groupID uuid.UUID) ([]string, []string, error) {
	var results []ideaTitleStatus
	err := r.db.Model(&models.GiftIdea{}).
		Select("title, status").
		Where("group_id = ?", groupID).
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

// GetDownvoteCount bir hediyeye verilen toplam negatif oyların (-1) sayısını döner
func (r *ideaRepository) GetDownvoteCount(ideaID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Vote{}).
		Where("gift_idea_id = ? AND vote_type = -1", ideaID).
		Count(&count).Error
	return count, err
}


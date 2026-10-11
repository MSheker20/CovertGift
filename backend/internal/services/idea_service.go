package services

import (
	"errors"
	"strings"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Hediye ve oylama hata tanımları
var (
	ErrIdeaNotFound       = errors.New("hediye önerisi bulunamadı")
	ErrInvalidVoteType    = errors.New("geçersiz oy türü (+1 veya -1 olmalıdır)")
	ErrUnauthorizedDelete = errors.New("bu hediye önerisini silme yetkiniz yok (sadece öneren veya grup admini silebilir)")
)

// IdeaService hediye havuzu ve oylama iş mantığı arayüzüdür
type IdeaService interface {
	AddManualIdea(groupID, userID uuid.UUID, title, description, searchQuery, productURL, imageURL string, price decimal.Decimal) (*models.GiftIdea, error)
	GetGroupIdeas(groupID, currentUserID uuid.UUID) ([]repository.IdeaWithScore, error)
	VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error
	DeleteIdea(ideaID, currentUserID uuid.UUID) error
}

type ideaService struct {
	ideaRepo  repository.IdeaRepository
	groupRepo repository.GroupRepository
}

// NewIdeaService yeni bir IdeaService örneği oluşturur
func NewIdeaService(ideaRepo repository.IdeaRepository, groupRepo repository.GroupRepository) IdeaService {
	return &ideaService{
		ideaRepo:  ideaRepo,
		groupRepo: groupRepo,
	}
}

// AddManualIdea gruba kullanıcı tarafından manuel bir hediye önerisi ekler
func (s *ideaService) AddManualIdea(groupID, userID uuid.UUID, title, description, searchQuery, productURL, imageURL string, price decimal.Decimal) (*models.GiftIdea, error) {
	// 1. Grubu getir
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}

	// 2. Anti-Spoiler: Hedef kişi hediye önerisi ekleyemez
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, ErrAntiSpoilerTriggered
	}

	// 3. Üyelik kontrolü: Sadece grup üyeleri öneri yapabilir
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}

	// search_query boş bırakılmışsa otomatik başlıktan türet
	cleanQuery := strings.TrimSpace(searchQuery)
	if cleanQuery == "" {
		cleanQuery = title
	}

	idea := &models.GiftIdea{
		GroupID:        groupID,
		Title:          title,
		Description:    description,
		EstimatedPrice: price,
		SourceType:     "MANUAL",
		SearchQuery:    cleanQuery,
		ProductURL:     productURL,
		ImageURL:       imageURL,
		CreatedBy:      userID,
	}

	if err := s.ideaRepo.CreateIdea(idea); err != nil {
		return nil, err
	}

	return idea, nil
}

// GetGroupIdeas grubun hediye havuzunu skorlarıyla birlikte listeler
func (s *ideaService) GetGroupIdeas(groupID, currentUserID uuid.UUID) ([]repository.IdeaWithScore, error) {
	// 1. Grubu getir
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}

	// 2. Anti-Spoiler: Hedef kişi hediye havuzunu asla göremez
	if group.TargetUserID != nil && *group.TargetUserID == currentUserID {
		return nil, ErrAntiSpoilerTriggered
	}

	// 3. Üyelik kontrolü
	member, err := s.groupRepo.GetMember(groupID, currentUserID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}

	return s.ideaRepo.GetIdeasWithScores(groupID, currentUserID)
}

// VoteIdea bir hediye fikrine oy verir (+1 veya -1)
func (s *ideaService) VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error {
	// 1. Oy geçerli mi kontrol et
	if voteType != 1 && voteType != -1 {
		return ErrInvalidVoteType
	}

	// 2. Hediye fikrini bul
	idea, err := s.ideaRepo.GetIdeaByID(ideaID)
	if err != nil {
		return nil
	}
	if idea == nil {
		return ErrIdeaNotFound
	}

	// 3. Hediyenin grubunu bul
	group, err := s.groupRepo.GetGroupByID(idea.GroupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}

	// 4. KRİTİK: Fikir üzerinden Anti-Spoiler Kontrolü!
	if group.TargetUserID != nil && *group.TargetUserID == currentUserID {
		return ErrAntiSpoilerTriggered
	}

	// 5. Kullanıcı bu grubun üyesi mi?
	member, err := s.groupRepo.GetMember(idea.GroupID, currentUserID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}

	// 6. Oyu kaydet (Upsert)
	vote := &models.Vote{
		GiftIdeaID: ideaID,
		UserID:     currentUserID,
		VoteType:   voteType,
	}

	return s.ideaRepo.Vote(vote)
}

// DeleteIdea hediye fikrini siler (sadece öneren kişi veya grup admini)
func (s *ideaService) DeleteIdea(ideaID, currentUserID uuid.UUID) error {
	// 1. Hediyeyi bul
	idea, err := s.ideaRepo.GetIdeaByID(ideaID)
	if err != nil {
		return err
	}
	if idea == nil {
		return ErrIdeaNotFound
	}

	// 2. Kullanıcının gruptaki rolünü kontrol et
	member, err := s.groupRepo.GetMember(idea.GroupID, currentUserID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}

	// 3. YETKİ KONTROLÜ: Sadece öneren kişi VEYA grup admini silebilir!
	isCreator := idea.CreatedBy == currentUserID
	isAdmin := member.Role == "admin"

	if !isCreator && !isAdmin {
		return ErrUnauthorizedDelete
	}

	return s.ideaRepo.DeleteIdea(ideaID)
}

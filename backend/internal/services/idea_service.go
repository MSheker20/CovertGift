package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Hediye ve oylama hata tanımları
var (
	ErrIdeaNotFound       = errors.New("hediye önerisi bulunamadı")
	ErrInvalidVoteType    = errors.New("geçersiz oy türü (+1 veya -1 olmalıdır)")
	ErrUnauthorizedDelete = errors.New("bu hediye önerisini silme yetkiniz yok (sadece öneren veya grup admini silebilir)")
	ErrUnauthorizedVeto   = errors.New("bu hediye önerisini sadece grup yöneticisi (admin) veto edebilir")
	ErrNoAIIdeasGenerated = errors.New("yapay zeka kriterlere uygun yeni bir hediye önerisi üretemedi")
)

// IdeaService hediye havuzu ve oylama iş mantığı arayüzüdür
type IdeaService interface {
	AddManualIdea(groupID, userID uuid.UUID, title, description, searchQuery, productURL, imageURL string, price decimal.Decimal) (*models.GiftIdea, error)
	GetGroupIdeas(groupID, currentUserID uuid.UUID) ([]repository.IdeaWithScore, error)
	VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error
	DeleteIdea(ideaID, currentUserID uuid.UUID) error
	RejectIdea(ideaID, currentUserID uuid.UUID) error
	GenerateAIIdeas(ctx context.Context, groupID, currentUserID uuid.UUID) ([]models.GiftIdea, error)
}

type ideaService struct {
	ideaRepo     repository.IdeaRepository
	groupRepo    repository.GroupRepository
	geminiClient *gemini.Client
}

// NewIdeaService yeni bir IdeaService örneği oluşturur
func NewIdeaService(ideaRepo repository.IdeaRepository, groupRepo repository.GroupRepository, geminiClient *gemini.Client) IdeaService {
	return &ideaService{
		ideaRepo:     ideaRepo,
		groupRepo:    groupRepo,
		geminiClient: geminiClient,
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

	isOverBudget := group.TotalBudget.GreaterThan(decimal.Zero) && price.GreaterThan(group.TotalBudget)

	idea := &models.GiftIdea{
		GroupID:        groupID,
		Title:          title,
		Description:    description,
		EstimatedPrice: price,
		SourceType:     "MANUAL",
		Status:         "ACTIVE",
		IsOverBudget:   isOverBudget,
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

// VoteIdea bir hediye fikrine oy verir (+1 veya -1) ve gerekirse demokrasi vetosunu işletir
func (s *ideaService) VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error {
	// 1. Oy geçerli mi kontrol et
	if voteType != 1 && voteType != -1 {
		return ErrInvalidVoteType
	}

	// 2. Hediye fikrini bul
	idea, err := s.ideaRepo.GetIdeaByID(ideaID)
	if err != nil {
		return err
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

	if err := s.ideaRepo.Vote(vote); err != nil {
		return err
	}

	// 7. Demokrasi Vetosu Kontrolü (Downvote verilmişse):
	// Eğer toplam downvote sayısı gruptaki toplam üye sayısının yarısından fazlaysa otomatik REJECTED yap
	if voteType == -1 {
		downvotes, err := s.ideaRepo.GetDownvoteCount(ideaID)
		if err == nil {
			memberCount, err := s.groupRepo.GetMemberCount(idea.GroupID)
			if err == nil && memberCount > 0 {
				if downvotes > memberCount/2 {
					_ = s.ideaRepo.UpdateStatus(ideaID, "REJECTED")
				}
			}
		}
	}

	return nil
}

// RejectIdea grup yöneticisinin bir hediye fikrini doğrudan veto etmesini sağlar
func (s *ideaService) RejectIdea(ideaID, currentUserID uuid.UUID) error {
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

	// 3. YETKİ KONTROLÜ: Sadece grup admini doğrudan veto edebilir!
	if member.Role != "admin" {
		return ErrUnauthorizedVeto
	}

	// 4. Durumu 'REJECTED' olarak güncelle
	return s.ideaRepo.UpdateStatus(ideaID, "REJECTED")
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

// GenerateAIIdeas Gemini AI kullanarak gruba yeni hediye önerileri üretir ve havuza ekler
func (s *ideaService) GenerateAIIdeas(ctx context.Context, groupID, currentUserID uuid.UUID) ([]models.GiftIdea, error) {
	if s.geminiClient == nil {
		return nil, gemini.ErrAPIKeyMissing
	}

	// 1. Grubu getir
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}

	// 2. Anti-Spoiler: Hedef kişi asla AI önerisi tetikleyemez!
	if group.TargetUserID != nil && *group.TargetUserID == currentUserID {
		return nil, ErrAntiSpoilerTriggered
	}

	// 3. Üyelik kontrolü: Sadece grup üyeleri AI önerisi tetikleyebilir
	member, err := s.groupRepo.GetMember(groupID, currentUserID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}

	// 4. State-Based Context: Mevcut ve reddedilmiş hediye başlıklarını DB'den al
	existingTitles, rejectedTitles, err := s.ideaRepo.GetGroupIdeaTitlesByStatus(groupID)
	if err != nil {
		return nil, err
	}

	// 5. Dinamik Prompt İnşası
	prompt := s.buildAIPrompt(group, existingTitles, rejectedTitles)

	// 6. Gemini API Çağrısı (Structured JSON)
	suggestions, err := s.geminiClient.GenerateGiftIdeas(ctx, prompt)
	if err != nil {
		return nil, err
	}

	if len(suggestions) == 0 {
		return nil, ErrNoAIIdeasGenerated
	}

	// 7. Sunucu Tarafı Bütçe & Kalite Filtresi
	var newIdeas []models.GiftIdea
	for _, item := range suggestions {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}

		searchQuery := strings.TrimSpace(item.SearchQuery)
		if searchQuery == "" {
			searchQuery = title
		}

		// Bütçe aşımı kontrolü (Gruba karar hakkı tanımak için filtrelemiyoruz, sadece işaretliyoruz)
		isOverBudget := group.TotalBudget.GreaterThan(decimal.Zero) && item.EstimatedPrice.GreaterThan(group.TotalBudget)

		idea := models.GiftIdea{
			GroupID:        groupID,
			Title:          title,
			Description:    item.Description,
			EstimatedPrice: item.EstimatedPrice,
			SourceType:     "AI",
			Status:         "ACTIVE",
			IsOverBudget:   isOverBudget,
			SearchQuery:    searchQuery,
			CreatedBy:      currentUserID,
		}
		newIdeas = append(newIdeas, idea)
	}

	if len(newIdeas) == 0 {
		return nil, ErrNoAIIdeasGenerated
	}

	// 8. Önerileri veritabanına topluca kaydet
	if err := s.ideaRepo.CreateIdeasBatch(newIdeas); err != nil {
		return nil, err
	}

	return newIdeas, nil
}

// buildAIPrompt hedef kişi bilgileri ve state kısıtlarından dinamik prompt üretir
func (s *ideaService) buildAIPrompt(group *models.Group, existingTitles, rejectedTitles []string) string {
	var sb strings.Builder

	sb.WriteString("Sen profesyonel, yaratıcı ve zevkli bir hediye danışmanısın. ")
	sb.WriteString("Bir grup arkadaş, sevdikleri bir kişi için sürpriz hediye seçecek.\n\n")

	sb.WriteString("HEDEF KİŞİ BİLGİLERİ:\n")
	sb.WriteString(fmt.Sprintf("- İsim: %s\n", group.TargetPersonName))

	if strings.TrimSpace(group.TargetInterests) != "" {
		sb.WriteString(fmt.Sprintf("- İlgi Alanları ve Hobileri: %s\n", group.TargetInterests))
	} else {
		sb.WriteString("- İlgi Alanları: Belirtilmemiş (Genel, kullanışlı ve modern hediyeler öner)\n")
	}

	if strings.TrimSpace(group.Description) != "" {
		sb.WriteString(fmt.Sprintf("- Grup/Kutlama Amacı: %s\n", group.Description))
	}

	if group.TotalBudget.GreaterThan(decimal.Zero) {
		sb.WriteString(fmt.Sprintf("- Toplam Bütçe Sınırı: %s TL\n", group.TotalBudget.StringFixed(2)))
	}

	sb.WriteString("\nÖNEMLİ KURALLAR VE KISITLAR:\n")

	if len(existingTitles) > 0 {
		sb.WriteString(fmt.Sprintf("1. HAVUZDA ZATEN MEVCUT OLAN HEDİYELER (BUNLARI ASLA TEKRAR ETME): %s\n", strings.Join(existingTitles, ", ")))
	}

	if len(rejectedTitles) > 0 {
		sb.WriteString(fmt.Sprintf("2. GRUBUN REDDETTİĞİ / VETO ETTİĞİ HEDİYELER (BU HEDİYELERİ GRUP BEĞENMEDİ, KESİNLİKLE BUNLARA BENZER BİR ŞEY ÖNERME): %s\n", strings.Join(rejectedTitles, ", ")))
	}

	sb.WriteString("3. SIFIR KIRIK LİNK KURALI: Asla web sitesi linki veya URL üretme. Sadece Türkiye'deki pazaryerlerinde (Trendyol, Hepsiburada, Amazon TR) aratıldığında doğrudan ürünü çıkaracak temiz, spesifik 'search_query' kelimesi oluştur.\n")
	sb.WriteString("4. FİYAT VE BÜTÇE: Hediyelerin tahmini fiyatlarını gerçekçi belirle ve hedef bütçe civarında kalmaya özen göster. Ancak hedef kişiye çok iyi uyacağını düşündüğün özel veya kaliteli bir hediye bütçeyi bir miktar aşıyorsa da önerebilirsin (grup üyeleri bütçe artışını tolere edebilir).\n")
	sb.WriteString("5. ADET: Hedef kişinin zevkine tam hitap eden 3 ile 5 adet arasında birbirinden farklı, orijinal öneri üret.")

	return sb.String()
}

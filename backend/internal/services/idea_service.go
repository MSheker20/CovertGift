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
	ErrInvalidPrice       = errors.New("tahmini fiyat 0–99.999.999,99 ₺ arasında ve en fazla iki ondalık basamaklı olmalıdır")
	ErrUnauthorizedDelete = errors.New("bu hediye önerisini silme yetkiniz yok (sadece öneren veya grup admini silebilir)")
	ErrUnauthorizedVeto   = errors.New("bu hediye önerisini sadece grup yöneticisi (admin) veto edebilir")
	ErrNoAIIdeasGenerated = errors.New("yapay zeka kriterlere uygun yeni bir hediye önerisi üretemedi")
)

// IdeaService hediye havuzu ve oylama iş mantığı arayüzüdür
type IdeaService interface {
	AddManualIdea(groupID, userID uuid.UUID, title, description, searchQuery, productURL, imageURL string, price decimal.Decimal) (*models.GiftIdea, error)
	GetGroupIdeas(groupID, currentUserID uuid.UUID) ([]repository.IdeaWithScore, error)
	VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error
	UnvoteIdea(ideaID, currentUserID uuid.UUID) error
	DeleteIdea(ideaID, currentUserID uuid.UUID) error
	RejectIdea(ideaID, currentUserID uuid.UUID) error
	GenerateAIIdeas(ctx context.Context, groupID, currentUserID uuid.UUID) ([]models.GiftIdea, error)
}

type ideaService struct {
	ideaRepo     repository.IdeaRepository
	groupRepo    repository.GroupRepository
	geminiClient GiftIdeaGenerator
}

type GiftIdeaGenerator interface {
	GenerateGiftIdeas(context.Context, string) ([]gemini.AIIdeaSuggestion, error)
}

// NewIdeaService yeni bir IdeaService örneği oluşturur
func NewIdeaService(ideaRepo repository.IdeaRepository, groupRepo repository.GroupRepository, geminiClient GiftIdeaGenerator) IdeaService {
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
	if group.Status == models.GroupStatusCompleted {
		return nil, ErrGroupCompleted
	}

	// 3. Üyelik kontrolü: Sadece grup üyeleri öneri yapabilir
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}

	if !validMoney(price) {
		return nil, ErrInvalidPrice
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

	ideas, err := s.ideaRepo.GetIdeasWithScores(groupID, currentUserID)
	if err != nil {
		return nil, err
	}
	// An idea generation/addition may have started before a concurrent budget update.
	// Always display warnings using the budget read for this request.
	for i := range ideas {
		ideas[i].IsOverBudget = group.TotalBudget.GreaterThan(decimal.Zero) && ideas[i].EstimatedPrice.GreaterThan(group.TotalBudget)
	}
	return ideas, nil
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
	if group.Status == models.GroupStatusCompleted {
		return ErrGroupCompleted
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

// /ideas/:id/* uçları grup middleware'inden geçmediği için Anti-Spoiler bu katmanda,
// her fikir işleminde ayrıca denetlenir.
// openGroupForIdea fikrin grubunu getirir; istek atan hedefse ErrAntiSpoilerTriggered,
// grup tamamlandıysa ErrGroupCompleted döner. Anti-Spoiler her zaman önce denetlenir.
func (s *ideaService) openGroupForIdea(groupID, userID uuid.UUID) (*models.Group, error) {
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, ErrAntiSpoilerTriggered
	}
	if group.Status == models.GroupStatusCompleted {
		return nil, ErrGroupCompleted
	}
	return group, nil
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

	// 2. KRİTİK: Fikir üzerinden Anti-Spoiler Kontrolü (üyelikten önce); tamamlanan grupta kapalı
	if _, err := s.openGroupForIdea(idea.GroupID, currentUserID); err != nil {
		return err
	}

	// 3. Kullanıcının gruptaki rolünü kontrol et
	member, err := s.groupRepo.GetMember(idea.GroupID, currentUserID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}

	// 4. YETKİ KONTROLÜ: Sadece grup admini doğrudan veto edebilir!
	if member.Role != "admin" {
		return ErrUnauthorizedVeto
	}

	// 5. Durumu 'REJECTED' olarak güncelle
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

	// 2. KRİTİK: Fikir üzerinden Anti-Spoiler Kontrolü (üyelikten önce); tamamlanan grupta kapalı
	if _, err := s.openGroupForIdea(idea.GroupID, currentUserID); err != nil {
		return err
	}

	// 3. Kullanıcının gruptaki rolünü kontrol et
	member, err := s.groupRepo.GetMember(idea.GroupID, currentUserID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}

	// 4. YETKİ KONTROLÜ: Sadece öneren kişi VEYA grup admini silebilir!
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
	if group.Status == models.GroupStatusCompleted {
		return nil, ErrGroupCompleted
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
	seenTitles := make(map[string]bool)
	seenQueries := make(map[string]bool)
	for _, title := range append(append([]string{}, existingTitles...), rejectedTitles...) {
		seenTitles[gemini.NormalizeProduct(title)] = true
	}
	for _, item := range suggestions {
		item.Title = strings.TrimSpace(item.Title)
		item.Description = strings.TrimSpace(item.Description)
		item.SearchQuery = strings.TrimSpace(item.SearchQuery)
		if !gemini.ValidSuggestion(item) || (len(item.Sources) == 0 && !item.Unverified) {
			continue
		}
		titleKey, queryKey := gemini.NormalizeProduct(item.Title), gemini.NormalizeProduct(item.SearchQuery)
		if seenTitles[titleKey] || seenTitles[queryKey] || seenQueries[queryKey] {
			continue
		}
		seenTitles[titleKey], seenQueries[queryKey] = true, true
		if item.Unverified {
			item.Description += "\n\nWeb araması yapılmadı. Fiyat tahminidir; güncel fiyat ve stok satın almadan önce kontrol edilmelidir."
			item.Sources, item.SearchSuggestions = nil, ""
		}
		var sources []models.AISource
		for _, source := range item.Sources {
			sources = append(sources, models.AISource{Title: source.Title, URL: source.URL})
		}

		// Bütçe aşımı kontrolü (Gruba karar hakkı tanımak için filtrelemiyoruz, sadece işaretliyoruz)
		isOverBudget := group.TotalBudget.GreaterThan(decimal.Zero) && item.EstimatedPrice.GreaterThan(group.TotalBudget)

		idea := models.GiftIdea{
			GroupID:             groupID,
			Title:               item.Title,
			Description:         item.Description,
			EstimatedPrice:      item.EstimatedPrice,
			SourceType:          "AI",
			Status:              "ACTIVE",
			IsOverBudget:        isOverBudget,
			SearchQuery:         item.SearchQuery,
			AISources:           sources,
			AISearchSuggestions: item.SearchSuggestions,
			CreatedBy:           currentUserID,
		}
		newIdeas = append(newIdeas, idea)
		if len(newIdeas) == 5 {
			break
		}
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
	sb.WriteString("Pazar: Türkiye. Para birimi: TRY/TL. Hedef kişinin adı üzerinden yaş, cinsiyet veya kişilik varsayma.\n")

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
	sb.WriteString("5. KALİTE: Türkiye mağazalarında satış ilanı bulunan somut ürünler seç. Hayali marka/model, yalnızca yurt dışı satışı, abonelik veya soyut deneyim önerme. Açıklamada ürünün hangi ilgi alanına nasıl uyduğunu belirt.\n")
	sb.WriteString("6. BÜTÇE DAĞILIMI: Bütçe her bir alternatif hediye içindir; öneri sayısına bölme. Bütçe verilmişse öncelikle bütçe içindeki adayları seç.\n")
	sb.WriteString("7. ADET: Tercihen 3-5 farklı öneri üret; yeterli uygun ve kaynaklı ürün bulunamazsa daha az üret. Adet doldurmak için ürün uydurma.")

	return sb.String()
}

// UnvoteIdea kullanıcının fikre verdiği oyu kaldırır. Oy yoksa hata vermez.
// Çoğunluk vetosuyla reddedilmiş bir fikir, oy kaldırılınca kendiliğinden geri açılmaz;
// REJECTED durumu admin vetosuyla da oluşabildiği için ayırt edilemez.
func (s *ideaService) UnvoteIdea(ideaID, currentUserID uuid.UUID) error {
	idea, err := s.ideaRepo.GetIdeaByID(ideaID)
	if err != nil {
		return err
	}
	if idea == nil {
		return ErrIdeaNotFound
	}
	if _, err := s.openGroupForIdea(idea.GroupID, currentUserID); err != nil {
		return err
	}
	member, err := s.groupRepo.GetMember(idea.GroupID, currentUserID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}
	return s.ideaRepo.RemoveVote(ideaID, currentUserID)
}

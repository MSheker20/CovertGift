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
	"gorm.io/gorm"
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

// IdeaService hediye havuzu ve oylama iş mantığı arayüzüdür. Liste, ekleme ve AI önerisi bir
// kapsamda (SINGLE grup ya da MULTI gruptaki havuz) çalışır; /ideas/:id/* işlemleri kapsamı
// fikrin kendisinden tek bir yardımcıyla (resolveIdea) çözer.
type IdeaService interface {
	AddManualIdea(scope repository.IdeaScope, userID uuid.UUID, title, description, searchQuery, productURL, imageURL string, price decimal.Decimal) (*models.GiftIdea, error)
	GetGroupIdeas(scope repository.IdeaScope, currentUserID uuid.UUID) ([]repository.IdeaWithScore, error)
	VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error
	UnvoteIdea(ideaID, currentUserID uuid.UUID) error
	DeleteIdea(ideaID, currentUserID uuid.UUID) error
	RejectIdea(ideaID, currentUserID uuid.UUID) error
	GenerateAIIdeas(ctx context.Context, scope repository.IdeaScope, currentUserID uuid.UUID) ([]models.GiftIdea, error)
	// GuardIdea yalnızca Anti-Spoiler'ı denetler: fikrin grubunun hedef kişisi ya da havuzunun
	// alıcısı için ErrAntiSpoilerTriggered döner, diğer her durumda nil (gövde doğrulamasından
	// önce çalışan middleware için).
	GuardIdea(ideaID, userID uuid.UUID) error
}

type ideaService struct {
	ideaRepo     repository.IdeaRepository
	groupRepo    repository.GroupRepository
	poolRepo     repository.PoolRepository
	geminiClient GiftIdeaGenerator
}

type GiftIdeaGenerator interface {
	GenerateGiftIdeas(context.Context, string) ([]gemini.AIIdeaSuggestion, error)
}

// NewIdeaService yeni bir IdeaService örneği oluşturur. poolRepo nil olabilir; o zaman havuz
// kapsamları ErrPoolNotFound döner.
func NewIdeaService(ideaRepo repository.IdeaRepository, groupRepo repository.GroupRepository, poolRepo repository.PoolRepository, geminiClient GiftIdeaGenerator) IdeaService {
	return &ideaService{
		ideaRepo:     ideaRepo,
		groupRepo:    groupRepo,
		poolRepo:     poolRepo,
		geminiClient: geminiClient,
	}
}

// loadScope kapsamın grubunu (ve havuzunu) getirir. Anti-Spoiler üyelikten önce denetlenir:
// grubun hedef kişisi ya da havuzun alıcısı ErrAntiSpoilerTriggered alır. Havuzsuz kapsam MULTI
// grupta kullanılamaz (ErrWrongGroupMode).
func (s *ideaService) loadScope(scope repository.IdeaScope, userID uuid.UUID) (giftScope, error) {
	group, err := s.groupRepo.GetGroupByID(scope.GroupID)
	if err != nil {
		return giftScope{}, err
	}
	if group == nil {
		return giftScope{}, ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return giftScope{}, ErrAntiSpoilerTriggered
	}
	if scope.PoolID == nil {
		if group.IsMulti() {
			return giftScope{}, ErrWrongGroupMode
		}
		return giftScope{Group: group}, nil
	}
	if s.poolRepo == nil {
		return giftScope{}, ErrPoolNotFound
	}
	pool, err := s.poolRepo.GetPool(group.ID, *scope.PoolID)
	if err != nil {
		return giftScope{}, err
	}
	if pool == nil {
		return giftScope{}, ErrPoolNotFound
	}
	if pool.RecipientUserID == userID {
		return giftScope{}, ErrAntiSpoilerTriggered
	}
	return giftScope{Group: group, Pool: pool}, nil
}

func (s *ideaService) requireMember(groupID, userID uuid.UUID) (*models.GroupMember, error) {
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}
	return member, nil
}

// createIdeas fikirleri kapsam satırı FOR SHARE kilitliyken ekler; durum kilit altında yeniden
// denetlenir (eşzamanlı kazanan seçimiyle yarış). Kapsam bu arada silindiyse ErrPoolNotFound /
// ErrGroupNotFound döner.
func (s *ideaService) createIdeas(sc giftScope, ideas []models.GiftIdea) error {
	err := s.ideaRepo.CreateIdeas(sc.IdeaScope(), ideas, func(status string) error {
		if status == models.GroupStatusCompleted {
			return sc.CompletedErr()
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if sc.IsPool() {
			return ErrPoolNotFound
		}
		return ErrGroupNotFound
	}
	return err
}

// AddManualIdea kapsama kullanıcı tarafından manuel bir hediye önerisi ekler
func (s *ideaService) AddManualIdea(scope repository.IdeaScope, userID uuid.UUID, title, description, searchQuery, productURL, imageURL string, price decimal.Decimal) (*models.GiftIdea, error) {
	// 1-2. Kapsamı getir; Anti-Spoiler: hedef kişi / havuzun alıcısı öneri ekleyemez
	sc, err := s.loadScope(scope, userID)
	if err != nil {
		return nil, err
	}
	if sc.Completed() {
		return nil, sc.CompletedErr()
	}

	// 3. Üyelik kontrolü: Sadece grup üyeleri öneri yapabilir
	if _, err := s.requireMember(sc.Group.ID, userID); err != nil {
		return nil, err
	}

	if !validMoney(price) {
		return nil, ErrInvalidPrice
	}

	// search_query boş bırakılmışsa otomatik başlıktan türet
	cleanQuery := strings.TrimSpace(searchQuery)
	if cleanQuery == "" {
		cleanQuery = title
	}

	budget := sc.Budget()
	isOverBudget := budget.GreaterThan(decimal.Zero) && price.GreaterThan(budget)

	ideas := []models.GiftIdea{{
		GroupID:        sc.Group.ID,
		PoolID:         scope.PoolID,
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
	}}

	if err := s.createIdeas(sc, ideas); err != nil {
		return nil, err
	}

	return &ideas[0], nil
}

// GetGroupIdeas kapsamın hediye havuzunu skorlarıyla birlikte listeler
func (s *ideaService) GetGroupIdeas(scope repository.IdeaScope, currentUserID uuid.UUID) ([]repository.IdeaWithScore, error) {
	// 1-2. Kapsamı getir; Anti-Spoiler: hedef kişi / havuzun alıcısı hediye havuzunu asla göremez
	sc, err := s.loadScope(scope, currentUserID)
	if err != nil {
		return nil, err
	}

	// 3. Üyelik kontrolü
	if _, err := s.requireMember(sc.Group.ID, currentUserID); err != nil {
		return nil, err
	}

	ideas, err := s.ideaRepo.GetIdeasWithScores(sc.IdeaScope(), currentUserID)
	if err != nil {
		return nil, err
	}
	// An idea generation/addition may have started before a concurrent budget update.
	// Always display warnings using the budget read for this request.
	budget := sc.Budget()
	for i := range ideas {
		ideas[i].IsOverBudget = budget.GreaterThan(decimal.Zero) && ideas[i].EstimatedPrice.GreaterThan(budget)
	}
	return ideas, nil
}

// resolvedIdea /ideas/:id/* işlemlerinde doğrulanmış fikir, kapsamı, üyelik ve yönetim yetkisidir.
type resolvedIdea struct {
	idea   *models.GiftIdea
	scope  giftScope
	member *models.GroupMember
	// canManage veto ve başkasının fikrini silme yetkisidir: SINGLE grupta admin, havuzda etkin yönetici.
	canManage bool
}

// lookupIdea fikri, grubunu ve (varsa) havuzunu getirir; grubun hedef kişisi ya da havuzun
// alıcısı için ErrAntiSpoilerTriggered döner. Durum ve üyelik denetlenmez.
func (s *ideaService) lookupIdea(ideaID, userID uuid.UUID) (*models.GiftIdea, giftScope, error) {
	idea, err := s.ideaRepo.GetIdeaByID(ideaID)
	if err != nil {
		return nil, giftScope{}, err
	}
	if idea == nil {
		return nil, giftScope{}, ErrIdeaNotFound
	}
	group, err := s.groupRepo.GetGroupByID(idea.GroupID)
	if err != nil {
		return nil, giftScope{}, err
	}
	if group == nil {
		return nil, giftScope{}, ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, giftScope{}, ErrAntiSpoilerTriggered
	}
	sc := giftScope{Group: group}
	if idea.PoolID != nil {
		if s.poolRepo == nil {
			return nil, giftScope{}, ErrIdeaNotFound
		}
		pool, err := s.poolRepo.GetPool(group.ID, *idea.PoolID)
		if err != nil {
			return nil, giftScope{}, err
		}
		if pool == nil {
			return nil, giftScope{}, ErrIdeaNotFound
		}
		if pool.RecipientUserID == userID {
			return nil, giftScope{}, ErrAntiSpoilerTriggered
		}
		sc.Pool = pool
	}
	return idea, sc, nil
}

// resolveIdea /ideas/:id/* uçlarının TEK erişim denetimidir (bu uçlar grup middleware'inden
// geçmediği için Anti-Spoiler bu katmanda her işlemde denetlenir). Sıra:
//  1. fikir (yoksa ErrIdeaNotFound)
//  2. grup ve SINGLE hedef kontrolü (ErrAntiSpoilerTriggered)
//  3. fikir bir havuzdaysa havuz; çağıran alıcıysa ErrAntiSpoilerTriggered
//  4. kapsamın durumu (tamamlandıysa ErrGroupCompleted / ErrPoolCompleted)
//  5. üyelik (ErrNotGroupMember)
//  6. yönetim yetkisi: SINGLE'da admin, havuzda etkin yönetici
func (s *ideaService) resolveIdea(ideaID, userID uuid.UUID) (*resolvedIdea, error) {
	idea, sc, err := s.lookupIdea(ideaID, userID)
	if err != nil {
		return nil, err
	}
	if sc.Completed() {
		return nil, sc.CompletedErr()
	}
	member, err := s.requireMember(sc.Group.ID, userID)
	if err != nil {
		return nil, err
	}
	canManage := member.Role == "admin"
	if sc.IsPool() {
		adminID, err := s.groupRepo.GetAdminID(sc.Group.ID)
		if err != nil {
			return nil, err
		}
		canManage = canManagePool(sc.Pool, adminID, userID)
	}
	return &resolvedIdea{idea: idea, scope: sc, member: member, canManage: canManage}, nil
}

func (s *ideaService) GuardIdea(ideaID, userID uuid.UUID) error {
	if _, _, err := s.lookupIdea(ideaID, userID); errors.Is(err, ErrAntiSpoilerTriggered) {
		return err
	}
	return nil
}

// VoteIdea bir hediye fikrine oy verir (+1 veya -1) ve gerekirse demokrasi vetosunu işletir
func (s *ideaService) VoteIdea(ideaID, currentUserID uuid.UUID, voteType int8) error {
	// 1. Oy geçerli mi kontrol et
	if voteType != 1 && voteType != -1 {
		return ErrInvalidVoteType
	}

	// 2-5. Fikir, kapsam, Anti-Spoiler, durum ve üyelik
	r, err := s.resolveIdea(ideaID, currentUserID)
	if err != nil {
		return err
	}

	// 6. Oyu kaydet (Upsert) ve demokrasi vetosu: tek transaction'da, kapsam satırı kilitliyken
	// durum yeniden denetlenir; eksi oylar seçmenlerin (havuzda alıcı hariç üyeler) yarısını
	// aşarsa fikir otomatik REJECTED olur.
	err = s.ideaRepo.CastVote(r.scope.IdeaScope(), &models.Vote{
		GiftIdeaID: ideaID,
		UserID:     currentUserID,
		VoteType:   voteType,
	}, r.openGuard())
	return ideaNotFound(err)
}

// openGuard kilit altında kapsamın hâlâ açık olduğunu denetler.
func (r *resolvedIdea) openGuard() func(string) error {
	return func(status string) error {
		if status == models.GroupStatusCompleted {
			return r.scope.CompletedErr()
		}
		return nil
	}
}

// ideaNotFound fikir ya da kapsamı bu arada silindiyse kayıt bulunamadı hatasını ErrIdeaNotFound'a çevirir.
func ideaNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrIdeaNotFound
	}
	return err
}

// RejectIdea grup yöneticisinin (havuzda etkin yöneticinin) bir hediye fikrini doğrudan veto etmesini sağlar
func (s *ideaService) RejectIdea(ideaID, currentUserID uuid.UUID) error {
	r, err := s.resolveIdea(ideaID, currentUserID)
	if err != nil {
		return err
	}
	// YETKİ KONTROLÜ: Sadece grup admini (havuzda etkin yönetici) doğrudan veto edebilir!
	if !r.canManage {
		return ErrUnauthorizedVeto
	}
	return ideaNotFound(s.ideaRepo.UpdateStatus(r.scope.IdeaScope(), ideaID, "REJECTED", r.openGuard()))
}

// DeleteIdea hediye fikrini siler (sadece öneren kişi veya grup admini / havuzun etkin yöneticisi)
func (s *ideaService) DeleteIdea(ideaID, currentUserID uuid.UUID) error {
	r, err := s.resolveIdea(ideaID, currentUserID)
	if err != nil {
		return err
	}
	if r.idea.CreatedBy != currentUserID && !r.canManage {
		return ErrUnauthorizedDelete
	}
	return ideaNotFound(s.ideaRepo.DeleteIdea(r.scope.IdeaScope(), ideaID, r.openGuard()))
}

// GenerateAIIdeas Gemini AI kullanarak kapsama yeni hediye önerileri üretir ve havuza ekler
func (s *ideaService) GenerateAIIdeas(ctx context.Context, scope repository.IdeaScope, currentUserID uuid.UUID) ([]models.GiftIdea, error) {
	if s.geminiClient == nil {
		return nil, gemini.ErrAPIKeyMissing
	}

	// 1-2. Kapsamı getir; Anti-Spoiler: hedef kişi / havuzun alıcısı asla AI önerisi tetikleyemez!
	sc, err := s.loadScope(scope, currentUserID)
	if err != nil {
		return nil, err
	}
	if sc.Completed() {
		return nil, sc.CompletedErr()
	}

	// 3. Üyelik kontrolü: Sadece grup üyeleri AI önerisi tetikleyebilir
	if _, err := s.requireMember(sc.Group.ID, currentUserID); err != nil {
		return nil, err
	}

	// 4. State-Based Context: Mevcut ve reddedilmiş hediye başlıklarını DB'den al
	existingTitles, rejectedTitles, err := s.ideaRepo.GetIdeaTitlesByStatus(sc.IdeaScope())
	if err != nil {
		return nil, err
	}

	// 5. Dinamik Prompt İnşası
	prompt := s.buildAIPrompt(sc, existingTitles, rejectedTitles)

	// 6. Gemini API Çağrısı (Structured JSON)
	suggestions, err := s.geminiClient.GenerateGiftIdeas(ctx, prompt)
	if err != nil {
		return nil, err
	}

	if len(suggestions) == 0 {
		return nil, ErrNoAIIdeasGenerated
	}

	// 7. Sunucu Tarafı Bütçe & Kalite Filtresi
	budget := sc.Budget()
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
		isOverBudget := budget.GreaterThan(decimal.Zero) && item.EstimatedPrice.GreaterThan(budget)

		idea := models.GiftIdea{
			GroupID:             sc.Group.ID,
			PoolID:              scope.PoolID,
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

	// 8. Önerileri, kapsam satırı kilitliyken durumu yeniden denetleyerek topluca kaydet
	if err := s.createIdeas(sc, newIdeas); err != nil {
		return nil, err
	}

	return newIdeas, nil
}

// buildAIPrompt hedef kişi bilgileri ve state kısıtlarından dinamik prompt üretir
// Havuz kapsamında alıcının adı, havuzun ilgi alanları ve bütçesi, grubun açıklaması ve
// havuzun mevcut/reddedilmiş başlıkları kullanılır.
func (s *ideaService) buildAIPrompt(sc giftScope, existingTitles, rejectedTitles []string) string {
	var sb strings.Builder

	sb.WriteString("Sen profesyonel, yaratıcı ve zevkli bir hediye danışmanısın. ")
	sb.WriteString("Bir grup arkadaş, sevdikleri bir kişi için sürpriz hediye seçecek.\n\n")
	sb.WriteString("Pazar: Türkiye. Para birimi: TRY/TL. Hedef kişinin adı üzerinden yaş, cinsiyet veya kişilik varsayma.\n")

	sb.WriteString("HEDEF KİŞİ BİLGİLERİ:\n")
	sb.WriteString(fmt.Sprintf("- İsim: %s\n", sc.RecipientName()))

	if interests := sc.InterestsText(); strings.TrimSpace(interests) != "" {
		sb.WriteString(fmt.Sprintf("- İlgi Alanları ve Hobileri: %s\n", interests))
	} else {
		sb.WriteString("- İlgi Alanları: Belirtilmemiş (Genel, kullanışlı ve modern hediyeler öner)\n")
	}

	if strings.TrimSpace(sc.Group.Description) != "" {
		sb.WriteString(fmt.Sprintf("- Grup/Kutlama Amacı: %s\n", sc.Group.Description))
	}

	if budget := sc.Budget(); budget.GreaterThan(decimal.Zero) {
		sb.WriteString(fmt.Sprintf("- Toplam Bütçe Sınırı: %s TL\n", budget.StringFixed(2)))
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
	r, err := s.resolveIdea(ideaID, currentUserID)
	if err != nil {
		return err
	}
	return ideaNotFound(s.ideaRepo.RemoveVote(r.scope.IdeaScope(), ideaID, currentUserID, r.openGuard()))
}

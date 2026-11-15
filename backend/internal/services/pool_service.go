package services

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// MULTI grup (birden çok alıcı, alıcı başına havuz) hata tanımları
var (
	ErrWrongGroupMode         = errors.New("bu işlem bu grup türünde kullanılamaz")
	ErrMultiTargetFields      = errors.New("birden fazla kişi için kurulan grupta hedef kişi, bütçe, ilgi alanı ve tarih gönderilemez; bunlar her kişinin havuzunda ayarlanır")
	ErrPoolNotFound           = errors.New("havuz bulunamadı")
	ErrRecipientNotFound      = errors.New("bu üye hediye alacak kişiler arasında değil")
	ErrAlreadyRecipient       = errors.New("bu üye zaten hediye alacak kişiler arasında")
	ErrRecipientNotMember     = errors.New("hediye alacak kişi grubun üyesi olmalıdır")
	ErrManagerNotMember       = errors.New("havuzu yönetecek kişi grubun üyesi olmalıdır")
	ErrManagerIsRecipient     = errors.New("havuzu yönetecek kişi hediye alacak kişinin kendisi olamaz")
	ErrManagerNotAllowed      = errors.New("havuz yöneticisi yalnızca grup yöneticisinin kendi havuzu için seçilir")
	ErrManagerRequired        = errors.New("kendinizi hediye alacak kişi yaparken havuzu yönetecek başka bir üye seçmelisiniz")
	ErrUnauthorizedRecipients = errors.New("hediye alacak kişileri sadece grup yöneticisi (admin) belirleyebilir")
	ErrPoolHasIdeas           = errors.New("bu havuzda hediye fikirleri olabilir; kaldırmak için onaylayın (confirm=true)")
	ErrUnauthorizedPoolManage = errors.New("bu işlemi sadece havuzun yöneticisi yapabilir")
	ErrPoolCompleted          = errors.New("havuz tamamlandı; hediye fikirleri, oylar ve katkı değişikliği kapalı")
	ErrPoolNotCompleted       = errors.New("havuz zaten açık")
	ErrWinnerNotInPool        = errors.New("seçilen hediye önerisi bu havuza ait değil")
)

// PoolView grup detayında ve havuz listesinde dönen havuz kartıdır: çağıranın kendi havuzu
// için LockedPool, diğerleri için OpenPool. İki ayrı tip kullanılır; kilitli karta yanlışlıkla
// alan eklenemez.
type PoolView interface {
	isPoolView()
}

// LockedPool çağıranın alıcısı olduğu havuzdur. Yalnızca kimlik ve alıcı (çağıranın kendisi)
// döner; durum, tarih, bütçe ve sayaçlar bilerek yoktur (Anti-Spoiler). Manager yalnızca çağıran
// grup yöneticisiyse dolar: sorumluyu kendisi atadığı için bu bilgi ona yeni bir şey söylemez ve
// sorumlu değiştirme penceresinin mevcut seçimi göstermesini sağlar. Diğer alıcılarda alan hiç yoktur.
type LockedPool struct {
	ID        uuid.UUID       `json:"id" format:"uuid"`
	Recipient AccountSummary  `json:"recipient"`
	IsMine    bool            `json:"is_mine" example:"true"`
	Locked    bool            `json:"locked" example:"true"`
	Manager   *AccountSummary `json:"manager,omitempty"`
}

func (LockedPool) isPoolView() {}

// OpenPool çağıranın alıcısı olmadığı havuzun kartıdır.
type OpenPool struct {
	ID        uuid.UUID      `json:"id" format:"uuid"`
	Recipient AccountSummary `json:"recipient"`
	IsMine    bool           `json:"is_mine" example:"false"`
	Locked    bool           `json:"locked" example:"false"`
	// Manager havuzun etkin yöneticisidir; yoksa null (grup yöneticisinin havuzunda yönetici atanmamış).
	Manager *AccountSummary `json:"manager"`
	// CanManage çağıranın bütçe, tarih, kazanan, veto, ilgi alanı silme ve fikir silme yetkisidir.
	CanManage   bool            `json:"can_manage"`
	Status      string          `json:"status" enums:"ACTIVE,COMPLETED" example:"ACTIVE"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	EventDate   *models.Date    `json:"event_date,omitempty" swaggertype:"string" format:"date" example:"2026-12-31"`
	TotalBudget decimal.Decimal `json:"total_budget" swaggertype:"string" example:"3000"`
	// PerPersonBudget toplam bütçe / katkı veren sayısıdır; katkı veren yoksa null.
	PerPersonBudget  *decimal.Decimal `json:"per_person_budget" swaggertype:"string" example:"1000"`
	NoContributors   bool             `json:"no_contributors"`
	ContributorCount int64            `json:"contributor_count" example:"3"`
	MyContribution   bool             `json:"my_contribution" example:"true"`
	IdeaCount        int64            `json:"idea_count" example:"4"`
	PendingVotes     int64            `json:"pending_votes" example:"1"`
}

func (OpenPool) isPoolView() {}

// PoolDetail tek havuz ucunun yanıtıdır: OpenPool alanları, ilgi alanları ve alıcı dışındaki
// tüm üyelerin katkı durumu.
type PoolDetail struct {
	OpenPool
	Interests    []string                 `json:"interests"`
	Contributors []repository.Contributor `json:"contributors"`
}

// RecipientAdded POST /groups/:id/recipients yanıtıdır. Pool çağırana göre üretilir: yönetici
// kendini alıcı yaptıysa LockedPool, değilse OpenPool.
type RecipientAdded struct {
	Pool PoolView `json:"pool" swaggertype:"object"`
}

// ManagerChanged PATCH /groups/:id/recipients/:userId/manager sonucudur.
type ManagerChanged struct {
	PoolID uuid.UUID `json:"pool_id" format:"uuid"`
	// Manager yeni yönetici; Changed değer gerçekten değiştiyse true (yalnızca sunucu içi).
	Manager *uuid.UUID `json:"-"`
	Changed bool       `json:"-"`
}

// ContributionResult PUT .../contribution yanıtıdır.
type ContributionResult struct {
	Contributing     bool             `json:"contributing" example:"false"`
	ContributorCount int64            `json:"contributor_count" example:"2"`
	PerPersonBudget  *decimal.Decimal `json:"per_person_budget" swaggertype:"string" example:"1500"`
	NoContributors   bool             `json:"no_contributors"`
}

// PoolService MULTI grupların alıcı ve havuz iş kurallarıdır. Her yöntem Anti-Spoiler'ı kendisi
// de denetler (savunma derinliği): havuzun alıcısı ErrAntiSpoilerTriggered alır.
type PoolService interface {
	AddRecipient(groupID, actorID, recipientID uuid.UUID, managerID *uuid.UUID) (*RecipientAdded, *models.GiftPool, error)
	SetManager(groupID, actorID, recipientID uuid.UUID, managerID *uuid.UUID) (*ManagerChanged, error)
	RemoveRecipient(groupID, actorID, recipientID uuid.UUID, confirm bool) error
	ListPools(groupID, viewerID uuid.UUID) ([]PoolView, error)
	GetPool(groupID, poolID, viewerID uuid.UUID) (*PoolDetail, error)
	UpdateBudget(groupID, poolID, userID uuid.UUID, budget decimal.Decimal) error
	AddInterest(groupID, poolID, userID uuid.UUID, interest string) ([]string, error)
	DeleteInterest(groupID, poolID, userID uuid.UUID, interest string) ([]string, error)
	SetEventDate(groupID, poolID, userID uuid.UUID, date *models.Date) error
	SelectWinners(groupID, poolID, userID uuid.UUID, ideaIDs []uuid.UUID) (int, error)
	Reopen(groupID, poolID, userID uuid.UUID) error
	SetContribution(groupID, poolID, userID uuid.UUID, contributing bool) (*ContributionResult, error)
	// CanManage kullanıcının havuzun etkin yöneticisi olduğunu söyler (PoolAccess middleware'i için).
	CanManage(pool *models.GiftPool, userID uuid.UUID) (bool, error)
	// VisiblePoolIDs çağıranın alıcısı olmadığı havuzlardır (canlı akış anahtarları).
	VisiblePoolIDs(groupID, viewerID uuid.UUID) ([]uuid.UUID, error)
}

type poolService struct {
	groupRepo repository.GroupRepository
	poolRepo  repository.PoolRepository
}

// NewPoolService yeni bir PoolService oluşturur.
func NewPoolService(groupRepo repository.GroupRepository, poolRepo repository.PoolRepository) PoolService {
	return &poolService{groupRepo: groupRepo, poolRepo: poolRepo}
}

// poolCtx doğrulanmış bir havuz erişimidir.
type poolCtx struct {
	group     *models.Group
	pool      *models.GiftPool
	member    *models.GroupMember
	adminID   uuid.UUID
	canManage bool
}

// access sırası: grup → hedef (Anti-Spoiler) → havuz → havuzun alıcısı (Anti-Spoiler) → üyelik.
func (s *poolService) access(groupID, poolID, userID uuid.UUID) (*poolCtx, error) {
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
	pool, err := s.poolRepo.GetPool(groupID, poolID)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, ErrPoolNotFound
	}
	if pool.RecipientUserID == userID {
		return nil, ErrAntiSpoilerTriggered
	}
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}
	adminID, err := s.groupRepo.GetAdminID(groupID)
	if err != nil {
		return nil, err
	}
	return &poolCtx{group: group, pool: pool, member: member, adminID: adminID, canManage: canManagePool(pool, adminID, userID)}, nil
}

func (s *poolService) manage(groupID, poolID, userID uuid.UUID) (*poolCtx, error) {
	ctx, err := s.access(groupID, poolID, userID)
	if err != nil {
		return nil, err
	}
	if !ctx.canManage {
		return nil, ErrUnauthorizedPoolManage
	}
	return ctx, nil
}

func (s *poolService) CanManage(pool *models.GiftPool, userID uuid.UUID) (bool, error) {
	adminID, err := s.groupRepo.GetAdminID(pool.GroupID)
	if err != nil {
		return false, err
	}
	return canManagePool(pool, adminID, userID), nil
}

func (s *poolService) VisiblePoolIDs(groupID, viewerID uuid.UUID) ([]uuid.UUID, error) {
	return s.poolRepo.VisiblePoolIDs(groupID, viewerID)
}

// requireMultiAdmin grubun yöneticisi olduğunu ve grubun MULTI olduğunu denetler.
func (s *poolService) requireMultiAdmin(groupID, userID uuid.UUID) (*models.Group, error) {
	group, err := requireGroupAdmin(s.groupRepo, groupID, userID, ErrUnauthorizedRecipients)
	if err != nil {
		return nil, err
	}
	if !group.IsMulti() {
		return nil, ErrWrongGroupMode
	}
	return group, nil
}

// managerRules alıcı/yönetici kurallarıdır. Grup yöneticisi kendini alıcı yapabilir ama yalnızca
// havuzu yönetecek başka bir üyeyle; başka bir alıcı için yönetici verilmez (etkin yönetici grup
// yöneticisidir).
func managerRules(recipientIsAdmin bool, recipientID uuid.UUID, managerID *uuid.UUID) error {
	if managerID != nil && *managerID == recipientID {
		return ErrManagerIsRecipient
	}
	if !recipientIsAdmin && managerID != nil {
		return ErrManagerNotAllowed
	}
	if recipientIsAdmin && managerID == nil {
		return ErrManagerRequired
	}
	return nil
}

func (s *poolService) AddRecipient(groupID, actorID, recipientID uuid.UUID, managerID *uuid.UUID) (*RecipientAdded, *models.GiftPool, error) {
	if _, err := s.requireMultiAdmin(groupID, actorID); err != nil {
		return nil, nil, err
	}
	// Çağıran grubun yöneticisidir; alıcı yöneticiyse kendisidir.
	if err := managerRules(recipientID == actorID, recipientID, managerID); err != nil {
		return nil, nil, err
	}
	pool, err := s.poolRepo.CreatePool(groupID, recipientID, managerID, func(group *models.Group, m repository.PoolMembership) error {
		if !group.IsMulti() {
			return ErrWrongGroupMode
		}
		if !m.RecipientIsMember {
			return ErrRecipientNotMember
		}
		if m.PoolExists {
			return ErrAlreadyRecipient
		}
		if managerID != nil && !m.ManagerIsMember {
			return ErrManagerNotMember
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	views, err := s.poolViews(groupID, actorID, []models.GiftPool{*pool})
	if err != nil {
		return nil, nil, err
	}
	return &RecipientAdded{Pool: views[0]}, pool, nil
}

func (s *poolService) SetManager(groupID, actorID, recipientID uuid.UUID, managerID *uuid.UUID) (*ManagerChanged, error) {
	if _, err := s.requireMultiAdmin(groupID, actorID); err != nil {
		return nil, err
	}
	if err := managerRules(recipientID == actorID, recipientID, managerID); err != nil {
		return nil, err
	}
	var previous *uuid.UUID
	pool, err := s.poolRepo.SetManager(groupID, recipientID, managerID, func(pool *models.GiftPool, managerIsMember bool) error {
		if managerID != nil && !managerIsMember {
			return ErrManagerNotMember
		}
		previous = pool.ManagerUserID
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRecipientNotFound
	}
	if err != nil {
		return nil, err
	}
	changed := (previous == nil) != (managerID == nil) || (previous != nil && managerID != nil && *previous != *managerID)
	return &ManagerChanged{PoolID: pool.ID, Manager: managerID, Changed: changed}, nil
}

func (s *poolService) RemoveRecipient(groupID, actorID, recipientID uuid.UUID, confirm bool) error {
	if _, err := s.requireMultiAdmin(groupID, actorID); err != nil {
		return err
	}
	err := s.poolRepo.DeletePool(groupID, recipientID, func(_ *models.GiftPool, ideaCount int64) error {
		// Anti-Spoiler: çağıran havuzun alıcısıysa fikir olsun olmasın aynı 409 döner; böylece
		// onaysız bir istekle havuzunda fikir olup olmadığı anlaşılamaz.
		if !confirm && (ideaCount > 0 || recipientID == actorID) {
			return ErrPoolHasIdeas
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRecipientNotFound
	}
	return err
}

func (s *poolService) ListPools(groupID, viewerID uuid.UUID) ([]PoolView, error) {
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == viewerID {
		return nil, ErrAntiSpoilerTriggered
	}
	member, err := s.groupRepo.GetMember(groupID, viewerID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}
	if !group.IsMulti() {
		return nil, ErrWrongGroupMode
	}
	pools, err := s.poolRepo.ListPools(groupID)
	if err != nil {
		return nil, err
	}
	return s.poolViews(groupID, viewerID, pools)
}

// poolViews havuzları çağırana göre kartlara çevirir: kendi havuzu LockedPool (yalnızca kimlik
// ve alıcı; o havuz için hiçbir sayaç hesaplanmaz), diğerleri OpenPool.
func (s *poolService) poolViews(groupID, viewerID uuid.UUID, pools []models.GiftPool) ([]PoolView, error) {
	if len(pools) == 0 {
		return []PoolView{}, nil
	}
	members, err := s.groupRepo.GetMembers(groupID)
	if err != nil {
		return nil, err
	}
	return s.poolViewsWithMembers(groupID, viewerID, pools, members)
}

// poolViewsWithMembers çağıran üye listesini zaten okuduysa (grup detayı) onu yeniden kullanır.
func (s *poolService) poolViewsWithMembers(groupID, viewerID uuid.UUID, pools []models.GiftPool, members []models.GroupMember) ([]PoolView, error) {
	views := make([]PoolView, 0, len(pools))
	if len(pools) == 0 {
		return views, nil
	}
	stats, err := s.poolRepo.PoolStats(groupID, viewerID)
	if err != nil {
		return nil, err
	}
	people := make(map[uuid.UUID]AccountSummary, len(members))
	adminID := uuid.Nil
	for _, m := range members {
		people[m.UserID] = memberSummary(m.UserID, m.User)
		if m.Role == "admin" && adminID == uuid.Nil {
			adminID = m.UserID
		}
	}
	for i := range pools {
		p := &pools[i]
		recipient := memberSummary(p.RecipientUserID, p.Recipient)
		if p.RecipientUserID == viewerID {
			locked := LockedPool{ID: p.ID, Recipient: recipient, IsMine: true, Locked: true}
			if viewerID == adminID && p.ManagerUserID != nil {
				if manager, ok := people[*p.ManagerUserID]; ok {
					locked.Manager = &manager
				}
			}
			views = append(views, locked)
			continue
		}
		views = append(views, openPool(p, stats[p.ID], adminID, people, viewerID))
	}
	return views, nil
}

func openPool(p *models.GiftPool, st repository.PoolStats, adminID uuid.UUID, people map[uuid.UUID]AccountSummary, viewerID uuid.UUID) OpenPool {
	var manager *AccountSummary
	if id := effectiveManagerID(p, adminID); id != nil && *id != uuid.Nil {
		summary, ok := people[*id]
		if !ok {
			summary = AccountSummary{ID: *id}
			if p.Manager != nil && p.Manager.ID == *id {
				summary = memberSummary(*id, *p.Manager)
			}
		}
		manager = &summary
	}
	share := PerPersonShare(p.TotalBudget, st.ContributorCount)
	return OpenPool{
		ID:               p.ID,
		Recipient:        memberSummary(p.RecipientUserID, p.Recipient),
		Manager:          manager,
		CanManage:        canManagePool(p, adminID, viewerID),
		Status:           p.Status,
		CompletedAt:      p.CompletedAt,
		EventDate:        p.EventDate,
		TotalBudget:      p.TotalBudget,
		PerPersonBudget:  share,
		NoContributors:   share == nil,
		ContributorCount: st.ContributorCount,
		MyContribution:   st.MyContribution,
		IdeaCount:        st.IdeaCount,
		PendingVotes:     st.PendingVotes,
	}
}

func (s *poolService) GetPool(groupID, poolID, viewerID uuid.UUID) (*PoolDetail, error) {
	ctx, err := s.access(groupID, poolID, viewerID)
	if err != nil {
		return nil, err
	}
	views, err := s.poolViews(groupID, viewerID, []models.GiftPool{*ctx.pool})
	if err != nil {
		return nil, err
	}
	open, ok := views[0].(OpenPool)
	if !ok {
		return nil, ErrAntiSpoilerTriggered
	}
	contributors, err := s.poolRepo.ListContributors(ctx.pool.ID)
	if err != nil {
		return nil, err
	}
	return &PoolDetail{OpenPool: open, Interests: models.ParseInterests(ctx.pool.Interests), Contributors: contributors}, nil
}

func (s *poolService) UpdateBudget(groupID, poolID, userID uuid.UUID, budget decimal.Decimal) error {
	if _, err := s.manage(groupID, poolID, userID); err != nil {
		return err
	}
	if !validMoney(budget) {
		return ErrInvalidBudget
	}
	return poolNotFound(s.poolRepo.UpdateBudget(poolID, budget))
}

func (s *poolService) AddInterest(groupID, poolID, userID uuid.UUID, interest string) ([]string, error) {
	if _, err := s.access(groupID, poolID, userID); err != nil {
		return nil, err
	}
	interest, err := normalizeInterest(interest)
	if err != nil {
		return nil, err
	}
	items, err := s.poolRepo.MutateInterests(poolID, appendInterest(interest))
	return items, poolNotFound(err)
}

func (s *poolService) DeleteInterest(groupID, poolID, userID uuid.UUID, interest string) ([]string, error) {
	if _, err := s.manage(groupID, poolID, userID); err != nil {
		return nil, err
	}
	interest = strings.TrimSpace(interest)
	if interest == "" {
		return nil, ErrInvalidInterest
	}
	items, err := s.poolRepo.MutateInterests(poolID, removeInterest(interest))
	return items, poolNotFound(err)
}

func (s *poolService) SetEventDate(groupID, poolID, userID uuid.UUID, date *models.Date) error {
	if _, err := s.manage(groupID, poolID, userID); err != nil {
		return err
	}
	if date != nil && !validEventDate(*date, time.Now()) {
		return ErrInvalidEventDate
	}
	return poolNotFound(s.poolRepo.SetEventDate(poolID, date))
}

func (s *poolService) SelectWinners(groupID, poolID, userID uuid.UUID, ideaIDs []uuid.UUID) (int, error) {
	ids := uniqueIDs(ideaIDs)
	if len(ids) == 0 || len(ids) > MaxWinners {
		return 0, ErrInvalidWinners
	}
	if _, err := s.manage(groupID, poolID, userID); err != nil {
		return 0, err
	}
	err := s.poolRepo.Complete(poolID, ids, func(pool *models.GiftPool, ideas []models.GiftIdea) error {
		if pool.Status == models.GroupStatusCompleted {
			return ErrPoolCompleted
		}
		if len(ideas) != len(ids) {
			return ErrWinnerNotInPool
		}
		for _, idea := range ideas {
			if idea.Status != "ACTIVE" {
				return ErrWinnerNotActive
			}
		}
		return nil
	})
	if err != nil {
		return 0, poolNotFound(err)
	}
	return len(ids), nil
}

func (s *poolService) Reopen(groupID, poolID, userID uuid.UUID) error {
	if _, err := s.manage(groupID, poolID, userID); err != nil {
		return err
	}
	return poolNotFound(s.poolRepo.Reopen(poolID, func(pool *models.GiftPool) error {
		if pool.Status != models.GroupStatusCompleted {
			return ErrPoolNotCompleted
		}
		return nil
	}))
}

func (s *poolService) SetContribution(groupID, poolID, userID uuid.UUID, contributing bool) (*ContributionResult, error) {
	ctx, err := s.access(groupID, poolID, userID)
	if err != nil {
		return nil, err
	}
	err = s.poolRepo.SetContribution(poolID, userID, contributing, func(pool *models.GiftPool) error {
		if pool.Status == models.GroupStatusCompleted {
			return ErrPoolCompleted
		}
		return nil
	})
	if err != nil {
		return nil, poolNotFound(err)
	}
	stats, err := s.poolRepo.PoolStats(groupID, userID)
	if err != nil {
		return nil, err
	}
	st := stats[poolID]
	share := PerPersonShare(ctx.pool.TotalBudget, st.ContributorCount)
	return &ContributionResult{
		Contributing: st.MyContribution, ContributorCount: st.ContributorCount,
		PerPersonBudget: share, NoContributors: share == nil,
	}, nil
}

// poolNotFound havuz bu arada silindiyse kayıt bulunamadı hatasını ErrPoolNotFound'a çevirir.
func poolNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrPoolNotFound
	}
	return err
}

// uniqueIDs boş ve tekrar eden kimlikleri ayıklar.
func uniqueIDs(in []uuid.UUID) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(in))
	seen := map[uuid.UUID]bool{}
	for _, id := range in {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// normalizeInterest tek bir ilgi alanını kırpar ve denetler (1–100 karakter, ayırıcı yok).
func normalizeInterest(interest string) (string, error) {
	interest = strings.TrimSpace(interest)
	if interest == "" || utf8.RuneCountInString(interest) > 100 || strings.ContainsAny(interest, ",;\r\n") {
		return "", ErrInvalidInterest
	}
	return interest, nil
}

// appendInterest tekrarı (büyük/küçük harf duyarsız) reddederek listeye ekler.
func appendInterest(interest string) func([]string) ([]string, error) {
	return func(items []string) ([]string, error) {
		for _, item := range items {
			if strings.EqualFold(item, interest) {
				return nil, ErrDuplicateInterest
			}
		}
		return append(items, interest), nil
	}
}

// removeInterest listeden birebir eşleşen öğeyi çıkarır.
func removeInterest(interest string) func([]string) ([]string, error) {
	return func(items []string) ([]string, error) {
		for i, item := range items {
			if item == interest {
				return append(items[:i], items[i+1:]...), nil
			}
		}
		return nil, ErrInterestNotFound
	}
}

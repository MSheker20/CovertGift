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
)

// Standart grup hata tanımları (Sentinel Errors)
var (
	ErrGroupNotFound              = errors.New("grup bulunamadı")
	ErrNotGroupMember             = errors.New("bu grubun üyesi değilsiniz")
	ErrAntiSpoilerTriggered       = errors.New("Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir.")
	ErrCannotAddTargetAsMember    = errors.New("hediye alınacak hedef kişi gruba üye eklenemez")
	ErrAlreadyGroupMember         = errors.New("bu kullanıcı zaten grubun üyesidir")
	ErrSelfTarget                 = errors.New("kendiniz için sürpriz hediye grubu oluşturamazsınız")
	ErrUnauthorizedGroupDelete    = errors.New("bu grubu sadece grup yöneticisi (admin) silebilir")
	ErrUnauthorizedInterestDelete = errors.New("ilgi alanlarını sadece grup yöneticisi (admin) silebilir")
	ErrInvalidInterest            = errors.New("tek bir ilgi alanı girin: 1–100 karakter, virgül, noktalı virgül veya satır sonu kullanmadan")
	ErrDuplicateInterest          = errors.New("bu ilgi alanı zaten listede")
	ErrInterestNotFound           = errors.New("ilgi alanı bulunamadı")
	ErrUnauthorizedBudgetUpdate   = errors.New("bütçeyi sadece grup yöneticisi (admin) güncelleyebilir")
	ErrInvalidBudget              = errors.New("bütçe 0–99.999.999,99 ₺ arasında ve en fazla iki ondalık basamaklı olmalıdır")
	ErrTargetUserNotFound         = errors.New("hedef kişinin e-postası sistemde kayıtlı değil")
	ErrTargetInputConflict        = errors.New("hedef kişi için target_email veya target_user_id alanlarından yalnızca biri gönderilebilir")
	ErrUnauthorizedTargetUpdate   = errors.New("hedef kişiyi sadece grup yöneticisi (admin) bağlayabilir")
	ErrTargetAlreadyLinked        = errors.New("bu grubun hedef kişisi zaten bağlı; değiştirilemez")
	ErrTargetIsMember             = errors.New("grubun üyesi hedef kişi olarak bağlanamaz")
	ErrGroupCompleted             = errors.New("grup tamamlandı; hediye fikirleri ve oylar kapalı")
	ErrGroupNotCompleted          = errors.New("grup zaten açık")
	ErrUnauthorizedWinner         = errors.New("kazananı seçme ve grubu yeniden açma yetkisi sadece grup yöneticisindedir (admin)")
	ErrWinnerNotInGroup           = errors.New("seçilen hediye önerisi bu gruba ait değil")
	ErrWinnerNotActive            = errors.New("yalnızca aktif (reddedilmemiş) öneriler kazanan seçilebilir")
	ErrInvalidWinners             = errors.New("en az 1, en fazla 20 farklı öneri seçilmelidir")
	ErrUnauthorizedEventDate      = errors.New("hediye tarihini sadece grup yöneticisi (admin) değiştirebilir")
	ErrInvalidEventDate           = errors.New("hediye tarihi bugünden en fazla 1 yıl önce, en fazla 5 yıl sonra olabilir")
	ErrUnauthorizedCoverChange    = errors.New("bu grupta kapak fotoğrafını sadece grup yöneticisi (admin) değiştirebilir")
	ErrUnauthorizedCoverPolicy    = errors.New("kapak fotoğrafı iznini sadece grup yöneticisi (admin) değiştirebilir")
	ErrCoverNotFound              = errors.New("bu grubun kapak fotoğrafı yok")
	ErrUnauthorizedDescription    = errors.New("grup açıklamasını sadece grup yöneticisi (admin) değiştirebilir")
	ErrInvalidDescription         = errors.New("grup açıklaması en fazla 1000 karakter olabilir")
)

// MaxGroupDescription grup açıklamasının (baştaki/sondaki boşluklar atıldıktan sonra) en
// fazla karakter (rune) sayısıdır. Oluşturma ve düzenleme aynı sınırı kullanır.
const MaxGroupDescription = 1000

// normalizeDescription açıklamayı kırpar ve uzunluğunu denetler. Boş sonuç açıklamayı siler.
func normalizeDescription(desc string) (string, error) {
	desc = strings.TrimSpace(desc)
	if utf8.RuneCountInString(desc) > MaxGroupDescription {
		return "", ErrInvalidDescription
	}
	return desc, nil
}

// AccountSummary bir kullanıcı hesabının yalnızca kimliği ve adıdır. E-posta, şifre özeti
// veya avatar taşımaz; hedef kişinin hesabını üyelere göstermek için kullanılır.
type AccountSummary struct {
	ID   uuid.UUID `json:"id" format:"uuid"`
	Name string    `json:"name" example:"Ahmet Yılmaz"`
}

// GroupDetailResponse grup detay uç noktasının döneceği zenginleştirilmiş veri yapısıdır
type GroupDetailResponse struct {
	Group           *models.Group        `json:"group"`
	MemberCount     int64                `json:"member_count"`
	PerPersonBudget decimal.Decimal      `json:"per_person_budget"`
	Members         []models.GroupMember `json:"members,omitempty"`
	MyRole          string               `json:"my_role"`
	Interests       []string             `json:"interests"`
	// Hedef kişinin bağlı hesabı (yalnızca id + ad). Hedef bağlı değilse yer almaz. Bu yanıtı
	// yalnızca üyeler alır; hedef kişinin kendisi Anti-Spoiler ile 403 alır.
	TargetAccount *AccountSummary `json:"target_account,omitempty"`
}

// GroupService grup iş mantığını tanımlayan arayüzdür
type GroupService interface {
	CreateGroup(creatorID uuid.UUID, title, desc, targetPersonName, targetInterests string, targetUserID *uuid.UUID, targetEmail string, totalBudget decimal.Decimal, eventDate *models.Date) (*models.Group, error)
	SelectWinners(groupID, userID uuid.UUID, ideaIDs []uuid.UUID) error
	ReopenGroup(groupID, userID uuid.UUID) error
	SetEventDate(groupID, userID uuid.UUID, date *models.Date) error
	GetUserGroups(userID uuid.UUID) ([]GroupListItem, error)
	GetGroupDetail(groupID, userID uuid.UUID) (*GroupDetailResponse, error)
	AddMemberByEmail(groupID, currentUserID uuid.UUID, email string) (*models.GroupMember, error)
	AddInterest(groupID, userID uuid.UUID, interest string) ([]string, error)
	DeleteInterest(groupID, userID uuid.UUID, interest string) ([]string, error)
	UpdateBudget(groupID, userID uuid.UUID, budget decimal.Decimal) error
	LinkTarget(groupID, userID uuid.UUID, email string) (*models.Group, error)
	DeleteGroup(groupID, userID uuid.UUID) error
	SetCover(groupID, userID uuid.UUID, raw []byte) (*models.GroupCover, error)
	DeleteCover(groupID, userID uuid.UUID) error
	GetCover(groupID, userID uuid.UUID) (*models.GroupCover, error)
	SetCoverAdminOnly(groupID, userID uuid.UUID, adminOnly bool) error
	UpdateDescription(groupID, userID uuid.UUID, description string) (*models.Group, error)
}

type groupService struct {
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
	// coverSlots aynı anda işlenen kapak görseli sayısını sınırlar (bellek koruması).
	coverSlots chan struct{}
}

// NewGroupService yeni bir GroupService örneği oluşturur
func NewGroupService(groupRepo repository.GroupRepository, userRepo repository.UserRepository) GroupService {
	return &groupService{
		groupRepo:  groupRepo,
		userRepo:   userRepo,
		coverSlots: make(chan struct{}, maxConcurrentCoverJobs),
	}
}

// CreateGroup yeni grup oluşturur ve oluşturan kişiyi admin üye yapar.
// Hedef kişi isteğe bağlı olarak e-postayla seçilir; kimlik sunucuda çözülür.
func (s *groupService) CreateGroup(creatorID uuid.UUID, title, desc, targetPersonName, targetInterests string, targetUserID *uuid.UUID, targetEmail string, totalBudget decimal.Decimal, eventDate *models.Date) (*models.Group, error) {
	if !validMoney(totalBudget) {
		return nil, ErrInvalidBudget
	}
	if eventDate != nil && !validEventDate(*eventDate, time.Now()) {
		return nil, ErrInvalidEventDate
	}
	desc, err := normalizeDescription(desc)
	if err != nil {
		return nil, err
	}
	targetEmail = strings.TrimSpace(targetEmail)
	if targetEmail != "" {
		if targetUserID != nil {
			return nil, ErrTargetInputConflict
		}
		targetUser, err := s.userRepo.FindByEmail(targetEmail)
		if err != nil {
			return nil, err
		}
		if targetUser == nil {
			return nil, ErrTargetUserNotFound
		}
		targetUserID = &targetUser.ID
	}

	// Mantık kontrolü: Kullanıcı kendine sürpriz grubu açamaz
	if targetUserID != nil && *targetUserID == creatorID {
		return nil, ErrSelfTarget
	}

	group := &models.Group{
		Title:            title,
		Description:      desc,
		TargetPersonName: targetPersonName,
		TargetInterests:  targetInterests,
		TargetUserID:     targetUserID,
		TotalBudget:      totalBudget,
		EventDate:        eventDate,
		Status:           models.GroupStatusActive,
		CreatorID:        creatorID,
	}

	if err := s.groupRepo.CreateGroupWithAdmin(group, creatorID); err != nil {
		return nil, err
	}

	return group, nil
}

// GroupListItem grup listesindeki tek satırdır: grup alanları ve ana ekran sayaçları.
// Sayaçlar veritabanı sütunu değildir; şema etkilenmez.
type GroupListItem struct {
	models.Group
	MemberCount int64 `json:"member_count" example:"4"`
	// Reddedilmemiş fikir sayısı.
	IdeaCount int64 `json:"idea_count" example:"6"`
	// Çağıran kullanıcıya özel: açık gruptaki, kendisinin eklemediği ve henüz oy vermediği aktif fikirler.
	PendingVotes int64 `json:"pending_votes" example:"2"`
}

// GetUserGroups kullanıcının üye olduğu grupları sayaçlarıyla listeler (hedef olduğu gruplar filtrelenir).
// Sayaçlar tüm gruplar için tek ek sorguyla gelir.
func (s *groupService) GetUserGroups(userID uuid.UUID) ([]GroupListItem, error) {
	groups, err := s.groupRepo.GetUserGroups(userID)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, len(groups))
	for i, g := range groups {
		ids[i] = g.ID
	}
	stats, err := s.groupRepo.GetGroupListStats(userID, ids)
	if err != nil {
		return nil, err
	}

	items := make([]GroupListItem, len(groups))
	for i, g := range groups {
		st := stats[g.ID]
		items[i] = GroupListItem{Group: g, MemberCount: st.MemberCount, IdeaCount: st.IdeaCount, PendingVotes: st.PendingVotes}
	}
	return items, nil
}

// GetGroupDetail Anti-Spoiler kontrolünü yapar ve kişi başı bütçeyi anlık hesaplar
func (s *groupService) GetGroupDetail(groupID, userID uuid.UUID) (*GroupDetailResponse, error) {
	// 1. Grubu getir
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}

	// 2. Anti-Spoiler Kontrolü: İstek atan kişi hedef kişi ise anında engelle!
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, ErrAntiSpoilerTriggered
	}

	// 3. Kullanıcı bu grubun üyesi mi?
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}

	// 4. Üye sayısını ve üyeleri çek
	memberCount, err := s.groupRepo.GetMemberCount(groupID)
	if err != nil {
		return nil, err
	}

	members, err := s.groupRepo.GetMembers(groupID)
	if err != nil {
		return nil, err
	}

	// 5. Dinamik Kişi Başı Bütçe Hesabı (0'a bölme korumalı)
	perPersonBudget := decimal.Zero
	if memberCount > 0 {
		perPersonBudget = group.TotalBudget.Div(decimal.NewFromInt(memberCount)).Round(2)
	}

	return &GroupDetailResponse{
		Group:           group,
		MemberCount:     memberCount,
		PerPersonBudget: perPersonBudget,
		Members:         members,
		MyRole:          member.Role,
		Interests:       models.ParseInterests(group.TargetInterests),
		TargetAccount:   targetAccount(group),
	}, nil
}

// targetAccount bağlı hedef hesabın yalnızca kimliğini ve adını döndürür. Hesap yüklenmediyse
// (ör. silinmiş) nil döner; istemci target_person_name'e düşer.
func targetAccount(group *models.Group) *AccountSummary {
	if group.TargetUserID == nil || group.TargetUser == nil {
		return nil
	}
	return &AccountSummary{ID: group.TargetUser.ID, Name: group.TargetUser.Name}
}

// UpdateDescription grup açıklamasını değiştirir (yalnızca admin). Metin kırpılır; boş
// değer açıklamayı siler. Güncel grubu döndürür.
func (s *groupService) UpdateDescription(groupID, userID uuid.UUID, description string) (*models.Group, error) {
	if _, err := s.requireAdmin(groupID, userID, ErrUnauthorizedDescription); err != nil {
		return nil, err
	}
	desc, err := normalizeDescription(description)
	if err != nil {
		return nil, err
	}
	if err := s.groupRepo.SetDescription(groupID, desc); err != nil {
		return nil, err
	}
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

// AddMemberByEmail gruba e-posta adresiyle yeni üye ekler
func (s *groupService) AddMemberByEmail(groupID, currentUserID uuid.UUID, email string) (*models.GroupMember, error) {
	// 1. Grubu getir
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}

	// 2. Eklemek isteyen kişi grupta mı kontrol et
	currentMember, err := s.groupRepo.GetMember(groupID, currentUserID)
	if err != nil {
		return nil, err
	}
	if currentMember == nil {
		return nil, ErrNotGroupMember
	}

	// 3. Eklenecek kullanıcıyı e-postasından bul
	targetUser, err := s.userRepo.FindByEmail(email)
	if err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, ErrUserNotFound
	}

	// 4. Anti-Spoiler: Eklenecek kişi bu grubun hedef kişisi mi?
	if group.TargetUserID != nil && *group.TargetUserID == targetUser.ID {
		return nil, ErrCannotAddTargetAsMember
	}

	// 5. Bu kullanıcı zaten gruba üye mi?
	existingMember, err := s.groupRepo.GetMember(groupID, targetUser.ID)
	if err != nil {
		return nil, err
	}
	if existingMember != nil {
		return nil, ErrAlreadyGroupMember
	}

	// 6. Üyeyi kaydet
	newMember := &models.GroupMember{
		GroupID: groupID,
		UserID:  targetUser.ID,
		Role:    "member",
	}

	if err := s.groupRepo.AddMember(newMember); err != nil {
		if errors.Is(err, repository.ErrMemberIsGroupTarget) {
			return nil, ErrCannotAddTargetAsMember
		}
		if errors.Is(err, repository.ErrMemberExists) {
			return nil, ErrAlreadyGroupMember
		}
		return nil, err
	}

	newMember.User = *targetUser
	return newMember, nil
}

// checkInterestAccess preserves membership and Anti-Spoiler for both operations.
func (s *groupService) checkInterestAccess(groupID, userID uuid.UUID, adminOnly bool) error {
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return ErrAntiSpoilerTriggered
	}
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}
	if adminOnly && member.Role != "admin" {
		return ErrUnauthorizedInterestDelete
	}
	return nil
}

// AddInterest appends one item. Members cannot replace or clear the existing list.
func (s *groupService) AddInterest(groupID, userID uuid.UUID, interest string) ([]string, error) {
	if err := s.checkInterestAccess(groupID, userID, false); err != nil {
		return nil, err
	}
	interest = strings.TrimSpace(interest)
	if interest == "" || utf8.RuneCountInString(interest) > 100 || strings.ContainsAny(interest, ",;\r\n") {
		return nil, ErrInvalidInterest
	}
	return s.groupRepo.MutateInterests(groupID, func(items []string) ([]string, error) {
		for _, item := range items {
			if strings.EqualFold(item, interest) {
				return nil, ErrDuplicateInterest
			}
		}
		return append(items, interest), nil
	})
}

// DeleteInterest removes one existing item, and is restricted to group admins.
func (s *groupService) DeleteInterest(groupID, userID uuid.UUID, interest string) ([]string, error) {
	if err := s.checkInterestAccess(groupID, userID, true); err != nil {
		return nil, err
	}
	interest = strings.TrimSpace(interest)
	if interest == "" {
		return nil, ErrInvalidInterest
	}
	return s.groupRepo.MutateInterests(groupID, func(items []string) ([]string, error) {
		for i, item := range items {
			if item == interest {
				return append(items[:i], items[i+1:]...), nil
			}
		}
		return nil, ErrInterestNotFound
	})
}

// UpdateBudget is restricted to admins and validates the database's money range.
func (s *groupService) UpdateBudget(groupID, userID uuid.UUID, budget decimal.Decimal) error {
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return ErrAntiSpoilerTriggered
	}
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}
	if member.Role != "admin" {
		return ErrUnauthorizedBudgetUpdate
	}
	if !validMoney(budget) {
		return ErrInvalidBudget
	}
	return s.groupRepo.UpdateBudget(groupID, budget)
}

// LinkTarget mevcut gruba hedef kişiyi e-postayla bağlar (yalnızca admin, yalnızca bir kez).
// Hedef bağlandıktan sonra Anti-Spoiler o kişiyi gruptan dışlar.
func (s *groupService) LinkTarget(groupID, userID uuid.UUID, email string) (*models.Group, error) {
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
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}
	if member.Role != "admin" {
		return nil, ErrUnauthorizedTargetUpdate
	}

	targetUser, err := s.userRepo.FindByEmail(strings.TrimSpace(email))
	if err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, ErrTargetUserNotFound
	}
	if targetUser.ID == userID {
		return nil, ErrSelfTarget
	}

	// Kontroller satır kilidi altında tekrarlanır; eşzamanlı bağlama veya üye ekleme yarışı kazanamaz.
	err = s.groupRepo.SetTargetUser(groupID, targetUser.ID, func(current *uuid.UUID, isMember bool) error {
		if current != nil {
			return ErrTargetAlreadyLinked
		}
		if isMember {
			return ErrTargetIsMember
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	group.TargetUserID = &targetUser.ID
	group.TargetUser = targetUser
	return group, nil
}

// DeleteGroup grubu ve tüm ilişkili verilerini siler (sadece grup kurucusu / admin üye silebilir)
func (s *groupService) DeleteGroup(groupID, userID uuid.UUID) error {
	// 1. Grubu getir
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return err
	}
	if group == nil {
		return ErrGroupNotFound
	}

	// 2. Anti-Spoiler: Hedef kişi grubu silemez
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return ErrAntiSpoilerTriggered
	}

	// 3. Yetki kontrolü: İstek atan kullanıcı grupta mı ve admin rolünde mi (veya Creator mı)?
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotGroupMember
	}

	if member.Role != "admin" && group.CreatorID != userID {
		return ErrUnauthorizedGroupDelete
	}

	// 4. Grubu sil
	return s.groupRepo.DeleteGroup(groupID)
}

// validMoney numeric(10,2) sütununa sığan, negatif olmayan ve en fazla iki ondalık
// basamaklı tutarları kabul eder (0 – 99.999.999,99).
func validMoney(amount decimal.Decimal) bool {
	return !amount.IsNegative() && !amount.GreaterThan(decimal.New(9999999999, -2)) && amount.Equal(amount.Round(2))
}

// turkeyTZ hediye tarihi doğrulamasında "bugün"ü belirler. Türkiye 2016'dan beri yaz
// saati uygulamaz; sabit UTC+3 kullanmak konteynerde tz veritabanına bağımlılığı kaldırır.
var turkeyTZ = time.FixedZone("TRT", 3*60*60)

// validEventDate tarihin bugünden en fazla 1 yıl önce, en fazla 5 yıl sonra olduğunu denetler.
// Geçmiş tarih kabul edilir: grup doğum gününden sonra da kurulabilir veya kapatılmadan kalabilir.
func validEventDate(d models.Date, now time.Time) bool {
	n := now.In(turkeyTZ)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return !d.Before(today.AddDate(-1, 0, 0)) && !d.After(today.AddDate(5, 0, 0))
}

// requireAdmin grubu getirir; Anti-Spoiler, üyelik ve admin rolünü denetler.
func (s *groupService) requireAdmin(groupID, userID uuid.UUID, notAdmin error) (*models.Group, error) {
	return requireGroupAdmin(s.groupRepo, groupID, userID, notAdmin)
}

// requireGroupAdmin grubun var olduğunu, çağıranın hedef kişi olmadığını (Anti-Spoiler) ve
// grubun admini olduğunu denetler. Grup servisi ve davet servisi aynı kuralı paylaşır.
func requireGroupAdmin(groupRepo repository.GroupRepository, groupID, userID uuid.UUID, notAdmin error) (*models.Group, error) {
	group, err := groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, ErrAntiSpoilerTriggered
	}
	member, err := groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}
	if member.Role != "admin" {
		return nil, notAdmin
	}
	return group, nil
}

// MaxWinners bir grupta aynı anda seçilebilecek en fazla kazanan sayısıdır.
const MaxWinners = 20

// SelectWinners admin'in seçtiği bir ya da daha fazla aktif fikri kazanan yapar ve grubu
// tamamlar. Tamamlanan grupta fikir ekleme, AI önerisi, oy, veto ve fikir silme kapanır.
func (s *groupService) SelectWinners(groupID, userID uuid.UUID, ideaIDs []uuid.UUID) error {
	ids := make([]uuid.UUID, 0, len(ideaIDs))
	seen := map[uuid.UUID]bool{}
	for _, id := range ideaIDs {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 || len(ids) > MaxWinners {
		return ErrInvalidWinners
	}
	if _, err := s.requireAdmin(groupID, userID, ErrUnauthorizedWinner); err != nil {
		return err
	}
	return s.groupRepo.CompleteGroup(groupID, ids, func(group *models.Group, ideas []models.GiftIdea) error {
		if group.Status == models.GroupStatusCompleted {
			return ErrGroupCompleted
		}
		if len(ideas) != len(ids) {
			return ErrWinnerNotInGroup
		}
		for _, idea := range ideas {
			if idea.Status != "ACTIVE" {
				return ErrWinnerNotActive
			}
		}
		return nil
	})
}

// ReopenGroup yanlış seçimi geri almak için grubu yeniden açar; tüm kazanan fikirler aktife döner.
func (s *groupService) ReopenGroup(groupID, userID uuid.UUID) error {
	if _, err := s.requireAdmin(groupID, userID, ErrUnauthorizedWinner); err != nil {
		return err
	}
	return s.groupRepo.ReopenGroup(groupID, func(group *models.Group) error {
		if group.Status != models.GroupStatusCompleted {
			return ErrGroupNotCompleted
		}
		return nil
	})
}

// SetEventDate hediye tarihini ayarlar veya (nil ile) kaldırır. Yalnızca admin.
func (s *groupService) SetEventDate(groupID, userID uuid.UUID, date *models.Date) error {
	if _, err := s.requireAdmin(groupID, userID, ErrUnauthorizedEventDate); err != nil {
		return err
	}
	if date != nil && !validEventDate(*date, time.Now()) {
		return ErrInvalidEventDate
	}
	return s.groupRepo.SetEventDate(groupID, date)
}

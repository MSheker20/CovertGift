package services

import (
	"errors"
	"strings"
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
)

// GroupDetailResponse grup detay uç noktasının döneceği zenginleştirilmiş veri yapısıdır
type GroupDetailResponse struct {
	Group           *models.Group        `json:"group"`
	MemberCount     int64                `json:"member_count"`
	PerPersonBudget decimal.Decimal      `json:"per_person_budget"`
	Members         []models.GroupMember `json:"members,omitempty"`
	MyRole          string               `json:"my_role"`
	Interests       []string             `json:"interests"`
}

// GroupService grup iş mantığını tanımlayan arayüzdür
type GroupService interface {
	CreateGroup(creatorID uuid.UUID, title, desc, targetPersonName, targetInterests string, targetUserID *uuid.UUID, totalBudget decimal.Decimal) (*models.Group, error)
	GetUserGroups(userID uuid.UUID) ([]models.Group, error)
	GetGroupDetail(groupID, userID uuid.UUID) (*GroupDetailResponse, error)
	AddMemberByEmail(groupID, currentUserID uuid.UUID, email string) (*models.GroupMember, error)
	AddInterest(groupID, userID uuid.UUID, interest string) ([]string, error)
	DeleteInterest(groupID, userID uuid.UUID, interest string) ([]string, error)
	UpdateBudget(groupID, userID uuid.UUID, budget decimal.Decimal) error
	DeleteGroup(groupID, userID uuid.UUID) error
}

type groupService struct {
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
}

// NewGroupService yeni bir GroupService örneği oluşturur
func NewGroupService(groupRepo repository.GroupRepository, userRepo repository.UserRepository) GroupService {
	return &groupService{
		groupRepo: groupRepo,
		userRepo:  userRepo,
	}
}

// CreateGroup yeni grup oluşturur ve oluşturan kişiyi admin üye yapar
func (s *groupService) CreateGroup(creatorID uuid.UUID, title, desc, targetPersonName, targetInterests string, targetUserID *uuid.UUID, totalBudget decimal.Decimal) (*models.Group, error) {
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
		CreatorID:        creatorID,
	}

	if err := s.groupRepo.CreateGroupWithAdmin(group, creatorID); err != nil {
		return nil, err
	}

	return group, nil
}

// GetUserGroups kullanıcının üye olduğu grupları listeler (hedef olduğu gruplar filtrelenir)
func (s *groupService) GetUserGroups(userID uuid.UUID) ([]models.Group, error) {
	return s.groupRepo.GetUserGroups(userID)
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
	}, nil
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
	if budget.IsNegative() || budget.GreaterThan(decimal.New(9999999999, -2)) || !budget.Equal(budget.Round(2)) {
		return ErrInvalidBudget
	}
	return s.groupRepo.UpdateBudget(groupID, budget)
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

package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

// Davet linki hataları. Bilinmeyen, süresi dolmuş, iptal edilmiş ve biçimi bozuk token için
// tek hata (ErrInviteNotFound) döner; hangisi olduğu sızdırılmaz.
var (
	ErrInviteNotFound       = errors.New("davet linki geçersiz veya süresi dolmuş")
	ErrUnauthorizedInvite   = errors.New("davet linkini sadece grup yöneticisi (admin) yönetebilir")
	ErrInvalidInviteExpiry  = errors.New("davet linki süresi 1, 7 veya 30 gün olmalıdır")
	inviteTokenPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	allowedInviteExpiryDays = map[int]bool{1: true, 7: true, 30: true}
)

// inviteTokenBytes ham davet token'ının rastgele bayt sayısıdır (base64url ile 43 karakter).
const inviteTokenBytes = 32

// InviteStatus GET /groups/:id/invite yanıtıdır. Token asla dönmez.
type InviteStatus struct {
	Active    bool       `json:"active" example:"true"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// InviteCreated POST /groups/:id/invite yanıtıdır; ham token yalnızca burada, bir kez döner.
type InviteCreated struct {
	Token     string    `json:"token" example:"3q2-7wEAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`
	ExpiresAt time.Time `json:"expires_at"`
}

// InvitePreview GET /invites/:token yanıtıdır.
type InvitePreview struct {
	// GroupID yalnızca çağıran zaten üyeyse döner; katılınca accept yanıtı kimliği verir.
	GroupID       *uuid.UUID `json:"group_id,omitempty" format:"uuid"`
	GroupTitle    string     `json:"group_title" example:"Ahmet'in 30. Yaş Sürprizi"`
	InviterName   string     `json:"inviter_name" example:"Zeynep"`
	MemberCount   int64      `json:"member_count" example:"4"`
	AlreadyMember bool       `json:"already_member" example:"false"`
}

// InviteAccepted POST /invites/:token/accept sonucudur. Joined ve MemberName yanıta yazılmaz;
// controller bildirim üretmek için kullanır.
type InviteAccepted struct {
	GroupID    uuid.UUID `json:"group_id" format:"uuid"`
	Joined     bool      `json:"-"`
	MemberName string    `json:"-"`
}

// InviteService davet linki iş mantığıdır.
type InviteService interface {
	GetStatus(groupID, userID uuid.UUID) (*InviteStatus, error)
	Create(groupID, userID uuid.UUID, expiresInDays int) (*InviteCreated, error)
	Revoke(groupID, userID uuid.UUID) error
	Preview(token string, userID uuid.UUID) (*InvitePreview, error)
	Accept(token string, userID uuid.UUID) (*InviteAccepted, error)
}

type inviteService struct {
	inviteRepo repository.InviteRepository
	groupRepo  repository.GroupRepository
	userRepo   repository.UserRepository
	now        func() time.Time
}

// NewInviteService yeni bir InviteService oluşturur.
func NewInviteService(inviteRepo repository.InviteRepository, groupRepo repository.GroupRepository, userRepo repository.UserRepository) InviteService {
	return &inviteService{inviteRepo: inviteRepo, groupRepo: groupRepo, userRepo: userRepo, now: time.Now}
}

// hashInviteToken ham token'ın hex SHA-256 özetini döndürür (veritabanında saklanan değer).
func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *inviteService) GetStatus(groupID, userID uuid.UUID) (*InviteStatus, error) {
	if _, err := requireGroupAdmin(s.groupRepo, groupID, userID, ErrUnauthorizedInvite); err != nil {
		return nil, err
	}
	invite, err := s.inviteRepo.GetActive(groupID, s.now())
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return &InviteStatus{Active: false}, nil
	}
	return &InviteStatus{Active: true, ExpiresAt: &invite.ExpiresAt, CreatedAt: &invite.CreatedAt}, nil
}

func (s *inviteService) Create(groupID, userID uuid.UUID, expiresInDays int) (*InviteCreated, error) {
	if !allowedInviteExpiryDays[expiresInDays] {
		return nil, ErrInvalidInviteExpiry
	}
	if _, err := requireGroupAdmin(s.groupRepo, groupID, userID, ErrUnauthorizedInvite); err != nil {
		return nil, err
	}
	raw := make([]byte, inviteTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.now()
	invite := &models.GroupInvite{
		GroupID:   groupID,
		TokenHash: hashInviteToken(token),
		CreatedBy: userID,
		ExpiresAt: now.Add(time.Duration(expiresInDays) * 24 * time.Hour),
		CreatedAt: now,
	}
	if err := s.inviteRepo.Replace(invite, now); err != nil {
		return nil, err
	}
	return &InviteCreated{Token: token, ExpiresAt: invite.ExpiresAt}, nil
}

func (s *inviteService) Revoke(groupID, userID uuid.UUID) error {
	if _, err := requireGroupAdmin(s.groupRepo, groupID, userID, ErrUnauthorizedInvite); err != nil {
		return err
	}
	return s.inviteRepo.RevokeActive(groupID, s.now())
}

func (s *inviteService) Preview(token string, userID uuid.UUID) (*InvitePreview, error) {
	if !inviteTokenPattern.MatchString(token) {
		return nil, ErrInviteNotFound
	}
	invite, err := s.inviteRepo.FindActiveByHash(hashInviteToken(token), s.now())
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return nil, ErrInviteNotFound
	}
	group, err := s.groupRepo.GetGroupByID(invite.GroupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrInviteNotFound
	}
	// Anti-Spoiler: grubun hakkında hiçbir şey döndürülmeden önce denetlenir.
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, ErrAntiSpoilerTriggered
	}
	inviter, err := s.userRepo.FindByID(invite.CreatedBy)
	if err != nil {
		return nil, err
	}
	inviterName := ""
	if inviter != nil {
		inviterName = inviter.Name
	}
	count, err := s.groupRepo.GetMemberCount(group.ID)
	if err != nil {
		return nil, err
	}
	member, err := s.groupRepo.GetMember(group.ID, userID)
	if err != nil {
		return nil, err
	}
	preview := &InvitePreview{
		GroupTitle: group.Title, InviterName: inviterName,
		MemberCount: count, AlreadyMember: member != nil,
	}
	if member != nil {
		preview.GroupID = &group.ID
	}
	return preview, nil
}

func (s *inviteService) Accept(token string, userID uuid.UUID) (*InviteAccepted, error) {
	if !inviteTokenPattern.MatchString(token) {
		return nil, ErrInviteNotFound
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	res, err := s.inviteRepo.Join(hashInviteToken(token), userID, s.now())
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInviteInactive):
			return nil, ErrInviteNotFound
		case errors.Is(err, repository.ErrMemberIsGroupTarget):
			return nil, ErrAntiSpoilerTriggered
		}
		return nil, err
	}
	return &InviteAccepted{GroupID: res.GroupID, Joined: res.Joined, MemberName: user.Name}, nil
}

package services

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

// Arkadaşlık ve grup daveti hataları.
var (
	ErrSelfFriend               = errors.New("kendine arkadaşlık isteği gönderemezsin")
	ErrFriendshipExists         = errors.New("bu kişiyle zaten arkadaşsın ya da bekleyen bir istek var")
	ErrFriendRequestMissing     = errors.New("arkadaşlık isteği bulunamadı")
	ErrFriendNotFound           = errors.New("arkadaş bulunamadı")
	ErrFriendLinkNotFound       = errors.New("arkadaşlık linki geçersiz")
	ErrNotFriends               = errors.New("yalnızca arkadaşlarını davet edebilirsin")
	ErrTooManyInvitees          = errors.New("tek seferde en fazla 30 kişi davet edilebilir")
	ErrInviteeIsTarget          = errors.New("hediye alınacak kişi gruba davet edilemez")
	ErrUnauthorizedMemberInvite = errors.New("davetleri sadece grup yöneticisi (admin) yönetebilir")
	ErrMemberInviteNotFound     = errors.New("davet bulunamadı")
	friendCodePattern           = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)
)

// MaxInvitees tek istekte davet edilebilecek en fazla kişi sayısıdır.
const MaxInvitees = 30

// friendCodeBytes kişisel link kodunun rastgele bayt sayısıdır (128 bit, base64url ile 22 karakter).
const friendCodeBytes = 16

// FriendView arkadaş listesindeki bir kişidir. E-posta dönmez.
type FriendView struct {
	// FriendshipID istek kabul/ret/geri çekme uçlarında kullanılır.
	FriendshipID  uuid.UUID  `json:"friendship_id" format:"uuid"`
	ID            uuid.UUID  `json:"id" format:"uuid"`
	Name          string     `json:"name" example:"Zeynep Kaya"`
	AvatarVersion string     `json:"avatar_version,omitempty"`
	Since         time.Time  `json:"since"`
	AcceptedAt    *time.Time `json:"accepted_at,omitempty"`
}

// FriendList GET /friends yanıtıdır.
type FriendList struct {
	Friends  []FriendView `json:"friends"`
	Incoming []FriendView `json:"incoming"`
	Outgoing []FriendView `json:"outgoing"`
}

// FriendRequestResult POST /friends/requests yanıtıdır.
type FriendRequestResult struct {
	// Status: PENDING (istek gönderildi) ya da ACCEPTED (karşı taraf zaten istek göndermişti).
	Status string `json:"status" enums:"PENDING,ACCEPTED" example:"PENDING"`
	// UserID işlemden etkilenen karşı taraftır; yanıta yazılmaz (bildirim akışı için).
	UserID uuid.UUID `json:"-"`
}

// FriendLinkView kullanıcının kendi kişisel linkidir.
type FriendLinkView struct {
	Code string `json:"code" example:"q9Zt3bW1xYk2LmN0pQr7sA"`
}

// FriendLinkPreview GET /friend-links/:code yanıtıdır.
type FriendLinkPreview struct {
	UserID        uuid.UUID `json:"user_id" format:"uuid"`
	Name          string    `json:"name" example:"Zeynep Kaya"`
	AvatarVersion string    `json:"avatar_version,omitempty"`
	IsSelf        bool      `json:"is_self"`
	AlreadyFriend bool      `json:"already_friend"`
}

// MemberInviteChange grup daveti işleminin sonucudur (yanıt + bildirim için).
type MemberInviteChange struct {
	GroupID    uuid.UUID
	Joined     bool
	MemberName string
}

// FriendService arkadaşlık, kişisel link ve arkadaşların gruba daveti iş mantığıdır.
type FriendService interface {
	List(userID uuid.UUID) (*FriendList, error)
	Request(userID uuid.UUID, email string) (*FriendRequestResult, error)
	// Accept / DeleteRequest karşı tarafın kimliğini döner (akışına sinyal göndermek için).
	Accept(userID, friendshipID uuid.UUID) (uuid.UUID, error)
	DeleteRequest(userID, friendshipID uuid.UUID) (uuid.UUID, error)
	Unfriend(userID, otherID uuid.UUID) error
	GetLink(userID uuid.UUID) (*FriendLinkView, error)
	ResetLink(userID uuid.UUID) (*FriendLinkView, error)
	PreviewLink(userID uuid.UUID, code string) (*FriendLinkPreview, error)
	// AcceptLink link sahibini döner.
	AcceptLink(userID uuid.UUID, code string) (uuid.UUID, error)

	// ValidateInvitees davet edilecek kişilerin çağıranın arkadaşı olduğunu ve hedef kişi
	// olmadığını denetler; tekrarları ayıklanmış listeyi döner.
	ValidateInvitees(userID uuid.UUID, ids []uuid.UUID, targetID *uuid.UUID) ([]uuid.UUID, error)
	// InviteToGroup yalnızca admin; yeni davet edilenleri döner.
	InviteToGroup(groupID, userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)
	// InviteCreatedGroup grubu yeni kuran admin için doğrulanmış davetleri yazar.
	InviteCreatedGroup(groupID, userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)
	ListGroupInvites(groupID, userID uuid.UUID) ([]repository.PendingInviteRow, error)
	CancelGroupInvite(groupID, userID, inviteeID uuid.UUID) error
	MyInvites(userID uuid.UUID) ([]repository.MyInviteRow, error)
	AcceptInvite(userID, inviteID uuid.UUID) (*MemberInviteChange, error)
	DeclineInvite(userID, inviteID uuid.UUID) (*MemberInviteChange, error)
}

type friendService struct {
	friends   repository.FriendRepository
	invites   repository.MemberInviteRepository
	groupRepo repository.GroupRepository
	userRepo  repository.UserRepository
	now       func() time.Time
}

// NewFriendService yeni bir FriendService oluşturur.
func NewFriendService(friends repository.FriendRepository, invites repository.MemberInviteRepository, groupRepo repository.GroupRepository, userRepo repository.UserRepository) FriendService {
	return &friendService{friends: friends, invites: invites, groupRepo: groupRepo, userRepo: userRepo, now: time.Now}
}

func (s *friendService) List(userID uuid.UUID) (*FriendList, error) {
	rows, err := s.friends.List(userID)
	if err != nil {
		return nil, err
	}
	out := &FriendList{Friends: []FriendView{}, Incoming: []FriendView{}, Outgoing: []FriendView{}}
	for _, r := range rows {
		v := FriendView{FriendshipID: r.FriendshipID, ID: r.UserID, Name: r.Name, AvatarVersion: r.AvatarVersion, Since: r.CreatedAt, AcceptedAt: r.AcceptedAt}
		switch {
		case r.Status == models.FriendshipAccepted:
			out.Friends = append(out.Friends, v)
		case r.RequesterID == userID:
			// Karşı taraf henüz kabul etmedi: fotoğrafını göremez (avatar ucu da vermez).
			v.AvatarVersion = ""
			out.Outgoing = append(out.Outgoing, v)
		default:
			out.Incoming = append(out.Incoming, v)
		}
	}
	return out, nil
}

func (s *friendService) Request(userID uuid.UUID, email string) (*FriendRequestResult, error) {
	other, err := s.userRepo.FindByEmail(strings.TrimSpace(email))
	if err != nil {
		return nil, err
	}
	if other == nil {
		return nil, ErrUserNotFound
	}
	if other.ID == userID {
		return nil, ErrSelfFriend
	}
	res, err := s.friends.Request(userID, other.ID, s.now())
	if err != nil {
		if errors.Is(err, repository.ErrFriendshipExists) {
			return nil, ErrFriendshipExists
		}
		return nil, err
	}
	status := models.FriendshipPending
	if res.Accepted {
		status = models.FriendshipAccepted
	}
	return &FriendRequestResult{Status: status, UserID: other.ID}, nil
}

func otherParty(f *models.Friendship, userID uuid.UUID) uuid.UUID {
	if f.UserLowID == userID {
		return f.UserHighID
	}
	return f.UserLowID
}

func (s *friendService) Accept(userID, friendshipID uuid.UUID) (uuid.UUID, error) {
	f, err := s.friends.Accept(friendshipID, userID, s.now())
	if err != nil {
		if errors.Is(err, repository.ErrFriendshipNotFound) {
			return uuid.Nil, ErrFriendRequestMissing
		}
		return uuid.Nil, err
	}
	return otherParty(f, userID), nil
}

func (s *friendService) DeleteRequest(userID, friendshipID uuid.UUID) (uuid.UUID, error) {
	f, err := s.friends.DeletePending(friendshipID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrFriendshipNotFound) {
			return uuid.Nil, ErrFriendRequestMissing
		}
		return uuid.Nil, err
	}
	return otherParty(f, userID), nil
}

func (s *friendService) Unfriend(userID, otherID uuid.UUID) error {
	if err := s.friends.Unfriend(userID, otherID); err != nil {
		if errors.Is(err, repository.ErrFriendshipNotFound) {
			return ErrFriendNotFound
		}
		return err
	}
	return nil
}

func newFriendCode() (string, error) {
	raw := make([]byte, friendCodeBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *friendService) GetLink(userID uuid.UUID) (*FriendLinkView, error) {
	link, err := s.friends.GetLink(userID)
	if err != nil {
		return nil, err
	}
	if link != nil {
		return &FriendLinkView{Code: link.Code}, nil
	}
	return s.ResetLink(userID)
}

func (s *friendService) ResetLink(userID uuid.UUID) (*FriendLinkView, error) {
	code, err := newFriendCode()
	if err != nil {
		return nil, err
	}
	if err := s.friends.SaveLink(&models.FriendLink{UserID: userID, Code: code, CreatedAt: s.now()}); err != nil {
		return nil, err
	}
	return &FriendLinkView{Code: code}, nil
}

func (s *friendService) findLink(code string) (*models.FriendLink, error) {
	if !friendCodePattern.MatchString(code) {
		return nil, ErrFriendLinkNotFound
	}
	link, err := s.friends.FindLinkByCode(code)
	if err != nil {
		return nil, err
	}
	if link == nil {
		return nil, ErrFriendLinkNotFound
	}
	return link, nil
}

func (s *friendService) PreviewLink(userID uuid.UUID, code string) (*FriendLinkPreview, error) {
	link, err := s.findLink(code)
	if err != nil {
		return nil, err
	}
	owner, err := s.userRepo.FindByID(link.UserID)
	if err != nil {
		return nil, err
	}
	if owner == nil {
		return nil, ErrFriendLinkNotFound
	}
	preview := &FriendLinkPreview{UserID: owner.ID, Name: owner.Name, AvatarVersion: owner.AvatarVersion, IsSelf: owner.ID == userID}
	if !preview.IsSelf {
		ids, err := s.friends.AcceptedFriendIDs(userID, []uuid.UUID{owner.ID})
		if err != nil {
			return nil, err
		}
		preview.AlreadyFriend = len(ids) > 0
	}
	return preview, nil
}

func (s *friendService) AcceptLink(userID uuid.UUID, code string) (uuid.UUID, error) {
	link, err := s.findLink(code)
	if err != nil {
		return uuid.Nil, err
	}
	if link.UserID == userID {
		return uuid.Nil, ErrSelfFriend
	}
	if _, err := s.friends.Befriend(userID, link.UserID, s.now()); err != nil {
		return uuid.Nil, err
	}
	return link.UserID, nil
}

func dedupeIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func (s *friendService) ValidateInvitees(userID uuid.UUID, ids []uuid.UUID, targetID *uuid.UUID) ([]uuid.UUID, error) {
	ids = dedupeIDs(ids)
	if len(ids) > MaxInvitees {
		return nil, ErrTooManyInvitees
	}
	for _, id := range ids {
		if targetID != nil && id == *targetID {
			return nil, ErrInviteeIsTarget
		}
		if id == userID {
			return nil, ErrNotFriends
		}
	}
	friends, err := s.friends.AcceptedFriendIDs(userID, ids)
	if err != nil {
		return nil, err
	}
	if len(friends) != len(ids) {
		return nil, ErrNotFriends
	}
	return ids, nil
}

func (s *friendService) requireAdmin(groupID, userID uuid.UUID) (*models.Group, error) {
	return requireGroupAdmin(s.groupRepo, groupID, userID, ErrUnauthorizedMemberInvite)
}

func (s *friendService) InviteToGroup(groupID, userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	group, err := s.requireAdmin(groupID, userID)
	if err != nil {
		return nil, err
	}
	return s.writeInvites(group, userID, ids)
}

func (s *friendService) InviteCreatedGroup(groupID, userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return []uuid.UUID{}, nil
	}
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrGroupNotFound
	}
	return s.writeInvites(group, userID, ids)
}

func (s *friendService) writeInvites(group *models.Group, userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.ValidateInvitees(userID, ids, group.TargetUserID)
	if err != nil {
		return nil, err
	}
	invited, err := s.invites.Invite(group.ID, userID, ids)
	if err != nil {
		if errors.Is(err, repository.ErrMemberIsGroupTarget) {
			return nil, ErrInviteeIsTarget
		}
		return nil, err
	}
	return invited, nil
}

func (s *friendService) ListGroupInvites(groupID, userID uuid.UUID) ([]repository.PendingInviteRow, error) {
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotGroupMember
	}
	return s.invites.ListForGroup(groupID)
}

func (s *friendService) CancelGroupInvite(groupID, userID, inviteeID uuid.UUID) error {
	if _, err := s.requireAdmin(groupID, userID); err != nil {
		return err
	}
	if err := s.invites.Cancel(groupID, inviteeID); err != nil {
		if errors.Is(err, repository.ErrMemberInviteNotFound) {
			return ErrMemberInviteNotFound
		}
		return err
	}
	return nil
}

func (s *friendService) MyInvites(userID uuid.UUID) ([]repository.MyInviteRow, error) {
	return s.invites.ListForUser(userID)
}

func (s *friendService) AcceptInvite(userID, inviteID uuid.UUID) (*MemberInviteChange, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	res, err := s.invites.Accept(inviteID, userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrMemberInviteNotFound):
			return nil, ErrMemberInviteNotFound
		case errors.Is(err, repository.ErrMemberIsGroupTarget):
			return nil, ErrAntiSpoilerTriggered
		}
		return nil, err
	}
	return &MemberInviteChange{GroupID: res.GroupID, Joined: res.Joined, MemberName: user.Name}, nil
}

func (s *friendService) DeclineInvite(userID, inviteID uuid.UUID) (*MemberInviteChange, error) {
	res, err := s.invites.Decline(inviteID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrMemberInviteNotFound) {
			return nil, ErrMemberInviteNotFound
		}
		return nil, err
	}
	return &MemberInviteChange{GroupID: res.GroupID}, nil
}

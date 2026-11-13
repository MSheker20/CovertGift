package services

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeInviteRepo struct {
	repository.InviteRepository
	replaced *models.GroupInvite
	revoked  bool
	active   *models.GroupInvite
	joinRes  *repository.InviteJoinResult
	joinErr  error
	gotHash  string
}

func (r *fakeInviteRepo) GetActive(uuid.UUID, time.Time) (*models.GroupInvite, error) {
	return r.active, nil
}
func (r *fakeInviteRepo) Replace(i *models.GroupInvite, _ time.Time) error {
	r.replaced = i
	return nil
}
func (r *fakeInviteRepo) RevokeActive(uuid.UUID, time.Time) error { r.revoked = true; return nil }
func (r *fakeInviteRepo) FindActiveByHash(hash string, _ time.Time) (*models.GroupInvite, error) {
	r.gotHash = hash
	return r.active, nil
}
func (r *fakeInviteRepo) Join(hash string, _ uuid.UUID, _ time.Time) (*repository.InviteJoinResult, error) {
	r.gotHash = hash
	return r.joinRes, r.joinErr
}

type fakeInviteGroupRepo struct {
	repository.GroupRepository
	group   *models.Group
	members map[uuid.UUID]*models.GroupMember
}

func (r *fakeInviteGroupRepo) GetGroupByID(uuid.UUID) (*models.Group, error) { return r.group, nil }
func (r *fakeInviteGroupRepo) GetMember(_, userID uuid.UUID) (*models.GroupMember, error) {
	return r.members[userID], nil
}
func (r *fakeInviteGroupRepo) GetMemberCount(uuid.UUID) (int64, error) {
	return int64(len(r.members)), nil
}

type fakeInviteUserRepo struct {
	repository.UserRepository
	users map[uuid.UUID]*models.User
}

func (r *fakeInviteUserRepo) FindByID(id uuid.UUID) (*models.User, error) { return r.users[id], nil }

type inviteFixture struct {
	svc                  InviteService
	invites              *fakeInviteRepo
	groupID              uuid.UUID
	adminID, memberID    uuid.UUID
	targetID, outsiderID uuid.UUID
}

func newInviteFixture() *inviteFixture {
	f := &inviteFixture{
		invites: &fakeInviteRepo{}, groupID: uuid.New(),
		adminID: uuid.New(), memberID: uuid.New(), targetID: uuid.New(), outsiderID: uuid.New(),
	}
	groups := &fakeInviteGroupRepo{
		group: &models.Group{ID: f.groupID, Title: "Sürpriz", TargetUserID: &f.targetID},
		members: map[uuid.UUID]*models.GroupMember{
			f.adminID:  {Role: "admin"},
			f.memberID: {Role: "member"},
		},
	}
	users := &fakeInviteUserRepo{users: map[uuid.UUID]*models.User{
		f.adminID: {ID: f.adminID, Name: "Admin"}, f.outsiderID: {ID: f.outsiderID, Name: "Dışarıdan"},
	}}
	f.svc = NewInviteService(f.invites, groups, users)
	return f
}

func TestCreateInviteRejectsInvalidDurations(t *testing.T) {
	f := newInviteFixture()
	for _, days := range []int{0, -1, 2, 8, 31, 365} {
		if _, err := f.svc.Create(f.groupID, f.adminID, days); !errors.Is(err, ErrInvalidInviteExpiry) {
			t.Errorf("%d gün: beklenen ErrInvalidInviteExpiry, gelen %v", days, err)
		}
	}
	if f.invites.replaced != nil {
		t.Fatal("geçersiz süreyle davet kaydedildi")
	}
}

func TestCreateInviteStoresOnlyTheHashOfTheReturnedToken(t *testing.T) {
	f := newInviteFixture()
	for _, days := range []int{1, 7, 30} {
		created, err := f.svc.Create(f.groupID, f.adminID, days)
		if err != nil {
			t.Fatalf("%d gün: %v", days, err)
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(created.Token) {
			t.Fatalf("token biçimi beklenmedik: %q", created.Token)
		}
		stored := f.invites.replaced
		if stored.TokenHash != hashInviteToken(created.Token) || stored.TokenHash == created.Token || len(stored.TokenHash) != 64 {
			t.Fatalf("saklanan değer token'ın SHA-256 özeti değil: %q", stored.TokenHash)
		}
		if got := stored.ExpiresAt.Sub(stored.CreatedAt); got != time.Duration(days)*24*time.Hour {
			t.Fatalf("%d gün: süre %v", days, got)
		}
		if stored.GroupID != f.groupID || stored.CreatedBy != f.adminID {
			t.Fatalf("grup/oluşturan yanlış: %+v", stored)
		}
	}
}

func TestInviteManagementIsAdminOnly(t *testing.T) {
	f := newInviteFixture()
	for name, call := range map[string]func(uuid.UUID) error{
		"create": func(u uuid.UUID) error { _, err := f.svc.Create(f.groupID, u, 7); return err },
		"status": func(u uuid.UUID) error { _, err := f.svc.GetStatus(f.groupID, u); return err },
		"revoke": func(u uuid.UUID) error { return f.svc.Revoke(f.groupID, u) },
	} {
		if err := call(f.memberID); !errors.Is(err, ErrUnauthorizedInvite) {
			t.Errorf("%s: üye için beklenen ErrUnauthorizedInvite, gelen %v", name, err)
		}
		if err := call(f.outsiderID); !errors.Is(err, ErrNotGroupMember) {
			t.Errorf("%s: üye olmayan için beklenen ErrNotGroupMember, gelen %v", name, err)
		}
		if err := call(f.targetID); !errors.Is(err, ErrAntiSpoilerTriggered) {
			t.Errorf("%s: hedef için beklenen ErrAntiSpoilerTriggered, gelen %v", name, err)
		}
	}
	if f.invites.replaced != nil || f.invites.revoked {
		t.Fatal("yetkisiz çağrı davet durumunu değiştirdi")
	}
}

func TestGetInviteStatusNeverCarriesAToken(t *testing.T) {
	f := newInviteFixture()
	status, err := f.svc.GetStatus(f.groupID, f.adminID)
	if err != nil || status.Active {
		t.Fatalf("davet yokken aktif görünüyor: %+v %v", status, err)
	}
	exp := time.Now().Add(time.Hour)
	f.invites.active = &models.GroupInvite{TokenHash: "abc", ExpiresAt: exp, CreatedAt: time.Now()}
	status, err = f.svc.GetStatus(f.groupID, f.adminID)
	if err != nil || !status.Active || status.ExpiresAt == nil || !status.ExpiresAt.Equal(exp) {
		t.Fatalf("aktif davet yanlış: %+v %v", status, err)
	}
}

func TestPreviewMalformedOrUnknownTokenIsGenericNotFound(t *testing.T) {
	f := newInviteFixture()
	for _, token := range []string{"", "kisa", "bu/token/geçersiz", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if _, err := f.svc.Preview(token, f.outsiderID); !errors.Is(err, ErrInviteNotFound) {
			t.Errorf("%q: beklenen ErrInviteNotFound, gelen %v", token, err)
		}
		if _, err := f.svc.Accept(token, f.outsiderID); !errors.Is(err, ErrInviteNotFound) {
			t.Errorf("accept %q: beklenen ErrInviteNotFound, gelen %v", token, err)
		}
	}
}

func TestPreviewForTargetIsSurpriseAndLeaksNothing(t *testing.T) {
	f := newInviteFixture()
	f.invites.active = &models.GroupInvite{GroupID: f.groupID, CreatedBy: f.adminID, ExpiresAt: time.Now().Add(time.Hour)}
	token := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	preview, err := f.svc.Preview(token, f.targetID)
	if !errors.Is(err, ErrAntiSpoilerTriggered) || preview != nil {
		t.Fatalf("hedef için beklenen Anti-Spoiler, gelen %+v %v", preview, err)
	}
	preview, err = f.svc.Preview(token, f.outsiderID)
	if err != nil || preview.GroupTitle != "Sürpriz" || preview.InviterName != "Admin" || preview.MemberCount != 2 || preview.AlreadyMember || preview.GroupID != nil {
		t.Fatalf("önizleme yanlış: %+v %v", preview, err)
	}
	preview, err = f.svc.Preview(token, f.memberID)
	if err != nil || !preview.AlreadyMember || preview.GroupID == nil || *preview.GroupID != f.groupID {
		t.Fatalf("üye için already_member false: %+v %v", preview, err)
	}
}

func TestAcceptMapsRepositoryResults(t *testing.T) {
	f := newInviteFixture()
	token := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	f.invites.joinRes = &repository.InviteJoinResult{GroupID: f.groupID, Joined: true}
	res, err := f.svc.Accept(token, f.outsiderID)
	if err != nil || !res.Joined || res.GroupID != f.groupID || res.MemberName != "Dışarıdan" {
		t.Fatalf("katılım yanlış: %+v %v", res, err)
	}
	if f.invites.gotHash != hashInviteToken(token) {
		t.Fatal("depoya ham token ya da yanlış özet gitti")
	}
	f.invites.joinRes = &repository.InviteJoinResult{GroupID: f.groupID}
	if res, _ := f.svc.Accept(token, f.outsiderID); res.Joined {
		t.Fatal("zaten üye için Joined true")
	}
	f.invites.joinRes, f.invites.joinErr = nil, repository.ErrMemberIsGroupTarget
	if _, err := f.svc.Accept(token, f.outsiderID); !errors.Is(err, ErrAntiSpoilerTriggered) {
		t.Fatalf("hedef için beklenen Anti-Spoiler, gelen %v", err)
	}
	f.invites.joinErr = repository.ErrInviteInactive
	if _, err := f.svc.Accept(token, f.outsiderID); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("pasif davet için beklenen ErrInviteNotFound, gelen %v", err)
	}
}

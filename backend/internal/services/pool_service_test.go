package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// poolGroupRepository MULTI grup testleri için grup deposu: üyeler rol ile tutulur.
type poolGroupRepository struct {
	repository.GroupRepository
	group   *models.Group
	members map[uuid.UUID]string // kullanıcı → rol
}

func (r *poolGroupRepository) GetGroupByID(uuid.UUID) (*models.Group, error) { return r.group, nil }

func (r *poolGroupRepository) GetMember(_ uuid.UUID, userID uuid.UUID) (*models.GroupMember, error) {
	role, ok := r.members[userID]
	if !ok {
		return nil, nil
	}
	return &models.GroupMember{UserID: userID, Role: role}, nil
}

func (r *poolGroupRepository) GetAdminID(uuid.UUID) (uuid.UUID, error) {
	for id, role := range r.members {
		if role == "admin" {
			return id, nil
		}
	}
	return uuid.Nil, nil
}

func (r *poolGroupRepository) GetMembers(uuid.UUID) ([]models.GroupMember, error) {
	out := []models.GroupMember{}
	for id, role := range r.members {
		out = append(out, models.GroupMember{UserID: id, Role: role, User: models.User{ID: id, Name: "U-" + id.String()[:4]}})
	}
	return out, nil
}

// fakePoolRepository havuz deposunun bellek içi sahtesidir; guard'lar gerçek depodaki gibi çağrılır.
type fakePoolRepository struct {
	repository.PoolRepository
	pools      map[uuid.UUID]*models.GiftPool
	members    map[uuid.UUID]string
	group      *models.Group
	ideaCount  int64
	written    []string
	deleted    bool
	contribOff map[uuid.UUID]bool
}

func (r *fakePoolRepository) CreatePool(groupID, recipientID uuid.UUID, managerID *uuid.UUID, guard func(*models.Group, repository.PoolMembership) error) (*models.GiftPool, error) {
	m := repository.PoolMembership{}
	_, m.RecipientIsMember = r.members[recipientID]
	if managerID != nil {
		_, m.ManagerIsMember = r.members[*managerID]
	}
	for _, p := range r.pools {
		if p.RecipientUserID == recipientID {
			m.PoolExists = true
		}
	}
	if err := guard(r.group, m); err != nil {
		return nil, err
	}
	pool := &models.GiftPool{ID: uuid.New(), GroupID: groupID, RecipientUserID: recipientID, ManagerUserID: managerID, Status: models.GroupStatusActive}
	r.pools[pool.ID] = pool
	r.written = append(r.written, "create")
	return pool, nil
}

func (r *fakePoolRepository) GetPool(_ uuid.UUID, poolID uuid.UUID) (*models.GiftPool, error) {
	return r.pools[poolID], nil
}

func (r *fakePoolRepository) PoolStats(_ uuid.UUID, viewerID uuid.UUID) (map[uuid.UUID]repository.PoolStats, error) {
	out := map[uuid.UUID]repository.PoolStats{}
	for id, p := range r.pools {
		if p.RecipientUserID == viewerID {
			continue
		}
		count := int64(0)
		for member := range r.members {
			if member != p.RecipientUserID && !r.contribOff[member] {
				count++
			}
		}
		out[id] = repository.PoolStats{PoolID: id, ContributorCount: count, MyContribution: !r.contribOff[viewerID]}
	}
	return out, nil
}

func (r *fakePoolRepository) ListPools(uuid.UUID) ([]models.GiftPool, error) {
	out := []models.GiftPool{}
	for _, p := range r.pools {
		out = append(out, *p)
	}
	return out, nil
}

func (r *fakePoolRepository) ListContributors(uuid.UUID) ([]repository.Contributor, error) {
	return []repository.Contributor{}, nil
}

func (r *fakePoolRepository) DeletePool(_ uuid.UUID, recipientID uuid.UUID, guard func(*models.GiftPool, int64) error) error {
	for _, p := range r.pools {
		if p.RecipientUserID == recipientID {
			if err := guard(p, r.ideaCount); err != nil {
				return err
			}
			r.deleted = true
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakePoolRepository) SetManager(_ uuid.UUID, recipientID uuid.UUID, managerID *uuid.UUID, guard func(*models.GiftPool, bool) error) (*models.GiftPool, error) {
	for _, p := range r.pools {
		if p.RecipientUserID == recipientID {
			isMember := false
			if managerID != nil {
				_, isMember = r.members[*managerID]
			}
			if err := guard(p, isMember); err != nil {
				return nil, err
			}
			p.ManagerUserID = managerID
			return p, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakePoolRepository) UpdateBudget(uuid.UUID, decimal.Decimal) error {
	r.written = append(r.written, "budget")
	return nil
}

func (r *fakePoolRepository) MutateInterests(_ uuid.UUID, change func([]string) ([]string, error)) ([]string, error) {
	r.written = append(r.written, "interests")
	return change([]string{"Kamp"})
}

func (r *fakePoolRepository) SetEventDate(uuid.UUID, *models.Date) error {
	r.written = append(r.written, "date")
	return nil
}

func (r *fakePoolRepository) Complete(poolID uuid.UUID, ids []uuid.UUID, guard func(*models.GiftPool, []models.GiftIdea) error) error {
	ideas := make([]models.GiftIdea, len(ids))
	for i, id := range ids {
		ideas[i] = models.GiftIdea{ID: id, Status: "ACTIVE"}
	}
	if err := guard(r.pools[poolID], ideas); err != nil {
		return err
	}
	r.written = append(r.written, "complete")
	return nil
}

func (r *fakePoolRepository) Reopen(poolID uuid.UUID, guard func(*models.GiftPool) error) error {
	if err := guard(r.pools[poolID]); err != nil {
		return err
	}
	r.written = append(r.written, "reopen")
	return nil
}

func (r *fakePoolRepository) SetContribution(poolID, userID uuid.UUID, contributing bool, guard func(*models.GiftPool) error) error {
	if err := guard(r.pools[poolID]); err != nil {
		return err
	}
	if r.contribOff == nil {
		r.contribOff = map[uuid.UUID]bool{}
	}
	r.contribOff[userID] = !contributing
	r.written = append(r.written, "contribution")
	return nil
}

type poolFixture struct {
	groupID, adminID, aliceID, bobID, outsiderID uuid.UUID
	groups                                       *poolGroupRepository
	pools                                        *fakePoolRepository
	svc                                          PoolService
}

func newPoolFixture(mode string) *poolFixture {
	f := &poolFixture{groupID: uuid.New(), adminID: uuid.New(), aliceID: uuid.New(), bobID: uuid.New(), outsiderID: uuid.New()}
	members := map[uuid.UUID]string{f.adminID: "admin", f.aliceID: "member", f.bobID: "member"}
	group := &models.Group{ID: f.groupID, Mode: mode, CreatorID: f.adminID}
	f.groups = &poolGroupRepository{group: group, members: members}
	f.pools = &fakePoolRepository{pools: map[uuid.UUID]*models.GiftPool{}, members: members, group: group}
	f.svc = NewPoolService(f.groups, f.pools)
	return f
}

func (f *poolFixture) addPool(recipient uuid.UUID, manager *uuid.UUID) *models.GiftPool {
	p := &models.GiftPool{ID: uuid.New(), GroupID: f.groupID, RecipientUserID: recipient, ManagerUserID: manager, Status: models.GroupStatusActive,
		TotalBudget: decimal.NewFromInt(300), Recipient: models.User{ID: recipient, Name: "Alıcı"}}
	f.pools.pools[p.ID] = p
	return p
}

func TestAddRecipientManagerRules(t *testing.T) {
	ptr := func(id uuid.UUID) *uuid.UUID { return &id }
	for _, tc := range []struct {
		name      string
		mode      string
		caller    func(f *poolFixture) uuid.UUID
		recipient func(f *poolFixture) uuid.UUID
		manager   func(f *poolFixture) *uuid.UUID
		existing  bool
		wantErr   error
		locked    bool
	}{
		{name: "admin adds member", recipient: func(f *poolFixture) uuid.UUID { return f.aliceID }},
		{name: "admin adds self with another member as manager", recipient: func(f *poolFixture) uuid.UUID { return f.adminID },
			manager: func(f *poolFixture) *uuid.UUID { return ptr(f.bobID) }, locked: true},
		{name: "member cannot add", caller: func(f *poolFixture) uuid.UUID { return f.bobID }, recipient: func(f *poolFixture) uuid.UUID { return f.aliceID }, wantErr: ErrUnauthorizedRecipients},
		{name: "single group", mode: models.GroupModeSingle, recipient: func(f *poolFixture) uuid.UUID { return f.aliceID }, wantErr: ErrWrongGroupMode},
		{name: "admin self without manager", recipient: func(f *poolFixture) uuid.UUID { return f.adminID }, wantErr: ErrManagerRequired},
		{name: "admin self as own manager", recipient: func(f *poolFixture) uuid.UUID { return f.adminID },
			manager: func(f *poolFixture) *uuid.UUID { return ptr(f.adminID) }, wantErr: ErrManagerIsRecipient},
		{name: "manager given for member recipient", recipient: func(f *poolFixture) uuid.UUID { return f.aliceID },
			manager: func(f *poolFixture) *uuid.UUID { return ptr(f.bobID) }, wantErr: ErrManagerNotAllowed},
		{name: "recipient not a member", recipient: func(f *poolFixture) uuid.UUID { return f.outsiderID }, wantErr: ErrRecipientNotMember},
		{name: "manager not a member", recipient: func(f *poolFixture) uuid.UUID { return f.adminID },
			manager: func(f *poolFixture) *uuid.UUID { return ptr(f.outsiderID) }, wantErr: ErrManagerNotMember},
		{name: "already a recipient", recipient: func(f *poolFixture) uuid.UUID { return f.aliceID }, existing: true, wantErr: ErrAlreadyRecipient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			if mode == "" {
				mode = models.GroupModeMulti
			}
			f := newPoolFixture(mode)
			caller := f.adminID
			if tc.caller != nil {
				caller = tc.caller(f)
			}
			var manager *uuid.UUID
			if tc.manager != nil {
				manager = tc.manager(f)
			}
			if tc.existing {
				f.addPool(tc.recipient(f), nil)
			}
			before := len(f.pools.pools)
			added, pool, err := f.svc.AddRecipient(f.groupID, caller, tc.recipient(f), manager)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if len(f.pools.pools) != before {
					t.Fatal("rejected request created a pool")
				}
				return
			}
			if pool == nil || added == nil {
				t.Fatal("no pool returned")
			}
			_, isLocked := added.Pool.(LockedPool)
			if isLocked != tc.locked {
				t.Fatalf("view locked = %v, want %v (%#v)", isLocked, tc.locked, added.Pool)
			}
		})
	}
}

func TestSetManagerAndRemoveRecipientRules(t *testing.T) {
	f := newPoolFixture(models.GroupModeMulti)
	f.addPool(f.adminID, &f.bobID)
	f.addPool(f.aliceID, nil)

	if _, err := f.svc.SetManager(f.groupID, f.adminID, f.adminID, nil); !errors.Is(err, ErrManagerRequired) {
		t.Fatalf("null manager for admin pool: %v", err)
	}
	if _, err := f.svc.SetManager(f.groupID, f.adminID, f.aliceID, &f.bobID); !errors.Is(err, ErrManagerNotAllowed) {
		t.Fatalf("manager for member pool: %v", err)
	}
	if _, err := f.svc.SetManager(f.groupID, f.bobID, f.adminID, &f.aliceID); !errors.Is(err, ErrUnauthorizedRecipients) {
		t.Fatalf("member sets manager: %v", err)
	}
	if _, err := f.svc.SetManager(f.groupID, f.adminID, f.bobID, nil); !errors.Is(err, ErrRecipientNotFound) {
		t.Fatalf("not a recipient: %v", err)
	}
	if _, err := f.svc.SetManager(f.groupID, f.adminID, f.adminID, &f.outsiderID); !errors.Is(err, ErrManagerNotMember) {
		t.Fatalf("outsider manager: %v", err)
	}
	same, err := f.svc.SetManager(f.groupID, f.adminID, f.adminID, &f.bobID)
	if err != nil || same.Changed {
		t.Fatalf("same manager: %+v %v", same, err)
	}
	changed, err := f.svc.SetManager(f.groupID, f.adminID, f.adminID, &f.aliceID)
	if err != nil || !changed.Changed || *changed.Manager != f.aliceID {
		t.Fatalf("new manager: %+v %v", changed, err)
	}

	for _, tc := range []struct {
		name      string
		recipient uuid.UUID
		ideas     int64
		confirm   bool
		wantErr   error
	}{
		{"empty pool without confirm", f.aliceID, 0, false, nil},
		{"pool with ideas needs confirm", f.aliceID, 2, false, ErrPoolHasIdeas},
		{"pool with ideas confirmed", f.aliceID, 2, true, nil},
		{"caller's own empty pool still needs confirm", f.adminID, 0, false, ErrPoolHasIdeas},
		{"caller's own pool with ideas needs confirm", f.adminID, 3, false, ErrPoolHasIdeas},
		{"caller's own pool confirmed", f.adminID, 0, true, nil},
		{"not a recipient", f.bobID, 0, true, ErrRecipientNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.pools.ideaCount, f.pools.deleted = tc.ideas, false
			err := f.svc.RemoveRecipient(f.groupID, f.adminID, tc.recipient, tc.confirm)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if f.pools.deleted != (tc.wantErr == nil) {
				t.Fatalf("deleted = %v", f.pools.deleted)
			}
		})
	}
}

func TestPoolOperationsAntiSpoilerAndManagerPowers(t *testing.T) {
	f := newPoolFixture(models.GroupModeMulti)
	alicePool := f.addPool(f.aliceID, nil) // yönetici: grup yöneticisi
	adminPool := f.addPool(f.adminID, nil) // grup yöneticisinin havuzu, yönetici atanmamış (silinmiş)
	date := models.Date{Time: time.Now().UTC().Truncate(24 * time.Hour)}
	ops := map[string]func(poolID, user uuid.UUID) error{
		"get":         func(p, u uuid.UUID) error { _, err := f.svc.GetPool(f.groupID, p, u); return err },
		"budget":      func(p, u uuid.UUID) error { return f.svc.UpdateBudget(f.groupID, p, u, decimal.NewFromInt(10)) },
		"addInterest": func(p, u uuid.UUID) error { _, err := f.svc.AddInterest(f.groupID, p, u, "Satranç"); return err },
		"delInterest": func(p, u uuid.UUID) error { _, err := f.svc.DeleteInterest(f.groupID, p, u, "Kamp"); return err },
		"date":        func(p, u uuid.UUID) error { return f.svc.SetEventDate(f.groupID, p, u, &date) },
		"winner": func(p, u uuid.UUID) error {
			_, err := f.svc.SelectWinners(f.groupID, p, u, []uuid.UUID{uuid.New()})
			return err
		},
		"reopen":       func(p, u uuid.UUID) error { return f.svc.Reopen(f.groupID, p, u) },
		"contribution": func(p, u uuid.UUID) error { _, err := f.svc.SetContribution(f.groupID, p, u, false); return err },
	}
	managerOnly := map[string]bool{"budget": true, "delInterest": true, "date": true, "winner": true, "reopen": true}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			f.pools.written = nil
			if err := op(alicePool.ID, f.aliceID); !errors.Is(err, ErrAntiSpoilerTriggered) {
				t.Fatalf("recipient: %v", err)
			}
			if err := op(adminPool.ID, f.adminID); !errors.Is(err, ErrAntiSpoilerTriggered) {
				t.Fatalf("admin recipient: %v", err)
			}
			if len(f.pools.written) != 0 {
				t.Fatalf("recipient request wrote: %v", f.pools.written)
			}
			if err := op(uuid.New(), f.bobID); !errors.Is(err, ErrPoolNotFound) {
				t.Fatalf("missing pool: %v", err)
			}
			if managerOnly[name] {
				if err := op(alicePool.ID, f.bobID); !errors.Is(err, ErrUnauthorizedPoolManage) {
					t.Fatalf("non-manager: %v", err)
				}
				// Yöneticisi silinmiş grup yöneticisi havuzunu kimse yönetemez (grup yöneticisi alıcı).
				if err := op(adminPool.ID, f.bobID); !errors.Is(err, ErrUnauthorizedPoolManage) {
					t.Fatalf("unmanaged pool: %v", err)
				}
			}
		})
	}
	if err := ops["budget"](alicePool.ID, f.adminID); err != nil {
		t.Fatalf("group admin manages member pool: %v", err)
	}
	adminPool.ManagerUserID = &f.bobID
	if err := ops["budget"](adminPool.ID, f.bobID); err != nil {
		t.Fatalf("appointed manager manages admin pool: %v", err)
	}
	if err := ops["budget"](adminPool.ID, f.aliceID); !errors.Is(err, ErrUnauthorizedPoolManage) {
		t.Fatalf("other member manages admin pool: %v", err)
	}
	if err := f.svc.UpdateBudget(f.groupID, alicePool.ID, f.adminID, decimal.RequireFromString("1.005")); !errors.Is(err, ErrInvalidBudget) {
		t.Fatalf("invalid budget: %v", err)
	}

	// Tamamlanan havuzda katkı değişikliği 409; yeniden açılınca serbest.
	alicePool.Status = models.GroupStatusCompleted
	if _, err := f.svc.SetContribution(f.groupID, alicePool.ID, f.bobID, false); !errors.Is(err, ErrPoolCompleted) {
		t.Fatalf("completed pool contribution: %v", err)
	}
	if _, err := f.svc.SelectWinners(f.groupID, alicePool.ID, f.adminID, []uuid.UUID{uuid.New()}); !errors.Is(err, ErrPoolCompleted) {
		t.Fatalf("completed pool winner: %v", err)
	}
	if err := f.svc.Reopen(f.groupID, alicePool.ID, f.adminID); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	alicePool.Status = models.GroupStatusActive
	if err := f.svc.Reopen(f.groupID, alicePool.ID, f.adminID); !errors.Is(err, ErrPoolNotCompleted) {
		t.Fatalf("reopen open pool: %v", err)
	}
	res, err := f.svc.SetContribution(f.groupID, alicePool.ID, f.bobID, false)
	if err != nil || res.Contributing || res.ContributorCount != 1 || res.PerPersonBudget == nil || res.PerPersonBudget.String() != "300" {
		t.Fatalf("contribution result: %+v %v", res, err)
	}
}

func TestPoolViewsLockOwnPool(t *testing.T) {
	f := newPoolFixture(models.GroupModeMulti)
	own := f.addPool(f.aliceID, nil)
	other := f.addPool(f.bobID, nil)
	views, err := f.svc.ListPools(f.groupID, f.aliceID)
	if err != nil {
		t.Fatal(err)
	}
	var sawLocked, sawOpen bool
	for _, v := range views {
		switch p := v.(type) {
		case LockedPool:
			sawLocked = p.ID == own.ID && p.IsMine && p.Locked && p.Recipient.ID == f.aliceID
		case OpenPool:
			sawOpen = p.ID == other.ID && !p.IsMine && !p.Locked && p.Manager != nil && p.Manager.ID == f.adminID && !p.CanManage
		}
	}
	if !sawLocked || !sawOpen {
		t.Fatalf("views = %#v", views)
	}
	single := newPoolFixture(models.GroupModeSingle)
	if _, err := single.svc.ListPools(single.groupID, single.aliceID); !errors.Is(err, ErrWrongGroupMode) {
		t.Fatalf("single group list: %v", err)
	}
}

// Havuz fikirleri: alıcı her /ideas/:id/* işleminde ErrAntiSpoilerTriggered alır; veto ve silme
// yetkisi etkin yöneticidedir.
type poolIdeaRepository struct {
	repository.IdeaRepository
	idea     *models.GiftIdea
	status   string // kilit altında okunan kapsam durumu (boş = ACTIVE)
	rejected bool
	deleted  bool
	votes    int
}

func (r *poolIdeaRepository) GetIdeaByID(uuid.UUID) (*models.GiftIdea, error) { return r.idea, nil }
func (r *poolIdeaRepository) UpdateStatus(_ repository.IdeaScope, _ uuid.UUID, _ string, guard func(string) error) error {
	if err := guard(r.status); err != nil {
		return err
	}
	r.rejected = true
	return nil
}
func (r *poolIdeaRepository) DeleteIdea(_ repository.IdeaScope, _ uuid.UUID, guard func(string) error) error {
	if err := guard(r.status); err != nil {
		return err
	}
	r.deleted = true
	return nil
}
func (r *poolIdeaRepository) RemoveVote(_ repository.IdeaScope, _, _ uuid.UUID, guard func(string) error) error {
	return guard(r.status)
}
func (r *poolIdeaRepository) CastVote(_ repository.IdeaScope, _ *models.Vote, guard func(string) error) error {
	if err := guard(models.GroupStatusActive); err != nil {
		return err
	}
	r.votes++
	return nil
}

func TestPoolIdeaResolution(t *testing.T) {
	f := newPoolFixture(models.GroupModeMulti)
	pool := f.addPool(f.aliceID, nil)
	ideas := &poolIdeaRepository{idea: &models.GiftIdea{ID: uuid.New(), GroupID: f.groupID, PoolID: &pool.ID, CreatedBy: f.bobID}}
	svc := NewIdeaService(ideas, f.groups, f.pools, nil)
	id := ideas.idea.ID

	for name, op := range map[string]func(uuid.UUID) error{
		"vote":   func(u uuid.UUID) error { return svc.VoteIdea(id, u, 1) },
		"unvote": func(u uuid.UUID) error { return svc.UnvoteIdea(id, u) },
		"reject": func(u uuid.UUID) error { return svc.RejectIdea(id, u) },
		"delete": func(u uuid.UUID) error { return svc.DeleteIdea(id, u) },
		"guard":  func(u uuid.UUID) error { return svc.GuardIdea(id, u) },
	} {
		if err := op(f.aliceID); !errors.Is(err, ErrAntiSpoilerTriggered) {
			t.Errorf("%s by recipient: %v", name, err)
		}
	}
	if ideas.rejected || ideas.deleted || ideas.votes != 0 {
		t.Fatal("recipient changed the idea")
	}
	if err := svc.GuardIdea(id, f.bobID); err != nil {
		t.Fatalf("guard for member: %v", err)
	}
	if err := svc.RejectIdea(id, f.bobID); !errors.Is(err, ErrUnauthorizedVeto) {
		t.Fatalf("non-manager veto: %v", err)
	}
	if err := svc.RejectIdea(id, f.adminID); err != nil || !ideas.rejected {
		t.Fatalf("manager veto: %v", err)
	}
	if err := svc.VoteIdea(id, f.bobID, 1); err != nil || ideas.votes != 1 {
		t.Fatalf("member vote: %v", err)
	}
	// Kilit altında tamamlanmış bulunan havuz (kazanan seçimiyle yarış): veto, silme ve oy geri alma 409.
	ideas.status, ideas.rejected = models.GroupStatusCompleted, false
	for name, err := range map[string]error{
		"reject": svc.RejectIdea(id, f.adminID), "delete": svc.DeleteIdea(id, f.bobID), "unvote": svc.UnvoteIdea(id, f.bobID),
	} {
		if !errors.Is(err, ErrPoolCompleted) {
			t.Errorf("%s racing a winner: %v", name, err)
		}
	}
	if ideas.rejected || ideas.deleted {
		t.Fatal("idea changed after the pool completed under the lock")
	}
	ideas.status = ""
	pool.Status = models.GroupStatusCompleted
	if err := svc.VoteIdea(id, f.bobID, 1); !errors.Is(err, ErrPoolCompleted) {
		t.Fatalf("vote in completed pool: %v", err)
	}
	if _, err := svc.GetGroupIdeas(repository.PoolScope(f.groupID, pool.ID), f.aliceID); !errors.Is(err, ErrAntiSpoilerTriggered) {
		t.Fatalf("recipient lists pool ideas: %v", err)
	}
	if _, err := svc.AddManualIdea(repository.PoolScope(f.groupID, pool.ID), f.aliceID, "x", "", "", "", "", decimal.Zero); !errors.Is(err, ErrAntiSpoilerTriggered) {
		t.Fatalf("recipient adds pool idea: %v", err)
	}
	if _, err := svc.GetGroupIdeas(repository.GroupScope(f.groupID), f.bobID); !errors.Is(err, ErrWrongGroupMode) {
		t.Fatalf("group scope on MULTI: %v", err)
	}
	if _, err := NewIdeaService(ideas, f.groups, f.pools, &aiGenerator{}).GenerateAIIdeas(context.Background(), repository.PoolScope(f.groupID, pool.ID), f.aliceID); !errors.Is(err, ErrAntiSpoilerTriggered) {
		t.Fatalf("recipient triggers AI: %v", err)
	}
}

func TestAIPromptUsesPoolData(t *testing.T) {
	s := &ideaService{}
	sc := giftScope{
		Group: &models.Group{Mode: models.GroupModeMulti, Description: "Ofis yılbaşı", TargetInterests: "GrupAlanı", TotalBudget: decimal.NewFromInt(99)},
		Pool:  &models.GiftPool{Interests: "Bitki\nKitap", TotalBudget: decimal.NewFromInt(1500), Recipient: models.User{Name: "Zeynep"}},
	}
	prompt := s.buildAIPrompt(sc, []string{"Bonsai"}, []string{"Saksı"})
	for _, want := range []string{"İsim: Zeynep", "Bitki\nKitap", "1500.00 TL", "Ofis yılbaşı", "Bonsai", "Saksı"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "GrupAlanı") || strings.Contains(prompt, "99.00") {
		t.Fatal("prompt uses group-level fields for a pool")
	}
}

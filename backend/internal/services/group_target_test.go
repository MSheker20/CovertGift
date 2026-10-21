package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type emailUserRepository struct {
	repository.UserRepository
	users map[string]*models.User
	err   error
}

func (r *emailUserRepository) FindByEmail(email string) (*models.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.users[strings.ToLower(email)], nil
}

type targetRepository struct {
	interestsRepository
	targetIsMember bool
	created        *models.Group
	linkedID       *uuid.UUID
	addedMember    *models.GroupMember
	addMemberErr   error
}

func (r *targetRepository) CreateGroupWithAdmin(group *models.Group, _ uuid.UUID) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.created = group
	return nil
}

func (r *targetRepository) SetTargetUser(_ uuid.UUID, targetID uuid.UUID, guard func(*uuid.UUID, bool) error) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	if err := guard(r.group.TargetUserID, r.targetIsMember); err != nil {
		return err
	}
	r.linkedID = &targetID
	return nil
}

func (r *targetRepository) AddMember(member *models.GroupMember) error {
	if r.addMemberErr != nil {
		return r.addMemberErr
	}
	r.addedMember = member
	return nil
}

func TestCreateGroupResolvesTargetEmail(t *testing.T) {
	creatorID, targetID, otherID := uuid.New(), uuid.New(), uuid.New()
	databaseErr := errors.New("database failure")
	users := map[string]*models.User{
		"ahmet@demo.com":   {ID: targetID, Email: "ahmet@demo.com"},
		"creator@demo.com": {ID: creatorID, Email: "creator@demo.com"},
	}
	for _, tc := range []struct {
		name, email  string
		targetUserID *uuid.UUID
		lookupErr    error
		wantErr      error
		wantTarget   *uuid.UUID
	}{
		{name: "no target keeps name only"},
		{name: "email resolves to user id", email: "ahmet@demo.com", wantTarget: &targetID},
		{name: "email is trimmed and case insensitive", email: "  Ahmet@Demo.com ", wantTarget: &targetID},
		{name: "legacy uuid still accepted", targetUserID: &otherID, wantTarget: &otherID},
		{name: "unregistered email", email: "nobody@demo.com", wantErr: ErrTargetUserNotFound},
		{name: "creator cannot target self by email", email: "creator@demo.com", wantErr: ErrSelfTarget},
		{name: "creator cannot target self by uuid", targetUserID: &creatorID, wantErr: ErrSelfTarget},
		{name: "email and uuid together rejected", email: "ahmet@demo.com", targetUserID: &otherID, wantErr: ErrTargetInputConflict},
		{name: "lookup error", email: "ahmet@demo.com", lookupErr: databaseErr, wantErr: databaseErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &targetRepository{}
			svc := NewGroupService(repo, &emailUserRepository{users: users, err: tc.lookupErr})
			group, err := svc.CreateGroup(creatorID, "Sürpriz", "", "Ahmet", "", tc.targetUserID, tc.email, decimal.Zero)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if repo.created != nil {
					t.Fatal("rejected request created a group")
				}
				return
			}
			if group.TargetPersonName != "Ahmet" {
				t.Fatalf("target name = %q, want the typed name", group.TargetPersonName)
			}
			switch {
			case tc.wantTarget == nil && group.TargetUserID != nil:
				t.Fatalf("target = %v, want none", *group.TargetUserID)
			case tc.wantTarget != nil && (group.TargetUserID == nil || *group.TargetUserID != *tc.wantTarget):
				t.Fatalf("target = %v, want %v", group.TargetUserID, *tc.wantTarget)
			}
		})
	}
}

func TestLinkTargetPermissionsAndRules(t *testing.T) {
	groupID, adminID, targetID, existingTargetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	databaseErr := errors.New("database failure")
	users := map[string]*models.User{
		"ahmet@demo.com": {ID: targetID, Name: "Ahmet", Email: "ahmet@demo.com"},
		"admin@demo.com": {ID: adminID, Email: "admin@demo.com"},
	}
	for _, tc := range []struct {
		name, role, email              string
		alreadyLinked, callerIsTarget  bool
		targetIsMember, missing        bool
		groupErr, memberErr, lookupErr error
		updateErr                      error
		wantErr                        error
	}{
		{name: "admin links registered user", role: "admin", email: "ahmet@demo.com"},
		{name: "email is trimmed", role: "admin", email: " ahmet@demo.com "},
		{name: "member cannot link", role: "member", email: "ahmet@demo.com", wantErr: ErrUnauthorizedTargetUpdate},
		{name: "outsider cannot link", email: "ahmet@demo.com", wantErr: ErrNotGroupMember},
		{name: "current target is blocked", role: "admin", alreadyLinked: true, callerIsTarget: true, email: "ahmet@demo.com", wantErr: ErrAntiSpoilerTriggered},
		{name: "unknown group", missing: true, email: "ahmet@demo.com", wantErr: ErrGroupNotFound},
		{name: "unregistered email", role: "admin", email: "nobody@demo.com", wantErr: ErrTargetUserNotFound},
		{name: "admin cannot target self", role: "admin", email: "admin@demo.com", wantErr: ErrSelfTarget},
		{name: "existing target cannot be replaced", role: "admin", alreadyLinked: true, email: "ahmet@demo.com", wantErr: ErrTargetAlreadyLinked},
		{name: "group member cannot become target", role: "admin", targetIsMember: true, email: "ahmet@demo.com", wantErr: ErrTargetIsMember},
		{name: "group lookup error", groupErr: databaseErr, email: "ahmet@demo.com", wantErr: databaseErr},
		{name: "membership lookup error", memberErr: databaseErr, email: "ahmet@demo.com", wantErr: databaseErr},
		{name: "user lookup error", role: "admin", lookupErr: databaseErr, email: "ahmet@demo.com", wantErr: databaseErr},
		{name: "write error", role: "admin", updateErr: databaseErr, email: "ahmet@demo.com", wantErr: databaseErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callerID := adminID
			group := &models.Group{ID: groupID, TargetPersonName: "Ahmet abi"}
			if tc.alreadyLinked {
				group.TargetUserID = &existingTargetID
				if tc.callerIsTarget {
					callerID = existingTargetID
				}
			}
			repo := &targetRepository{targetIsMember: tc.targetIsMember}
			repo.group, repo.groupErr, repo.memberErr, repo.updateErr = group, tc.groupErr, tc.memberErr, tc.updateErr
			if tc.missing {
				repo.group = nil
			}
			if tc.role != "" {
				repo.member = &models.GroupMember{Role: tc.role, UserID: callerID, GroupID: groupID}
			}
			svc := NewGroupService(repo, &emailUserRepository{users: users, err: tc.lookupErr})
			linked, err := svc.LinkTarget(groupID, callerID, tc.email)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if repo.linkedID != nil {
					t.Fatal("rejected request linked a target")
				}
				return
			}
			if repo.linkedID == nil || *repo.linkedID != targetID {
				t.Fatalf("linked = %v, want %v", repo.linkedID, targetID)
			}
			if linked.TargetUserID == nil || *linked.TargetUserID != targetID || linked.TargetPersonName != "Ahmet abi" {
				t.Fatalf("unexpected group after linking: %+v", linked)
			}
		})
	}
}

func TestAddMemberMapsLockedTargetCheck(t *testing.T) {
	groupID, adminID, newID := uuid.New(), uuid.New(), uuid.New()
	repo := &targetRepository{addMemberErr: repository.ErrMemberIsGroupTarget}
	repo.group = &models.Group{ID: groupID}
	repo.member = &models.GroupMember{Role: "admin", UserID: adminID, GroupID: groupID}
	users := map[string]*models.User{"new@demo.com": {ID: newID}}
	// GetMember returns the caller for every lookup, so make the "already member" check pass.
	svc := NewGroupService(&memberLookupRepository{targetRepository: repo, callerID: adminID}, &emailUserRepository{users: users})
	if _, err := svc.AddMemberByEmail(groupID, adminID, "new@demo.com"); !errors.Is(err, ErrCannotAddTargetAsMember) {
		t.Fatalf("error = %v, want %v", err, ErrCannotAddTargetAsMember)
	}
}

type memberLookupRepository struct {
	*targetRepository
	callerID uuid.UUID
}

func (r *memberLookupRepository) GetMember(_ uuid.UUID, userID uuid.UUID) (*models.GroupMember, error) {
	if userID == r.callerID {
		return r.member, nil
	}
	return nil, nil
}

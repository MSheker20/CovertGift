package services

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

type interestsRepository struct {
	repository.GroupRepository
	group     *models.Group
	member    *models.GroupMember
	groupErr  error
	memberErr error
	updateErr error
	written   bool
}

func (r *interestsRepository) GetGroupByID(uuid.UUID) (*models.Group, error) {
	return r.group, r.groupErr
}

func (r *interestsRepository) GetMember(uuid.UUID, uuid.UUID) (*models.GroupMember, error) {
	return r.member, r.memberErr
}

func (r *interestsRepository) MutateInterests(_ uuid.UUID, change func([]string) ([]string, error)) ([]string, error) {
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	items, err := change(models.ParseInterests(r.group.TargetInterests))
	if err == nil {
		r.written = true
		r.group.TargetInterests = strings.Join(items, "\n")
	}
	return items, err
}

func TestInterestPermissionsAndMutation(t *testing.T) {
	groupID, userID, targetID := uuid.New(), uuid.New(), uuid.New()
	databaseErr := errors.New("database failure")
	for _, tc := range []struct {
		name, role, operation, interest string
		target, missing                 bool
		groupErr, memberErr, updateErr  error
		wantErr                         error
		want                            []string
	}{
		{name: "member adds without replacing existing items", role: "member", interest: " Kamp ", want: []string{"Kahve", "Müzik", "Kamp"}},
		{name: "admin can add", role: "admin", interest: "Kamp", want: []string{"Kahve", "Müzik", "Kamp"}},
		{name: "member cannot delete any item", role: "member", operation: "delete", interest: "Kahve", wantErr: ErrUnauthorizedInterestDelete},
		{name: "admin deletes one item", role: "admin", operation: "delete", interest: "Kahve", want: []string{"Müzik"}},
		{name: "duplicate case insensitive", role: "member", interest: "kahve", wantErr: ErrDuplicateInterest},
		{name: "empty add cannot clear list", role: "member", interest: "  ", wantErr: ErrInvalidInterest},
		{name: "single item required", role: "member", interest: "Kamp, Trekking", wantErr: ErrInvalidInterest},
		{name: "newline injection rejected", role: "member", interest: "Kamp\nTrekking", wantErr: ErrInvalidInterest},
		{name: "long item rejected", role: "member", interest: strings.Repeat("ö", 101), wantErr: ErrInvalidInterest},
		{name: "missing item", role: "admin", operation: "delete", interest: "Yok", wantErr: ErrInterestNotFound},
		{name: "target cannot add", role: "member", target: true, interest: "Kamp", wantErr: ErrAntiSpoilerTriggered},
		{name: "target cannot delete even as admin", role: "admin", target: true, operation: "delete", interest: "Kahve", wantErr: ErrAntiSpoilerTriggered},
		{name: "outsider cannot add", interest: "Kamp", wantErr: ErrNotGroupMember},
		{name: "outsider cannot delete", operation: "delete", interest: "Kahve", wantErr: ErrNotGroupMember},
		{name: "unknown group", missing: true, interest: "Kamp", wantErr: ErrGroupNotFound},
		{name: "group lookup error", groupErr: databaseErr, interest: "Kamp", wantErr: databaseErr},
		{name: "membership lookup error", memberErr: databaseErr, interest: "Kamp", wantErr: databaseErr},
		{name: "write error", role: "member", updateErr: databaseErr, interest: "Kamp", wantErr: databaseErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := userID
			if tc.target {
				id = targetID
			}
			repo := &interestsRepository{group: &models.Group{ID: groupID, TargetUserID: &targetID, TargetInterests: "Kahve, Müzik"}, groupErr: tc.groupErr, memberErr: tc.memberErr, updateErr: tc.updateErr}
			if tc.missing {
				repo.group = nil
			}
			if tc.role != "" {
				repo.member = &models.GroupMember{Role: tc.role, UserID: id, GroupID: groupID}
			}
			svc := NewGroupService(repo, nil, nil)
			var items []string
			var err error
			if tc.operation == "delete" {
				items, err = svc.DeleteInterest(groupID, id, tc.interest)
			} else {
				items, err = svc.AddInterest(groupID, id, tc.interest)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && repo.written {
				t.Fatal("rejected operation modified the list")
			}
			if tc.wantErr == nil && (!repo.written || !reflect.DeepEqual(items, tc.want)) {
				t.Fatalf("items = %v, want %v", items, tc.want)
			}
		})
	}
}

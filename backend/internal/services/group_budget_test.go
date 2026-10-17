package services

import (
	"errors"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type budgetRepository struct {
	interestsRepository
	updatedID uuid.UUID
}

func (r *budgetRepository) UpdateBudget(id uuid.UUID, budget decimal.Decimal) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.written, r.updatedID = true, id
	r.group.TotalBudget = budget
	return nil
}

func TestUpdateBudgetPermissionsAndMoneyValidation(t *testing.T) {
	groupID, userID, targetID := uuid.New(), uuid.New(), uuid.New()
	databaseErr := errors.New("database failure")
	for _, tc := range []struct {
		name, role, budget             string
		target, missing                bool
		groupErr, memberErr, updateErr error
		wantErr                        error
	}{
		{name: "admin updates exact money", role: "admin", budget: "4000.50"},
		{name: "zero budget", role: "admin", budget: "0"},
		{name: "maximum numeric 10 2", role: "admin", budget: "99999999.99"},
		{name: "member denied", role: "member", budget: "4000", wantErr: ErrUnauthorizedBudgetUpdate},
		{name: "outsider denied", budget: "4000", wantErr: ErrNotGroupMember},
		{name: "target denied even as admin", role: "admin", target: true, budget: "4000", wantErr: ErrAntiSpoilerTriggered},
		{name: "negative denied", role: "admin", budget: "-0.01", wantErr: ErrInvalidBudget},
		{name: "out of range", role: "admin", budget: "100000000", wantErr: ErrInvalidBudget},
		{name: "fractional cent", role: "admin", budget: "1.001", wantErr: ErrInvalidBudget},
		{name: "unknown group", missing: true, budget: "4000", wantErr: ErrGroupNotFound},
		{name: "group lookup error", groupErr: databaseErr, budget: "4000", wantErr: databaseErr},
		{name: "membership lookup error", memberErr: databaseErr, budget: "4000", wantErr: databaseErr},
		{name: "write error", role: "admin", updateErr: databaseErr, budget: "4000", wantErr: databaseErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := userID
			if tc.target {
				id = targetID
			}
			repo := &budgetRepository{interestsRepository: interestsRepository{
				group:    &models.Group{ID: groupID, TargetUserID: &targetID, TotalBudget: decimal.NewFromInt(3000)},
				groupErr: tc.groupErr, memberErr: tc.memberErr, updateErr: tc.updateErr,
			}}
			if tc.missing {
				repo.group = nil
			}
			if tc.role != "" {
				repo.member = &models.GroupMember{Role: tc.role, UserID: id, GroupID: groupID}
			}
			budget := decimal.RequireFromString(tc.budget)
			err := NewGroupService(repo, nil).UpdateBudget(groupID, id, budget)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && repo.written {
				t.Fatal("rejected update changed the budget")
			}
			if tc.wantErr == nil && (!repo.written || repo.updatedID != groupID || !repo.group.TotalBudget.Equal(budget)) {
				t.Fatal("wrong group or budget updated")
			}
		})
	}
}

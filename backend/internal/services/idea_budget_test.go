package services

import (
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type budgetIdeaRepository struct {
	repository.IdeaRepository
	ideas []repository.IdeaWithScore
}

func (r *budgetIdeaRepository) GetIdeasWithScores(repository.IdeaScope, uuid.UUID) ([]repository.IdeaWithScore, error) {
	return r.ideas, nil
}

func TestGiftWarningsUseCurrentBudget(t *testing.T) {
	for _, tc := range []struct {
		name, budget, price string
		want                bool
	}{
		{"lowered budget", "1000", "1500", true},
		{"raised budget", "2000", "1500", false},
		{"equal to budget", "1500", "1500", false},
		{"zero budget disables warnings", "0", "1500", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID, userID := uuid.New(), uuid.New()
			groups := &interestsRepository{group: &models.Group{ID: groupID, TotalBudget: decimal.RequireFromString(tc.budget)}, member: &models.GroupMember{Role: "member"}}
			ideas := &budgetIdeaRepository{ideas: []repository.IdeaWithScore{{GiftIdea: models.GiftIdea{EstimatedPrice: decimal.RequireFromString(tc.price), IsOverBudget: !tc.want}}}}
			got, err := NewIdeaService(ideas, groups, nil, nil).GetGroupIdeas(repository.GroupScope(groupID), userID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].IsOverBudget != tc.want {
				t.Fatalf("warning = %v, want %v", got, tc.want)
			}
		})
	}
}

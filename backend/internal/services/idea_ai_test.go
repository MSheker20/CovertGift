package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type aiIdeaRepository struct {
	repository.IdeaRepository
	existing, rejected []string
	written            []models.GiftIdea
}

func (r *aiIdeaRepository) GetGroupIdeaTitlesByStatus(uuid.UUID) ([]string, []string, error) {
	return r.existing, r.rejected, nil
}
func (r *aiIdeaRepository) CreateIdeasBatch(ideas []models.GiftIdea) error {
	r.written = ideas
	return nil
}

type aiGenerator struct {
	items []gemini.AIIdeaSuggestion
	calls int
}

func (g *aiGenerator) GenerateGiftIdeas(context.Context, string) ([]gemini.AIIdeaSuggestion, error) {
	g.calls++
	return g.items, nil
}
func aiProduct(title, query string) gemini.AIIdeaSuggestion {
	return gemini.AIIdeaSuggestion{Title: title, SearchQuery: query, Description: "Kamp ve kahve hobisine uygun taşınabilir ekipman.", EstimatedPrice: decimal.NewFromInt(1500), Sources: []gemini.Source{{Title: "trendyol.com", URL: "https://www.trendyol.com/test"}}, SearchSuggestions: "<div>Google</div>"}
}

func TestAIRejectsDuplicatesAndInvalidProductsBeforeSaving(t *testing.T) {
	id, user := uuid.New(), uuid.New()
	groups := &interestsRepository{group: &models.Group{ID: id, TotalBudget: decimal.NewFromInt(1000)}, member: &models.GroupMember{Role: "member"}}
	ideas := &aiIdeaRepository{existing: []string{"KAHVE-DEĞİRMENİ"}, rejected: []string{"KAMP TERMOSU"}}
	invalid := aiProduct("Boş ürün", "Boş ürün")
	invalid.EstimatedPrice = decimal.Zero
	unsourced := aiProduct("Kaynaksız ürün", "Kaynaksız ürün")
	unsourced.Sources = nil
	generator := &aiGenerator{items: []gemini.AIIdeaSuggestion{
		aiProduct("kahve değirmeni", "kahve değirmeni"),
		aiProduct("kamp termosu", "kamp termosu"),
		invalid, unsourced,
		aiProduct("AeroPress Go", "AeroPress Go Kahve Seti"),
		aiProduct("AeroPress taşınabilir set", "AeroPress Go Kahve Seti"),
	}}
	got, err := NewIdeaService(ideas, groups, generator).GenerateAIIdeas(context.Background(), id, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(ideas.written) != 1 || got[0].Title != "AeroPress Go" || !got[0].IsOverBudget || got[0].ProductURL != "" || len(got[0].AISources) != 1 || got[0].AISearchSuggestions == "" {
		t.Fatalf("unexpected saved ideas: %+v", got)
	}
}

func TestAIStillProtectsSurpriseAndMembership(t *testing.T) {
	for _, tc := range []struct {
		name           string
		target, member bool
		want           error
	}{
		{"target", true, true, ErrAntiSpoilerTriggered},
		{"outsider", false, false, ErrNotGroupMember},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, user := uuid.New(), uuid.New()
			group := &models.Group{ID: id}
			if tc.target {
				group.TargetUserID = &user
			}
			groups := &interestsRepository{group: group}
			if tc.member {
				groups.member = &models.GroupMember{Role: "admin"}
			}
			generator := &aiGenerator{}
			_, err := NewIdeaService(&aiIdeaRepository{}, groups, generator).GenerateAIIdeas(context.Background(), id, user)
			if !errors.Is(err, tc.want) || generator.calls != 0 {
				t.Fatalf("err = %v, generator calls = %d", err, generator.calls)
			}
		})
	}
}

func TestAIPromptPreservesLocalMarketAndBudgetRules(t *testing.T) {
	s := &ideaService{}
	prompt := s.buildAIPrompt(&models.Group{TargetInterests: "Kahve\nKamp", TotalBudget: decimal.NewFromInt(3000)}, []string{"Termos"}, []string{"Çadır"})
	for _, want := range []string{"Türkiye", "TRY/TL", "Kahve\nKamp", "3000.00 TL", "Termos", "Çadır", "öneri sayısına bölme", "ürün uydurma"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

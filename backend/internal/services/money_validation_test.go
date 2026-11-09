package services

import (
	"errors"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestValidMoney(t *testing.T) {
	for _, tc := range []struct {
		amount string
		want   bool
	}{
		{"0", true},
		{"1450.50", true},
		{"99999999.99", true},
		{"-0.01", false},
		{"100000000", false},
		{"10.005", false},
	} {
		if got := validMoney(decimal.RequireFromString(tc.amount)); got != tc.want {
			t.Errorf("validMoney(%s) = %v, want %v", tc.amount, got, tc.want)
		}
	}
}

// Geçersiz bütçe, depoya dokunulmadan reddedilmelidir (depolar nil).
func TestCreateGroupRejectsInvalidBudget(t *testing.T) {
	for _, budget := range []string{"-1", "100000000", "1.234"} {
		_, err := NewGroupService(nil, nil).CreateGroup(uuid.New(), "Grup", "", "Hedef", "", nil, "", decimal.RequireFromString(budget), nil)
		if !errors.Is(err, ErrInvalidBudget) {
			t.Errorf("budget %s: err = %v, want ErrInvalidBudget", budget, err)
		}
	}
}

// Geçersiz fiyat, fikir kaydedilmeden reddedilmelidir (fikir deposu boş gömülü arayüz).
func TestAddManualIdeaRejectsInvalidPrice(t *testing.T) {
	groupID := uuid.New()
	groups := &interestsRepository{group: &models.Group{ID: groupID}, member: &models.GroupMember{Role: "member"}}
	svc := NewIdeaService(&budgetIdeaRepository{}, groups, nil)
	for _, price := range []string{"-5", "100000000", "9.999"} {
		_, err := svc.AddManualIdea(groupID, uuid.New(), "Termos", "", "", "", "", decimal.RequireFromString(price))
		if !errors.Is(err, ErrInvalidPrice) {
			t.Errorf("price %s: err = %v, want ErrInvalidPrice", price, err)
		}
	}
}

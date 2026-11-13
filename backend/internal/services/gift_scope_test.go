package services

import (
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestVetoElectorateAndMajority(t *testing.T) {
	group := giftScope{Group: &models.Group{}}
	pool := giftScope{Group: &models.Group{}, Pool: &models.GiftPool{}}
	for _, tc := range []struct {
		scope      giftScope
		members    int64
		downvotes  int64
		electorate int64
		rejected   bool
	}{
		{group, 3, 1, 3, false},
		{group, 3, 2, 3, true},
		{group, 4, 2, 4, false}, // 2 > 4/2 değil
		{pool, 4, 1, 3, false},  // alıcı oy kullanamaz: 3 seçmen
		{pool, 4, 2, 3, true},
		{pool, 2, 1, 1, true}, // yalnızca bir seçmen
		{pool, 0, 0, 0, false},
	} {
		electorate := tc.scope.VetoElectorate(tc.members)
		if electorate != tc.electorate {
			t.Errorf("pool=%v members=%d: electorate %d, want %d", tc.scope.IsPool(), tc.members, electorate, tc.electorate)
		}
		if got := MajorityVeto(tc.downvotes, electorate); got != tc.rejected {
			t.Errorf("pool=%v members=%d downvotes=%d: rejected %v, want %v", tc.scope.IsPool(), tc.members, tc.downvotes, got, tc.rejected)
		}
	}
}

func TestPerPersonShare(t *testing.T) {
	budget := decimal.RequireFromString("1000")
	if PerPersonShare(budget, 0) != nil {
		t.Fatal("0 katkı veren: pay nil olmalı")
	}
	for contributors, want := range map[int64]string{1: "1000", 3: "333.33", 4: "250"} {
		if got := PerPersonShare(budget, contributors); got == nil || got.String() != want {
			t.Errorf("%d katkı veren: %v, want %s", contributors, got, want)
		}
	}
}

func TestGiftScopeReadsPoolOrGroup(t *testing.T) {
	date := models.Date{Time: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)}
	group := &models.Group{ID: uuid.New(), TargetPersonName: "Ahmet", TargetInterests: "Kamp", TotalBudget: decimal.NewFromInt(100), Status: models.GroupStatusCompleted}
	pool := &models.GiftPool{ID: uuid.New(), Interests: "Kahve\nKitap", TotalBudget: decimal.NewFromInt(500), EventDate: &date, Status: models.GroupStatusActive, Recipient: models.User{Name: "Zeynep"}}
	g, p := giftScope{Group: group}, giftScope{Group: group, Pool: pool}
	if g.RecipientName() != "Ahmet" || p.RecipientName() != "Zeynep" {
		t.Fatal("alıcı adı")
	}
	if !g.Budget().Equal(decimal.NewFromInt(100)) || !p.Budget().Equal(decimal.NewFromInt(500)) {
		t.Fatal("bütçe")
	}
	if len(p.Interests()) != 2 || g.Interests()[0] != "Kamp" || p.EventDate() != &date {
		t.Fatal("ilgi alanı / tarih")
	}
	if !g.Completed() || p.Completed() || g.CompletedErr() != ErrGroupCompleted || p.CompletedErr() != ErrPoolCompleted {
		t.Fatal("durum")
	}
	if g.IdeaScope() != repository.GroupScope(group.ID) || p.IdeaScope().PoolID == nil || *p.IdeaScope().PoolID != pool.ID {
		t.Fatal("kapsam")
	}
}

func TestEffectiveManager(t *testing.T) {
	admin, alice, bob := uuid.New(), uuid.New(), uuid.New()
	memberPool := &models.GiftPool{RecipientUserID: alice, ManagerUserID: &bob} // manager_user_id yok sayılır
	if m := effectiveManagerID(memberPool, admin); m == nil || *m != admin || !canManagePool(memberPool, admin, admin) || canManagePool(memberPool, admin, bob) {
		t.Fatal("üye alıcının havuzunu grup yöneticisi yönetir")
	}
	adminPool := &models.GiftPool{RecipientUserID: admin, ManagerUserID: &bob}
	if m := effectiveManagerID(adminPool, admin); m == nil || *m != bob || canManagePool(adminPool, admin, admin) || !canManagePool(adminPool, admin, bob) {
		t.Fatal("grup yöneticisinin havuzunu atanan yönetici yönetir")
	}
	orphan := &models.GiftPool{RecipientUserID: admin}
	if effectiveManagerID(orphan, admin) != nil || canManagePool(orphan, admin, admin) || canManagePool(orphan, admin, bob) {
		t.Fatal("yöneticisi silinen havuzu kimse yönetemez")
	}
}

func TestViewerStatusAndNextEventDate(t *testing.T) {
	me, other1, other2 := uuid.New(), uuid.New(), uuid.New()
	t1 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	d1 := models.Date{Time: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)}
	completed := func(r uuid.UUID, at time.Time) models.GiftPool {
		return models.GiftPool{RecipientUserID: r, Status: models.GroupStatusCompleted, CompletedAt: &at}
	}
	for _, tc := range []struct {
		name  string
		pools []models.GiftPool
		want  string
		at    *time.Time
	}{
		{"no pools", nil, models.GroupStatusActive, nil},
		{"only own pool (completed)", []models.GiftPool{completed(me, t1)}, models.GroupStatusActive, nil},
		{"all visible completed", []models.GiftPool{completed(other1, t1), completed(other2, t2)}, models.GroupStatusCompleted, &t2},
		{"own pool open is ignored", []models.GiftPool{completed(other1, t1), {RecipientUserID: me, Status: models.GroupStatusActive}}, models.GroupStatusCompleted, &t1},
		{"one visible open", []models.GiftPool{completed(other1, t1), {RecipientUserID: other2, Status: models.GroupStatusActive}}, models.GroupStatusActive, nil},
	} {
		status, at := viewerStatus(tc.pools, me)
		if status != tc.want || (tc.at == nil) != (at == nil) || (at != nil && !at.Equal(*tc.at)) {
			t.Errorf("%s: %s %v, want %s %v", tc.name, status, at, tc.want, tc.at)
		}
	}
	today := models.Date{Time: time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC)}
	overdue := models.Date{Time: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)}
	upcoming := models.Date{Time: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)}
	later := models.Date{Time: time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)}
	mineSoon := models.Date{Time: time.Date(2026, 12, 6, 0, 0, 0, 0, time.UTC)}
	active := func(r uuid.UUID, d *models.Date) models.GiftPool {
		return models.GiftPool{RecipientUserID: r, Status: models.GroupStatusActive, EventDate: d}
	}
	for _, tc := range []struct {
		name  string
		pools []models.GiftPool
		want  string
	}{
		{"one overdue + one upcoming → upcoming", []models.GiftPool{active(other1, &overdue), active(other2, &later), active(other2, &upcoming)}, upcoming.String()},
		{"only overdue → earliest overdue", []models.GiftPool{active(other1, &overdue), active(other2, &d1)}, overdue.String()},
		{"today counts as upcoming", []models.GiftPool{active(other1, &overdue), active(other2, &today)}, today.String()},
		{"own pool never counted (upcoming)", []models.GiftPool{active(me, &mineSoon), active(other1, &later)}, later.String()},
		{"own pool never counted (overdue)", []models.GiftPool{active(me, &overdue), active(other1, &upcoming)}, upcoming.String()},
		{"completed pools ignored", []models.GiftPool{{RecipientUserID: other1, Status: models.GroupStatusCompleted, EventDate: &mineSoon}, active(other2, &later)}, later.String()},
		{"only own pool → none", []models.GiftPool{active(me, &mineSoon)}, ""},
		{"no dates → none", []models.GiftPool{active(other1, nil)}, ""},
	} {
		got := ""
		if d := nextEventDate(tc.pools, me, today); d != nil {
			got = d.String()
		}
		if got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	if turkeyToday(time.Date(2026, 12, 4, 22, 30, 0, 0, time.UTC)).String() != "2026-12-05" {
		t.Fatal("bugün UTC+3'e göre hesaplanmalı")
	}
}

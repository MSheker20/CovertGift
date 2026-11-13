package services

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
)

type reminderRepository struct {
	repository.NotificationRepository
	pools    []repository.ReminderPool
	members  []repository.ReminderMember
	inserted []models.Notification
}

func (r *reminderRepository) ReminderGroups(models.Date, models.Date) ([]repository.ReminderGroup, error) {
	return nil, nil
}
func (r *reminderRepository) ReminderPools(models.Date, models.Date) ([]repository.ReminderPool, error) {
	return r.pools, nil
}
func (r *reminderRepository) MembersOfGroups([]uuid.UUID) ([]repository.ReminderMember, error) {
	return r.members, nil
}
func (r *reminderRepository) Insert(n *models.Notification) (bool, error) {
	r.inserted = append(r.inserted, *n)
	return true, nil
}

type nopNotifier struct{}

func (nopNotifier) Publish(uuid.UUID) {}

func TestReminderForUsesKeyInDedupe(t *testing.T) {
	pool := uuid.New()
	today := models.Date{Time: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)}
	event := models.Date{Time: today.AddDate(0, 0, 3)}
	r, ok := reminderFor(pool, event, models.GroupStatusActive, today)
	if !ok || r.dedupeKey != "reminder:3d:"+pool.String()+":"+event.String() || r.adminsOnly {
		t.Fatalf("3 gün kala: %+v %v", r, ok)
	}
	past := models.Date{Time: today.AddDate(0, 0, -1)}
	if r, ok := reminderFor(pool, past, models.GroupStatusActive, today); !ok || !r.adminsOnly || !strings.HasPrefix(r.dedupeKey, "overdue:"+pool.String()) {
		t.Fatalf("süre doldu: %+v %v", r, ok)
	}
	if _, ok := reminderFor(pool, past, models.GroupStatusCompleted, today); ok {
		t.Fatal("tamamlanan havuz için süre doldu üretildi")
	}
}

func TestPoolRemindersSkipRecipientAndGoToEffectiveManager(t *testing.T) {
	group, admin, alice, bob, carol := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	names := map[uuid.UUID]string{admin: "admin", alice: "alice", bob: "bob", carol: "carol"}
	now := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	today := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	soon := models.Date{Time: today.AddDate(0, 0, 1)}
	past := models.Date{Time: today.AddDate(0, 0, -2)}
	alicePool, adminPool, orphanPool := uuid.New(), uuid.New(), uuid.New()
	members := []repository.ReminderMember{
		{GroupID: group, UserID: admin, Role: "admin"}, {GroupID: group, UserID: alice, Role: "member"},
		{GroupID: group, UserID: bob, Role: "member"}, {GroupID: group, UserID: carol, Role: "member"},
	}
	run := func(pools ...repository.ReminderPool) map[uuid.UUID][]string {
		t.Helper()
		repo := &reminderRepository{pools: pools, members: members}
		s := NewReminderScheduler(repo, nopNotifier{})
		s.Now = func() time.Time { return now }
		if _, err := s.RunOnce(); err != nil {
			t.Fatal(err)
		}
		out := map[uuid.UUID][]string{}
		for _, n := range repo.inserted {
			if n.PoolID == nil || n.Params["recipient_name"] == nil || !strings.Contains(*n.DedupeKey, n.PoolID.String()) {
				t.Fatalf("havuz bilgisi eksik: %+v", n)
			}
			out[*n.PoolID] = append(out[*n.PoolID], names[n.UserID]+":"+n.Type)
		}
		for k := range out {
			sort.Strings(out[k])
		}
		return out
	}
	got := run(
		repository.ReminderPool{ID: alicePool, GroupID: group, RecipientUserID: alice, RecipientName: "Alice", EventDate: soon, Status: models.GroupStatusActive},
		repository.ReminderPool{ID: adminPool, GroupID: group, RecipientUserID: admin, ManagerUserID: &bob, RecipientName: "Admin", EventDate: soon, Status: models.GroupStatusActive},
	)
	if s := strings.Join(got[alicePool], ","); s != "admin:event_reminder,bob:event_reminder,carol:event_reminder" {
		t.Fatalf("Alice havuzu: %s", s)
	}
	if s := strings.Join(got[adminPool], ","); s != "alice:event_reminder,bob:event_reminder,carol:event_reminder" {
		t.Fatalf("Admin havuzu: %s", s)
	}
	got = run(
		repository.ReminderPool{ID: alicePool, GroupID: group, RecipientUserID: alice, RecipientName: "Alice", EventDate: past, Status: models.GroupStatusActive},
		repository.ReminderPool{ID: adminPool, GroupID: group, RecipientUserID: admin, ManagerUserID: &bob, RecipientName: "Admin", EventDate: past, Status: models.GroupStatusActive},
		repository.ReminderPool{ID: orphanPool, GroupID: group, RecipientUserID: admin, RecipientName: "Admin", EventDate: past, Status: models.GroupStatusActive},
	)
	if s := strings.Join(got[alicePool], ","); s != "admin:winner_overdue" {
		t.Fatalf("Alice havuzu süre doldu: %s", s)
	}
	if s := strings.Join(got[adminPool], ","); s != "bob:winner_overdue" {
		t.Fatalf("Admin havuzu süre doldu: %s", s)
	}
	if len(got[orphanPool]) != 0 {
		t.Fatalf("yöneticisiz havuz için süre doldu üretildi: %v", got[orphanPool])
	}
}

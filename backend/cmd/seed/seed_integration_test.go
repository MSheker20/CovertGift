package main

// Seed entegrasyon testi. Yalnızca TEST_DATABASE_URL tanımlıysa çalışır ve veritabanı
// adı "_test" ile bitmek zorundadır (tablolar boşaltılır). Ayrıntı: cmd/api/antispoiler_integration_test.go.

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if raw == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil; seed entegrasyon testi atlandı")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if name := strings.TrimPrefix(u.Path, "/"); !strings.HasSuffix(name, "_test") {
		t.Fatalf("TEST_DATABASE_URL veritabanı adı %q; adı _test ile bitmelidir", name)
	}
	db, err := gorm.Open(postgres.Open(raw), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := models.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("TRUNCATE TABLE votes, gift_ideas, group_members, groups, users RESTART IDENTITY CASCADE").Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func counts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, table := range []string{"users", "groups", "group_members", "gift_ideas", "votes", "gift_pools", "pool_contributions"} {
		var n int64
		if err := db.Table(table).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		out[table] = n
	}
	return out
}

func TestSeedIsAdditiveIdempotentAndRespectsAntiSpoiler(t *testing.T) {
	db := openTestDB(t)

	// Önceden var olan gerçek bir kullanıcı seed'den etkilenmemeli.
	existing := &models.User{Name: "Gerçek Kullanıcı", Email: "real@example.com", PasswordHash: "x"}
	if err := db.Create(existing).Error; err != nil {
		t.Fatal(err)
	}

	if err := seed(db, "Demo1234!"); err != nil {
		t.Fatalf("ilk seed: %v", err)
	}
	// SINGLE grup: 3 üye, 3 fikir, 4 oy. MULTI "Ofis Yılbaşı": 4 üye, 2 havuz, 4 fikir, 3 oy,
	// 6 katkı satırı (her havuzda alıcı dışındaki 3 üye; biri katkı vermiyor). Vitrin: "Selin'in
	// Mezuniyeti" (4 üye, 3 fikir, 5 oy) ve kazananı seçilmiş "Elif'in Doğum Günü" (3 üye, 1 fikir).
	// Havuz bildirimleri vitrini: MULTI "Ekip Doğum Günleri" (4 üye, 2 havuz, 2 fikir, 6 katkı satırı).
	want := map[string]int64{"users": 5, "groups": 5, "group_members": 18, "gift_ideas": 13, "votes": 12, "gift_pools": 4, "pool_contributions": 12}
	got := counts(t, db)
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s = %d, want %d (all: %v)", k, got[k], v, got)
		}
	}

	if err := seed(db, "Demo1234!"); !errors.Is(err, errAlreadySeeded) {
		t.Fatalf("ikinci seed: err = %v, want errAlreadySeeded", err)
	}
	if again := counts(t, db); again["users"] != got["users"] || again["gift_ideas"] != got["gift_ideas"] {
		t.Fatalf("ikinci seed veri ekledi: %v → %v", got, again)
	}

	var still models.User
	if err := db.First(&still, "id = ?", existing.ID).Error; err != nil || still.Email != "real@example.com" {
		t.Fatalf("mevcut kullanıcı değişti veya silindi: %v %+v", err, still)
	}

	// Hedef kişi grubu listesinde görmemeli; yönetici görmeli.
	users := repository.NewUserRepository(db)
	groups := repository.NewGroupRepository(db)
	target, _ := users.FindByEmail(demoTarget.Email)
	admin, _ := users.FindByEmail(demoAdmin.Email)
	// Hedef kişi "Ahmet'in 30. Yaş Sürprizi"ni görmez; diğer dört grupta (iki MULTI + vitrin) sıradan üyedir.
	list, err := groups.GetUserGroups(target.ID)
	if err != nil || len(list) != 4 {
		t.Fatalf("hedef kişinin grup listesi yanlış (4 grup beklenir): %v %v", err, list)
	}
	for _, g := range list {
		if g.Title == "Ahmet'in 30. Yaş Sürprizi" {
			t.Fatalf("hedef kişi kendi sürpriz grubunu görüyor: %+v", g)
		}
	}
	if list, err := groups.GetUserGroups(admin.ID); err != nil || len(list) != 5 {
		t.Fatalf("yönetici demo gruplarını göremiyor: %v %v", err, list)
	}

	// MULTI grup: Zeynep'in havuzunu Can (grup yöneticisi), Can'ın havuzunu Elif yönetir; Ahmet
	// Can'ın havuzuna katkı vermiyor. Can kendi havuzunun hiçbir sayacını görmez.
	member, _ := users.FindByEmail(demoMember.Email)
	friend, _ := users.FindByEmail(demoFriend.Email)
	var multi models.Group
	if err := db.First(&multi, "mode = ? AND title = ?", models.GroupModeMulti, multiGroupTitle).Error; err != nil {
		t.Fatalf("MULTI demo grubu yok: %v %+v", err, multi)
	}
	// Sıra kararlıdır (created_at açıkça yazılır; liste "created_at, id" ile sıralanır): önce Zeynep.
	pools, err := repository.NewPoolRepository(db).ListPools(multi.ID)
	if err != nil || len(pools) != 2 {
		t.Fatalf("havuzlar: %v %d", err, len(pools))
	}
	if pools[0].RecipientUserID != member.ID || pools[0].ManagerUserID != nil {
		t.Fatalf("Zeynep havuzu yanlış: %+v", pools[0])
	}
	if pools[1].RecipientUserID != admin.ID || pools[1].ManagerUserID == nil || *pools[1].ManagerUserID != friend.ID {
		t.Fatalf("Can havuzu yanlış: %+v", pools[1])
	}
	var optedOut int64
	if err := db.Model(&models.PoolContribution{}).Where("pool_id = ? AND user_id = ? AND contributing = false", pools[1].ID, target.ID).
		Count(&optedOut).Error; err != nil || optedOut != 1 {
		t.Fatalf("Ahmet'in katkı tercihi yazılmadı: %v %d", err, optedOut)
	}
	poolRepo := repository.NewPoolRepository(db)
	stats, err := poolRepo.PoolStats(multi.ID, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, leaked := stats[pools[1].ID]; leaked || stats[pools[0].ID].IdeaCount != 2 {
		t.Fatalf("yöneticinin havuz sayaçları yanlış (kendi havuzu görünmemeli): %+v", stats)
	}
	friendStats, err := poolRepo.PoolStats(multi.ID, friend.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := friendStats[pools[1].ID]; got.ContributorCount != 2 || got.IdeaCount != 2 {
		t.Fatalf("Can havuzunun sayaçları yanlış: %+v", got)
	}
}

// Havuz bildirimleri: her türden en az bir satır var, hepsi pool_id taşıyor ve recipient_name
// içeriyor; hiçbir havuz bildirimi o havuzun alıcısına yazılmamış (Anti-Spoiler). "Süre doldu"
// yalnızca havuzun etkin sorumlusuna gider (atanmadıysa grup yöneticisi).
func TestSeedPoolNotificationsRespectAntiSpoiler(t *testing.T) {
	db := openTestDB(t)
	if err := seed(db, "Demo1234!"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	type row struct {
		Type      string
		UserID    string
		Recipient string
		Manager   *string
		HasName   bool
	}
	var rows []row
	if err := db.Raw(`SELECT n.type, n.user_id::text AS user_id, p.recipient_user_id::text AS recipient,
			COALESCE(p.manager_user_id, (SELECT gm.user_id FROM group_members gm
				WHERE gm.group_id = p.group_id AND gm.role = 'admin' LIMIT 1))::text AS manager,
			(n.params ? 'recipient_name') AS has_name
		FROM notifications n JOIN gift_pools p ON p.id = n.pool_id`).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	byType := map[string]int{}
	for _, r := range rows {
		byType[r.Type]++
		if r.UserID == r.Recipient {
			t.Fatalf("havuzun alıcısına kendi havuzundan bildirim yazılmış: %+v", r)
		}
		if !r.HasName {
			t.Fatalf("havuz bildiriminde recipient_name yok: %+v", r)
		}
		if r.Type == models.NotificationWinnerOverdue && (r.Manager == nil || *r.Manager != r.UserID) {
			t.Fatalf("süre doldu bildirimi sorumlu dışında birine gitmiş: %+v", r)
		}
	}
	for _, typ := range []string{
		models.NotificationPoolAdded, models.NotificationPoolManagerAssigned, models.NotificationIdeaAdded,
		models.NotificationEventDateChanged, models.NotificationEventReminder, models.NotificationWinnerOverdue,
	} {
		if byType[typ] == 0 {
			t.Errorf("%s türünde havuz bildirimi yok (%v)", typ, byType)
		}
	}
	// İkinci çalıştırma hiçbir şey eklemez.
	before := len(rows)
	if err := seed(db, "Demo1234!"); !errors.Is(err, errAlreadySeeded) {
		t.Fatalf("ikinci seed errAlreadySeeded dönmeli: %v", err)
	}
	var after int64
	if err := db.Raw("SELECT count(*) FROM notifications WHERE pool_id IS NOT NULL").Scan(&after).Error; err != nil || int(after) != before {
		t.Fatalf("ikinci seed havuz bildirimi ekledi: %v %d → %d", err, before, after)
	}
}

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
	for _, table := range []string{"users", "groups", "group_members", "gift_ideas", "votes"} {
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
	want := map[string]int64{"users": 5, "groups": 1, "group_members": 3, "gift_ideas": 3, "votes": 4}
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
	if list, err := groups.GetUserGroups(target.ID); err != nil || len(list) != 0 {
		t.Fatalf("hedef kişi demo grubunu görüyor: %v %v", err, list)
	}
	if list, err := groups.GetUserGroups(admin.ID); err != nil || len(list) != 1 {
		t.Fatalf("yönetici demo grubunu göremiyor: %v %v", err, list)
	}
}

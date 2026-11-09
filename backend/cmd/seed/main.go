// Command seed yerel geliştirme veritabanına demo kullanıcılar ve bir sürpriz grup ekler.
//
// Güvenlik: yalnızca EKLER, hiçbir şeyi silmez veya değiştirmez. Demo yönetici zaten
// varsa hiçbir şey yapmadan çıkar; her şey tek bir transaction içinde yapılır, yarıda
// kalan bir çalıştırma iz bırakmaz. APP_ENV=production iken çalışmayı reddeder.
//
//	cd backend && go run ./cmd/seed
//
// Demo parolası SEED_PASSWORD ile değiştirilebilir (varsayılan: Demo1234!).
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/config"
	"github.com/MSheker20/CovertGift/backend/internal/logging"
	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const defaultPassword = "Demo1234!"

type demoUser struct{ Name, Email string }

var (
	demoAdmin  = demoUser{"Can Yılmaz", "can@demo.covertgift.test"}
	demoMember = demoUser{"Zeynep Kaya", "zeynep@demo.covertgift.test"}
	demoFriend = demoUser{"Elif Demir", "elif@demo.covertgift.test"}
	demoTarget = demoUser{"Ahmet Şahin", "ahmet@demo.covertgift.test"}
	demoUsers  = []demoUser{demoAdmin, demoMember, demoFriend, demoTarget}
)

// errAlreadySeeded demo verisi daha önce eklendiğinde döner; hata değil, bilgi amaçlıdır.
var errAlreadySeeded = errors.New("demo verisi zaten mevcut")

func main() {
	config.LoadDotEnv()
	logging.FromEnv()
	if strings.TrimSpace(os.Getenv("APP_ENV")) == "production" {
		slog.Error("seed production ortamında çalıştırılamaz (APP_ENV=production)")
		os.Exit(1)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL tanımlı değil")
		os.Exit(1)
	}
	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		password = defaultPassword
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		slog.Error("veritabanına bağlanılamadı", "error", err)
		os.Exit(1)
	}
	if err := models.Migrate(db); err != nil {
		slog.Error("migrasyon hatası", "error", err)
		os.Exit(1)
	}

	err = seed(db, password)
	switch {
	case errors.Is(err, errAlreadySeeded):
		slog.Info("demo verisi zaten mevcut; hiçbir şey değiştirilmedi", "admin", demoAdmin.Email)
	case err != nil:
		slog.Error("seed başarısız; hiçbir değişiklik kaydedilmedi", "error", err)
		os.Exit(1)
	default:
		fmt.Println("Demo verisi eklendi. Giriş bilgileri (parola hepsi için aynı):")
		for _, u := range demoUsers {
			role := map[string]string{demoAdmin.Email: "grup yöneticisi", demoMember.Email: "üye", demoFriend.Email: "üye", demoTarget.Email: "hedef kişi (grubu göremez)"}[u.Email]
			fmt.Printf("  %-30s %s\n", u.Email, role)
		}
		if password == defaultPassword {
			fmt.Printf("  parola: %s\n", defaultPassword)
		}
	}
}

// seed demo kullanıcıları, hedefi bağlı bir grubu, üyeleri, fikirleri ve oyları tek bir
// transaction içinde ekler. Demo yöneticisi varsa errAlreadySeeded döner ve hiçbir şey yazmaz.
// Kayıtlar gerçek servisler üzerinden yapılır; Anti-Spoiler ve doğrulama kuralları geçerlidir.
func seed(db *gorm.DB, password string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		userRepo := repository.NewUserRepository(tx)
		groupRepo := repository.NewGroupRepository(tx)
		ideaRepo := repository.NewIdeaRepository(tx)

		existing, err := userRepo.FindByEmail(demoAdmin.Email)
		if err != nil {
			return err
		}
		if existing != nil {
			return errAlreadySeeded
		}

		auth := services.NewAuthService(userRepo, "seed-unused-secret")
		users := map[string]*models.User{}
		for _, u := range demoUsers {
			created, err := auth.Register(u.Name, u.Email, password)
			if err != nil {
				return fmt.Errorf("%s kaydı: %w", u.Email, err)
			}
			users[u.Email] = created
		}
		admin, member, friend := users[demoAdmin.Email], users[demoMember.Email], users[demoFriend.Email]

		groupSvc := services.NewGroupService(groupRepo, userRepo)
		// Hediye tarihi 3 hafta sonrası: geri sayım demo'da görünsün.
		now := time.Now()
		eventDate := models.Date{Time: time.Date(now.Year(), now.Month(), now.Day()+21, 0, 0, 0, 0, time.UTC)}
		group, err := groupSvc.CreateGroup(admin.ID, "Ahmet'in 30. Yaş Sürprizi",
			"Yakın arkadaşlarla ortak doğum günü hediyesi", "Ahmet", "Kamp\n3. nesil kahve\nTrekking",
			nil, demoTarget.Email, decimal.RequireFromString("3000"), &eventDate)
		if err != nil {
			return fmt.Errorf("grup: %w", err)
		}
		for _, email := range []string{demoMember.Email, demoFriend.Email} {
			if _, err := groupSvc.AddMemberByEmail(group.ID, admin.ID, email); err != nil {
				return fmt.Errorf("%s üyeliği: %w", email, err)
			}
		}

		ideaSvc := services.NewIdeaService(ideaRepo, groupRepo, nil)
		ideas := []struct {
			by                 *models.User
			title, desc, price string
			query              string
		}{
			{member, "Stanley Klasik Vakumlu Termos 1 L", "Kampta sıcak kahve için", "1450", "stanley klasik termos 1 lt"},
			{friend, "Hario V60 Demleme Seti", "3. nesil kahve demlemeye başlangıç", "1100", "hario v60 demleme seti"},
			{admin, "Katlanır Trekking Batonu (çift)", "Uzun yürüyüşler için hafif alüminyum", "850", "katlanır trekking batonu"},
		}
		ideaIDs := make([]*models.GiftIdea, 0, len(ideas))
		for _, it := range ideas {
			idea, err := ideaSvc.AddManualIdea(group.ID, it.by.ID, it.title, it.desc, it.query, "", "", decimal.RequireFromString(it.price))
			if err != nil {
				return fmt.Errorf("fikir %q: %w", it.title, err)
			}
			ideaIDs = append(ideaIDs, idea)
		}
		votes := []struct {
			idea *models.GiftIdea
			by   *models.User
			v    int8
		}{
			{ideaIDs[0], admin, 1}, {ideaIDs[0], friend, 1}, {ideaIDs[1], member, 1}, {ideaIDs[2], member, -1},
		}
		for _, v := range votes {
			if err := ideaSvc.VoteIdea(v.idea.ID, v.by.ID, v.v); err != nil {
				return fmt.Errorf("oy: %w", err)
			}
		}
		return nil
	})
}

// Command seed yerel geliştirme veritabanına demo kullanıcılar, tek kişilik bir sürpriz grup
// (SINGLE) ve birden çok kişi için bir grup (MULTI, "Ofis Yılbaşı") ekler.
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

// multiGroupTitle MULTI demo grubunun adıdır.
const multiGroupTitle = "Ofis Yılbaşı"

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
		fmt.Printf("\"%s\" (birden çok kişi için): %s ve %s hediye alacak; %s'nin havuzunu %s yönetir.\n",
			multiGroupTitle, demoMember.Name, demoAdmin.Name, demoAdmin.Name, demoFriend.Name)
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
		poolRepo := repository.NewPoolRepository(tx)

		existing, err := userRepo.FindByEmail(demoAdmin.Email)
		if err != nil {
			return err
		}
		if existing != nil {
			// Temel demo verisi var; yalnızca eksik vitrin senaryolarını ekle.
			users := map[string]*models.User{}
			for _, u := range demoUsers {
				found, err := userRepo.FindByEmail(u.Email)
				if err != nil {
					return err
				}
				if found == nil {
					return errAlreadySeeded
				}
				users[u.Email] = found
			}
			return seedExtras(tx, users)
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

		groupSvc := services.NewGroupService(groupRepo, userRepo, poolRepo)
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

		ideaSvc := services.NewIdeaService(ideaRepo, groupRepo, poolRepo, nil)
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
			idea, err := ideaSvc.AddManualIdea(repository.GroupScope(group.ID), it.by.ID, it.title, it.desc, it.query, "", "", decimal.RequireFromString(it.price))
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
		if err := seedMultiGroup(tx, users); err != nil {
			return err
		}
		return seedExtras(tx, users)
	})
}

// seedMultiGroup birden çok kişi için bir grup ekler: Can (yönetici), Zeynep, Elif ve Ahmet üye;
// Zeynep'in havuzunu grup yöneticisi Can, Can'ın havuzunu Elif yönetir. Ahmet, SINGLE grubun hedefi
// olduğu için o grubu göremez ama burada sıradan bir üyedir; Can'ın havuzuna katkı vermez.
func seedMultiGroup(tx *gorm.DB, users map[string]*models.User) error {
	userRepo := repository.NewUserRepository(tx)
	groupRepo := repository.NewGroupRepository(tx)
	poolRepo := repository.NewPoolRepository(tx)
	groupSvc := services.NewGroupService(groupRepo, userRepo, poolRepo)
	poolSvc := services.NewPoolService(groupRepo, poolRepo)
	ideaSvc := services.NewIdeaService(repository.NewIdeaRepository(tx), groupRepo, poolRepo, nil)
	admin, member, friend, target := users[demoAdmin.Email], users[demoMember.Email], users[demoFriend.Email], users[demoTarget.Email]

	group, err := groupSvc.CreateMultiGroup(admin.ID, multiGroupTitle, "Ofisteki yılbaşı hediyeleşmesi: herkes için ayrı havuz")
	if err != nil {
		return fmt.Errorf("MULTI grup: %w", err)
	}
	for _, email := range []string{demoMember.Email, demoFriend.Email, demoTarget.Email} {
		if _, err := groupSvc.AddMemberByEmail(group.ID, admin.ID, email); err != nil {
			return fmt.Errorf("MULTI %s üyeliği: %w", email, err)
		}
	}
	_, zeynepPool, err := poolSvc.AddRecipient(group.ID, admin.ID, member.ID, nil)
	if err != nil {
		return fmt.Errorf("Zeynep havuzu: %w", err)
	}
	_, canPool, err := poolSvc.AddRecipient(group.ID, admin.ID, admin.ID, &friend.ID)
	if err != nil {
		return fmt.Errorf("Can havuzu: %w", err)
	}

	now := time.Now()
	newYear := models.Date{Time: time.Date(now.Year(), now.Month(), now.Day()+30, 0, 0, 0, 0, time.UTC)}
	setup := []struct {
		pool      *models.GiftPool
		manager   *models.User
		budget    string
		interests []string
	}{
		{zeynepPool, admin, "1500", []string{"Kitap", "Bitki"}},
		{canPool, friend, "2000", []string{"Kahve"}},
	}
	for _, p := range setup {
		if err := poolSvc.UpdateBudget(group.ID, p.pool.ID, p.manager.ID, decimal.RequireFromString(p.budget)); err != nil {
			return fmt.Errorf("havuz bütçesi: %w", err)
		}
		if err := poolSvc.SetEventDate(group.ID, p.pool.ID, p.manager.ID, &newYear); err != nil {
			return fmt.Errorf("havuz tarihi: %w", err)
		}
		for _, interest := range p.interests {
			if _, err := poolSvc.AddInterest(group.ID, p.pool.ID, p.manager.ID, interest); err != nil {
				return fmt.Errorf("havuz ilgi alanı: %w", err)
			}
		}
	}

	ideas := []struct {
		pool               *models.GiftPool
		by                 *models.User
		title, desc, price string
		query              string
	}{
		{zeynepPool, friend, "Bonsai Başlangıç Seti", "Masasına küçük bir bitki", "600", "bonsai başlangıç seti"},
		{zeynepPool, target, "Kindle Paperwhite", "Kitap kurdu için e-okuyucu", "1400", "kindle paperwhite"},
		{canPool, member, "Bialetti Moka Pot 3 Cup", "Ofiste kahve demlemek için", "900", "bialetti moka pot 3 cup"},
		{canPool, friend, "Manuel Kahve Öğütücü", "Taze çekilmiş kahve için", "1200", "manuel kahve öğütücü"},
	}
	created := make([]*models.GiftIdea, 0, len(ideas))
	for _, it := range ideas {
		idea, err := ideaSvc.AddManualIdea(repository.PoolScope(group.ID, it.pool.ID), it.by.ID, it.title, it.desc, it.query, "", "", decimal.RequireFromString(it.price))
		if err != nil {
			return fmt.Errorf("havuz fikri %q: %w", it.title, err)
		}
		created = append(created, idea)
	}
	votes := []struct {
		idea *models.GiftIdea
		by   *models.User
	}{
		{created[0], admin}, {created[2], friend}, {created[2], target},
	}
	for _, v := range votes {
		if err := ideaSvc.VoteIdea(v.idea.ID, v.by.ID, 1); err != nil {
			return fmt.Errorf("havuz oyu: %w", err)
		}
	}
	// Ahmet, Can'ın havuzuna katkı vermiyor (fikir ve oy hakkı sürer).
	if _, err := poolSvc.SetContribution(group.ID, canPool.ID, target.ID, false); err != nil {
		return fmt.Errorf("katkı tercihi: %w", err)
	}
	return nil
}

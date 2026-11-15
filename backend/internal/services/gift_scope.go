package services

import (
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// giftScope fikirlerin ait olduğu kapsamdır: SINGLE grubun kendisi (Pool nil) ya da MULTI
// gruptaki bir havuz. Fikir, oy, AI ve kazanan kuralları kapsam üzerinden bir kez yazılır.
type giftScope struct {
	Group *models.Group
	Pool  *models.GiftPool
}

// IsPool kapsamın bir havuz olduğunu söyler.
func (s giftScope) IsPool() bool { return s.Pool != nil }

// Budget kapsamın toplam bütçesidir.
func (s giftScope) Budget() decimal.Decimal {
	if s.Pool != nil {
		return s.Pool.TotalBudget
	}
	return s.Group.TotalBudget
}

// InterestsText kapsamın ilgi alanlarının saklanan metnidir (satır sonuyla ayrılmış).
func (s giftScope) InterestsText() string {
	if s.Pool != nil {
		return s.Pool.Interests
	}
	return s.Group.TargetInterests
}

// Interests kapsamın ayrıştırılmış ilgi alanlarıdır.
func (s giftScope) Interests() []string {
	return models.ParseInterests(s.InterestsText())
}

// EventDate kapsamın hediye tarihidir.
func (s giftScope) EventDate() *models.Date {
	if s.Pool != nil {
		return s.Pool.EventDate
	}
	return s.Group.EventDate
}

// Status kapsamın durumudur (ACTIVE / COMPLETED).
func (s giftScope) Status() string {
	if s.Pool != nil {
		return s.Pool.Status
	}
	return s.Group.Status
}

// Completed kapsamın tamamlanmış olduğunu söyler.
func (s giftScope) Completed() bool {
	return s.Status() == models.GroupStatusCompleted
}

// CompletedErr kapsam tamamlandığında dönen hatadır.
func (s giftScope) CompletedErr() error {
	if s.Pool != nil {
		return ErrPoolCompleted
	}
	return ErrGroupCompleted
}

// RecipientName hediye alacak kişinin adıdır (havuzda alıcının hesap adı).
func (s giftScope) RecipientName() string {
	if s.Pool != nil {
		return s.Pool.Recipient.Name
	}
	return s.Group.TargetPersonName
}

// IdeaScope kapsamın repository karşılığıdır.
func (s giftScope) IdeaScope() repository.IdeaScope {
	if s.Pool != nil {
		return repository.PoolScope(s.Group.ID, s.Pool.ID)
	}
	return repository.GroupScope(s.Group.ID)
}

// VetoElectorate çoğunluk vetosunda oy hakkı olanların sayısıdır: SINGLE grupta tüm üyeler,
// havuzda alıcı hariç üyeler.
func (s giftScope) VetoElectorate(memberCount int64) int64 {
	if s.Pool != nil && memberCount > 0 {
		return memberCount - 1
	}
	return memberCount
}

// MajorityVeto eksi oyların seçmenlerin yarısını (tam sayı bölmesi) aşıp aşmadığını söyler.
func MajorityVeto(downvotes, electorate int64) bool {
	return electorate > 0 && downvotes > electorate/2
}

// effectiveManagerID havuzun etkin yöneticisidir: alıcı grubun yöneticisiyse manager_user_id
// (NULL ise kimse), değilse grubun yöneticisi.
func effectiveManagerID(pool *models.GiftPool, adminID uuid.UUID) *uuid.UUID {
	if pool.RecipientUserID == adminID {
		return pool.ManagerUserID
	}
	id := adminID
	return &id
}

// canManagePool kullanıcının havuzun etkin yöneticisi olduğunu söyler.
func canManagePool(pool *models.GiftPool, adminID, userID uuid.UUID) bool {
	manager := effectiveManagerID(pool, adminID)
	return manager != nil && *manager == userID
}

// PerPersonShare havuz bütçesini katkı veren sayısına böler (2 basamak). Katkı veren yoksa nil.
func PerPersonShare(budget decimal.Decimal, contributors int64) *decimal.Decimal {
	if contributors <= 0 {
		return nil
	}
	share := budget.Div(decimal.NewFromInt(contributors)).Round(2)
	return &share
}

// viewerStatus MULTI grubun çağırana göre durumudur: çağıranın alıcısı olmadığı en az bir havuz
// varsa ve bu havuzların hepsi tamamlandıysa COMPLETED (tamamlanma anı en geç olanıdır), aksi
// halde ACTIVE. Çağıranın kendi havuzu hesaba katılmaz (Anti-Spoiler).
func viewerStatus(pools []models.GiftPool, viewerID uuid.UUID) (string, *time.Time) {
	visible := 0
	var latest *time.Time
	for i := range pools {
		p := &pools[i]
		if p.RecipientUserID == viewerID {
			continue
		}
		visible++
		if p.Status != models.GroupStatusCompleted {
			return models.GroupStatusActive, nil
		}
		if p.CompletedAt != nil && (latest == nil || p.CompletedAt.After(*latest)) {
			t := *p.CompletedAt
			latest = &t
		}
	}
	if visible == 0 {
		return models.GroupStatusActive, nil
	}
	return models.GroupStatusCompleted, latest
}

// nextEvent çağıranın alıcısı olmadığı açık havuzların en yakın yaklaşan (bugün veya sonrası) tarihini,
// yaklaşan yoksa en erken geçmiş tarihi ve o tarihi taşıyan havuzların alıcılarını (havuz sırasıyla;
// aynı gün birden çok kişi olabilir) döner. Çağıranın kendi havuzu hiçbir zaman sayılmaz (Anti-Spoiler).
// Tarih yoksa ikisi de nil'dir. "Bugün" UTC+3'e göredir (today, gece yarısı UTC olarak verilir).
func nextEvent(pools []models.GiftPool, viewerID uuid.UUID, today models.Date) (*models.Date, []AccountSummary) {
	var upcoming, overdue *models.Date
	for i := range pools {
		p := &pools[i]
		if p.RecipientUserID == viewerID || p.Status != models.GroupStatusActive || p.EventDate == nil {
			continue
		}
		d := *p.EventDate
		if !d.Before(today.Time) {
			if upcoming == nil || d.Before(upcoming.Time) {
				upcoming = &d
			}
		} else if overdue == nil || d.Before(overdue.Time) {
			overdue = &d
		}
	}
	next := upcoming
	if next == nil {
		next = overdue
	}
	if next == nil {
		return nil, nil
	}
	var recipients []AccountSummary
	for i := range pools {
		p := &pools[i]
		if p.RecipientUserID == viewerID || p.Status != models.GroupStatusActive || p.EventDate == nil {
			continue
		}
		if p.EventDate.Equal(next.Time) {
			recipients = append(recipients, memberSummary(p.RecipientUserID, p.Recipient))
		}
	}
	return next, recipients
}

// turkeyToday "bugün"ü UTC+3'e göre, gece yarısı UTC olarak döner (tarih karşılaştırmaları için).
func turkeyToday(now time.Time) models.Date {
	n := now.In(turkeyTZ)
	return models.Date{Time: time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)}
}

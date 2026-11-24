package repository

import (
	"errors"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Arkadaşlık repository hataları.
var (
	ErrFriendshipExists   = errors.New("friendship or request already exists")
	ErrFriendshipNotFound = errors.New("friendship not found")
)

// FriendRow arkadaş listesinde dönen satırdır: karşı taraf ve ilişkinin durumu.
type FriendRow struct {
	FriendshipID  uuid.UUID
	UserID        uuid.UUID
	Name          string
	AvatarVersion string
	Status        string
	RequesterID   uuid.UUID
	CreatedAt     time.Time
	AcceptedAt    *time.Time
}

// RequestOutcome istek gönderme sonucudur.
type RequestOutcome struct {
	FriendshipID uuid.UUID
	// Accepted karşı taraf zaten istek göndermişse true: istek doğrudan kabul edildi.
	Accepted bool
}

// FriendRepository arkadaşlık ve kişisel arkadaşlık linki işlemlerini tanımlar.
type FriendRepository interface {
	List(userID uuid.UUID) ([]FriendRow, error)
	// Request requester'dan addressee'ye istek açar. Karşı taraftan bekleyen istek varsa onu kabul
	// eder (Accepted=true). Zaten arkadaşlarsa ya da aynı yönde istek varsa ErrFriendshipExists.
	Request(requesterID, addresseeID uuid.UUID, now time.Time) (*RequestOutcome, error)
	// Accept yalnızca isteğin alıcısı tarafından çağrılabilir; diğer durumlarda ErrFriendshipNotFound.
	Accept(friendshipID, userID uuid.UUID, now time.Time) (*models.Friendship, error)
	// DeletePending bekleyen isteği reddeder (alıcı) ya da geri çeker (gönderen).
	DeletePending(friendshipID, userID uuid.UUID) (*models.Friendship, error)
	// Unfriend kabul edilmiş arkadaşlığı siler; grup üyeliklerine dokunmaz.
	Unfriend(userID, otherID uuid.UUID) error
	// Befriend iki kullanıcıyı doğrudan arkadaş yapar (kişisel link). Bekleyen istek varsa kabul
	// edilir; zaten arkadaşlarsa created=false döner.
	Befriend(a, b uuid.UUID, now time.Time) (created bool, err error)
	// AcceptedFriendIDs ids içinden userID'nin kabul edilmiş arkadaşı olanları döner.
	AcceptedFriendIDs(userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)

	GetLink(userID uuid.UUID) (*models.FriendLink, error)
	// SaveLink kullanıcının linkini ekler ya da kodunu değiştirir.
	SaveLink(link *models.FriendLink) error
	FindLinkByCode(code string) (*models.FriendLink, error)
}

type friendRepository struct {
	db *gorm.DB
}

// NewFriendRepository yeni bir FriendRepository oluşturur.
func NewFriendRepository(db *gorm.DB) FriendRepository {
	return &friendRepository{db: db}
}

func (r *friendRepository) List(userID uuid.UUID) ([]FriendRow, error) {
	rows := []FriendRow{}
	err := r.db.Raw(`SELECT f.id AS friendship_id, u.id AS user_id, u.name, u.avatar_version,
			f.status, f.requester_id, f.created_at, f.accepted_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.user_low_id = ? THEN f.user_high_id ELSE f.user_low_id END
		WHERE f.user_low_id = ? OR f.user_high_id = ?
		ORDER BY lower(u.name), u.id`, userID, userID, userID).Scan(&rows).Error
	return rows, err
}

// lockPair çiftin satırını FOR UPDATE ile okur; yoksa nil döner.
func lockPair(tx *gorm.DB, a, b uuid.UUID) (*models.Friendship, error) {
	low, high := models.FriendPair(a, b)
	var f models.Friendship
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_low_id = ? AND user_high_id = ?", low, high).First(&f).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

// insertPair çift için satır ekler. Eşzamanlı aynı çift eklenirse benzersiz index çakışması
// DoNothing ile yutulur ve inserted=false döner; çağıran satırı yeniden okur.
func insertPair(tx *gorm.DB, f *models.Friendship) (bool, error) {
	res := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_low_id"}, {Name: "user_high_id"}},
		DoNothing: true,
	}).Create(f)
	return res.RowsAffected > 0, res.Error
}

func (r *friendRepository) Request(requesterID, addresseeID uuid.UUID, now time.Time) (*RequestOutcome, error) {
	var out *RequestOutcome
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 2; attempt++ {
			existing, err := lockPair(tx, requesterID, addresseeID)
			if err != nil {
				return err
			}
			if existing != nil {
				if existing.Status == models.FriendshipPending && existing.RequesterID == addresseeID {
					if err := tx.Model(existing).Updates(map[string]any{"status": models.FriendshipAccepted, "accepted_at": now}).Error; err != nil {
						return err
					}
					out = &RequestOutcome{FriendshipID: existing.ID, Accepted: true}
					return nil
				}
				return ErrFriendshipExists
			}
			low, high := models.FriendPair(requesterID, addresseeID)
			f := &models.Friendship{UserLowID: low, UserHighID: high, RequesterID: requesterID, Status: models.FriendshipPending, CreatedAt: now}
			inserted, err := insertPair(tx, f)
			if err != nil {
				return err
			}
			if inserted {
				out = &RequestOutcome{FriendshipID: f.ID}
				return nil
			}
		}
		return ErrFriendshipExists
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *friendRepository) Accept(friendshipID, userID uuid.UUID, now time.Time) (*models.Friendship, error) {
	var f models.Friendship
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&f, "id = ?", friendshipID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFriendshipNotFound
			}
			return err
		}
		isParty := f.UserLowID == userID || f.UserHighID == userID
		if !isParty || f.Status != models.FriendshipPending || f.RequesterID == userID {
			return ErrFriendshipNotFound
		}
		f.Status = models.FriendshipAccepted
		f.AcceptedAt = &now
		return tx.Model(&f).Updates(map[string]any{"status": f.Status, "accepted_at": now}).Error
	})
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *friendRepository) DeletePending(friendshipID, userID uuid.UUID) (*models.Friendship, error) {
	var f models.Friendship
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&f, "id = ?", friendshipID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFriendshipNotFound
			}
			return err
		}
		if (f.UserLowID != userID && f.UserHighID != userID) || f.Status != models.FriendshipPending {
			return ErrFriendshipNotFound
		}
		return tx.Delete(&f).Error
	})
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *friendRepository) Unfriend(userID, otherID uuid.UUID) error {
	low, high := models.FriendPair(userID, otherID)
	res := r.db.Where("user_low_id = ? AND user_high_id = ? AND status = ?", low, high, models.FriendshipAccepted).
		Delete(&models.Friendship{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrFriendshipNotFound
	}
	return nil
}

func (r *friendRepository) Befriend(a, b uuid.UUID, now time.Time) (bool, error) {
	created := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 2; attempt++ {
			existing, err := lockPair(tx, a, b)
			if err != nil {
				return err
			}
			if existing != nil {
				if existing.Status == models.FriendshipAccepted {
					return nil
				}
				created = true
				return tx.Model(existing).Updates(map[string]any{"status": models.FriendshipAccepted, "accepted_at": now}).Error
			}
			low, high := models.FriendPair(a, b)
			f := &models.Friendship{UserLowID: low, UserHighID: high, RequesterID: a, Status: models.FriendshipAccepted, CreatedAt: now, AcceptedAt: &now}
			inserted, err := insertPair(tx, f)
			if err != nil {
				return err
			}
			if inserted {
				created = true
				return nil
			}
		}
		return nil
	})
	return created, err
}

func (r *friendRepository) AcceptedFriendIDs(userID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.Raw(`SELECT CASE WHEN user_low_id = ? THEN user_high_id ELSE user_low_id END
		FROM friendships
		WHERE status = ? AND (user_low_id = ? OR user_high_id = ?)
		  AND (CASE WHEN user_low_id = ? THEN user_high_id ELSE user_low_id END) IN ?`,
		userID, models.FriendshipAccepted, userID, userID, userID, ids).Scan(&out).Error
	return out, err
}

func (r *friendRepository) GetLink(userID uuid.UUID) (*models.FriendLink, error) {
	var link models.FriendLink
	if err := r.db.First(&link, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &link, nil
}

func (r *friendRepository) SaveLink(link *models.FriendLink) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"code", "created_at"}),
	}).Create(link).Error
}

func (r *friendRepository) FindLinkByCode(code string) (*models.FriendLink, error) {
	var link models.FriendLink
	if err := r.db.First(&link, "code = ?", code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &link, nil
}

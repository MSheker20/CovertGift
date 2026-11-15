package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
	"github.com/google/uuid"
)

// ErrAvatarNotFound fotoğraf yoksa YA DA çağıran onu görmeye yetkili değilse döner. İkisi
// bilerek ayırt edilmez: yetkisiz biri bir hesabın var olup olmadığını öğrenemez.
var ErrAvatarNotFound = errors.New("profil fotoğrafı bulunamadı")

// maxConcurrentAvatarJobs aynı anda çözülüp ölçeklenen profil fotoğrafı sayısıdır.
const maxConcurrentAvatarJobs = 2

// AvatarService profil fotoğrafı iş mantığıdır.
type AvatarService interface {
	// Set yüklenen görseli işler (kare, en çok 512x512, metadata'sız JPEG) ve kaydeder.
	Set(userID uuid.UUID, raw []byte) (*models.UserAvatar, error)
	// Delete fotoğrafı kaldırır; fotoğraf yoksa da başarılıdır (idempotent).
	Delete(userID uuid.UUID) error
	// Get sahibin fotoğrafını döner. Görebilenler: sahibi ve onunla en az bir grubu paylaşanlar.
	Get(viewerID, ownerID uuid.UUID) (*models.UserAvatar, error)
}

type avatarService struct {
	repo  repository.AvatarRepository
	slots chan struct{}
}

// NewAvatarService yeni bir AvatarService oluşturur.
func NewAvatarService(repo repository.AvatarRepository) AvatarService {
	return &avatarService{repo: repo, slots: make(chan struct{}, maxConcurrentAvatarJobs)}
}

func (s *avatarService) Set(userID uuid.UUID, raw []byte) (*models.UserAvatar, error) {
	s.slots <- struct{}{}
	processed, err := imageproc.ProcessAvatar(raw)
	<-s.slots
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(processed.Data)
	avatar := &models.UserAvatar{
		UserID:      userID,
		Data:        processed.Data,
		ContentType: processed.ContentType,
		ETag:        hex.EncodeToString(sum[:16]),
		Width:       processed.Width,
		Height:      processed.Height,
	}
	if err := s.repo.Save(avatar); err != nil {
		return nil, err
	}
	return avatar, nil
}

func (s *avatarService) Delete(userID uuid.UUID) error {
	return s.repo.Delete(userID)
}

func (s *avatarService) Get(viewerID, ownerID uuid.UUID) (*models.UserAvatar, error) {
	if viewerID != ownerID {
		shared, err := s.repo.SharesGroup(viewerID, ownerID)
		if err != nil {
			return nil, err
		}
		if !shared {
			return nil, ErrAvatarNotFound
		}
	}
	avatar, err := s.repo.Get(ownerID)
	if err != nil {
		return nil, err
	}
	if avatar == nil {
		return nil, ErrAvatarNotFound
	}
	return avatar, nil
}

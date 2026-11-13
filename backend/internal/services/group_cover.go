package services

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
	"github.com/google/uuid"
)

// maxConcurrentCoverJobs aynı anda çözülüp ölçeklenen görsel sayısıdır. Sınırdaki bir
// görsel (4 MP) çözülürken ~16 MB RGBA tamponu ve ölçekleme tamponu kullanır; fazlası
// sırada bekler.
const maxConcurrentCoverJobs = 2

// requireMember grubu ve çağıranın üyeliğini getirir; Anti-Spoiler ve üyeliği denetler.
func (s *groupService) requireMember(groupID, userID uuid.UUID) (*models.Group, *models.GroupMember, error) {
	group, err := s.groupRepo.GetGroupByID(groupID)
	if err != nil {
		return nil, nil, err
	}
	if group == nil {
		return nil, nil, ErrGroupNotFound
	}
	if group.TargetUserID != nil && *group.TargetUserID == userID {
		return nil, nil, ErrAntiSpoilerTriggered
	}
	member, err := s.groupRepo.GetMember(groupID, userID)
	if err != nil {
		return nil, nil, err
	}
	if member == nil {
		return nil, nil, ErrNotGroupMember
	}
	return group, member, nil
}

// coverGuard kapağı değiştirme iznini denetler: kısıt kapalıyken her üye, açıkken yalnızca
// admin. Repo bunu grup satırı kilitliyken güncel politikayla çağırır.
func coverGuard(role string) func(group *models.Group) error {
	return func(group *models.Group) error {
		if group.CoverAdminOnly && role != "admin" {
			return ErrUnauthorizedCoverChange
		}
		return nil
	}
}

// SetCover yüklenen görseli işler (4:3, en çok 1600x1200, metadata'sız JPEG) ve grubun
// kapağı olarak kaydeder. Görsel hataları imageproc hatalarıyla döner.
func (s *groupService) SetCover(groupID, userID uuid.UUID, raw []byte) (*models.GroupCover, error) {
	group, member, err := s.requireMember(groupID, userID)
	if err != nil {
		return nil, err
	}
	// Yetkisiz isteğin görseli boşuna işlenmesin diye izin önce de denetlenir; asıl denetim
	// kayıt anında kilit altındadır.
	guard := coverGuard(member.Role)
	if err := guard(group); err != nil {
		return nil, err
	}

	s.coverSlots <- struct{}{}
	processed, err := imageproc.ProcessCover(raw)
	<-s.coverSlots
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(processed.Data)
	cover := &models.GroupCover{
		GroupID:     groupID,
		Data:        processed.Data,
		ContentType: processed.ContentType,
		ETag:        hex.EncodeToString(sum[:16]),
		Width:       processed.Width,
		Height:      processed.Height,
		UpdatedBy:   &userID,
	}
	if err := s.groupRepo.SaveCover(cover, guard); err != nil {
		return nil, err
	}
	return cover, nil
}

// DeleteCover grubun kapağını kaldırır; kapak yoksa da başarılıdır (idempotent).
func (s *groupService) DeleteCover(groupID, userID uuid.UUID) error {
	_, member, err := s.requireMember(groupID, userID)
	if err != nil {
		return err
	}
	return s.groupRepo.DeleteCover(groupID, coverGuard(member.Role))
}

// GetCover kapağı baytlarıyla döner. Her üye görebilir; hedef kişi ve üye olmayan göremez.
func (s *groupService) GetCover(groupID, userID uuid.UUID) (*models.GroupCover, error) {
	if _, _, err := s.requireMember(groupID, userID); err != nil {
		return nil, err
	}
	cover, err := s.groupRepo.GetCover(groupID)
	if err != nil {
		return nil, err
	}
	if cover == nil {
		return nil, ErrCoverNotFound
	}
	return cover, nil
}

// SetCoverAdminOnly kapağı yalnızca yöneticinin değiştirebilmesini açar/kapatır. Yalnızca admin.
func (s *groupService) SetCoverAdminOnly(groupID, userID uuid.UUID, adminOnly bool) error {
	if _, err := s.requireAdmin(groupID, userID, ErrUnauthorizedCoverPolicy); err != nil {
		return err
	}
	return s.groupRepo.SetCoverAdminOnly(groupID, adminOnly)
}

package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// groupChangedKey bir istekte değişen grubun gin context anahtarıdır.
const groupChangedKey = "groupChanged"

// RecordGroupChanged /groups/:id altında olmayan bir rotanın (ör. davetle katılma) bir grubu
// değiştirdiğini bildirir; middleware.PublishRecordedGroupChanges grup akışına sinyal gönderir.
func RecordGroupChanged(c *gin.Context, groupID uuid.UUID) {
	c.Set(groupChangedKey, groupID)
}

// RecordedGroupChanged istekte bildirilen grubu döner.
func RecordedGroupChanged(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(groupChangedKey)
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// InviteController davet linki HTTP isteklerini yönetir.
type InviteController struct {
	inviteService services.InviteService
}

// NewInviteController yeni bir InviteController oluşturur.
func NewInviteController(inviteService services.InviteService) *InviteController {
	return &InviteController{inviteService: inviteService}
}

// CreateInviteRequest davet linki oluşturma isteği için DTO
type CreateInviteRequest struct {
	ExpiresInDays int `json:"expires_in_days" binding:"required" enums:"1,7,30" example:"7"`
}

// AcceptInviteResponse davetle katılma yanıtıdır
type AcceptInviteResponse struct {
	GroupID uuid.UUID `json:"group_id" format:"uuid"`
}

// respondInviteErr servis hatalarını HTTP yanıtına çevirir; eşleşmeyen hata 500 olur.
func respondInviteErr(c *gin.Context, internalMsg string, err error) {
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		// Geçerli JWT ama hesap silinmiş: oturum artık geçerli değil (istemci 401'de oturumu kapatır).
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
	case errors.Is(err, services.ErrInvalidInviteExpiry):
		RespondErr(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrUnauthorizedInvite), errors.Is(err, services.ErrNotGroupMember):
		RespondErr(c, http.StatusForbidden, err.Error())
	case errors.Is(err, services.ErrGroupNotFound), errors.Is(err, services.ErrInviteNotFound):
		RespondErr(c, http.StatusNotFound, err.Error())
	default:
		RespondInternal(c, internalMsg, err)
	}
}

// GetStatus aktif davet linki durumunu döner
// @Summary      Davet Linki Durumu (Yalnızca Admin)
// @Description  Grubun aktif davet linki olup olmadığını ve bitiş zamanını döner. Token asla dönmez (yalnızca özeti saklanır). Üye 403, hedef kişi Anti-Spoiler ile 403 alır.
// @Tags         Davet Linki (Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=services.InviteStatus} "Davet durumu"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Admin değil veya Anti-Spoiler"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/invite [get]
func (ctl *InviteController) GetStatus(c *gin.Context) {
	groupID, userID, ok := groupAndUser(c)
	if !ok {
		return
	}
	status, err := ctl.inviteService.GetStatus(groupID, userID)
	if err != nil {
		respondInviteErr(c, "Davet linki sorgulanırken bir hata oluştu", err)
		return
	}
	RespondOK(c, status)
}

// Create yeni davet linki oluşturur; öncekini iptal eder
// @Summary      Davet Linki Oluştur / Yenile (Yalnızca Admin)
// @Description  Grup için yeni bir paylaşılabilir davet linki oluşturur; varsa önceki aktif link aynı transaction'da iptal edilir. Geçerlilik 1, 7 veya 30 gün olabilir. Ham token yalnızca bu yanıtta, bir kez döner. Bildirim üretmez.
// @Tags         Davet Linki (Invites)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body CreateInviteRequest true "Geçerlilik süresi (gün)"
// @Success      201 {object} Response{data=services.InviteCreated} "Davet oluşturuldu"
// @Failure      400 {object} Response "Geçersiz süre"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Admin değil veya Anti-Spoiler"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/invite [post]
func (ctl *InviteController) Create(c *gin.Context) {
	groupID, userID, ok := groupAndUser(c)
	if !ok {
		return
	}
	var req CreateInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, services.ErrInvalidInviteExpiry.Error())
		return
	}
	created, err := ctl.inviteService.Create(groupID, userID, req.ExpiresInDays)
	if err != nil {
		respondInviteErr(c, "Davet linki oluşturulurken bir hata oluştu", err)
		return
	}
	RespondCreated(c, created)
}

// Revoke aktif davet linkini iptal eder
// @Summary      Davet Linkini İptal Et (Yalnızca Admin)
// @Description  Grubun aktif davet linkini iptal eder. Aktif link yoksa da 200 döner (idempotent). Bildirim üretmez.
// @Tags         Davet Linki (Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Link iptal edildi"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Admin değil veya Anti-Spoiler"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/invite [delete]
func (ctl *InviteController) Revoke(c *gin.Context) {
	groupID, userID, ok := groupAndUser(c)
	if !ok {
		return
	}
	if err := ctl.inviteService.Revoke(groupID, userID); err != nil {
		respondInviteErr(c, "Davet linki iptal edilirken bir hata oluştu", err)
		return
	}
	RespondOK(c, MessageData{Message: "Davet linki iptal edildi"})
}

// Preview davet önizlemesini döner
// @Summary      Davet Önizleme
// @Description  Davet linkini açan kullanıcıya grup başlığını, davet edenin adını ve üye sayısını gösterir. Bilinmeyen, süresi dolmuş, iptal edilmiş veya biçimi bozuk token için aynı 404 döner. Anti-Spoiler: çağıran grubun hedef kişisiyse 403 + sürpriz mesajı döner ve gruba dair hiçbir veri sızmaz.
// @Tags         Davet Linki (Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        token path string true "Davet token'ı"
// @Success      200 {object} Response{data=services.InvitePreview} "Davet önizlemesi"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler"
// @Failure      404 {object} Response "Davet linki geçersiz veya süresi dolmuş"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/invites/{token} [get]
func (ctl *InviteController) Preview(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	preview, err := ctl.inviteService.Preview(c.Param("token"), userID)
	if err != nil {
		respondInviteErr(c, "Davet önizlenirken bir hata oluştu", err)
		return
	}
	RespondOK(c, preview)
}

// Accept çağıranı davetin grubuna üye yapar
// @Summary      Davetle Gruba Katıl
// @Description  Çağıranı davet linkinin grubuna `member` olarak ekler (admin onayı yok; tamamlanmış gruba da katılınabilir). Zaten üyeyse 200 döner, yeni satır ve bildirim üretilmez. Yeni üye için diğer üyelere "yeni üye katıldı" bildirimi gider ve grup akışına `changed` olayı yayınlanır. Anti-Spoiler: çağıran hedef kişiyse (kontrol grup satırı kilitliyken tekrarlanır) 403 + sürpriz mesajı döner. Geçersiz token için 404.
// @Tags         Davet Linki (Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        token path string true "Davet token'ı"
// @Success      200 {object} Response{data=AcceptInviteResponse} "Katılındı (ya da zaten üye)"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler"
// @Failure      404 {object} Response "Davet linki geçersiz veya süresi dolmuş"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/invites/{token}/accept [post]
func (ctl *InviteController) Accept(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	res, err := ctl.inviteService.Accept(c.Param("token"), userID)
	if err != nil {
		respondInviteErr(c, "Davete katılırken bir hata oluştu", err)
		return
	}
	if res.Joined {
		// Katılan kişi kendisi işlemi yaptığı için "gruba eklendin" almaz; Dispatch işlemi
		// yapanı zaten hariç tutar. Diğer üyelere "yeni üye katıldı" gider.
		RecordNotification(c, services.NotificationEvent{
			GroupID: res.GroupID, Type: models.NotificationMemberJoined,
			Params: models.NotificationParams{"member_name": res.MemberName},
		})
		RecordGroupChanged(c, res.GroupID)
	}
	RespondOK(c, AcceptInviteResponse{GroupID: res.GroupID})
}

// groupAndUser :id parametresini ve oturum kullanıcısını okur; geçersizse yanıtı yazar.
func groupAndUser(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return uuid.Nil, uuid.Nil, false
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return uuid.Nil, uuid.Nil, false
	}
	return groupID, userID, true
}

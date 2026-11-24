package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// FriendController arkadaşlık, kişisel arkadaşlık linki ve grup daveti isteklerini yönetir.
//
// Arkadaşlık olayları zil bildirimi üretmez (zil gruba bağlıdır); karşı tarafın bildirim akışına
// yalnızca "yeniden çek" sinyali gider ve arayüz Arkadaşlar rozetini günceller.
type FriendController struct {
	friends  services.FriendService
	notifier services.UserNotifier
}

// NewFriendController yeni bir FriendController oluşturur.
func NewFriendController(friends services.FriendService, notifier services.UserNotifier) *FriendController {
	return &FriendController{friends: friends, notifier: notifier}
}

// FriendRequestBody e-postayla arkadaşlık isteği gövdesidir.
type FriendRequestBody struct {
	Email string `json:"email" binding:"required,email,max=150" example:"zeynep@demo.com"`
}

// InviteFriendsBody gruba arkadaş davet etme gövdesidir.
type InviteFriendsBody struct {
	UserIDs []uuid.UUID `json:"user_ids" binding:"required,min=1,max=30" swaggertype:"array,string"`
}

// InvitedResponse davet edilen kişilerin kimlikleridir (zaten üye/davetli olanlar dahil değildir).
type InvitedResponse struct {
	Invited []uuid.UUID `json:"invited" swaggertype:"array,string"`
}

func respondFriendErr(c *gin.Context, internalMsg string, err error) {
	switch {
	case errors.Is(err, services.ErrSelfFriend), errors.Is(err, services.ErrNotFriends),
		errors.Is(err, services.ErrTooManyInvitees), errors.Is(err, services.ErrInviteeIsTarget):
		RespondErr(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrUnauthorizedMemberInvite),
		errors.Is(err, services.ErrNotGroupMember):
		RespondErr(c, http.StatusForbidden, err.Error())
	case errors.Is(err, services.ErrUserNotFound):
		RespondErr(c, http.StatusNotFound, "Bu e-postayla kayıtlı bir kullanıcı yok")
	case errors.Is(err, services.ErrFriendRequestMissing), errors.Is(err, services.ErrFriendNotFound),
		errors.Is(err, services.ErrFriendLinkNotFound), errors.Is(err, services.ErrMemberInviteNotFound),
		errors.Is(err, services.ErrGroupNotFound):
		RespondErr(c, http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrFriendshipExists):
		RespondErr(c, http.StatusConflict, err.Error())
	default:
		RespondInternal(c, internalMsg, err)
	}
}

func requireUser(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
	}
	return userID, ok
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz kimlik")
		return uuid.Nil, false
	}
	return id, true
}

// List arkadaşları ve bekleyen istekleri döner
// @Summary      Arkadaşlar
// @Description  Kabul edilmiş arkadaşlar, gelen ve giden istekler. E-posta dönmez; giden isteklerde karşı tarafın fotoğrafı yoktur.
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=services.FriendList}
// @Failure      401 {object} Response
// @Router       /api/v1/friends [get]
func (ctl *FriendController) List(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	list, err := ctl.friends.List(userID)
	if err != nil {
		respondFriendErr(c, "Arkadaşlar listelenirken bir hata oluştu", err)
		return
	}
	RespondOK(c, list)
}

// Request e-postayla arkadaşlık isteği gönderir
// @Summary      Arkadaşlık İsteği Gönder
// @Description  Tam e-posta adresiyle istek gönderir (kullanıcı arama yoktur). Karşı taraf zaten istek gönderdiyse doğrudan kabul edilir (status=ACCEPTED). Kendine 400, kayıtlı olmayan e-posta 404, zaten arkadaş ya da bekleyen istek 409.
// @Tags         Arkadaşlar (Friends)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body FriendRequestBody true "E-posta"
// @Success      201 {object} Response{data=services.FriendRequestResult}
// @Failure      400 {object} Response
// @Failure      404 {object} Response
// @Failure      409 {object} Response
// @Router       /api/v1/friends/requests [post]
func (ctl *FriendController) Request(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	var body FriendRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçerli bir e-posta adresi girin")
		return
	}
	res, err := ctl.friends.Request(userID, body.Email)
	if err != nil {
		respondFriendErr(c, "Arkadaşlık isteği gönderilirken bir hata oluştu", err)
		return
	}
	ctl.notifier.Publish(res.UserID)
	RespondCreated(c, res)
}

// AcceptRequest gelen isteği kabul eder
// @Summary      Arkadaşlık İsteğini Kabul Et
// @Description  Yalnızca isteğin alıcısı kabul edebilir; diğer durumlarda 404.
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Arkadaşlık (istek) kimliği" Format(uuid)
// @Success      200 {object} Response{data=MessageData}
// @Failure      404 {object} Response
// @Router       /api/v1/friends/requests/{id}/accept [post]
func (ctl *FriendController) AcceptRequest(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	other, err := ctl.friends.Accept(userID, id)
	if err != nil {
		respondFriendErr(c, "Arkadaşlık isteği kabul edilirken bir hata oluştu", err)
		return
	}
	ctl.notifier.Publish(other)
	RespondOK(c, MessageData{Message: "Artık arkadaşsınız"})
}

// DeleteRequest bekleyen isteği reddeder ya da geri çeker
// @Summary      Arkadaşlık İsteğini Reddet / Geri Çek
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Arkadaşlık (istek) kimliği" Format(uuid)
// @Success      200 {object} Response{data=MessageData}
// @Failure      404 {object} Response
// @Router       /api/v1/friends/requests/{id} [delete]
func (ctl *FriendController) DeleteRequest(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	other, err := ctl.friends.DeleteRequest(userID, id)
	if err != nil {
		respondFriendErr(c, "Arkadaşlık isteği silinirken bir hata oluştu", err)
		return
	}
	ctl.notifier.Publish(other)
	RespondOK(c, MessageData{Message: "İstek kaldırıldı"})
}

// Unfriend arkadaşlıktan çıkarır
// @Summary      Arkadaşlıktan Çıkar
// @Description  Kabul edilmiş arkadaşlığı siler. Ortak grup üyeliklerine dokunmaz.
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Param        userId path string true "Arkadaşın kullanıcı kimliği" Format(uuid)
// @Success      200 {object} Response{data=MessageData}
// @Failure      404 {object} Response
// @Router       /api/v1/friends/users/{userId} [delete]
func (ctl *FriendController) Unfriend(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	other, ok := uuidParam(c, "userId")
	if !ok {
		return
	}
	if err := ctl.friends.Unfriend(userID, other); err != nil {
		respondFriendErr(c, "Arkadaşlıktan çıkarılırken bir hata oluştu", err)
		return
	}
	ctl.notifier.Publish(other)
	RespondOK(c, MessageData{Message: "Arkadaşlıktan çıkarıldı"})
}

// GetLink kişisel arkadaşlık linkini döner (yoksa oluşturur)
// @Summary      Kişisel Arkadaşlık Linki
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=services.FriendLinkView}
// @Router       /api/v1/friends/link [get]
func (ctl *FriendController) GetLink(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	link, err := ctl.friends.GetLink(userID)
	if err != nil {
		respondFriendErr(c, "Arkadaşlık linki alınırken bir hata oluştu", err)
		return
	}
	RespondOK(c, link)
}

// ResetLink kişisel linki yeniler; eskisi geçersiz olur
// @Summary      Arkadaşlık Linkini Sıfırla
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=services.FriendLinkView}
// @Router       /api/v1/friends/link/reset [post]
func (ctl *FriendController) ResetLink(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	link, err := ctl.friends.ResetLink(userID)
	if err != nil {
		respondFriendErr(c, "Arkadaşlık linki sıfırlanırken bir hata oluştu", err)
		return
	}
	RespondOK(c, link)
}

// PreviewLink link sahibini gösterir
// @Summary      Arkadaşlık Linki Önizleme
// @Description  Bilinmeyen ya da biçimi bozuk kod için 404.
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Param        code path string true "Link kodu"
// @Success      200 {object} Response{data=services.FriendLinkPreview}
// @Failure      404 {object} Response
// @Router       /api/v1/friend-links/{code} [get]
func (ctl *FriendController) PreviewLink(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	preview, err := ctl.friends.PreviewLink(userID, c.Param("code"))
	if err != nil {
		respondFriendErr(c, "Arkadaşlık linki açılırken bir hata oluştu", err)
		return
	}
	RespondOK(c, preview)
}

// AcceptLink link sahibiyle doğrudan arkadaş yapar
// @Summary      Arkadaşlık Linkini Kabul Et
// @Description  Link sahibi linki bilerek paylaştığı için onay gerekmez; ikisi doğrudan arkadaş olur. Zaten arkadaşsa 200. Kendi linkine 400.
// @Tags         Arkadaşlar (Friends)
// @Produce      json
// @Security     BearerAuth
// @Param        code path string true "Link kodu"
// @Success      200 {object} Response{data=MessageData}
// @Failure      400 {object} Response
// @Failure      404 {object} Response
// @Router       /api/v1/friend-links/{code}/accept [post]
func (ctl *FriendController) AcceptLink(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	owner, err := ctl.friends.AcceptLink(userID, c.Param("code"))
	if err != nil {
		respondFriendErr(c, "Arkadaş eklenirken bir hata oluştu", err)
		return
	}
	ctl.notifier.Publish(owner)
	RespondOK(c, MessageData{Message: "Artık arkadaşsınız"})
}

// ListGroupInvites grubun bekleyen davetlerini döner
// @Summary      Grubun Bekleyen Davetleri
// @Description  Grubun tüm üyeleri görebilir. Hedef kişi Anti-Spoiler ile 403 alır.
// @Tags         Grup Davetleri (Member Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=[]repository.PendingInviteRow}
// @Router       /api/v1/groups/{id}/member-invites [get]
func (ctl *FriendController) ListGroupInvites(c *gin.Context) {
	groupID, userID, ok := groupAndUser(c)
	if !ok {
		return
	}
	rows, err := ctl.friends.ListGroupInvites(groupID, userID)
	if err != nil {
		respondFriendErr(c, "Davetler listelenirken bir hata oluştu", err)
		return
	}
	RespondOK(c, rows)
}

// InviteToGroup arkadaşları gruba davet eder
// @Summary      Arkadaşları Gruba Davet Et (Yalnızca Admin)
// @Description  Yalnızca kabul edilmiş arkadaşlar davet edilebilir (400). Grubun hedef kişisi davet edilemez (400). Zaten üye ya da davetli olanlar atlanır. Davetlilere "grup daveti" bildirimi gider.
// @Tags         Grup Davetleri (Member Invites)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body InviteFriendsBody true "Davet edilecek arkadaşlar"
// @Success      201 {object} Response{data=InvitedResponse}
// @Router       /api/v1/groups/{id}/member-invites [post]
func (ctl *FriendController) InviteToGroup(c *gin.Context) {
	groupID, userID, ok := groupAndUser(c)
	if !ok {
		return
	}
	var body InviteFriendsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: en az 1, en fazla 30 kişi seçin")
		return
	}
	invited, err := ctl.friends.InviteToGroup(groupID, userID, body.UserIDs)
	if err != nil {
		respondFriendErr(c, "Davet gönderilirken bir hata oluştu", err)
		return
	}
	RecordInviteNotifications(c, groupID, invited)
	RespondCreated(c, InvitedResponse{Invited: invited})
}

// RecordInviteNotifications davet edilenlere gidecek "grup daveti" bildirimini kaydeder.
func RecordInviteNotifications(c *gin.Context, groupID uuid.UUID, invited []uuid.UUID) {
	if len(invited) == 0 {
		return
	}
	RecordNotification(c, services.NotificationEvent{
		GroupID: groupID, Type: models.NotificationGroupInvite, Only: invited, NonMembers: true,
	})
}

// CancelGroupInvite bekleyen daveti iptal eder
// @Summary      Daveti İptal Et (Yalnızca Admin)
// @Tags         Grup Davetleri (Member Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        userId path string true "Davetli kullanıcı" Format(uuid)
// @Success      200 {object} Response{data=MessageData}
// @Failure      404 {object} Response
// @Router       /api/v1/groups/{id}/member-invites/{userId} [delete]
func (ctl *FriendController) CancelGroupInvite(c *gin.Context) {
	groupID, userID, ok := groupAndUser(c)
	if !ok {
		return
	}
	inviteeID, ok := uuidParam(c, "userId")
	if !ok {
		return
	}
	if err := ctl.friends.CancelGroupInvite(groupID, userID, inviteeID); err != nil {
		respondFriendErr(c, "Davet iptal edilirken bir hata oluştu", err)
		return
	}
	ctl.notifier.Publish(inviteeID)
	RespondOK(c, MessageData{Message: "Davet iptal edildi"})
}

// MyInvites kullanıcıya gelen grup davetlerini döner
// @Summary      Grup Davetlerim
// @Description  Anti-Spoiler: kullanıcının hedef kişisi olduğu grupların davetleri hiç dönmez.
// @Tags         Grup Davetleri (Member Invites)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=[]repository.MyInviteRow}
// @Router       /api/v1/me/group-invites [get]
func (ctl *FriendController) MyInvites(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	rows, err := ctl.friends.MyInvites(userID)
	if err != nil {
		respondFriendErr(c, "Davetler listelenirken bir hata oluştu", err)
		return
	}
	RespondOK(c, rows)
}

// AcceptInvite daveti kabul edip gruba katılır
// @Summary      Grup Davetini Kabul Et
// @Description  Kabul edilince üye olunur, diğer üyelere "yeni üye katıldı" bildirimi gider. Anti-Spoiler: hedef kişi 403 alır (kontrol grup satırı kilitliyken yapılır).
// @Tags         Grup Davetleri (Member Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        inviteId path string true "Davet kimliği" Format(uuid)
// @Success      200 {object} Response{data=AcceptInviteResponse}
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /api/v1/me/group-invites/{inviteId}/accept [post]
func (ctl *FriendController) AcceptInvite(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "inviteId")
	if !ok {
		return
	}
	res, err := ctl.friends.AcceptInvite(userID, id)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
			return
		}
		respondFriendErr(c, "Davet kabul edilirken bir hata oluştu", err)
		return
	}
	if res.Joined {
		RecordNotification(c, services.NotificationEvent{
			GroupID: res.GroupID, Type: models.NotificationMemberJoined,
			Params: models.NotificationParams{"member_name": res.MemberName},
		})
		RecordGroupChanged(c, res.GroupID)
	}
	ctl.notifier.Publish(userID)
	RespondOK(c, AcceptInviteResponse{GroupID: res.GroupID})
}

// DeclineInvite daveti reddeder
// @Summary      Grup Davetini Reddet
// @Tags         Grup Davetleri (Member Invites)
// @Produce      json
// @Security     BearerAuth
// @Param        inviteId path string true "Davet kimliği" Format(uuid)
// @Success      200 {object} Response{data=MessageData}
// @Failure      404 {object} Response
// @Router       /api/v1/me/group-invites/{inviteId} [delete]
func (ctl *FriendController) DeclineInvite(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	id, ok := uuidParam(c, "inviteId")
	if !ok {
		return
	}
	res, err := ctl.friends.DeclineInvite(userID, id)
	if err != nil {
		respondFriendErr(c, "Davet reddedilirken bir hata oluştu", err)
		return
	}
	RecordGroupChanged(c, res.GroupID)
	ctl.notifier.Publish(userID)
	RespondOK(c, MessageData{Message: "Davet reddedildi"})
}

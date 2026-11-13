package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PoolController MULTI grupların alıcı ve havuz HTTP isteklerini yönetir. Havuz fikirleri
// (listeleme, ekleme, AI) IdeaController'daki aynı handler'larla, havuz rotasında çalışır.
type PoolController struct {
	poolService services.PoolService
}

// NewPoolController yeni bir PoolController oluşturur.
func NewPoolController(poolService services.PoolService) *PoolController {
	return &PoolController{poolService: poolService}
}

// AddRecipientRequest bir üyeyi hediye alacak kişi yapma isteğidir. manager_user_id yalnızca
// grup yöneticisi kendini alıcı yaparken verilir (zorunludur); başka bir alıcı için verilmez.
type AddRecipientRequest struct {
	UserID        uuid.UUID  `json:"user_id" binding:"required" format:"uuid"`
	ManagerUserID *uuid.UUID `json:"manager_user_id,omitempty" format:"uuid"`
}

// SetManagerRequest grup yöneticisinin kendi havuzuna yönetici atama isteğidir. null yalnızca
// yöneticisi olmayan (grup yöneticisi dışındaki) alıcılar için geçerlidir.
type SetManagerRequest struct {
	ManagerUserID *uuid.UUID `json:"manager_user_id" format:"uuid"`
}

// ContributionRequest çağıranın bir havuza katkı verip vermediğini ayarlar.
type ContributionRequest struct {
	Contributing *bool `json:"contributing" binding:"required" example:"false"`
}

// requireAdminRole GroupAccess'in yazdığı rolü okur; yönetici değilse gövde okunmadan 403 döner
// (diğer yönetici uçlarıyla aynı: geçersiz gövdeli üye isteği 400 değil 403 alır). Servis yetkiyi
// ayrıca denetler.
func requireAdminRole(c *gin.Context) bool {
	if role, _ := c.Get("currentMemberRole"); role != "admin" {
		RespondErr(c, http.StatusForbidden, services.ErrUnauthorizedRecipients.Error())
		return false
	}
	return true
}

// poolError havuz ve alıcı işlemlerinin ortak hata eşlemesidir.
func poolError(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember),
		errors.Is(err, services.ErrUnauthorizedPoolManage), errors.Is(err, services.ErrUnauthorizedRecipients):
		RespondErr(c, http.StatusForbidden, err.Error())
	case errors.Is(err, services.ErrGroupNotFound), errors.Is(err, services.ErrPoolNotFound),
		errors.Is(err, services.ErrRecipientNotFound), errors.Is(err, services.ErrInterestNotFound):
		RespondErr(c, http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrInvalidBudget), errors.Is(err, services.ErrInvalidEventDate),
		errors.Is(err, services.ErrInvalidInterest), errors.Is(err, services.ErrInvalidWinners),
		errors.Is(err, services.ErrWinnerNotInPool), errors.Is(err, services.ErrRecipientNotMember),
		errors.Is(err, services.ErrManagerNotMember), errors.Is(err, services.ErrManagerIsRecipient),
		errors.Is(err, services.ErrManagerNotAllowed), errors.Is(err, services.ErrManagerRequired):
		RespondErr(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrWrongGroupMode), errors.Is(err, services.ErrAlreadyRecipient),
		errors.Is(err, services.ErrPoolHasIdeas), errors.Is(err, services.ErrPoolCompleted),
		errors.Is(err, services.ErrPoolNotCompleted), errors.Is(err, services.ErrWinnerNotActive),
		errors.Is(err, services.ErrDuplicateInterest):
		RespondErr(c, http.StatusConflict, err.Error())
	default:
		RespondInternal(c, internalMsg, err)
	}
}

// groupPoolUser grup ve havuz kimliklerini ve oturumdaki kullanıcıyı okur; hata yanıtını kendisi yazar.
func groupPoolUser(c *gin.Context) (groupID, poolID, userID uuid.UUID, ok bool) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	poolID, err = uuid.Parse(c.Param("poolId"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz havuz kimliği")
		return
	}
	userID, ok = getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
	}
	return
}

// recipientParams grup kimliğini, :userId parametresini ve oturumdaki kullanıcıyı okur.
func recipientParams(c *gin.Context) (groupID, recipientID, userID uuid.UUID, ok bool) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	recipientID, err = uuid.Parse(c.Param("userId"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz kullanıcı kimliği")
		return
	}
	userID, ok = getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
	}
	return
}

// ListPools grubun havuz kartlarını listeler
// @Summary      Havuzları Listele (MULTI, Anti-Spoiler Korumalı)
// @Description  Her üye çağırabilir. Kurulma sırasıyla havuz kartları döner: çağıranın kendi havuzu yalnızca `{id, recipient, is_mine: true, locked: true}` (LockedPool), diğerleri OpenPool. SINGLE grupta 409.
// @Tags         Havuzlar (Pools)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=[]services.OpenPool} "Havuz kartları (kendi havuzun LockedPool)"
// @Failure      403 {object} Response "Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      409 {object} Response "Grup türü uygun değil (SINGLE)"
// @Router       /api/v1/groups/{id}/pools [get]
func (ctl *PoolController) ListPools(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	views, err := ctl.poolService.ListPools(groupID, userID)
	if err != nil {
		poolError(c, err, "Havuzlar alınırken bir hata oluştu")
		return
	}
	RespondOK(c, views)
}

// AddRecipient bir üyeyi hediye alacak kişi yapar ve havuzunu kurar
// @Summary      Hediye Alacak Kişi Ekle (MULTI, Admin)
// @Description  Yalnızca grup yöneticisi. Üye alıcı yapılır ve havuzu kurulur; alıcı dışındaki tüm üyeler varsayılan olarak katkı verir. Yönetici kendini alıcı yapabilir ama yalnızca `manager_user_id` ile başka bir üyeyi havuz yöneticisi seçerek; başka bir alıcı için `manager_user_id` gönderilemez. Yanıttaki havuz çağırana göredir: yönetici kendini alıcı yaptıysa LockedPool.
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body AddRecipientRequest true "Alıcı ve (gerekirse) havuz yöneticisi"
// @Success      201 {object} Response{data=services.RecipientAdded} "Havuz kuruldu"
// @Failure      400 {object} Response "Kişi veya yönetici üye değil, yönetici alıcıyla aynı, yönetici gereksiz ya da eksik"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      409 {object} Response "Grup türü SINGLE veya kişi zaten alıcı"
// @Router       /api/v1/groups/{id}/recipients [post]
func (ctl *PoolController) AddRecipient(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	if !requireAdminRole(c) {
		return
	}
	var req AddRecipientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: user_id alanı zorunludur")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	added, pool, err := ctl.poolService.AddRecipient(groupID, userID, req.UserID, req.ManagerUserID)
	if err != nil {
		poolError(c, err, "Hediye alacak kişi eklenirken bir hata oluştu")
		return
	}
	// Alıcı ve işlemi yapan dışındaki üyelere; alıcı Dispatch'te ayrıca dışlanır.
	RecordNotification(c, services.NotificationEvent{GroupID: groupID, PoolID: &pool.ID, Type: models.NotificationPoolAdded})
	RespondCreated(c, added)
}

// SetRecipientManager grup yöneticisinin kendi havuzunun yöneticisini değiştirir
// @Summary      Havuz Yöneticisini Değiştir (MULTI, Admin)
// @Description  Yalnızca grup yöneticisi. Grup yöneticisinin kendi havuzu için `manager_user_id` zorunludur (null 400); hesabı silinen yöneticinin yerine yenisini atamak için de kullanılır. Diğer alıcıların havuzunu her zaman grup yöneticisi yönetir; onlar için yalnızca null kabul edilir (değişiklik yapmaz). Yeni yöneticiye `pool_manager_assigned` bildirimi gider.
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        userId path string true "Alıcının kullanıcı UUID'si" Format(uuid)
// @Param        request body SetManagerRequest true "Yeni havuz yöneticisi"
// @Success      200 {object} Response{data=services.ManagerChanged} "Havuz yöneticisi güncellendi"
// @Failure      400 {object} Response "Yönetici üye değil, alıcıyla aynı, gereksiz ya da eksik"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı veya kişi alıcı değil"
// @Failure      409 {object} Response "Grup türü SINGLE"
// @Router       /api/v1/groups/{id}/recipients/{userId}/manager [patch]
func (ctl *PoolController) SetRecipientManager(c *gin.Context) {
	groupID, recipientID, userID, ok := recipientParams(c)
	if !ok || !requireAdminRole(c) {
		return
	}
	var req SetManagerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: manager_user_id bir kullanıcı kimliği veya null olmalıdır")
		return
	}
	changed, err := ctl.poolService.SetManager(groupID, userID, recipientID, req.ManagerUserID)
	if err != nil {
		poolError(c, err, "Havuz yöneticisi güncellenirken bir hata oluştu")
		return
	}
	if changed.Changed && changed.Manager != nil {
		RecordNotification(c, services.NotificationEvent{
			GroupID: groupID, PoolID: &changed.PoolID, Type: models.NotificationPoolManagerAssigned,
			Only: []uuid.UUID{*changed.Manager},
		})
	}
	RespondOK(c, changed)
}

// RemoveRecipient bir alıcıyı ve havuzunu kaldırır
// @Summary      Hediye Alacak Kişiyi Kaldır (MULTI, Admin)
// @Description  Yalnızca grup yöneticisi. Havuz; fikirleri, oyları, katkıları ve bildirimleriyle birlikte silinir. Havuzda fikir varsa `confirm=true` olmadan 409 döner. Anti-Spoiler: çağıran o havuzun alıcısıysa (yönetici kendi havuzu) `confirm=true` olmadan fikir olsun olmasın her zaman 409 döner; böylece havuzda fikir olup olmadığı anlaşılamaz. Bildirim üretmez.
// @Tags         Havuzlar (Pools)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        userId path string true "Alıcının kullanıcı UUID'si" Format(uuid)
// @Param        confirm query bool false "Fikirli havuzu silmeyi onayla"
// @Success      200 {object} Response{data=MessageData} "Alıcı kaldırıldı"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı veya kişi alıcı değil"
// @Failure      409 {object} Response "Grup türü SINGLE veya onay gerekli"
// @Router       /api/v1/groups/{id}/recipients/{userId} [delete]
func (ctl *PoolController) RemoveRecipient(c *gin.Context) {
	groupID, recipientID, userID, ok := recipientParams(c)
	if !ok || !requireAdminRole(c) {
		return
	}
	confirm := c.Query("confirm") == "true"
	if err := ctl.poolService.RemoveRecipient(groupID, userID, recipientID, confirm); err != nil {
		poolError(c, err, "Hediye alacak kişi kaldırılırken bir hata oluştu")
		return
	}
	RespondOK(c, MessageData{Message: "Hediye alacak kişi ve havuzu kaldırıldı"})
}

// GetPool tek bir havuzun detayını döner
// @Summary      Havuz Detayı (MULTI, Anti-Spoiler Korumalı)
// @Description  Alıcı dışındaki üyeler. OpenPool alanlarına ek olarak ilgi alanları ve alıcı dışındaki tüm üyelerin katkı durumu. Havuzun alıcısı 403 + sürpriz mesajı alır.
// @Tags         Havuzlar (Pools)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Success      200 {object} Response{data=services.PoolDetail} "Havuz detayı"
// @Failure      403 {object} Response "Anti-Spoiler (havuzun alıcısı) veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Router       /api/v1/groups/{id}/pools/{poolId} [get]
func (ctl *PoolController) GetPool(c *gin.Context) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	detail, err := ctl.poolService.GetPool(groupID, poolID, userID)
	if err != nil {
		poolError(c, err, "Havuz alınırken bir hata oluştu")
		return
	}
	RespondOK(c, detail)
}

// UpdatePoolBudget havuzun bütçesini günceller
// @Summary      Havuz Bütçesini Güncelle (Havuz Yöneticisi)
// @Description  Yalnızca havuzun etkin yöneticisi. 0–99999999.99, en fazla iki ondalık. Kişi başı tutar = bütçe / katkı veren sayısı (anlık). Değer gerçekten değiştiyse `budget_changed` bildirimi (pool_id ile, alıcı hariç).
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Param        request body UpdateBudgetRequest true "Yeni toplam bütçe"
// @Success      200 {object} Response{data=MessageData} "Bütçe güncellendi"
// @Failure      400 {object} Response "Geçersiz bütçe"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya havuz yöneticisi değil"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Router       /api/v1/groups/{id}/pools/{poolId}/budget [patch]
func (ctl *PoolController) UpdatePoolBudget(c *gin.Context) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	var req UpdateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: total_budget sayısal alanı zorunludur")
		return
	}
	if err := ctl.poolService.UpdateBudget(groupID, poolID, userID, *req.TotalBudget); err != nil {
		poolError(c, err, "Bütçe güncellenirken bir hata oluştu")
		return
	}
	if before := poolBeforeRequest(c); before == nil || !before.TotalBudget.Equal(*req.TotalBudget) {
		RecordNotification(c, services.NotificationEvent{
			GroupID: groupID, PoolID: &poolID, Type: models.NotificationBudgetChanged,
			Params: models.NotificationParams{"total_budget": req.TotalBudget.StringFixed(2)},
		})
	}
	RespondOK(c, MessageData{Message: "Bütçe güncellendi"})
}

// AddPoolInterest havuza bir ilgi alanı ekler
// @Summary      Havuza İlgi Alanı Ekle
// @Description  Alıcı dışındaki tüm üyeler (alıcı kendi havuzuna ilgi alanı ekleyemez; 403 + sürpriz mesajı). 1–100 karakter, virgül/noktalı virgül/satır sonu yok; tekrar 409. Bildirim üretmez.
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Param        request body InterestRequest true "Eklenecek ilgi alanı"
// @Success      200 {object} Response{data=InterestListResponse} "Güncel ilgi alanı listesi"
// @Failure      400 {object} Response "Geçersiz ilgi alanı"
// @Failure      403 {object} Response "Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Failure      409 {object} Response "İlgi alanı zaten listede"
// @Router       /api/v1/groups/{id}/pools/{poolId}/interests [post]
func (ctl *PoolController) AddPoolInterest(c *gin.Context) {
	ctl.changePoolInterest(c, false)
}

// DeletePoolInterest havuzdan bir ilgi alanı siler
// @Summary      Havuzdan İlgi Alanı Sil (Havuz Yöneticisi)
// @Description  Yalnızca havuzun etkin yöneticisi. Bildirim üretmez.
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Param        request body InterestRequest true "Silinecek ilgi alanı"
// @Success      200 {object} Response{data=InterestListResponse} "Güncel ilgi alanı listesi"
// @Failure      400 {object} Response "Geçersiz istek"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya havuz yöneticisi değil"
// @Failure      404 {object} Response "Grup, havuz veya ilgi alanı bulunamadı"
// @Router       /api/v1/groups/{id}/pools/{poolId}/interests [delete]
func (ctl *PoolController) DeletePoolInterest(c *gin.Context) {
	ctl.changePoolInterest(c, true)
}

func (ctl *PoolController) changePoolInterest(c *gin.Context, remove bool) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	var req InterestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: interest metin alanı zorunludur")
		return
	}
	var (
		interests []string
		err       error
	)
	if remove {
		interests, err = ctl.poolService.DeleteInterest(groupID, poolID, userID, req.Interest)
	} else {
		interests, err = ctl.poolService.AddInterest(groupID, poolID, userID, req.Interest)
	}
	if err != nil {
		poolError(c, err, "İlgi alanları güncellenirken bir hata oluştu")
		return
	}
	RespondOK(c, InterestListResponse{Interests: interests})
}

// UpdatePoolEventDate havuzun hediye tarihini ayarlar veya kaldırır
// @Summary      Havuz Hediye Tarihini Güncelle (Havuz Yöneticisi)
// @Description  Yalnızca havuzun etkin yöneticisi. "YYYY-MM-DD" veya null. Bugünden en fazla 1 yıl önce, 5 yıl sonra. Tarih gerçekten değiştiyse `event_date_changed` bildirimi (pool_id ile, alıcı hariç).
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Param        request body UpdateEventDateRequest true "Hediye tarihi"
// @Success      200 {object} Response{data=MessageData} "Tarih güncellendi"
// @Failure      400 {object} Response "Geçersiz tarih"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya havuz yöneticisi değil"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Router       /api/v1/groups/{id}/pools/{poolId}/event-date [patch]
func (ctl *PoolController) UpdatePoolEventDate(c *gin.Context) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	var req UpdateEventDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: event_date YYYY-AA-GG biçiminde veya null olmalıdır")
		return
	}
	if err := ctl.poolService.SetEventDate(groupID, poolID, userID, req.EventDate); err != nil {
		poolError(c, err, "Hediye tarihi güncellenirken bir hata oluştu")
		return
	}
	if before := poolBeforeRequest(c); before == nil || !sameDate(before.EventDate, req.EventDate) {
		var date any
		if req.EventDate != nil {
			date = req.EventDate.String()
		}
		RecordNotification(c, services.NotificationEvent{
			GroupID: groupID, PoolID: &poolID, Type: models.NotificationEventDateChanged,
			Params: models.NotificationParams{"event_date": date},
		})
	}
	RespondOK(c, MessageData{Message: "Hediye tarihi güncellendi"})
}

// SelectPoolWinner havuzun kazananlarını seçer ve havuzu tamamlar
// @Summary      Havuzun Kazananlarını Seç (Havuz Yöneticisi)
// @Description  Yalnızca havuzun etkin yöneticisi. 1–20 aktif fikir SELECTED, havuz COMPLETED olur. Fikirlerden biri bu havuza ait değilse 400, aktif değilse ya da havuz zaten tamamlandıysa 409. Tamamlanan havuzda fikir ekleme, AI, oy, veto, fikir silme ve katkı değişikliği 409 döner.
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Param        request body SelectWinnerRequest true "Kazanan fikirler"
// @Success      200 {object} Response{data=MessageData} "Havuz tamamlandı"
// @Failure      400 {object} Response "Fikir bu havuza ait değil veya istek geçersiz"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya havuz yöneticisi değil"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Failure      409 {object} Response "Havuz zaten tamamlandı veya fikir aktif değil"
// @Router       /api/v1/groups/{id}/pools/{poolId}/winner [post]
func (ctl *PoolController) SelectPoolWinner(c *gin.Context) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	var req SelectWinnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: idea_ids 1–20 öneri kimliği içermelidir")
		return
	}
	count, err := ctl.poolService.SelectWinners(groupID, poolID, userID, req.IdeaIDs)
	if err != nil {
		poolError(c, err, "Kazanan seçilirken bir hata oluştu")
		return
	}
	RecordNotification(c, services.NotificationEvent{
		GroupID: groupID, PoolID: &poolID, Type: models.NotificationWinnerSelected,
		Params: models.NotificationParams{"count": count},
	})
	RespondOK(c, MessageData{Message: "Kazanan seçildi, havuz tamamlandı"})
}

// ReopenPool tamamlanan havuzu yeniden açar
// @Summary      Havuzu Yeniden Aç (Havuz Yöneticisi)
// @Description  Yalnızca havuzun etkin yöneticisi. Kazanan fikirler ACTIVE'e, havuz ACTIVE'e döner. Havuz zaten açıksa 409.
// @Tags         Havuzlar (Pools)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Havuz yeniden açıldı"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya havuz yöneticisi değil"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Failure      409 {object} Response "Havuz zaten açık"
// @Router       /api/v1/groups/{id}/pools/{poolId}/winner [delete]
func (ctl *PoolController) ReopenPool(c *gin.Context) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	if err := ctl.poolService.Reopen(groupID, poolID, userID); err != nil {
		poolError(c, err, "Havuz yeniden açılırken bir hata oluştu")
		return
	}
	RecordNotification(c, services.NotificationEvent{GroupID: groupID, PoolID: &poolID, Type: models.NotificationGroupReopened})
	RespondOK(c, MessageData{Message: "Havuz yeniden açıldı"})
}

// SetContribution çağıranın havuza katkı durumunu ayarlar
// @Summary      Havuza Katkımı Ayarla
// @Description  Alıcı dışındaki her üye yalnızca kendi katkısını açıp kapatır. Katkı vermeyen üye yine fikir ekler ve oy verir; hiçbir şey silinmez. Kişi başı tutar = bütçe / katkı veren sayısı; katkı veren yoksa `per_person_budget` null ve `no_contributors` true. Havuz tamamlandıysa 409. Bildirim üretmez.
// @Tags         Havuzlar (Pools)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        poolId path string true "Havuz UUID" Format(uuid)
// @Param        request body ContributionRequest true "Katkı durumu"
// @Success      200 {object} Response{data=services.ContributionResult} "Güncel katkı durumu"
// @Failure      400 {object} Response "Geçersiz istek"
// @Failure      403 {object} Response "Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup veya havuz bulunamadı"
// @Failure      409 {object} Response "Havuz tamamlandı"
// @Router       /api/v1/groups/{id}/pools/{poolId}/contribution [put]
func (ctl *PoolController) SetContribution(c *gin.Context) {
	groupID, poolID, userID, ok := groupPoolUser(c)
	if !ok {
		return
	}
	var req ContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: contributing alanı (true/false) zorunludur")
		return
	}
	result, err := ctl.poolService.SetContribution(groupID, poolID, userID, *req.Contributing)
	if err != nil {
		poolError(c, err, "Katkı durumu güncellenirken bir hata oluştu")
		return
	}
	RespondOK(c, result)
}

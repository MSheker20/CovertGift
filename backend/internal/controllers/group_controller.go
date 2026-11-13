package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// GroupController grup HTTP isteklerini yöneten yapıdır
type GroupController struct {
	groupService services.GroupService
}

// NewGroupController yeni bir GroupController örneği oluşturur
func NewGroupController(groupService services.GroupService) *GroupController {
	return &GroupController{groupService: groupService}
}

// CreateGroupRequest yeni grup açma isteği için DTO. Açıklama kırpıldıktan sonra en fazla
// 1000 karakterdir; servis denetler (düzenleme ucuyla aynı kural).
//
// mode verilmezse SINGLE'dır (eski istemciler aynı davranır). SINGLE'da target_person_name
// zorunludur. MULTI'de target_*, total_budget (> 0) ve event_date gönderilemez (400); alıcılar
// sonradan POST /groups/:id/recipients ile seçilir.
type CreateGroupRequest struct {
	Title            string          `json:"title" binding:"required,min=2,max=150" example:"Ahmet'in 30. Yaş Sürprizi"`
	Description      string          `json:"description" maxLength:"1000" example:"Yakın arkadaşlarla ortak doğum günü hediyesi organizasyonu"`
	Mode             string          `json:"mode" binding:"omitempty,oneof=SINGLE MULTI" enums:"SINGLE,MULTI" example:"SINGLE"`
	TargetPersonName string          `json:"target_person_name" binding:"omitempty,min=2,max=100" example:"Ahmet"`
	TargetInterests  string          `json:"target_interests" binding:"max=2000" example:"Kamp, 3. nesil kahve demleme, trekking, outdoor"`
	TargetUserID     *uuid.UUID      `json:"target_user_id,omitempty" format:"uuid" example:"b49770ed-7518-40e6-839d-5810f79bbd86"`
	TargetEmail      string          `json:"target_email,omitempty" binding:"omitempty,email,max=150" example:"ahmet@demo.com"`
	TotalBudget      decimal.Decimal `json:"total_budget" example:"2500.00"`
	EventDate        *models.Date    `json:"event_date,omitempty" swaggertype:"string" format:"date" example:"2026-12-31"`
}

// AddMemberRequest gruba üye ekleme isteği için DTO
type AddMemberRequest struct {
	Email string `json:"email" binding:"required,email,max=150" example:"zeynep@demo.com"`
}

// LinkTargetRequest mevcut gruba hedef kişiyi bağlama isteği için DTO
type LinkTargetRequest struct {
	Email string `json:"email" binding:"required,email,max=150" example:"ahmet@demo.com"`
}

type InterestRequest struct {
	Interest string `json:"interest" binding:"required" example:"Kamp"`
}

type InterestListResponse struct {
	Interests []string `json:"interests"`
}

// SelectWinnerRequest kazanan hediye fikirlerini taşır (1–20 farklı fikir)
type SelectWinnerRequest struct {
	IdeaIDs []uuid.UUID `json:"idea_ids" binding:"required,min=1,max=20" swaggertype:"array,string" example:"7d1e5a3c-2f4b-4c8e-9a6d-1b2c3d4e5f60"`
}

// UpdateEventDateRequest hediye tarihini taşır; null tarihi kaldırır
type UpdateEventDateRequest struct {
	EventDate *models.Date `json:"event_date" swaggertype:"string" format:"date" example:"2026-12-31"`
}

// UpdateDescriptionRequest grup açıklamasını taşır. Metin kırpılır; boş ya da yalnızca boşluk
// açıklamayı siler. Kırpıldıktan sonra en fazla 1000 karakter.
type UpdateDescriptionRequest struct {
	Description *string `json:"description" binding:"required" maxLength:"1000" example:"Kampı seven Ahmet için ortak hediye. Fikirleri hafta sonuna kadar ekleyelim."`
}

type UpdateBudgetRequest struct {
	TotalBudget *decimal.Decimal `json:"total_budget" binding:"required" example:"3500.00"`
}

// CreateGroup yeni grup oluşturur
// @Summary      Yeni Hediye Grubu Oluştur
// @Description  Yeni bir sürpriz hediye grubu açar ve oluşturan kullanıcıyı atomik transaction ile 'admin' rolüyle üye yapar. `mode` SINGLE (varsayılan, tek kişi için) veya MULTI (birden çok kişi için; her alıcının havuzu ayrı). SINGLE'da `target_person_name` zorunludur; hedef kişi isteğe bağlı `target_email` ile seçilir; sunucu kayıtlı kullanıcıyı bulur ve `target_user_id` alanını doldurur. `target_email` ve `target_user_id` birlikte gönderilemez. Hedef seçilmezse yalnızca `target_person_name` kullanılır ve Anti-Spoiler devreye girmez. MULTI'de hedef alanları, `total_budget` > 0 ve `event_date` 400 döner; alıcılar sonradan `POST /groups/{id}/recipients` ile seçilir. Tür sonradan değişmez.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreateGroupRequest true "Grup Oluşturma Bilgileri"
// @Success      201 {object} Response{data=models.Group} "Grup başarıyla oluşturuldu"
// @Failure      400 {object} Response "Geçersiz parametreler, iki hedef alanı birlikte gönderildi veya kullanıcı kendine sürpriz grubu açamaz"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      404 {object} Response "Hedef kişinin e-postası kayıtlı değil"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups [post]
func (ctl *GroupController) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	if req.Mode == models.GroupModeMulti {
		if req.TargetPersonName != "" || req.TargetInterests != "" || req.TargetUserID != nil || req.TargetEmail != "" ||
			!req.TotalBudget.IsZero() || req.EventDate != nil {
			RespondErr(c, http.StatusBadRequest, services.ErrMultiTargetFields.Error())
			return
		}
		group, err := ctl.groupService.CreateMultiGroup(userID, req.Title, req.Description)
		if err != nil {
			if errors.Is(err, services.ErrInvalidDescription) {
				RespondErr(c, http.StatusBadRequest, err.Error())
				return
			}
			RespondInternal(c, "Grup oluşturulurken bir hata oluştu", err)
			return
		}
		RespondCreated(c, group)
		return
	}
	if req.TargetPersonName == "" {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: target_person_name alanı zorunludur (2–100 karakter)")
		return
	}

	group, err := ctl.groupService.CreateGroup(
		userID,
		req.Title,
		req.Description,
		req.TargetPersonName,
		req.TargetInterests,
		req.TargetUserID,
		req.TargetEmail,
		req.TotalBudget,
		req.EventDate,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSelfTarget), errors.Is(err, services.ErrTargetInputConflict), errors.Is(err, services.ErrInvalidBudget), errors.Is(err, services.ErrInvalidEventDate), errors.Is(err, services.ErrInvalidDescription):
			RespondErr(c, http.StatusBadRequest, err.Error())
			return
		case errors.Is(err, services.ErrTargetUserNotFound):
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondInternal(c, "Grup oluşturulurken bir hata oluştu", err)
		return
	}

	RespondCreated(c, group)
}

// GetUserGroups kullanıcının dahil olduğu grupları listeler
// @Summary      Kullanıcının Gruplarını Listele (Anti-Spoiler Korumalı)
// @Description  Giriş yapmış kullanıcının üye olduğu grupları listeler. Anti-Spoiler kuralı gereğince kullanıcının hedef kişi olduğu sürpriz gruplar bu listede ASLA yer almaz. Her grupta member_count, idea_count (reddedilmemiş fikirler) ve çağıran kullanıcıya özel pending_votes (açık gruptaki, kendisinin eklemediği ve henüz oy vermediği aktif fikirler) bulunur. MULTI gruplarda ayrıca recipients ve next_event_date bulunur; sayaçlar, status/completed_at ve next_event_date yalnızca çağıranın alıcısı olmadığı havuzlardan hesaplanır (Anti-Spoiler).
// @Tags         Hediye Grupları (Groups)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=[]services.GroupListItem} "Kullanıcının dahil olduğu gruplar listesi"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups [get]
func (ctl *GroupController) GetUserGroups(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	groups, err := ctl.groupService.GetUserGroups(userID)
	if err != nil {
		RespondInternal(c, "Gruplar alınırken bir hata oluştu", err)
		return
	}

	if groups == nil {
		groups = []services.GroupListItem{}
	}

	RespondOK(c, groups)
}

// GetGroupDetail grubun detaylarını, bütçesini ve üyelerini getirir
// @Summary      Grup Detayı ve Dinamik Kişi Başı Bütçe (Anti-Spoiler Korumalı)
// @Description  Grup detayını, aktif üye listesini, toplam bütçeyi ve anlık hesaplanan kişi başı bütçeyi getirir. Hedef kişi bu uç noktaya erişmeye çalışırsa 403 Forbidden alır. MULTI grupta grup düzeyindeki bütçe/ilgi alanı boştur ve `pools` dizisi döner: çağıranın kendi havuzu yalnızca `{id, recipient, is_mine: true, locked: true}`, diğerleri OpenPool.
// @Tags         Hediye Grupları (Groups)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=services.GroupDetailResponse} "Grup detayları ve bütçe durumu"
// @Failure      400 {object} Response "Geçersiz grup kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Erişim engellendi: Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id} [get]
func (ctl *GroupController) GetGroupDetail(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	detail, err := ctl.groupService.GetGroupDetail(groupID, userID)
	if err != nil {
		if errors.Is(err, services.ErrAntiSpoilerTriggered) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrNotGroupMember) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrGroupNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondInternal(c, "Grup detayı alınırken bir hata oluştu", err)
		return
	}

	RespondOK(c, detail)
}

// AddMember gruba e-posta ile üye ekler
// @Summary      Gruba E-posta ile Üye Ekle
// @Description  Gruptaki mevcut bir üye tarafından sisteme kayıtlı başka bir kullanıcıyı e-posta adresiyle gruba ekler. Hedef kişi üye olarak eklenemez.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body AddMemberRequest true "Eklenecek Üyenin E-postası"
// @Success      201 {object} Response{data=models.GroupMember} "Üye başarıyla eklendi"
// @Failure      400 {object} Response "Hedef kişi gruba üye eklenemez veya geçersiz istek"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Bu gruba üye ekleme yetkiniz yok"
// @Failure      404 {object} Response "Kullanıcı veya grup bulunamadı"
// @Failure      409 {object} Response "Kullanıcı zaten grubun üyesi"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/members [post]
func (ctl *GroupController) AddMember(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	currentUserID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	member, err := ctl.groupService.AddMemberByEmail(groupID, currentUserID, req.Email)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			RespondErr(c, http.StatusNotFound, "Eklenmek istenen e-posta sistemde kayıtlı değil")
			return
		}
		if errors.Is(err, services.ErrCannotAddTargetAsMember) {
			RespondErr(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, services.ErrAlreadyGroupMember) {
			RespondErr(c, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, services.ErrNotGroupMember) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrGroupNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondInternal(c, "Üye eklenirken bir hata oluştu", err)
		return
	}

	// Eklenen kişiye "gruba eklendin", diğer üyelere "yeni üye katıldı".
	RecordNotification(c, services.NotificationEvent{GroupID: groupID, Type: models.NotificationAddedToGroup, Only: []uuid.UUID{member.UserID}})
	RecordNotification(c, services.NotificationEvent{
		GroupID: groupID, Type: models.NotificationMemberJoined, Exclude: []uuid.UUID{member.UserID},
		Params: models.NotificationParams{"member_name": member.User.Name},
	})
	RespondCreated(c, member)
}

// AddInterest adds one interest without replacing other members' contributions.
// @Summary      İlgi Alanı Ekle
// @Description  Tüm grup üyeleri tek bir ilgi alanı ekleyebilir (1–100 karakter). Hedef kişi ve grup dışı kullanıcılar erişemez. Tekrarlar kabul edilmez.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body InterestRequest true "Eklenecek ilgi alanı"
// @Success      200 {object} Response{data=InterestListResponse} "Güncel ilgi alanı listesi"
// @Failure      400 {object} Response "Geçersiz grup kimliği veya istek"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      409 {object} Response "İlgi alanı zaten listede"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/interests [post]
func (ctl *GroupController) AddInterest(c *gin.Context) {
	ctl.changeInterest(c, false)
}

// DeleteInterest removes one item. Only admins may delete, including their own additions.
// @Summary      İlgi Alanı Sil (Admin)
// @Description  Yalnızca grup yöneticisi listeden bir ilgi alanı silebilir. Normal üyeler kendi ekledikleri ilgi alanını da silemez.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body InterestRequest true "Silinecek ilgi alanı"
// @Success      200 {object} Response{data=InterestListResponse} "Güncel ilgi alanı listesi"
// @Failure      400 {object} Response "Geçersiz grup kimliği veya istek"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Sadece admin silebilir; Anti-Spoiler ve üyelik kontrolü zorunlu"
// @Failure      404 {object} Response "Grup veya ilgi alanı bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/interests [delete]
func (ctl *GroupController) DeleteInterest(c *gin.Context) {
	ctl.changeInterest(c, true)
}

func (ctl *GroupController) changeInterest(c *gin.Context, remove bool) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req InterestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: interest metin alanı zorunludur")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	var interests []string
	if remove {
		interests, err = ctl.groupService.DeleteInterest(groupID, userID, req.Interest)
	} else {
		interests, err = ctl.groupService.AddInterest(groupID, userID, req.Interest)
	}
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember), errors.Is(err, services.ErrUnauthorizedInterestDelete):
			RespondErr(c, http.StatusForbidden, err.Error())
		case errors.Is(err, services.ErrGroupNotFound), errors.Is(err, services.ErrInterestNotFound):
			RespondErr(c, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrInvalidInterest):
			RespondErr(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, services.ErrDuplicateInterest):
			RespondErr(c, http.StatusConflict, err.Error())
		default:
			RespondInternal(c, "İlgi alanları güncellenirken bir hata oluştu", err)
		}
		return
	}
	RespondOK(c, InterestListResponse{Interests: interests})
}

// UpdateBudget updates the total budget and existing gift budget warnings.
// @Summary      Grup Bütçesini Güncelle (Admin)
// @Description  Yalnızca admin toplam bütçeyi değiştirebilir. 0–99999999.99 aralığında, en fazla iki ondalık basamak kabul edilir. Kişi başı bütçe anlık hesaplanır; hediye bütçe aşımı uyarıları yenilenir. 0 bütçe aşımı uyarılarını kapatır.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body UpdateBudgetRequest true "Yeni toplam bütçe"
// @Success      200 {object} Response{data=MessageData} "Bütçe güncellendi"
// @Failure      400 {object} Response "Geçersiz bütçe veya grup kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Sadece admin güncelleyebilir; Anti-Spoiler ve üyelik kontrolü zorunlu"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/budget [patch]
func (ctl *GroupController) UpdateBudget(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req UpdateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: total_budget sayısal alanı zorunludur")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.groupService.UpdateBudget(groupID, userID, *req.TotalBudget); err != nil {
		switch {
		case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember), errors.Is(err, services.ErrUnauthorizedBudgetUpdate):
			RespondErr(c, http.StatusForbidden, err.Error())
		case errors.Is(err, services.ErrGroupNotFound):
			RespondErr(c, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrInvalidBudget):
			RespondErr(c, http.StatusBadRequest, err.Error())
		default:
			RespondInternal(c, "Bütçe güncellenirken bir hata oluştu", err)
		}
		return
	}
	// Aynı tutar yeniden kaydedildiyse bildirim üretilmez.
	if before := groupBeforeRequest(c); before == nil || !before.TotalBudget.Equal(*req.TotalBudget) {
		RecordNotification(c, services.NotificationEvent{
			GroupID: groupID, Type: models.NotificationBudgetChanged,
			Params: models.NotificationParams{"total_budget": req.TotalBudget.StringFixed(2)},
		})
	}
	RespondOK(c, MessageData{Message: "Bütçe güncellendi"})
}

// UpdateDescription grup açıklamasını değiştirir veya siler
// @Summary      Grup Açıklamasını Güncelle (Admin)
// @Description  Yalnızca admin. Metin baştan ve sondan kırpılır; kırpıldıktan sonra en fazla 1000 karakter. Boş ya da yalnızca boşluktan oluşan değer açıklamayı siler. Bildirim üretmez; grup akışına `changed` olayı gider. Hedef kişi Anti-Spoiler ile 403 alır.
// @Tags         Grup İşlemleri (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body UpdateDescriptionRequest true "Yeni açıklama"
// @Success      200 {object} Response{data=models.Group} "Güncel grup"
// @Failure      400 {object} Response "Geçersiz grup kimliği, eksik alan veya 1000 karakterden uzun açıklama"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/description [patch]
func (ctl *GroupController) UpdateDescription(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req UpdateDescriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: description metin alanı zorunludur (silmek için boş metin gönderin)")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	group, err := ctl.groupService.UpdateDescription(groupID, userID, *req.Description)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember), errors.Is(err, services.ErrUnauthorizedDescription):
			RespondErr(c, http.StatusForbidden, err.Error())
		case errors.Is(err, services.ErrGroupNotFound):
			RespondErr(c, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrInvalidDescription):
			RespondErr(c, http.StatusBadRequest, err.Error())
		default:
			RespondInternal(c, "Grup açıklaması güncellenirken bir hata oluştu", err)
		}
		return
	}
	RespondOK(c, group)
}

// LinkTarget mevcut gruba hedef kişiyi e-postayla bağlar
// @Summary      Hedef Kişiyi Bağla (Admin)
// @Description  Yalnızca admin, hedef kişisi bağlı olmayan bir gruba kayıtlı bir kullanıcıyı e-postayla hedef olarak bağlar. Hedef bir kez bağlanır; değiştirilemez ve kaldırılamaz. Grubun üyesi olan biri veya admin'in kendisi hedef olamaz. Bağlandıktan sonra hedef kişi Anti-Spoiler nedeniyle grubu göremez. `target_person_name` değişmez.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body LinkTargetRequest true "Hedef kişinin e-postası"
// @Success      200 {object} Response{data=models.Group} "Hedef kişi bağlandı"
// @Failure      400 {object} Response "Geçersiz istek, grup kimliği veya kendini hedef seçme"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Sadece admin bağlayabilir; Anti-Spoiler ve üyelik kontrolü zorunlu"
// @Failure      404 {object} Response "Grup bulunamadı veya e-posta kayıtlı değil"
// @Failure      409 {object} Response "Hedef zaten bağlı veya kişi grubun üyesi"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/target [patch]
func (ctl *GroupController) LinkTarget(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req LinkTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: geçerli bir e-posta adresi zorunludur")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	group, err := ctl.groupService.LinkTarget(groupID, userID, req.Email)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember), errors.Is(err, services.ErrUnauthorizedTargetUpdate):
			RespondErr(c, http.StatusForbidden, err.Error())
		case errors.Is(err, services.ErrGroupNotFound), errors.Is(err, services.ErrTargetUserNotFound):
			RespondErr(c, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrSelfTarget):
			RespondErr(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, services.ErrTargetAlreadyLinked), errors.Is(err, services.ErrTargetIsMember):
			RespondErr(c, http.StatusConflict, err.Error())
		default:
			RespondInternal(c, "Hedef kişi bağlanırken bir hata oluştu", err)
		}
		return
	}
	// Hedefin bu gruba ait bildirimleri bağlama transaction'ında silindi; açık zili yenilensin.
	if group.TargetUserID != nil {
		RecordNotification(c, services.NotificationEvent{GroupID: groupID, RefreshOnly: true, Only: []uuid.UUID{*group.TargetUserID}})
	}
	RespondOK(c, group)
}

// DeleteGroup grubu ve tüm ilişkili verilerini siler
// @Summary      Hediye Grubunu Sil
// @Description  Grubu, gruptaki hediye önerilerini, oyları ve üyelikleri kalıcı olarak siler. Sadece grup kurucusu / yöneticisi (admin) silebilir.
// @Tags         Hediye Grupları (Groups)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Grup başarıyla silindi"
// @Failure      400 {object} Response "Geçersiz grup kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Bu grubu silme yetkiniz yok (sadece admin silebilir) veya Anti-Spoiler"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id} [delete]
func (ctl *GroupController) DeleteGroup(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	err = ctl.groupService.DeleteGroup(groupID, userID)
	if err != nil {
		if errors.Is(err, services.ErrAntiSpoilerTriggered) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrNotGroupMember) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrUnauthorizedGroupDelete) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrGroupNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondInternal(c, "Grup silinirken bir hata oluştu", err)
		return
	}

	RespondOK(c, MessageData{Message: "Grup ve ilişkili tüm veriler başarıyla silindi"})
}

// getUserID Gin Context'ten oturum açmış kullanıcının UUID'sini güvenle döndürür
func getUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

// groupAdminError grup yönetici işlemlerinin ortak hata eşlemesidir.
func groupAdminError(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember),
		errors.Is(err, services.ErrUnauthorizedWinner), errors.Is(err, services.ErrUnauthorizedEventDate):
		RespondErr(c, http.StatusForbidden, err.Error())
	case errors.Is(err, services.ErrGroupNotFound):
		RespondErr(c, http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrWinnerNotInGroup), errors.Is(err, services.ErrInvalidEventDate), errors.Is(err, services.ErrInvalidWinners):
		RespondErr(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrGroupCompleted), errors.Is(err, services.ErrGroupNotCompleted), errors.Is(err, services.ErrWinnerNotActive):
		RespondErr(c, http.StatusConflict, err.Error())
	default:
		RespondInternal(c, internalMsg, err)
	}
}

// SelectWinner kazanan hediyeleri seçer ve grubu tamamlar
// @Summary      Kazananları Seç ve Grubu Tamamla
// @Description  Yalnızca admin. Seçilen 1–20 aktif fikir SELECTED, grup COMPLETED olur. Fikirlerden biri başka gruba aitse 400, aktif değilse 409. Tamamlanan grupta fikir ekleme, AI önerisi, oy, veto ve fikir silme 409 döner.
// @Tags         Grup İşlemleri (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body SelectWinnerRequest true "Kazanan fikirler"
// @Success      200 {object} Response{data=MessageData} "Grup tamamlandı"
// @Failure      400 {object} Response "Fikir bu gruba ait değil veya istek geçersiz"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      409 {object} Response "Grup zaten tamamlandı veya fikir aktif değil"
// @Router       /api/v1/groups/{id}/winner [post]
func (ctl *GroupController) SelectWinner(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req SelectWinnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: idea_ids 1–20 öneri kimliği içermelidir")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.groupService.SelectWinners(groupID, userID, req.IdeaIDs); err != nil {
		groupAdminError(c, err, "Kazanan seçilirken bir hata oluştu")
		return
	}
	unique := map[uuid.UUID]bool{}
	for _, id := range req.IdeaIDs {
		unique[id] = true
	}
	RecordNotification(c, services.NotificationEvent{
		GroupID: groupID, Type: models.NotificationWinnerSelected,
		Params: models.NotificationParams{"count": len(unique)},
	})
	RespondOK(c, MessageData{Message: "Kazanan seçildi, grup tamamlandı"})
}

// ReopenGroup tamamlanan grubu yeniden açar
// @Summary      Grubu Yeniden Aç
// @Description  Yalnızca admin. Yanlış seçimi geri almak içindir: kazanan fikir ACTIVE'e, grup ACTIVE'e döner.
// @Tags         Grup İşlemleri (Groups)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Grup yeniden açıldı"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      409 {object} Response "Grup zaten açık"
// @Router       /api/v1/groups/{id}/winner [delete]
func (ctl *GroupController) ReopenGroup(c *gin.Context) {
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
	if err := ctl.groupService.ReopenGroup(groupID, userID); err != nil {
		groupAdminError(c, err, "Grup yeniden açılırken bir hata oluştu")
		return
	}
	RecordNotification(c, services.NotificationEvent{GroupID: groupID, Type: models.NotificationGroupReopened})
	RespondOK(c, MessageData{Message: "Grup yeniden açıldı"})
}

// UpdateEventDate hediye tarihini ayarlar veya kaldırır
// @Summary      Hediye Tarihini Güncelle
// @Description  Yalnızca admin. "YYYY-MM-DD" biçiminde tarih; null tarihi kaldırır. Bugünden en fazla 1 yıl önce, en fazla 5 yıl sonra.
// @Tags         Grup İşlemleri (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body UpdateEventDateRequest true "Hediye tarihi"
// @Success      200 {object} Response{data=MessageData} "Tarih güncellendi"
// @Failure      400 {object} Response "Geçersiz tarih"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Router       /api/v1/groups/{id}/event-date [patch]
func (ctl *GroupController) UpdateEventDate(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req UpdateEventDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: event_date YYYY-AA-GG biçiminde veya null olmalıdır")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.groupService.SetEventDate(groupID, userID, req.EventDate); err != nil {
		groupAdminError(c, err, "Hediye tarihi güncellenirken bir hata oluştu")
		return
	}
	// Tarih gerçekten değiştiyse bildir (aynı tarih ya da zaten boşken kaldırma bildirilmez).
	before := groupBeforeRequest(c)
	if before == nil || !sameDate(before.EventDate, req.EventDate) {
		var date any
		if req.EventDate != nil {
			date = req.EventDate.String()
		}
		RecordNotification(c, services.NotificationEvent{
			GroupID: groupID, Type: models.NotificationEventDateChanged,
			Params: models.NotificationParams{"event_date": date},
		})
	}
	RespondOK(c, MessageData{Message: "Hediye tarihi güncellendi"})
}

// sameDate iki isteğe bağlı tarihin aynı gün (ya da ikisinin de boş) olduğunu söyler.
func sameDate(a, b *models.Date) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.String() == b.String()
}

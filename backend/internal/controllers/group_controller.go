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

// CreateGroupRequest yeni grup açma isteği için DTO
type CreateGroupRequest struct {
	Title            string          `json:"title" binding:"required,min=2,max=150" example:"Ahmet'in 30. Yaş Sürprizi"`
	Description      string          `json:"description" binding:"max=2000" example:"Yakın arkadaşlarla ortak doğum günü hediyesi organizasyonu"`
	TargetPersonName string          `json:"target_person_name" binding:"required,min=2,max=100" example:"Ahmet"`
	TargetInterests  string          `json:"target_interests" binding:"max=2000" example:"Kamp, 3. nesil kahve demleme, trekking, outdoor"`
	TargetUserID     *uuid.UUID      `json:"target_user_id,omitempty" format:"uuid" example:"b49770ed-7518-40e6-839d-5810f79bbd86"`
	TargetEmail      string          `json:"target_email,omitempty" binding:"omitempty,email,max=150" example:"ahmet@demo.com"`
	TotalBudget      decimal.Decimal `json:"total_budget" example:"2500.00"`
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

type UpdateBudgetRequest struct {
	TotalBudget *decimal.Decimal `json:"total_budget" binding:"required" example:"3500.00"`
}

// CreateGroup yeni grup oluşturur
// @Summary      Yeni Hediye Grubu Oluştur
// @Description  Yeni bir sürpriz hediye grubu açar ve oluşturan kullanıcıyı atomik transaction ile 'admin' rolüyle üye yapar. Hedef kişi isteğe bağlı `target_email` ile seçilir; sunucu kayıtlı kullanıcıyı bulur ve `target_user_id` alanını doldurur. `target_email` ve `target_user_id` birlikte gönderilemez. Hedef seçilmezse yalnızca `target_person_name` kullanılır ve Anti-Spoiler devreye girmez.
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

	group, err := ctl.groupService.CreateGroup(
		userID,
		req.Title,
		req.Description,
		req.TargetPersonName,
		req.TargetInterests,
		req.TargetUserID,
		req.TargetEmail,
		req.TotalBudget,
	)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrSelfTarget), errors.Is(err, services.ErrTargetInputConflict), errors.Is(err, services.ErrInvalidBudget):
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
// @Description  Giriş yapmış kullanıcının üye olduğu grupları listeler. Anti-Spoiler kuralı gereğince kullanıcının hedef kişi olduğu sürpriz gruplar bu listede ASLA yer almaz.
// @Tags         Hediye Grupları (Groups)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=[]models.Group} "Kullanıcının dahil olduğu gruplar listesi"
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
		groups = []models.Group{}
	}

	RespondOK(c, groups)
}

// GetGroupDetail grubun detaylarını, bütçesini ve üyelerini getirir
// @Summary      Grup Detayı ve Dinamik Kişi Başı Bütçe (Anti-Spoiler Korumalı)
// @Description  Grup detayını, aktif üye listesini, toplam bütçeyi ve anlık hesaplanan kişi başı bütçeyi getirir. Hedef kişi bu uç noktaya erişmeye çalışırsa 403 Forbidden alır.
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
	RespondOK(c, MessageData{Message: "Bütçe güncellendi"})
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

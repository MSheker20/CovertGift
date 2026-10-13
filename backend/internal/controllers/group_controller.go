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
	Description      string          `json:"description" example:"Yakın arkadaşlarla ortak doğum günü hediyesi organizasyonu"`
	TargetPersonName string          `json:"target_person_name" binding:"required,min=2,max=100" example:"Ahmet"`
	TargetInterests  string          `json:"target_interests" example:"Kamp, 3. nesil kahve demleme, trekking, outdoor"`
	TargetUserID     *uuid.UUID      `json:"target_user_id,omitempty" format:"uuid" example:"b49770ed-7518-40e6-839d-5810f79bbd86"`
	TotalBudget      decimal.Decimal `json:"total_budget" example:"2500.00"`
}

// AddMemberRequest gruba üye ekleme isteği için DTO
type AddMemberRequest struct {
	Email string `json:"email" binding:"required,email" example:"zeynep@demo.com"`
}

// CreateGroup yeni grup oluşturur
// @Summary      Yeni Hediye Grubu Oluştur
// @Description  Yeni bir sürpriz hediye grubu açar ve oluşturan kullanıcıyı atomik transaction ile 'admin' rolüyle üye yapar.
// @Tags         Hediye Grupları (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body CreateGroupRequest true "Grup Oluşturma Bilgileri"
// @Success      201 {object} Response{data=models.Group} "Grup başarıyla oluşturuldu"
// @Failure      400 {object} Response "Geçersiz parametreler veya kullanıcı kendine sürpriz grubu açamaz"
// @Failure      401 {object} Response "Oturum açılmamış"
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
		req.TotalBudget,
	)
	if err != nil {
		if errors.Is(err, services.ErrSelfTarget) {
			RespondErr(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Grup oluşturulurken bir hata oluştu")
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
		RespondErr(c, http.StatusInternalServerError, "Gruplar alınırken bir hata oluştu")
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
		RespondErr(c, http.StatusInternalServerError, "Grup detayı alınırken bir hata oluştu")
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
		RespondErr(c, http.StatusInternalServerError, "Üye eklenirken bir hata oluştu")
		return
	}

	RespondCreated(c, member)
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

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
	Title            string          `json:"title" binding:"required,min=2,max=150"`
	Description      string          `json:"description"`
	TargetPersonName string          `json:"target_person_name" binding:"required,min=2,max=100"`
	TargetUserID     *uuid.UUID      `json:"target_user_id,omitempty"`
	TotalBudget      decimal.Decimal `json:"total_budget"`
}

// AddMemberRequest gruba üye ekleme isteği için DTO
type AddMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// CreateGroup yeni grup oluşturur (POST /api/v1/groups)
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

// GetUserGroups kullanıcının dahil olduğu grupları listeler (GET /api/v1/groups)
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

// GetGroupDetail grubun detaylarını, bütçesini ve üyelerini getirir (GET /api/v1/groups/:id)
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

// AddMember gruba e-posta ile üye ekler (POST /api/v1/groups/:id/members)
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

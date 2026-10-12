package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// IdeaController hediye ve oy HTTP isteklerini yöneten yapıdır
type IdeaController struct {
	ideaService services.IdeaService
}

// NewIdeaController yeni bir IdeaController örneği oluşturur
func NewIdeaController(ideaService services.IdeaService) *IdeaController {
	return &IdeaController{ideaService: ideaService}
}

// AddIdeaRequest manuel hediye önerisi için DTO
type AddIdeaRequest struct {
	Title          string          `json:"title" binding:"required,min=2,max=200"`
	Description    string          `json:"description"`
	EstimatedPrice decimal.Decimal `json:"estimated_price"`
	SearchQuery    string          `json:"search_query"`
	ProductURL     string          `json:"product_url"`
	ImageURL       string          `json:"image_url"`
}

// VoteRequest oy verme isteği için DTO
type VoteRequest struct {
	VoteType int8 `json:"vote_type" binding:"required"`
}

// AddIdea gruba manuel hediye önerisi ekler (POST /api/v1/groups/:id/ideas)
func (ctl *IdeaController) AddIdea(c *gin.Context) {
	groupIDStr := c.Param("id")
	groupID, err := uuid.Parse(groupIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}

	var req AddIdeaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	idea, err := ctl.ideaService.AddManualIdea(
		groupID,
		userID,
		req.Title,
		req.Description,
		req.SearchQuery,
		req.ProductURL,
		req.ImageURL,
		req.EstimatedPrice,
	)
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
		RespondErr(c, http.StatusInternalServerError, "Hediye önerisi eklenirken bir hata oluştu")
		return
	}

	RespondCreated(c, idea)
}

// GetIdeas gruptaki hediyeleri skorlarıyla birlikte listeler (GET /api/v1/groups/:id/ideas)
func (ctl *IdeaController) GetIdeas(c *gin.Context) {
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

	ideas, err := ctl.ideaService.GetGroupIdeas(groupID, userID)
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
		RespondErr(c, http.StatusInternalServerError, "Hediye önerileri alınırken bir hata oluştu")
		return
	}

	if ideas == nil {
		ideas = []repository.IdeaWithScore{}
	}

	RespondOK(c, ideas)
}

// Vote bir hediye fikrine oy verir veya günceller (POST /api/v1/ideas/:id/vote)
func (ctl *IdeaController) Vote(c *gin.Context) {
	ideaIDStr := c.Param("id")
	ideaID, err := uuid.Parse(ideaIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz hediye kimliği")
		return
	}

	var req VoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	err = ctl.ideaService.VoteIdea(ideaID, userID, req.VoteType)
	if err != nil {
		if errors.Is(err, services.ErrInvalidVoteType) {
			RespondErr(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, services.ErrIdeaNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, services.ErrAntiSpoilerTriggered) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrNotGroupMember) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Oy kaydedilirken bir hata oluştu")
		return
	}

	RespondOK(c, gin.H{
		"message": "Oy başarıyla kaydedildi",
	})
}

// DeleteIdea hediye fikrini siler (DELETE /api/v1/ideas/:id)
func (ctl *IdeaController) DeleteIdea(c *gin.Context) {
	ideaIDStr := c.Param("id")
	ideaID, err := uuid.Parse(ideaIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz hediye kimliği")
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	err = ctl.ideaService.DeleteIdea(ideaID, userID)
	if err != nil {
		if errors.Is(err, services.ErrIdeaNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, services.ErrUnauthorizedDelete) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrNotGroupMember) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Hediye önerisi silinirken bir hata oluştu")
		return
	}

	RespondOK(c, gin.H{
		"message": "Hediye önerisi başarıyla silindi",
	})
}

// GenerateAI yapay zeka ile gruba hediye önerileri üretir (POST /api/v1/groups/:id/ideas/generate-ai)
func (ctl *IdeaController) GenerateAI(c *gin.Context) {
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

	ideas, err := ctl.ideaService.GenerateAIIdeas(c.Request.Context(), groupID, userID)
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
		if errors.Is(err, gemini.ErrAPIKeyMissing) {
			RespondErr(c, http.StatusServiceUnavailable, "Gemini API anahtarı yapılandırılmamış")
			return
		}
		if errors.Is(err, services.ErrNoAIIdeasGenerated) {
			RespondErr(c, http.StatusUnprocessableEntity, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Yapay zeka hediye önerisi üretirken bir hata oluştu: "+err.Error())
		return
	}

	RespondCreated(c, ideas)
}

// RejectIdea grup yöneticisinin bir hediye fikrini veto etmesini sağlar (POST /api/v1/ideas/:id/reject)
func (ctl *IdeaController) RejectIdea(c *gin.Context) {
	ideaIDStr := c.Param("id")
	ideaID, err := uuid.Parse(ideaIDStr)
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz hediye kimliği")
		return
	}

	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}

	err = ctl.ideaService.RejectIdea(ideaID, userID)
	if err != nil {
		if errors.Is(err, services.ErrIdeaNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, services.ErrUnauthorizedVeto) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, services.ErrNotGroupMember) {
			RespondErr(c, http.StatusForbidden, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Hediye önerisi veto edilirken bir hata oluştu")
		return
	}

	RespondOK(c, gin.H{
		"message": "Hediye önerisi başarıyla veto edildi (reddedildi)",
	})
}


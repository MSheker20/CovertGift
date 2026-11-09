package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/MSheker20/CovertGift/backend/pkg/linkpreview"
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

// PreviewLinkRequest ürün linki önizleme isteği için DTO
type PreviewLinkRequest struct {
	URL string `json:"url" binding:"required,max=2048" example:"https://ty.gl/gkyyduf4wm8u3"`
}

// AddIdeaRequest manuel hediye önerisi için DTO
type AddIdeaRequest struct {
	Title          string          `json:"title" binding:"required,min=2,max=200" example:"Stanley Klasik Vakumlu Termos 1.0L"`
	Description    string          `json:"description" binding:"max=5000" example:"Doğa yürüyüşlerinde ve kampta sıcak içecek taşımak için"`
	EstimatedPrice decimal.Decimal `json:"estimated_price" example:"1450.00"`
	SearchQuery    string          `json:"search_query" binding:"max=255" example:"Stanley klasik vakumlu termos 1 lt"`
	ProductURL     string          `json:"product_url" binding:"omitempty,max=1000,http_url" example:"https://www.trendyol.com/brand/product-p-123"`
	ImageURL       string          `json:"image_url" binding:"omitempty,max=500,http_url" example:"https://cdn.dsmcdn.com/product.jpg"`
}

// VoteRequest oy verme isteği için DTO
type VoteRequest struct {
	VoteType int8 `json:"vote_type" binding:"required,oneof=1 -1" example:"1"` // 1: Upvote, -1: Downvote
}

// AddIdea gruba manuel hediye önerisi ekler
// @Summary      Manuel Hediye Önerisi Ekle
// @Description  Grubun hediye havuzuna manuel bir hediye fikri ekler (isteğe bağlı pazar yeri linki ile). Anti-Spoiler korumalıdır.
// @Tags         Hediye Havuzu (Gift Ideas)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body AddIdeaRequest true "Hediye Önerisi Bilgileri"
// @Success      201 {object} Response{data=models.GiftIdea} "Hediye önerisi başarıyla eklendi"
// @Failure      400 {object} Response "Geçersiz istek parametreleri"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Erişim engellendi: Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/ideas [post]
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
		if errors.Is(err, services.ErrInvalidPrice) {
			RespondErr(c, http.StatusBadRequest, err.Error())
			return
		}
		RespondInternal(c, "Hediye önerisi eklenirken bir hata oluştu", err)
		return
	}

	RespondCreated(c, idea)
}

// GetIdeas gruptaki hediyeleri skorlarıyla birlikte listeler
// @Summary      Gruptaki Hediye Önerilerini ve Oy Skorlarını Listele
// @Description  Gruptaki hediye havuzunu, her hediyenin toplam oy skorunu (+1/-1 toplamı) ve istek yapan kullanıcının kendi oyunu listeler. Anti-Spoiler korumalıdır.
// @Tags         Hediye Havuzu (Gift Ideas)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=[]repository.IdeaWithScore} "Hediye önerileri ve oy skorları"
// @Failure      400 {object} Response "Geçersiz grup kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Erişim engellendi: Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/ideas [get]
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
		RespondInternal(c, "Hediye önerileri alınırken bir hata oluştu", err)
		return
	}

	if ideas == nil {
		ideas = []repository.IdeaWithScore{}
	}

	RespondOK(c, ideas)
}

// PreviewLink verilen ürün linkini ayrıştırarak önizleme verilerini döner
// @Summary      Ürün Linki Önizleme & SSRF Kalkanı (Faz 5)
// @Description  Kullanıcının yapıştırdığı e-ticaret linkini (Trendyol, Amazon TR, Hepsiburada) güvenle çekerek başlık, görsel, fiyat ve temiz arama terimini döner. Veritabanına yazmaz. Anti-Spoiler korumalıdır.
// @Tags         Hediye Havuzu (Gift Ideas)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body PreviewLinkRequest true "Önizlenecek Ürün URL'si"
// @Success      200 {object} Response{data=linkpreview.Preview} "Ayrıştırılmış ürün önizleme bilgisi"
// @Failure      400 {object} Response "URL parametresi eksik veya geçersiz"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Erişim engellendi: Anti-Spoiler veya gruba üye değilsiniz"
// @Router       /api/v1/groups/{id}/ideas/preview-link [post]
func (ctl *IdeaController) PreviewLink(c *gin.Context) {
	var req PreviewLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: url alanı zorunludur")
		return
	}

	preview, err := linkpreview.Fetch(c.Request.Context(), req.URL)
	if err != nil {
		// Kural 5: Önizleme başarısız olsa veya kısıtlansa bile HTTP 500 fırlatılmaz;
		// kullanıcının formu elle doldurabilmesi için partial: true ile 200 OK yanıtı verilir.
		if preview == nil {
			preview = &linkpreview.Preview{
				FinalURL: req.URL,
				Partial:  true,
				Source:   "generic",
				Warning:  err.Error(),
			}
		}
	}

	RespondOK(c, preview)
}

// GenerateAI yapay zeka ile gruba hediye önerileri üretir
// @Summary      Gemini 3.8 Flash ile Yapay Zekâ Hediye Önerileri Üret (Faz 6)
// @Description  Google Search ile Türkiye mağazalarında ürün ve TL fiyatı araştırır; kaynakla desteklenen sonuçlardan kişinin ilgi alanına uygun en fazla 5 yeni hediye üretir. Mevcut/veto edilmiş fikirler ve bütçe dikkate alınır. ai_sources araştırma kaynaklarını, ai_search_suggestions Google arama önerisi HTML'ini taşır; anlık stok garantisi vermez. Kaynaksız araştırma şu an 500, servis filtresinden sonra boş sonuç 422 döner. Geçici panel kaynak alanlarını henüz göstermez.
// @Tags         Hediye Havuzu (Gift Ideas)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      201 {object} Response{data=[]models.GiftIdea} "Yapay zekâ tarafından üretilen yeni hediye önerileri"
// @Failure      400 {object} Response "Geçersiz grup kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Erişim engellendi: Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      422 {object} Response "Yapay zekâ öneri üretemedi"
// @Failure      429 {object} Response "Gemini kullanım kotası veya istek sınırı aşıldı"
// @Failure      503 {object} Response "Gemini API anahtarı yapılandırılmamış veya servis geçici olarak kullanılamıyor"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/ideas/generate-ai [post]
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
		if errors.Is(err, gemini.ErrRateLimited) {
			RespondErr(c, http.StatusTooManyRequests, "Yapay zekâ servisinin kullanım kotası veya istek sınırı aşıldı. Bir süre sonra tekrar deneyin; sorun sürerse proje kotasını kontrol edin.")
			return
		}
		if errors.Is(err, services.ErrNoAIIdeasGenerated) {
			RespondErr(c, http.StatusUnprocessableEntity, err.Error())
			return
		}
		RespondInternal(c, "Yapay zeka hediye önerisi üretirken bir hata oluştu", err)
		return
	}

	RespondCreated(c, ideas)
}

// Vote bir hediye fikrine oy verir veya günceller
// @Summary      Hediye Fikrine Oy Ver veya Güncelle (+1 Upvote / -1 Downvote)
// @Description  Bir hediye önerisine oy verir (+1 veya -1). GORM Upsert kuralı ile aynı kullanıcı oy verdiğinde kayıt çoğalmaz, güncellenir. Eğer toplam downvote sayısı grup üye sayısının yarısını aşarsa ($downvotes > N/2$) hediye otomatik REJECTED (Demokrasi Vetosu) olur.
// @Tags         Oylama & Veto (Voting & Veto)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Hediye UUID" Format(uuid)
// @Param        request body VoteRequest true "Oy Türü (1: Upvote, -1: Downvote)"
// @Success      200 {object} Response{data=MessageData} "Oy başarıyla kaydedildi"
// @Failure      400 {object} Response "Geçersiz oy türü (sadece 1 veya -1)"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Erişim engellendi: Anti-Spoiler veya grup üyesi değilsiniz"
// @Failure      404 {object} Response "Hediye fikri bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/ideas/{id}/vote [post]
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
		RespondInternal(c, "Oy kaydedilirken bir hata oluştu", err)
		return
	}

	RespondOK(c, MessageData{
		Message: "Oy başarıyla kaydedildi",
	})
}

// RejectIdea grup yöneticisinin bir hediye fikrini veto etmesini sağlar
// @Summary      Admin Tek Tık Vetosu (Faz 6)
// @Description  Sadece grup yöneticisi (admin) tarafından bir hediye fikrini doğrudan veto ederek durumunu 'REJECTED' yapar. Normal üyeler veto edemez (403). Fikir silinmez, sonraki AI çağrısına negatif kısıt olarak beslenir.
// @Tags         Oylama & Veto (Voting & Veto)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Hediye UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Hediye önerisi başarıyla veto edildi"
// @Failure      400 {object} Response "Geçersiz hediye kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Yetkisiz işlem: Sadece grup yöneticisi veto edebilir"
// @Failure      404 {object} Response "Hediye fikri bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/ideas/{id}/reject [post]
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
		if errors.Is(err, services.ErrAntiSpoilerTriggered) {
			RespondErr(c, http.StatusForbidden, err.Error())
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
		RespondInternal(c, "Hediye önerisi veto edilirken bir hata oluştu", err)
		return
	}

	RespondOK(c, MessageData{
		Message: "Hediye önerisi başarıyla veto edildi (reddedildi)",
	})
}

// DeleteIdea hediye fikrini siler
// @Summary      Hediye Fikrini Sil
// @Description  Hediye önerisini kalıcı olarak siler. Sadece hediyeyi öneren kişi veya grup admini silebilir.
// @Tags         Hediye Havuzu (Gift Ideas)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Hediye UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Hediye önerisi başarıyla silindi"
// @Failure      400 {object} Response "Geçersiz hediye kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Yetkisiz işlem: Sadece öneren kişi veya grup admini silebilir"
// @Failure      404 {object} Response "Hediye fikri bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/ideas/{id} [delete]
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
		if errors.Is(err, services.ErrAntiSpoilerTriggered) {
			RespondErr(c, http.StatusForbidden, err.Error())
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
		RespondInternal(c, "Hediye önerisi silinirken bir hata oluştu", err)
		return
	}

	RespondOK(c, MessageData{
		Message: "Hediye önerisi başarıyla silindi",
	})
}

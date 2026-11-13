package controllers

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CoverResponse kapak yüklemesinin sonucudur. Görselin baytları JSON'da hiç yer almaz.
type CoverResponse struct {
	CoverVersion string `json:"cover_version" example:"3f2a9c0d1e4b5a6978879695a4b3c2d1"`
	Width        int    `json:"width" example:"1600"`
	Height       int    `json:"height" example:"1200"`
}

// UpdateCoverPolicyRequest kapağı kimin değiştirebileceğini taşır
type UpdateCoverPolicyRequest struct {
	CoverAdminOnly *bool `json:"cover_admin_only" binding:"required" example:"true"`
}

// CoverPolicyResponse güncel kapak politikasıdır
type CoverPolicyResponse struct {
	CoverAdminOnly bool `json:"cover_admin_only" example:"true"`
}

// coverError kapak uçlarının ortak hata eşlemesidir.
func coverError(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, services.ErrAntiSpoilerTriggered), errors.Is(err, services.ErrNotGroupMember),
		errors.Is(err, services.ErrUnauthorizedCoverChange), errors.Is(err, services.ErrUnauthorizedCoverPolicy):
		RespondErr(c, http.StatusForbidden, err.Error())
	case errors.Is(err, services.ErrGroupNotFound), errors.Is(err, services.ErrCoverNotFound):
		RespondErr(c, http.StatusNotFound, err.Error())
	case errors.Is(err, imageproc.ErrTooLarge):
		RespondErr(c, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, imageproc.ErrUnsupportedType), errors.Is(err, imageproc.ErrHEICNotSupported):
		RespondErr(c, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, imageproc.ErrCorrupt), errors.Is(err, imageproc.ErrDimensionsTooBig), errors.Is(err, imageproc.ErrDimensionsTooTiny):
		RespondErr(c, http.StatusBadRequest, err.Error())
	default:
		RespondInternal(c, internalMsg, err)
	}
}

// bodyTooLarge hatanın gövde sınırının (http.MaxBytesReader) aşılmasından kaynaklandığını söyler.
func bodyTooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large")
}

// UploadCover grubun kapak fotoğrafını yükler veya değiştirir
// @Summary      Kapak Fotoğrafı Yükle / Değiştir
// @Description  multipart/form-data ile `file` alanında JPEG, PNG veya WebP (en fazla 10 MB) alır. Tür, Content-Type başlığına değil dosyanın baytlarına göre doğrulanır; HEIC desteklenmez. Sunucu EXIF yönünü uygular, tüm metadata'yı (konum dahil) atar, ortadan 4:3 kırpar, en çok 1600x1200'e küçültür ve JPEG olarak saklar. Varsayılan olarak tüm üyeler yükleyebilir; `cover_admin_only` açıksa yalnızca admin. Hedef kişi 403 alır (Anti-Spoiler). Bildirim üretmez; grup akışına `changed` olayı gider.
// @Tags         Grup İşlemleri (Groups)
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        file formData file true "Kapak görseli (JPEG, PNG veya WebP; en fazla 10 MB)"
// @Success      200 {object} Response{data=CoverResponse} "Kapak kaydedildi"
// @Failure      400 {object} Response "Dosya yok, görsel bozuk, çok küçük veya çözünürlüğü çok yüksek"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya kapak yalnızca admin'e açık"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      413 {object} Response "Dosya 10 MB'tan büyük"
// @Failure      415 {object} Response "Desteklenmeyen görsel türü (ör. HEIC, GIF)"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/cover [put]
func (ctl *GroupController) UploadCover(c *gin.Context) {
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

	header, err := c.FormFile("file")
	if c.Request.MultipartForm != nil {
		defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
	}
	if err != nil {
		if bodyTooLarge(err) {
			RespondErr(c, http.StatusRequestEntityTooLarge, imageproc.ErrTooLarge.Error())
			return
		}
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: görseli multipart/form-data ile 'file' alanında gönderin")
		return
	}
	if header.Size > imageproc.MaxInputBytes {
		RespondErr(c, http.StatusRequestEntityTooLarge, imageproc.ErrTooLarge.Error())
		return
	}
	file, err := header.Open()
	if err != nil {
		RespondInternal(c, "Görsel okunurken bir hata oluştu", err)
		return
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, imageproc.MaxInputBytes+1))
	if err != nil {
		RespondInternal(c, "Görsel okunurken bir hata oluştu", err)
		return
	}

	cover, err := ctl.groupService.SetCover(groupID, userID, raw)
	if err != nil {
		coverError(c, err, "Kapak fotoğrafı kaydedilirken bir hata oluştu")
		return
	}
	RespondOK(c, CoverResponse{CoverVersion: cover.ETag, Width: cover.Width, Height: cover.Height})
}

// DeleteCover grubun kapak fotoğrafını kaldırır
// @Summary      Kapak Fotoğrafını Kaldır
// @Description  Kapağı siler; kapak yoksa da 200 döner (idempotent). Yetki yüklemeyle aynıdır: varsayılan olarak tüm üyeler, `cover_admin_only` açıksa yalnızca admin. Hedef kişi 403 alır (Anti-Spoiler).
// @Tags         Grup İşlemleri (Groups)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Kapak kaldırıldı"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya kapak yalnızca admin'e açık"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/cover [delete]
func (ctl *GroupController) DeleteCover(c *gin.Context) {
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
	if err := ctl.groupService.DeleteCover(groupID, userID); err != nil {
		coverError(c, err, "Kapak fotoğrafı kaldırılırken bir hata oluştu")
		return
	}
	RespondOK(c, MessageData{Message: "Kapak fotoğrafı kaldırıldı"})
}

// GetCover grubun kapak fotoğrafını görsel olarak döner
// @Summary      Kapak Fotoğrafını Getir (Anti-Spoiler Korumalı)
// @Description  Kapağın baytlarını `image/jpeg` olarak döner (JSON değil). `ETag` grubun `cover_version` değeridir; `If-None-Match` eşleşirse 304 döner. `Cache-Control: private, max-age=0, must-revalidate`: tarayıcı kopyayı saklar ama her kullanımda sunucuya sorar, böylece yetki her seferinde yeniden denetlenir. Hedef kişi ve üye olmayan 403 alır; kapak yoksa 404.
// @Tags         Grup İşlemleri (Groups)
// @Produce      image/jpeg
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        If-None-Match header string false "Önceki yanıtın ETag değeri"
// @Success      200 {file} binary "Kapak görseli (JPEG)"
// @Success      304 "Değişmedi"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler veya gruba üye değilsiniz"
// @Failure      404 {object} Response "Grup veya kapak fotoğrafı bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/cover [get]
func (ctl *GroupController) GetCover(c *gin.Context) {
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
	// Yetki (Anti-Spoiler + üyelik) GroupAccess middleware'inde bu istek için denetlendi. İstemcinin
	// elindeki sürüm güncelse görselin baytları veritabanından hiç okunmadan 304 döner.
	if group := groupBeforeRequest(c); group != nil && group.CoverVersion != "" {
		if etag := quoteETag(group.CoverVersion); etagMatches(c.GetHeader("If-None-Match"), etag) {
			setCoverHeaders(c, etag)
			c.Status(http.StatusNotModified)
			return
		}
	}

	cover, err := ctl.groupService.GetCover(groupID, userID)
	if err != nil {
		coverError(c, err, "Kapak fotoğrafı alınırken bir hata oluştu")
		return
	}
	etag := quoteETag(cover.ETag)
	setCoverHeaders(c, etag)
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, cover.ContentType, cover.Data)
}

func quoteETag(version string) string {
	return `"` + version + `"`
}

// setCoverHeaders görsel yanıtının önbellek başlıklarını yazar. Tarayıcı kopyayı saklar ama
// her kullanımda doğrular (max-age=0, must-revalidate): yetki her istekte yeniden denetlenir,
// değişmeyen görsel 304 ile gövdesiz döner. Paylaşılan önbellekler saklamaz (private).
func setCoverHeaders(c *gin.Context, etag string) {
	h := c.Writer.Header()
	h.Set("ETag", etag)
	h.Set("Cache-Control", "private, max-age=0, must-revalidate")
	h.Set("Vary", "Authorization")
	h.Set("X-Content-Type-Options", "nosniff")
}

// etagMatches If-None-Match başlığının (virgülle ayrılmış liste, W/ öneki veya *) verilen
// ETag ile eşleştiğini söyler.
func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == "*" || (candidate != "" && candidate == etag) {
			return true
		}
	}
	return false
}

// UpdateCoverPolicy kapağı kimin değiştirebileceğini ayarlar
// @Summary      Kapak Fotoğrafı İznini Ayarla (Admin)
// @Description  Yalnızca admin. `cover_admin_only` true ise kapağı yalnızca admin ekler/değiştirir/kaldırır; false (varsayılan) iken tüm üyeler yapabilir. Görüntüleme her üyeye açık kalır.
// @Tags         Grup İşlemleri (Groups)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Param        request body UpdateCoverPolicyRequest true "Kapak izni"
// @Success      200 {object} Response{data=CoverPolicyResponse} "İzin güncellendi"
// @Failure      400 {object} Response "Geçersiz istek"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler, üye değil veya admin değil"
// @Failure      404 {object} Response "Grup bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/groups/{id}/cover-policy [patch]
func (ctl *GroupController) UpdateCoverPolicy(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	var req UpdateCoverPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: cover_admin_only true veya false olmalıdır")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.groupService.SetCoverAdminOnly(groupID, userID, *req.CoverAdminOnly); err != nil {
		coverError(c, err, "Kapak fotoğrafı izni güncellenirken bir hata oluştu")
		return
	}
	RespondOK(c, CoverPolicyResponse{CoverAdminOnly: *req.CoverAdminOnly})
}

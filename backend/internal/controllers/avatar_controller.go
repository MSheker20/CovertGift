package controllers

import (
	"errors"
	"io"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/imageproc"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AvatarController profil fotoğrafı uçlarını sunar.
type AvatarController struct {
	avatarService services.AvatarService
}

// NewAvatarController yeni bir AvatarController oluşturur.
func NewAvatarController(avatarService services.AvatarService) *AvatarController {
	return &AvatarController{avatarService: avatarService}
}

// AvatarResponse yükleme sonucudur.
type AvatarResponse struct {
	AvatarVersion string `json:"avatar_version" example:"3f2a9c0d1e4b5a6978879695a4b3c2d1"`
	Width         int    `json:"width" example:"512"`
	Height        int    `json:"height" example:"512"`
}

func avatarError(c *gin.Context, err error, internalMsg string) {
	switch {
	case errors.Is(err, services.ErrAvatarNotFound):
		RespondErr(c, http.StatusNotFound, err.Error())
	case errors.Is(err, imageproc.ErrTooLarge):
		RespondErr(c, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, imageproc.ErrUnsupportedType), errors.Is(err, imageproc.ErrHEICNotSupported):
		RespondErr(c, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, imageproc.ErrCorrupt), errors.Is(err, imageproc.ErrDimensionsTooBig), errors.Is(err, imageproc.ErrAvatarTooTiny):
		RespondErr(c, http.StatusBadRequest, err.Error())
	default:
		RespondInternal(c, internalMsg, err)
	}
}

// UploadAvatar profil fotoğrafını yükler veya değiştirir
// @Summary      Profil Fotoğrafı Yükle / Değiştir
// @Description  multipart/form-data ile `file` alanında JPEG, PNG veya WebP alır: en fazla 2 MB, uzun kenar en çok 2000 piksel (4 megapiksel). Arayüz kare kırpılmış, en çok 512x512 JPEG gönderir. Tür dosyanın baytlarına göre doğrulanır; HEIC desteklenmez. Sunucu EXIF yönünü uygular, tüm metadata'yı (konum dahil) atar, ortadan kare kırpar, en çok 512x512'ye küçültür ve JPEG olarak saklar. Kaynakta kare kenarı en az 128 piksel olmalı.
// @Tags         Kimlik Doğrulama (Auth)
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        file formData file true "Profil fotoğrafı (JPEG, PNG veya WebP; en fazla 2 MB)"
// @Success      200 {object} Response{data=AvatarResponse} "Fotoğraf kaydedildi"
// @Failure      400 {object} Response "Dosya yok, görsel bozuk, çok küçük veya çözünürlüğü çok yüksek"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      413 {object} Response "Dosya 2 MB'tan büyük"
// @Failure      415 {object} Response "Desteklenmeyen görsel türü (ör. HEIC, GIF)"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/me/avatar [put]
func (ctl *AvatarController) Upload(c *gin.Context) {
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
	avatar, err := ctl.avatarService.Set(userID, raw)
	if err != nil {
		avatarError(c, err, "Profil fotoğrafı kaydedilirken bir hata oluştu")
		return
	}
	RespondOK(c, AvatarResponse{AvatarVersion: avatar.ETag, Width: avatar.Width, Height: avatar.Height})
}

// DeleteAvatar profil fotoğrafını kaldırır
// @Summary      Profil Fotoğrafını Kaldır
// @Description  Fotoğrafı siler; fotoğraf yoksa da 200 döner (idempotent).
// @Tags         Kimlik Doğrulama (Auth)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=MessageData} "Fotoğraf kaldırıldı"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/me/avatar [delete]
func (ctl *AvatarController) Delete(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.avatarService.Delete(userID); err != nil {
		avatarError(c, err, "Profil fotoğrafı kaldırılırken bir hata oluştu")
		return
	}
	RespondOK(c, MessageData{Message: "Profil fotoğrafı kaldırıldı"})
}

// GetAvatar bir kullanıcının profil fotoğrafını görsel olarak döner
// @Summary      Profil Fotoğrafını Getir
// @Description  Fotoğrafın baytlarını `image/jpeg` olarak döner (JSON değil). Yalnızca fotoğrafın sahibi ve onunla en az bir grubu paylaşanlar alabilir; diğer herkes, fotoğraf yokken olduğu gibi 404 alır (hesabın varlığı sızmaz). `ETag` kullanıcının `avatar_version` değeridir; `If-None-Match` eşleşirse 304 döner. `Cache-Control: private, max-age=0, must-revalidate`: yetki her kullanımda yeniden denetlenir.
// @Tags         Kimlik Doğrulama (Auth)
// @Produce      image/jpeg
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Kullanıcı UUID" Format(uuid)
// @Param        If-None-Match header string false "Önceki yanıtın ETag değeri"
// @Success      200 {file} binary "Profil fotoğrafı (JPEG)"
// @Success      304 "Değişmedi"
// @Failure      400 {object} Response "Geçersiz kullanıcı kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      404 {object} Response "Fotoğraf yok ya da görme yetkiniz yok"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/users/{id}/avatar [get]
func (ctl *AvatarController) Get(c *gin.Context) {
	ownerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz kullanıcı kimliği")
		return
	}
	viewerID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	avatar, err := ctl.avatarService.Get(viewerID, ownerID)
	if err != nil {
		avatarError(c, err, "Profil fotoğrafı alınırken bir hata oluştu")
		return
	}
	etag := quoteETag(avatar.ETag)
	setCoverHeaders(c, etag)
	if etagMatches(c.GetHeader("If-None-Match"), etag) {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, avatar.ContentType, avatar.Data)
}

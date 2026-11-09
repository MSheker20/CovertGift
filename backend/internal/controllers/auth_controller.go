package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthController struct {
	authService services.AuthService
}

func NewAuthController(authService services.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// RegisterRequest kullanıcı kaydı için DTO
type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100" example:"Can Yılmaz"`
	Email    string `json:"email" binding:"required,email,max=150" example:"can@demo.com"`
	Password string `json:"password" binding:"required,min=6,max=100" example:"Password123!"`
}

// LoginRequest kullanıcı girişi için DTO
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"can@demo.com"`
	Password string `json:"password" binding:"required,max=100" example:"Password123!"`
}

// LoginResponse kullanıcı girişi sonucunda dönen veri yapısıdır
type LoginResponse struct {
	Token string      `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  models.User `json:"user"`
}

// Register yeni kullanıcı kaydı oluşturur
// @Summary      Yeni Kullanıcı Kaydı
// @Description  E-posta, şifre ve isim ile sisteme yeni bir kullanıcı hesabı kaydeder. Şifre bcrypt ile hashlenir.
// @Tags         Kimlik Doğrulama (Auth)
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Kullanıcı Kayıt Bilgileri"
// @Success      201 {object} Response{data=models.User} "Kullanıcı başarıyla oluşturuldu"
// @Failure      400 {object} Response "Geçersiz istek parametreleri"
// @Failure      409 {object} Response "Bu e-posta adresi zaten kullanımda"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/auth/register [post]
func (ctl *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	user, err := ctl.authService.Register(req.Name, req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrEmailAlreadyExists) {
			RespondErr(c, http.StatusConflict, err.Error())
			return
		}
		RespondInternal(c, "Kayıt oluşturulurken bir hata oluştu", err)
		return
	}

	RespondCreated(c, user)
}

// Login kullanıcı girişi yapar ve JWT döner
// @Summary      Kullanıcı Girişi (JWT Üretimi)
// @Description  E-posta ve şifre doğrulaması yaparak 24 saat geçerli JWT Bearer token ve kullanıcı profilini döner.
// @Tags         Kimlik Doğrulama (Auth)
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Giriş Bilgileri"
// @Success      200 {object} Response{data=LoginResponse} "Giriş başarılı, JWT token üretildi"
// @Failure      400 {object} Response "Geçersiz istek formatı"
// @Failure      401 {object} Response "Geçersiz e-posta veya şifre"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/auth/login [post]
func (ctl *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	user, token, err := ctl.authService.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			RespondErr(c, http.StatusUnauthorized, err.Error())
			return
		}
		RespondInternal(c, "Giriş yapılırken bir hata oluştu", err)
		return
	}

	RespondOK(c, LoginResponse{
		Token: token,
		User:  *user,
	})
}

// Me oturum açmış kullanıcının profilini döner
// @Summary      Mevcut Kullanıcı Profili
// @Description  Authorization header'ında iletilen JWT token'a ait kullanıcının profil bilgilerini getirir.
// @Tags         Kimlik Doğrulama (Auth)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=models.User} "Kullanıcı profil bilgileri"
// @Failure      401 {object} Response "Oturum açılmamış veya geçersiz token"
// @Failure      404 {object} Response "Kullanıcı bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/me [get]
func (ctl *AuthController) Me(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		RespondErr(c, http.StatusUnauthorized, "Oturum bilgisi bulunamadı")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		RespondInternal(c, "Geçersiz kullanıcı kimliği", errors.New("context userID uuid.UUID değil"))
		return
	}

	user, err := ctl.authService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondInternal(c, "Kullanıcı bilgisi alınamadı", err)
		return
	}

	RespondOK(c, user)
}

// DeleteMe oturum açmış kullanıcının kendi profilini ve hesabını siler
// @Summary      Kendi Profilini ve Hesabını Sil
// @Description  Oturum açmış kullanıcı kendi profilini, oluşturduğu grupları, hediye önerilerini ve oylarını kalıcı olarak siler.
// @Tags         Kimlik Doğrulama (Auth)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=MessageData} "Hesap başarıyla silindi"
// @Failure      401 {object} Response "Oturum açılmamış veya geçersiz token"
// @Failure      404 {object} Response "Kullanıcı bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/me [delete]
func (ctl *AuthController) DeleteMe(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		RespondErr(c, http.StatusUnauthorized, "Oturum bilgisi bulunamadı")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		RespondInternal(c, "Geçersiz kullanıcı kimliği", errors.New("context userID uuid.UUID değil"))
		return
	}

	if err := ctl.authService.DeleteUser(userID); err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondInternal(c, "Hesap silinirken bir hata oluştu", err)
		return
	}

	RespondOK(c, MessageData{Message: "Hesabınız ve ilişkili tüm verileriniz başarıyla silindi"})
}


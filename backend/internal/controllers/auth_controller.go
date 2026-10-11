package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthController kimlik doğrulama HTTP isteklerini yöneten yapıdır
type AuthController struct {
	authService services.AuthService
}

// NewAuthController yeni bir AuthController örneği oluşturur (Dependency Injection)
func NewAuthController(authService services.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// RegisterRequest kayıt isteği için beklenen JSON veri modelidir (DTO)
type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email,max=150"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

// LoginRequest giriş isteği için beklenen JSON veri modelidir (DTO)
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register yeni kullanıcı kaydı oluşturur (POST /api/v1/auth/register)
func (ctl *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	// 1. Gelen JSON'ı doğrula (Validation)
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	// 2. Service katmanına kaydetmesini söyle
	user, err := ctl.authService.Register(req.Name, req.Email, req.Password)
	if err != nil {
		// E-posta zaten varsa 409 Conflict döner
		if errors.Is(err, services.ErrEmailAlreadyExists) {
			RespondErr(c, http.StatusConflict, err.Error())
			return
		}
		// Beklenmeyen veritabanı hatalarında 500 döner
		RespondErr(c, http.StatusInternalServerError, "Kayıt oluşturulurken bir hata oluştu")
		return
	}

	// 3. Başarılı kayıt: 201 Created
	RespondCreated(c, user)
}

// Login kullanıcı girişi yapar ve JWT token döner (POST /api/v1/auth/login)
func (ctl *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	// 1. JSON doğrulaması
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz istek: "+err.Error())
		return
	}

	// 2. Service üzerinden kullanıcıyı ve şifreyi doğrula
	user, token, err := ctl.authService.Login(req.Email, req.Password)
	if err != nil {
		// E-posta veya şifre yanlışsa 401 Unauthorized döner
		if errors.Is(err, services.ErrInvalidCredentials) {
			RespondErr(c, http.StatusUnauthorized, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Giriş yapılırken bir hata oluştu")
		return
	}

	// 3. Başarılı giriş: 200 OK + token ve kullanıcı bilgisi
	RespondOK(c, gin.H{
		"token": token,
		"user":  user,
	})
}

// Me oturum açmış kullanıcının profilini döner (GET /api/v1/me)
func (ctl *AuthController) Me(c *gin.Context) {
	// 1. Middleware'in Context'e yazdığı userID'yi al
	val, exists := c.Get("userID")
	if !exists {
		RespondErr(c, http.StatusUnauthorized, "Oturum bilgisi bulunamadı")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		RespondErr(c, http.StatusInternalServerError, "Geçersiz kullanıcı kimliği formatı")
		return
	}

	// 2. Kullanıcı bilgilerini Service'den getir
	user, err := ctl.authService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Kullanıcı bilgisi alınamadı")
		return
	}

	// 3. 200 OK ile kullanıcı bilgilerini dön
	RespondOK(c, user)
}

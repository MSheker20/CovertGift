package controllers

import (
	"errors"
	"net/http"

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

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email,max=150"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Register yeni kullanıcı kaydı oluşturur (POST /api/v1/auth/register)
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
		RespondErr(c, http.StatusInternalServerError, "Kayıt oluşturulurken bir hata oluştu")
		return
	}

	RespondCreated(c, user)
}

// Login kullanıcı girişi yapar ve JWT döner (POST /api/v1/auth/login)
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
		RespondErr(c, http.StatusInternalServerError, "Giriş yapılırken bir hata oluştu")
		return
	}

	RespondOK(c, gin.H{
		"token": token,
		"user":  user,
	})
}

// Me oturum açmış kullanıcının profilini döner (GET /api/v1/me)
func (ctl *AuthController) Me(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		RespondErr(c, http.StatusUnauthorized, "Oturum bilgisi bulunamadı")
		return
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		RespondErr(c, http.StatusInternalServerError, "Geçersiz kullanıcı kimliği")
		return
	}

	user, err := ctl.authService.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			RespondErr(c, http.StatusNotFound, err.Error())
			return
		}
		RespondErr(c, http.StatusInternalServerError, "Kullanıcı bilgisi alınamadı")
		return
	}

	RespondOK(c, user)
}

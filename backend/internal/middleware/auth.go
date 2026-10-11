package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Auth gelen isteklerdeki JWT token'ını doğrulayan Gin middleware'idir
func Auth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			controllers.RespondErr(c, http.StatusUnauthorized, "Yetkilendirme başlığı (Authorization) eksik")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			controllers.RespondErr(c, http.StatusUnauthorized, "Geçersiz yetkilendirme biçimi (Bearer <token> olmalı)")
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims := &services.JWTClaims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("beklenmeyen imza yöntemi")
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			controllers.RespondErr(c, http.StatusUnauthorized, "Geçersiz veya süresi dolmuş token")
			c.Abort()
			return
		}

		// Faz 3 Anti-Spoiler ve sonraki işlemler için Context'e yazıyoruz
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Next()
	}
}

// GetUserID context'ten oturum açmış kullanıcının UUID'sini döndüren yardımcı fonksiyondur
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}

package middleware

import (
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GroupAccess gruba erişim yetkisini ve Anti-Spoiler kuralını denetleyen middleware'dir
func GroupAccess(groupRepo repository.GroupRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. URL'den :id parametresini al
		groupIDStr := c.Param("id")
		groupID, err := uuid.Parse(groupIDStr)
		if err != nil {
			controllers.RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği formatı")
			c.Abort()
			return
		}

		// 2. Oturum açmış kullanıcının ID'sini context'ten al
		userID, ok := GetUserID(c)
		if !ok {
			controllers.RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
			c.Abort()
			return
		}

		// 3. Grubu veritabanından getir
		group, err := groupRepo.GetGroupByID(groupID)
		if err != nil {
			controllers.RespondInternal(c, "Grup sorgulanırken hata oluştu", err)
			c.Abort()
			return
		}
		if group == nil {
			controllers.RespondErr(c, http.StatusNotFound, "Grup bulunamadı")
			c.Abort()
			return
		}

		// 4. KRİTİK: Anti-Spoiler Kuralı!
		// Giriş yapmış kullanıcı bu grubun hedef kişisi ise ANINDA 403 Forbidden ile engelle!
		if group.TargetUserID != nil && *group.TargetUserID == userID {
			controllers.RespondErr(c, http.StatusForbidden, "Erişim engellendi: Bu hediye grubu senin için oluşturulmuş bir sürprizdir.")
			c.Abort()
			return
		}

		// 5. Kullanıcı bu grubun üyesi mi?
		member, err := groupRepo.GetMember(groupID, userID)
		if err != nil {
			controllers.RespondInternal(c, "Üyelik kontrolü yapılırken hata oluştu", err)
			c.Abort()
			return
		}
		if member == nil {
			controllers.RespondErr(c, http.StatusForbidden, "Bu gruba erişim yetkiniz bulunmuyor")
			c.Abort()
			return
		}

		// 6. Bilgileri Context'e yaz (Controller'lar tekrar DB sorgusu atmasın diye)
		c.Set("currentGroup", group)
		c.Set("currentMemberRole", member.Role)

		c.Next()
	}
}

// GetCurrentGroup context'ten doğrulanmış grubu döndüren yardımcı fonksiyondur
func GetCurrentGroup(c *gin.Context) (*models.Group, bool) {
	val, exists := c.Get("currentGroup")
	if !exists {
		return nil, false
	}
	group, ok := val.(*models.Group)
	return group, ok
}

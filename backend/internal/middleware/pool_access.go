package middleware

import (
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PoolAccess /groups/:id/pools/:poolId/* altındaki her rotada, GroupAccess'ten SONRA ve gövde
// okunmadan ÖNCE çalışır:
//  1. poolId ayrıştırılır (geçersizse 400),
//  2. havuz bu grupta aranır (yoksa 404 "Havuz bulunamadı"; SINGLE grupta havuz olmadığı için 404),
//  3. KRİTİK (Anti-Spoiler): çağıran havuzun alıcısıysa 403 + sürpriz mesajı, veri yok,
//  4. havuz ve çağıranın yönetim yetkisi context'e yazılır (currentPool, currentPoolCanManage).
//
// Havuzun alıcısı, kendisine verilen kilitli kartın kimliği dışında hiçbir havuz verisine ulaşamaz.
func PoolAccess(poolRepo repository.PoolRepository, pools services.PoolService) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			controllers.RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği formatı")
			c.Abort()
			return
		}
		poolID, err := uuid.Parse(c.Param("poolId"))
		if err != nil {
			controllers.RespondErr(c, http.StatusBadRequest, "Geçersiz havuz kimliği")
			c.Abort()
			return
		}
		userID, ok := GetUserID(c)
		if !ok {
			controllers.RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
			c.Abort()
			return
		}
		pool, err := poolRepo.GetPool(groupID, poolID)
		if err != nil {
			controllers.RespondInternal(c, "Havuz sorgulanırken hata oluştu", err)
			c.Abort()
			return
		}
		if pool == nil {
			controllers.RespondErr(c, http.StatusNotFound, "Havuz bulunamadı")
			c.Abort()
			return
		}
		if pool.RecipientUserID == userID {
			controllers.RespondErr(c, http.StatusForbidden, services.ErrAntiSpoilerTriggered.Error())
			c.Abort()
			return
		}
		canManage, err := pools.CanManage(pool, userID)
		if err != nil {
			controllers.RespondInternal(c, "Havuz yetkisi denetlenirken hata oluştu", err)
			c.Abort()
			return
		}
		c.Set(controllers.CurrentPoolKey, pool)
		c.Set(controllers.CurrentPoolCanManageKey, canManage)
		c.Next()
	}
}

// RequireMode rotayı yalnızca verilen grup türünde (SINGLE / MULTI) açar; diğer türde 409 döner.
// GroupAccess'ten sonra rota bazında bağlanır (Anti-Spoiler ve üyelik önce denetlenir).
func RequireMode(mode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		group, ok := GetCurrentGroup(c)
		if !ok {
			controllers.RespondErr(c, http.StatusInternalServerError, "Grup bilgisi bulunamadı")
			c.Abort()
			return
		}
		current := models.GroupModeSingle
		if group.IsMulti() {
			current = models.GroupModeMulti
		}
		if current != mode {
			controllers.RespondErr(c, http.StatusConflict, services.ErrWrongGroupMode.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}

// IdeaGuard /ideas/:id/* rotalarında gövde doğrulamasından ÖNCE Anti-Spoiler'ı denetler: fikrin
// grubunun hedef kişisi ya da fikrin havuzunun alıcısı 403 + sürpriz mesajı alır. Diğer tüm
// denetimler (varlık, durum, üyelik, yetki) servisteki resolveIdea'dadır.
func IdeaGuard(ideas services.IdeaService) gin.HandlerFunc {
	return func(c *gin.Context) {
		ideaID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.Next() // handler 400 döner
			return
		}
		userID, ok := GetUserID(c)
		if !ok {
			c.Next()
			return
		}
		if err := ideas.GuardIdea(ideaID, userID); err != nil {
			controllers.RespondErr(c, http.StatusForbidden, err.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}

package middleware

import (
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/controllers"
	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func isMutation(c *gin.Context) bool {
	m := c.Request.Method
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}

func succeeded(c *gin.Context) bool {
	return c.Writer.Status() < http.StatusBadRequest
}

// noPublishKey bir isteğin hiçbir şey yazmadığını işaretleyen gin context anahtarıdır; handler'lar
// aynı anahtarı controllers.SkipRealtimePublish ile koyar.
const noPublishKey = controllers.NoRealtimePublishKey

// NoPublish veritabanına yazmayan bir değişiklik rotasını (ör. POST .../ideas/preview-link) canlı
// yayından çıkarır. Anti-Spoiler: yazmayan bir istek de grup anahtarına "changed" yayınlasaydı,
// havuzun alıcısı görünür bir değişiklik olmadan gelen olaydan (ör. kendi havuzuna fikir eklenmeden
// hemen önceki link önizlemesinden) bir şeyler çıkarabilirdi. Rota bazında, handler'dan önce bağlanır.
func NoPublish() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(noPublishKey, true)
		c.Next()
	}
}

func publishSkipped(c *gin.Context) bool {
	skip, _ := c.Get(noPublishKey)
	v, _ := skip.(bool)
	return v
}

// PublishGroupChanges /groups/:id/* altında başarılı her değişiklikten sonra o grubun
// abonelerine sinyal gönderir. /groups/:id/pools/:poolId/* rotalarında sinyal grup anahtarına
// değil havuz anahtarına gider: grup akışı yalnızca çağıranın alıcısı olmadığı havuzlara abone
// olduğundan, alıcı kendi havuzundaki değişikliklerin zamanlamasını da öğrenemez (Anti-Spoiler).
func PublishGroupChanges(hub *realtime.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if !isMutation(c) || !succeeded(c) || publishSkipped(c) {
			return
		}
		if poolID, err := uuid.Parse(c.Param("poolId")); err == nil {
			hub.Publish(poolID)
			return
		}
		if groupID, err := uuid.Parse(c.Param("id")); err == nil {
			hub.Publish(groupID)
		}
	}
}

// PublishRecordedGroupChanges /groups/:id altında olmayan rotalar (ör. davetle katılma) için:
// handler controllers.RecordGroupChanged ile değişen grubu bildirdiyse ve istek başarılıysa o
// grubun abonelerine sinyal gönderir.
func PublishRecordedGroupChanges(hub *realtime.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if !isMutation(c) || !succeeded(c) {
			return
		}
		if groupID, ok := controllers.RecordedGroupChanged(c); ok {
			hub.Publish(groupID)
		}
	}
}

// PublishIdeaChanges /ideas/:id/* altında başarılı her değişiklikten sonra fikrin grubuna (fikir
// bir havuzdaysa havuzun anahtarına) sinyal gönderir. Anahtar işlemden ÖNCE bulunur; böylece
// silinen fikirde de olay gider.
func PublishIdeaChanges(hub *realtime.Hub, ideas repository.IdeaRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isMutation(c) {
			c.Next()
			return
		}
		var key uuid.UUID
		if ideaID, err := uuid.Parse(c.Param("id")); err == nil {
			if idea, err := ideas.GetIdeaByID(ideaID); err == nil && idea != nil {
				key = idea.GroupID
				if idea.PoolID != nil {
					key = *idea.PoolID
				}
			}
		}
		c.Next()
		if key != uuid.Nil && succeeded(c) {
			hub.Publish(key)
		}
	}
}

// Notify /groups/:id/* ve /ideas/:id/* altında başarılı her değişiklikten sonra, handler'ın
// controllers.RecordNotification ile bildirdiği olayları kalıcı bildirime çevirir ve
// alıcıların bildirim akışına sinyal gönderir. Rota grubuna bağlı olduğu için yeni eklenen
// bir değişiklik ucu da otomatik olarak bu zincirden geçer; başarısız istek (>= 400) ya da
// olay bildirmeyen handler hiçbir şey üretmez. Alıcı seçimi ve Anti-Spoiler
// NotificationService.Dispatch içindedir.
func Notify(notifications services.NotificationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if !isMutation(c) || !succeeded(c) {
			return
		}
		events := controllers.RecordedNotifications(c)
		if len(events) == 0 {
			return
		}
		actorID, ok := GetUserID(c)
		if !ok {
			return
		}
		// Güvenlik ağı: /pools/:poolId rotalarında handler PoolID'yi boş bıraktıysa rota
		// parametresinden doldurulur; böylece havuzun alıcısı Dispatch'te her zaman dışlanır.
		if poolID, err := uuid.Parse(c.Param("poolId")); err == nil {
			for i := range events {
				if events[i].PoolID == nil && !events[i].RefreshOnly {
					id := poolID
					events[i].PoolID = &id
				}
			}
		}
		notifications.Dispatch(actorID, events)
	}
}

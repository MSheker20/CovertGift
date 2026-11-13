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

// PublishGroupChanges /groups/:id/* altında başarılı her değişiklikten sonra o grubun
// abonelerine sinyal gönderir.
func PublishGroupChanges(hub *realtime.Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if !isMutation(c) || !succeeded(c) {
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

// PublishIdeaChanges /ideas/:id/* altında başarılı her değişiklikten sonra fikrin grubuna
// sinyal gönderir. Grup, işlemden ÖNCE bulunur; böylece silinen fikirde de olay gider.
func PublishIdeaChanges(hub *realtime.Hub, ideas repository.IdeaRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isMutation(c) {
			c.Next()
			return
		}
		var groupID uuid.UUID
		if ideaID, err := uuid.Parse(c.Param("id")); err == nil {
			if idea, err := ideas.GetIdeaByID(ideaID); err == nil && idea != nil {
				groupID = idea.GroupID
			}
		}
		c.Next()
		if groupID != uuid.Nil && succeeded(c) {
			hub.Publish(groupID)
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
		notifications.Dispatch(actorID, events)
	}
}

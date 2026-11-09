package middleware

import (
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
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

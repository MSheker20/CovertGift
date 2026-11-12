package controllers

import (
	"errors"
	"net/http"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// notificationEventsKey bir istekte bildirilen olayların gin context anahtarıdır.
const notificationEventsKey = "notificationEvents"

// RecordNotification başarılı bir değişikliğin bildirime dönüşmesi gerektiğini bildirir.
// Handler yalnızca "ne oldu"yu söyler; alıcı seçimi, Anti-Spoiler, kayıt ve SSE sinyali
// middleware.Notify üzerinden NotificationService.Dispatch içinde tek yerde yapılır ve
// yalnızca istek başarıyla (< 400) bittiyse çalışır.
func RecordNotification(c *gin.Context, event services.NotificationEvent) {
	c.Set(notificationEventsKey, append(RecordedNotifications(c), event))
}

// RecordedNotifications istekte bildirilen olayları döner.
func RecordedNotifications(c *gin.Context) []services.NotificationEvent {
	val, exists := c.Get(notificationEventsKey)
	if !exists {
		return nil
	}
	events, _ := val.([]services.NotificationEvent)
	return events
}

// groupBeforeRequest GroupAccess middleware'inin işlemden ÖNCE okuduğu grubu döner; bütçe ve
// tarih gibi alanların gerçekten değişip değişmediğini anlamak için kullanılır.
func groupBeforeRequest(c *gin.Context) *models.Group {
	val, exists := c.Get("currentGroup")
	if !exists {
		return nil
	}
	group, _ := val.(*models.Group)
	return group
}

// NotificationController bildirim zili uçlarını yönetir.
type NotificationController struct {
	service services.NotificationService
}

// NewNotificationController yeni bir NotificationController oluşturur.
func NewNotificationController(service services.NotificationService) *NotificationController {
	return &NotificationController{service: service}
}

// List kullanıcının bildirimlerini ve okunmamış sayısını döner
// @Summary      Bildirimleri Listele (Anti-Spoiler Korumalı)
// @Description  Giriş yapan kullanıcının en yeni 50 bildirimini (yeniden eskiye) ve okunmamış bildirim sayısını döner. Metin istemcide `type` ve `params` ile oluşturulur. Anti-Spoiler: kullanıcının hedef kişisi olduğu gruplara ait bildirimler hiçbir zaman üretilmez ve bu listede ayrıca süzülür.
// @Tags         Bildirimler (Notifications)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=services.NotificationList} "Bildirimler ve okunmamış sayısı"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/notifications [get]
func (ctl *NotificationController) List(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	list, err := ctl.service.List(userID)
	if err != nil {
		RespondInternal(c, "Bildirimler alınırken bir hata oluştu", err)
		return
	}
	RespondOK(c, list)
}

// MarkRead tek bir bildirimi okundu yapar
// @Summary      Bildirimi Okundu Yap
// @Description  Kullanıcının kendi bildirimini okundu olarak işaretler (tekrar çağırmak güvenlidir). Bildirim yoksa ya da başka bir kullanıcıya aitse 404 döner.
// @Tags         Bildirimler (Notifications)
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Bildirim UUID" Format(uuid)
// @Success      200 {object} Response{data=MessageData} "Bildirim okundu yapıldı"
// @Failure      400 {object} Response "Geçersiz bildirim kimliği"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      404 {object} Response "Bildirim bulunamadı"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/notifications/{id}/read [post]
func (ctl *NotificationController) MarkRead(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz bildirim kimliği")
		return
	}
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.service.MarkRead(userID, id); err != nil {
		if errors.Is(err, services.ErrNotificationNotFound) {
			RespondErr(c, http.StatusNotFound, "Bildirim bulunamadı")
			return
		}
		RespondInternal(c, "Bildirim güncellenirken bir hata oluştu", err)
		return
	}
	RespondOK(c, MessageData{Message: "Bildirim okundu olarak işaretlendi"})
}

// MarkAllRead kullanıcının tüm bildirimlerini okundu yapar
// @Summary      Tüm Bildirimleri Okundu Yap
// @Description  Giriş yapan kullanıcının okunmamış tüm bildirimlerini okundu olarak işaretler.
// @Tags         Bildirimler (Notifications)
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response{data=MessageData} "Tüm bildirimler okundu yapıldı"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      500 {object} Response "Sunucu hatası"
// @Router       /api/v1/notifications/read-all [post]
func (ctl *NotificationController) MarkAllRead(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if err := ctl.service.MarkAllRead(userID); err != nil {
		RespondInternal(c, "Bildirimler güncellenirken bir hata oluştu", err)
		return
	}
	RespondOK(c, MessageData{Message: "Tüm bildirimler okundu olarak işaretlendi"})
}

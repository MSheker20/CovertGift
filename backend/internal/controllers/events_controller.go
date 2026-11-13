package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EventsController Server-Sent Events akışlarını sağlar: grup sayfaları için grup akışı,
// bildirim zili için kullanıcı akışı. İkisi de aynı çerçeveyi (heartbeat, ömür) kullanır.
type EventsController struct {
	hub *realtime.Hub
	// userHub kullanıcı kimliğiyle anahtarlanır (bildirimler). nil ise bildirim akışı kapalıdır.
	userHub *realtime.Hub
	// Heartbeat bağlantıyı proxy'lerde canlı tutan yorum satırı aralığıdır.
	Heartbeat time.Duration
	// MaxLifetime bir akışın en uzun süresidir. Süre dolunca sunucu kapatır, istemci
	// yeniden bağlanır; böylece yetki (oturum, üyelik, Anti-Spoiler) düzenli olarak yeniden denetlenir.
	MaxLifetime time.Duration
	// PoolKeys MULTI grup akışında çağıranın alıcısı OLMADIĞI havuzları döner (hub anahtarları).
	// nil ise MULTI grup akışı yalnızca grup anahtarını dinler.
	PoolKeys func(groupID, userID uuid.UUID) ([]uuid.UUID, error)
}

// NewEventsController varsayılan sürelerle bir EventsController oluşturur.
func NewEventsController(hub, userHub *realtime.Hub) *EventsController {
	return &EventsController{hub: hub, userHub: userHub, Heartbeat: 25 * time.Second, MaxLifetime: 5 * time.Minute}
}

// Stream grubun değişiklik olaylarını SSE olarak yayınlar
// @Summary      Grup Olay Akışı (Server-Sent Events)
// @Description  Grupta bir fikir, oy, bütçe, üye veya durum değişikliği olduğunda "changed" olayı gönderir. MULTI grupta akış grup anahtarına ve yalnızca çağıranın alıcısı OLMADIĞI havuzlara abone olur; çağıranın kendi havuzundaki değişiklikler hiçbir olay üretmez (Anti-Spoiler). Görünen havuz kümesi değişirse (alıcı eklendi/kaldırıldı) olaydan sonra akış kapanır ve istemci yeniden bağlanır. Olay veri taşımaz; istemci grubu ve fikirleri yeniden çeker. Bağlantı en fazla 5 dakika açık kalır, ardından istemci yeniden bağlanır. Anti-Spoiler ve üyelik kontrolü bağlantı açılırken uygulanır.
// @Tags         Grup İşlemleri (Groups)
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        id path string true "Grup UUID" Format(uuid)
// @Success      200 {string} string "event: changed"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      403 {object} Response "Anti-Spoiler veya grup üyesi değil"
// @Failure      503 {object} Response "Açık bağlantı sınırı doldu"
// @Router       /api/v1/groups/{id}/events [get]
func (ctl *EventsController) Stream(c *gin.Context) {
	groupID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		RespondErr(c, http.StatusBadRequest, "Geçersiz grup kimliği")
		return
	}
	group := groupBeforeRequest(c)
	userID, ok := getUserID(c)
	if group == nil || !group.IsMulti() || ctl.PoolKeys == nil || !ok {
		ctl.serve(c, ctl.hub, []uuid.UUID{groupID}, nil)
		return
	}
	pools, err := ctl.PoolKeys(groupID, userID)
	if err != nil {
		RespondInternal(c, "Canlı güncelleme başlatılamadı", err)
		return
	}
	// Bir sinyalden sonra görünen havuz kümesi yeniden hesaplanır; değiştiyse (alıcı eklendi ya
	// da kaldırıldı) akış kapanır ve istemci yeni kümeyle yeniden bağlanır.
	stillValid := func() bool {
		current, err := ctl.PoolKeys(groupID, userID)
		return err == nil && sameKeys(current, pools)
	}
	ctl.serve(c, ctl.hub, append([]uuid.UUID{groupID}, pools...), stillValid)
}

// sameKeys iki kimlik listesinin aynı küme olduğunu söyler.
func sameKeys(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[uuid.UUID]bool, len(a))
	for _, id := range a {
		set[id] = true
	}
	for _, id := range b {
		if !set[id] {
			return false
		}
	}
	return true
}

// NotificationStream kullanıcının bildirim değişikliklerini SSE olarak yayınlar
// @Summary      Bildirim Olay Akışı (Server-Sent Events)
// @Description  Giriş yapan kullanıcıya yeni bir bildirim geldiğinde ya da bildirimleri okundu yapıldığında "changed" olayı gönderir. Olay veri taşımaz; istemci `GET /notifications` ucunu yeniden çeker (Anti-Spoiler süzgeci orada uygulanır). Bağlantı en fazla 5 dakika açık kalır, ardından istemci yeniden bağlanır; 25 sn'de bir `: ping`.
// @Tags         Bildirimler (Notifications)
// @Produce      text/event-stream
// @Security     BearerAuth
// @Success      200 {string} string "event: changed"
// @Failure      401 {object} Response "Oturum açılmamış"
// @Failure      503 {object} Response "Açık bağlantı sınırı doldu"
// @Router       /api/v1/notifications/stream [get]
func (ctl *EventsController) NotificationStream(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		RespondErr(c, http.StatusUnauthorized, "Oturum açmanız gerekiyor")
		return
	}
	if ctl.userHub == nil {
		RespondErr(c, http.StatusServiceUnavailable, "Canlı güncelleme şu an kullanılamıyor")
		return
	}
	ctl.serve(c, ctl.userHub, []uuid.UUID{userID}, nil)
}

// serve hub'daki anahtarlara tek kanalla abone olur ve her sinyalde veri taşımayan "changed"
// olayı yazar. afterEvent nil değilse her olaydan sonra çağrılır; false dönerse akış kapanır.
func (ctl *EventsController) serve(c *gin.Context, hub *realtime.Hub, keys []uuid.UUID, afterEvent func() bool) {
	events, cancel, err := hub.SubscribeMany(keys)
	if errors.Is(err, realtime.ErrTooManySubscribers) {
		RespondErr(c, http.StatusServiceUnavailable, "Canlı güncelleme şu an kullanılamıyor")
		return
	}
	if err != nil {
		RespondInternal(c, "Canlı güncelleme başlatılamadı", err)
		return
	}
	defer cancel()

	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// nginx'in yanıtı tamponlamaması için; olaylar hemen iletilir.
	h.Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	write := func(frame string) bool {
		if _, err := fmt.Fprint(c.Writer, frame); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}
	if !write("retry: 5000\n: connected\n\n") {
		return
	}

	heartbeat := time.NewTicker(ctl.Heartbeat)
	defer heartbeat.Stop()
	lifetime := time.NewTimer(ctl.MaxLifetime)
	defer lifetime.Stop()
	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-lifetime.C:
			return
		case <-heartbeat.C:
			if !write(": ping\n\n") {
				return
			}
		case <-events:
			if !write("event: changed\ndata: {}\n\n") {
				return
			}
			if afterEvent != nil && !afterEvent() {
				return
			}
		}
	}
}

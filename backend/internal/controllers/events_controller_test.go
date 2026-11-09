package controllers

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MSheker20/CovertGift/backend/internal/realtime"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestEventsStreamDeliversHeartbeatsEventsAndEnds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := realtime.NewHub(10)
	ctl := &EventsController{hub: hub, Heartbeat: 20 * time.Millisecond, MaxLifetime: 300 * time.Millisecond}
	r := gin.New()
	r.GET("/groups/:id/events", ctl.Stream)
	srv := httptest.NewServer(r)
	defer srv.Close()

	groupID := uuid.New()
	res, err := http.Get(srv.URL + "/groups/" + groupID.String() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" || res.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("başlıklar yanlış: %v", res.Header)
	}

	lines := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	waitFor := func(want string) {
		t.Helper()
		deadline := time.After(time.Second)
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatalf("akış %q gelmeden kapandı", want)
				}
				if l == want {
					return
				}
			case <-deadline:
				t.Fatalf("%q gelmedi", want)
			}
		}
	}
	waitFor(": connected")
	waitFor(": ping")
	if hub.Count() != 1 {
		t.Fatalf("abonelik sayısı %d", hub.Count())
	}
	hub.Publish(uuid.New()) // başka grup
	hub.Publish(groupID)
	waitFor("event: changed")

	// MaxLifetime dolunca sunucu akışı kapatır ve aboneliği bırakır.
	for range lines {
	}
	deadline := time.Now().Add(time.Second)
	for hub.Count() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.Count() != 0 {
		t.Fatal("akış kapandıktan sonra abonelik kaldı")
	}
}

func TestEventsStreamRejectsWhenFull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := realtime.NewHub(0)
	r := gin.New()
	r.GET("/groups/:id/events", NewEventsController(hub).Stream)
	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/groups/"+uuid.NewString()+"/events", nil).WithContext(context.Background())
	r.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable || !strings.Contains(res.Body.String(), "success") {
		t.Fatalf("HTTP %d %s", res.Code, res.Body)
	}
}

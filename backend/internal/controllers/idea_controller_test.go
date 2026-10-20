package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"github.com/MSheker20/CovertGift/backend/pkg/gemini"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type failingAIService struct {
	services.IdeaService
	err error
}

func (s failingAIService) GenerateAIIdeas(context.Context, uuid.UUID, uuid.UUID) ([]models.GiftIdea, error) {
	return nil, s.err
}

func TestGenerateAIReturns429ForWrappedProviderQuotaError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	controller := NewIdeaController(failingAIService{err: fmt.Errorf("internal-provider-detail: %w", gemini.ErrRateLimited)})
	router.POST("/groups/:id/ideas/generate-ai", func(c *gin.Context) {
		c.Set("userID", uuid.New())
		controller.GenerateAI(c)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/groups/"+uuid.NewString()+"/ideas/generate-ai", nil))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("got HTTP %d: %s", response.Code, response.Body)
	}
	var body Response
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Success || !strings.Contains(body.Error, "kota") || strings.Contains(body.Error, "internal-provider-detail") {
		t.Fatalf("unexpected quota response: %+v", body)
	}
}

package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
	"google.golang.org/genai"
)

var (
	// ErrAPIKeyMissing API anahtarı tanımlanmadığında döner
	ErrAPIKeyMissing = errors.New("GEMINI_API_KEY ortam değişkeni tanımlı değil")
	// ErrEmptyResponse Gemini yanıt üretmediğinde döner
	ErrEmptyResponse = errors.New("Gemini'den yanıt alınamadı")
)

// AIIdeaSuggestion Gemini'nin Structured Output olarak döneceği tekil hediye önerisi
type AIIdeaSuggestion struct {
	Title          string          `json:"title"`
	Description    string          `json:"description"`
	EstimatedPrice decimal.Decimal `json:"estimated_price"`
	SearchQuery    string          `json:"search_query"`
}

// Client Gemini API istemcisini saran yapıdır
type Client struct {
	client *genai.Client
	model  string
}

// NewClient yeni bir Gemini istemcisi oluşturur
func NewClient(apiKey, model string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, ErrAPIKeyMissing
	}

	if strings.TrimSpace(model) == "" {
		model = "gemini-3.8-flash"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("Gemini istemcisi oluşturulamadı: %w", err)
	}

	return &Client{
		client: client,
		model:  model,
	}, nil
}

// GenerateGiftIdeas verilen State-Based prompt ile Gemini'den yapılandırılmış JSON hediye listesi alır
func (c *Client) GenerateGiftIdeas(ctx context.Context, prompt string) ([]AIIdeaSuggestion, error) {
	if c == nil || c.client == nil {
		return nil, ErrAPIKeyMissing
	}

	// Kural 3 (Sıfır Kırık Link Kuralı):
	// Gemini asla URL/Link üretmez; sadece pazaryerleri için temiz 'search_query' üretir.
	// Structured JSON Schema tanımlıyoruz:
	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeArray,
			Items: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"title": {
						Type:        genai.TypeString,
						Description: "Önerilen hediyenin net ve çekici başlığı (ör. 'Gürültü Engelleyici Bluetooth Kulaklık')",
					},
					"description": {
						Type:        genai.TypeString,
						Description: "Bu hediyenin hedef kişinin ilgi alanlarına neden uygun olduğuna dair 1-2 cümlelik açıklama",
					},
					"estimated_price": {
						Type:        genai.TypeNumber,
						Description: "Hediyenin Türk Lirası (TL) cinsinden yaklaşık piyasa fiyatı (ör. 850.00)",
					},
					"search_query": {
						Type:        genai.TypeString,
						Description: "E-ticaret sitelerinde (Trendyol, Hepsiburada vb.) aratıldığında doğru ürünü bulacak temiz arama ifadesi. ASLA URL veya link içermez.",
					},
				},
				Required: []string{"title", "description", "estimated_price", "search_query"},
			},
		},
	}

	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), config)
	if err != nil {
		// Geçici yoğunluk (503 High Demand) durumunda hızlı yedek modeli dene
		if strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "high demand") {
			fallbackModel := "gemini-3.5-flash-lite"
			if c.model != fallbackModel {
				result, err = c.client.Models.GenerateContent(ctx, fallbackModel, genai.Text(prompt), config)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("Gemini içerik üretimi başarısız: %w", err)
		}
	}

	responseText := strings.TrimSpace(result.Text())
	if responseText == "" {
		return nil, ErrEmptyResponse
	}

	var suggestions []AIIdeaSuggestion
	if err := json.Unmarshal([]byte(responseText), &suggestions); err != nil {
		return nil, fmt.Errorf("Gemini JSON yanıtı ayrıştırılamadı: %w (yanıt: %s)", err, responseText)
	}

	return suggestions, nil
}

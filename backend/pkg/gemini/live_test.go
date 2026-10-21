package gemini

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

var liveAI = flag.Bool("live-ai", false, "Call the real Gemini API using backend/.env (may use API quota)")

func TestLiveTurkeyProducts(t *testing.T) {
	if !*liveAI {
		t.Skip("enable explicitly with -args -live-ai")
	}
	_ = godotenv.Load("../../.env")
	client, err := NewClientWithOptions(os.Getenv("GEMINI_API_KEY"), Options{Model: os.Getenv("GEMINI_MODEL"), ResearchModel: os.Getenv("GEMINI_RESEARCH_MODEL"), FallbackModel: os.Getenv("GEMINI_FALLBACK_MODEL"), SearchEnabled: os.Getenv("GEMINI_SEARCH_ENABLED") == "true", ThinkingLevel: os.Getenv("GEMINI_THINKING_LEVEL")})
	if err != nil {
		t.Fatal(err)
	}
	ideas, err := client.GenerateGiftIdeas(context.Background(), "Hedef kişinin ilgi alanları: kahve demleme ve kamp. Türkiye'den alınacak tek hediye için bütçe 3000 TL. Öncelikle bütçe içindeki ürünleri seç. Birbirinden farklı ürünler öner, URL üretme.")
	if err != nil {
		t.Fatal(err)
	}
	if len(ideas) == 0 {
		t.Fatal("no suggestions")
	}
	for _, idea := range ideas {
		if !client.searchEnabled && (!idea.Unverified || len(idea.Sources) != 0 || idea.SearchSuggestions != "") {
			t.Fatal("search-free result incorrectly marked verified")
		}
		t.Logf("%s | %s TL | query=%s | %d source(s)", idea.Title, idea.EstimatedPrice, idea.SearchQuery, len(idea.Sources))
	}
}

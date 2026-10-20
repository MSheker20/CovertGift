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
	client, err := NewClientWithModels(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_RESEARCH_MODEL"), os.Getenv("GEMINI_MODEL"))
	if err != nil {
		t.Fatal(err)
	}
	ideas, err := client.GenerateGiftIdeas(context.Background(), "Hedef kişinin ilgi alanları: kahve demleme ve kamp. Türkiye'den alınacak tek hediye için bütçe 3000 TL. Öncelikle bütçe içindeki ürünleri seç. Birbirinden farklı ürünler öner, URL üretme.")
	if err != nil {
		t.Fatal(err)
	}
	for _, idea := range ideas {
		t.Logf("%s | %s TL | query=%s | %d source(s)", idea.Title, idea.EstimatedPrice, idea.SearchQuery, len(idea.Sources))
	}
}

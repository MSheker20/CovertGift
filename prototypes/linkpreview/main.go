package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// extractMeta Tag regex ile meta tag içeriğini çeker
func extractMeta(html, property string) string {
	// <meta property="og:title" content="..."> veya <meta content="..." property="og:title">
	patterns := []string{
		fmt.Sprintf(`(?i)<meta[^>]+(?:property|name)=["']%s["'][^>]+content=["']([^"']+)["']`, regexp.QuoteMeta(property)),
		fmt.Sprintf(`(?i)<meta[^>]+content=["']([^"']+)["'][^>]+(?:property|name)=["']%s["']`, regexp.QuoteMeta(property)),
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

// extractTitle standart <title> etiketini çeker
func extractTitle(html string) string {
	re := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractJSONLDPrice JSON-LD (schema.org Product/Offer) içinden fiyatı çeker
func extractJSONLDPrice(html string) string {
	re := regexp.MustCompile(`"price"\s*:\s*"?([0-9]+(?:[.,][0-9]+)?)"?`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// fetch verilen URL'yi çeker; HTML gövdesini ve son varılan URL'yi döner
func fetch(client *http.Client, targetURL string) (string, *url.URL, error) {
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("istek oluşturulamadı: %w", err)
	}

	// WhatsApp / Sosyal Medya Bot kimliği
	req.Header.Set("User-Agent", "WhatsApp/2.21.12.21 A")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("bağlantı hatası: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("📌 HTTP Durum Kodu: %s\n", resp.Status)
	fmt.Printf("📌 Son Vardığı URL: %s\n", resp.Request.URL.String())

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("sayfa içeriği okunamadı: %w", err)
	}
	return string(bodyBytes), resp.Request.URL, nil
}

// resolveTrackerRedirect Adjust gibi takip sayfalarında gömülü gerçek hedef URL'yi bulur
func resolveTrackerRedirect(u *url.URL) string {
	if u == nil {
		return ""
	}
	if strings.HasSuffix(u.Host, "adj.st") || strings.HasSuffix(u.Host, "adjust.com") {
		return u.Query().Get("adjust_redirect") // Query().Get zaten URL decode eder
	}
	return ""
}

// isGenericImage görselin ürün resmi değil reklam/pazarlama görseli olup olmadığını kontrol eder
func isGenericImage(img string) bool {
	for _, k := range []string{"discovery-web", "banner", "mrktng", "/crm/", "popup", "automationimages"} {
		if strings.Contains(strings.ToLower(img), k) {
			return true
		}
	}
	return false
}

func testURL(targetURL string) {
	fmt.Printf("\n==================================================\n")
	fmt.Printf("🔍 Test Edilen URL: %s\n", targetURL)
	fmt.Printf("==================================================\n")

	// Çerezleri ve yönlendirmeleri (redirect) takip eden istemci
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			fmt.Printf("↪️  Yönlendirme Takip Ediliyor: %s\n", req.URL.Host+req.URL.Path)
			return nil
		},
	}

	html, finalURL, err := fetch(client, targetURL)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}

	// Takip (tracker) sayfasında kaldıysak gerçek ürün sayfasına ikinci istek at
	if real := resolveTrackerRedirect(finalURL); real != "" {
		fmt.Printf("\n🔁 Takip sayfası tespit edildi (%s). Gerçek hedefe gidiliyor:\n   %s\n", finalURL.Host, real)
		html2, _, err := fetch(client, real)
		if err != nil {
			fmt.Printf("❌ Gerçek hedef çekilemedi, takip sayfası verileri kullanılacak: %v\n", err)
		} else {
			html = html2
		}
	}

	// Verileri Ayrıştır
	ogTitle := extractMeta(html, "og:title")
	ogImage := extractMeta(html, "og:image")
	ogDesc := extractMeta(html, "og:description")
	ogPrice := extractMeta(html, "product:price:amount")
	if ogPrice == "" {
		ogPrice = extractJSONLDPrice(html)
	}
	pageTitle := extractTitle(html)

	fmt.Println("\n--- 📦 Çıkarılan Veriler ---")
	fmt.Printf("🔹 HTML <title>:      %s\n", pageTitle)
	fmt.Printf("🔹 og:title:          %s\n", ogTitle)
	fmt.Printf("🔹 og:description:    %s\n", ogDesc)
	fmt.Printf("🔹 og:image:          %s\n", ogImage)
	fmt.Printf("🔹 price:             %s\n", ogPrice)
	fmt.Printf("🔹 HTML boyutu:       %d byte\n", len(html))

	// Görsel Kontrolü
	if isGenericImage(ogImage) {
		fmt.Println("⚠️  UYARI: Çekilen görsel ürün resmi değil, jenerik reklam/pazarlama görseli!")
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Kullanım: go run main.go <URL>")
		fmt.Println("Örnek:   go run main.go \"https://ty.gl/...\"")
		return
	}

	testURL(os.Args[1])
}

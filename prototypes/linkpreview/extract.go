package main

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	metaTagRe = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	// content="..." ve content='...' ayrı kalıplarla: çift tırnak içindeki ' (Trendyol'da) değeri kesmez
	attrRe  = regexp.MustCompile(`(?is)([a-z][a-z0-9:_-]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	ldRe    = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// Document bir sayfanın ham HTML'i ve ortak ayrıştırma sonuçları
type Document struct {
	HTML  string
	URL   *url.URL
	Meta  map[string]string // og:title, description, product:price:amount ... (ilk geçen kazanır)
	Title string            // <title>
	LD    ldData            // schema.org JSON-LD
}

func newDocument(body string, u *url.URL) *Document {
	return &Document{
		HTML:  body,
		URL:   u,
		Meta:  parseMeta(body),
		Title: extractTitle(body),
		LD:    parseJSONLD(body),
	}
}

// cleanText HTML entity'lerini çözer (&amp; &#39; &imath; ...) ve boşlukları sadeleştirir
func cleanText(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(html.UnescapeString(s), " "))
}

// firstNonEmpty öncelik sırasına göre ilk dolu değeri döner
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseAttrs tek bir HTML etiketinin attribute'larını map olarak döner (anahtarlar küçük harf)
func parseAttrs(tag string) map[string]string {
	attrs := map[string]string{}
	for _, m := range attrRe.FindAllStringSubmatch(tag, -1) {
		key := strings.ToLower(m[1])
		if _, ok := attrs[key]; !ok {
			attrs[key] = m[2] + m[3] // eşleşmeyen grup boş string'dir
		}
	}
	return attrs
}

// parseMeta tüm <meta> etiketlerini property/name/itemprop → content map'ine çevirir
func parseMeta(body string) map[string]string {
	meta := map[string]string{}
	for _, tag := range metaTagRe.FindAllString(body, -1) {
		a := parseAttrs(tag)
		key := strings.ToLower(firstNonEmpty(a["property"], a["name"], a["itemprop"]))
		content, ok := a["content"]
		if key == "" || !ok {
			continue
		}
		if _, exists := meta[key]; !exists {
			meta[key] = cleanText(content)
		}
	}
	return meta
}

// extractTitle ilk <title> etiketini çeker (çok satırlı başlıkları da destekler)
func extractTitle(body string) string {
	if m := titleRe.FindStringSubmatch(body); len(m) > 1 {
		return cleanText(m[1])
	}
	return ""
}

// ---------- JSON-LD (schema.org) ----------

type ldData struct {
	Products []map[string]any // bulunan tüm Product nesneleri (@graph, dizi, itemReviewed dahil)
	PageName string           // WebPage.name (Hepsiburada'da Product.name'den daha eksiksiz)
}

// parseJSONLD tüm ld+json bloklarını gerçek JSON olarak ayrıştırır; bozuk blokları atlar
func parseJSONLD(body string) ldData {
	var d ldData
	for _, m := range ldRe.FindAllStringSubmatch(body, -1) {
		var v any
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &v); err != nil {
			continue
		}
		walkLD(v, &d)
	}
	return d
}

func walkLD(v any, d *ldData) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			walkLD(item, d)
		}
	case map[string]any:
		if ldHasType(t, "Product") {
			d.Products = append(d.Products, t)
		}
		if ldHasType(t, "WebPage") && d.PageName == "" {
			d.PageName = ldString(t["name"])
		}
		// Deterministik sıra için anahtarları sırala
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			walkLD(t[k], d)
		}
	}
}

func ldHasType(m map[string]any, typ string) bool {
	switch t := m["@type"].(type) {
	case string:
		return t == typ
	case []any:
		for _, x := range t {
			if s, _ := x.(string); s == typ {
				return true
			}
		}
	}
	return false
}

// ldString string / sayı / {url: ...} / dizi değerlerinden ilk anlamlı metni çıkarır
func ldString(v any) string {
	switch t := v.(type) {
	case string:
		return cleanText(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any:
		return firstNonEmpty(ldString(t["url"]), ldString(t["contentUrl"]))
	case []any:
		for _, x := range t {
			if s := ldString(x); s != "" {
				return s
			}
		}
	}
	return ""
}

// bestProduct en çok bilgi içeren Product'ı seçer (offers > image). Yorumlardaki
// itemReviewed gibi eksik Product kopyaları böylece elenir.
func (d ldData) bestProduct() map[string]any {
	var best map[string]any
	bestScore := -1
	for _, p := range d.Products {
		score := 0
		if p["offers"] != nil {
			score += 2
		}
		if p["image"] != nil {
			score++
		}
		if score > bestScore {
			best, bestScore = p, score
		}
	}
	return best
}

// ldOffer Product.offers içinden fiyat ve para birimini çıkarır (Offer, Offer[], AggregateOffer)
func ldOffer(product map[string]any) (price, currency string) {
	offer := product["offers"]
	if arr, ok := offer.([]any); ok && len(arr) > 0 {
		offer = arr[0]
	}
	o, ok := offer.(map[string]any)
	if !ok {
		return "", ""
	}
	price = firstNonEmpty(ldString(o["price"]), ldString(o["lowPrice"]))
	currency = ldString(o["priceCurrency"])
	if spec, ok := o["priceSpecification"].(map[string]any); ok {
		price = firstNonEmpty(price, ldString(spec["price"]))
		currency = firstNonEmpty(currency, ldString(spec["priceCurrency"]))
	}
	return price, currency
}

// ---------- Normalizasyon ----------

var priceCharsRe = regexp.MustCompile(`[0-9.,]+`)

// normalizePrice "1.553,94 TL", "1345.00", "699", "1553.94" → "1553.94" biçimine çevirir
func normalizePrice(raw string) string {
	s := priceCharsRe.FindString(raw)
	if s == "" {
		return ""
	}
	lastDot, lastComma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	switch {
	case lastComma > lastDot: // TR biçimi: 1.553,94
		s = strings.ReplaceAll(s, ".", "")
		s = strings.Replace(s, ",", ".", 1)
	case lastDot > lastComma && lastComma >= 0: // EN biçimi: 1,553.94
		s = strings.ReplaceAll(s, ",", "")
	case lastDot >= 0 && len(s)-lastDot-1 == 3: // "1.553" → binlik ayırıcı
		s = strings.ReplaceAll(s, ".", "")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return ""
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// isGenericImage görselin ürün resmi değil reklam/pazarlama görseli olup olmadığını kontrol eder
func isGenericImage(img string) bool {
	low := strings.ToLower(img)
	for _, k := range []string{"discovery-web", "banner", "mrktng", "/crm/", "popup", "automationimages"} {
		if strings.Contains(low, k) {
			return true
		}
	}
	return false
}

// absoluteURL göreli görsel yollarını sayfa URL'sine göre mutlak hale getirir
func absoluteURL(base *url.URL, ref string) string {
	if ref == "" || base == nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(r).String()
}

// Başlığın sonuna eklenen site/fiyat ekleri: "- Fiyatı", ": Amazon.com.tr: ...", "| Hepsiburada" ...
var titleSuffixRes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\s*[:|\-–]\s*(amazon\.com\.tr|trendyol|hepsiburada)\b.*$`),
	regexp.MustCompile(`(?i)\s*[\-–|]?\s*fiyatı.*$`),
	regexp.MustCompile(`(?i)\s+(en uygun fiyat|en ucuz).*$`),
}

// buildSearchQuery başlıktan marka/fiyat eklerini temizleyip arama sorgusu üretir
func buildSearchQuery(title string) string {
	q := title
	for _, re := range titleSuffixRes {
		q = re.ReplaceAllString(q, "")
	}
	return strings.TrimRight(strings.TrimSpace(q), ",;:-–| ")
}

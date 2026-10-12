package linkpreview

import (
	"context"
)

// Preview REST sözleşmesindeki preview-link yanıtı
// (POST /api/v1/groups/:id/ideas/preview-link → { title, description, image_url, price, final_url, search_query })
type Preview struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Price       string `json:"price"`
	Currency    string `json:"currency,omitempty"`
	FinalURL    string `json:"final_url"`
	SearchQuery string `json:"search_query"`
	Source      string `json:"source"`            // trendyol | amazon | hepsiburada | generic
	Partial     bool   `json:"partial"`           // eksik/başarısız önizleme: kullanıcı formu elle doldurur
	Warning     string `json:"warning,omitempty"` // tanı veya uyarı mesajı
}

// Extractor site bazlı ayrıştırıcı arayüzü. Her site kendi dosyasında (trendyol.go, amazon.go,
// hepsiburada.go) tanımlanır ve generic (OG + JSON-LD) sonucun üzerine yazar.
type Extractor interface {
	Name() string
	// Hosts site + kısa link domainleri. Hem SSRF allowlist'ine eklenir hem de
	// son URL'nin hangi extractor'a gideceğini belirler (alt domainler dahil).
	Hosts() []string
	// Refine generic olarak doldurulmuş Preview'u siteye özgü kaynaklarla iyileştirir
	Refine(doc *Document, p *Preview)
}

// extractors kayıtlı siteler. Yeni site = yeni dosya + buraya bir satır.
var extractors = []Extractor{
	trendyol{},
	amazon{},
	hepsiburada{},
}

// extractorFor son URL'nin host'una göre site extractor'ını bulur (yoksa nil → generic)
func extractorFor(host string) Extractor {
	for _, e := range extractors {
		if hostMatches(host, e.Hosts()) {
			return e
		}
	}
	return nil
}

// Fetch linki çeker ve önizleme üretir. Hata durumunda bile Preview döner (partial=true);
// böylece API "success: true, partial: true" yanıtı verebilir.
func Fetch(ctx context.Context, rawURL string) (*Preview, error) {
	doc, err := fetchDocument(ctx, rawURL)
	if err != nil {
		return &Preview{
			FinalURL: rawURL,
			Partial:  true,
			Source:   "generic",
			Warning:  err.Error(),
		}, err
	}

	p := &Preview{FinalURL: doc.URL.String(), Source: "generic"}
	applyGeneric(doc, p)

	if e := extractorFor(doc.URL.Hostname()); e != nil {
		p.Source = e.Name()
		e.Refine(doc, p)
	}

	finalize(doc, p)
	return p, nil
}

// finalize ortak son işlemler: normalizasyon, jenerik görsel filtresi, search_query, partial
func finalize(doc *Document, p *Preview) {
	p.Price = normalizePrice(p.Price)
	if p.Price != "" && p.Currency == "" {
		p.Currency = "TRY"
	}

	p.ImageURL = absoluteURL(doc.URL, p.ImageURL)
	if isGenericImage(p.ImageURL) {
		p.Warning = joinWarn(p.Warning, "jenerik reklam görseli atıldı: "+p.ImageURL)
		p.ImageURL = ""
	}

	if p.SearchQuery == "" {
		p.SearchQuery = buildSearchQuery(p.Title)
	}
	p.Partial = p.Title == "" || p.ImageURL == "" || p.Price == ""
}

func joinWarn(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

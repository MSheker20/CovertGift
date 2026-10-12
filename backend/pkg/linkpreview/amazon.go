package linkpreview

import (
	"regexp"
	"strings"
)

// amazon Amazon TR. Kısa link (amzn.eu/d/...) HTTP redirect ile amazon.com.tr/dp/<ASIN>'e gider.
//
// Test (2026-10-12, amzn.eu/d/08B1hxDx):
//   - og:title temiz ✅
//   - og:image ürün resmi DEĞİL: üzerine yıldız/puan basılmış 1910x1000 paylaşım kartı
//     (URL'de _SR1910,1000 / _PIRIOFOURANDHALF gibi transform parametreleri) → "hiRes" kullan
//   - Fiyat OG'de yok, JSON-LD'de offers yok → buybox JSON'u: "priceAmount":1553.94
//   - JSON-LD name içinde &imath; &scedil; gibi entity'ler var (cleanText çözüyor)
type amazon struct{}

func (amazon) Name() string { return "amazon" }

func (amazon) Hosts() []string { return []string{"amazon.com.tr", "amzn.eu", "amzn.to", "a.co"} }

var (
	// Görsel galerisindeki ilk yüksek çözünürlüklü ürün görseli
	amazonHiResRe = regexp.MustCompile(`"hiRes"\s*:\s*"(https://[^"]+)"`)
	// <img id="landingImage" data-old-hires="..."> (hiRes yoksa yedek)
	amazonLandingRe = regexp.MustCompile(`(?is)<img[^>]+id="landingImage"[^>]*>`)
	// Buybox fiyat verisi: {"displayPrice":"1.553,94 TL","priceAmount":1553.94,...}
	amazonPriceAmountRe = regexp.MustCompile(`"priceAmount"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)
	// Görünür fiyat: <span class="a-offscreen">1.553,94 TL</span>
	amazonOffscreenRe = regexp.MustCompile(`<span class="a-offscreen">\s*([0-9][^<]*)</span>`)
	// <span id="productTitle">
	amazonTitleRe = regexp.MustCompile(`(?is)<span[^>]+id="productTitle"[^>]*>(.*?)</span>`)
)

func (amazon) Refine(doc *Document, p *Preview) {
	// Bot koruması (captcha) sayfası 200 dönebilir
	if strings.Contains(doc.HTML, "/errors/validateCaptcha") {
		p.Warning = joinWarn(p.Warning, "Amazon captcha sayfası döndü")
		return
	}

	if m := amazonTitleRe.FindStringSubmatch(doc.HTML); len(m) > 1 && p.Title == "" {
		p.Title = cleanText(m[1])
	}

	// Görsel: hiRes → landingImage[data-old-hires] → (og:image paylaşım kartı, son çare)
	if m := amazonHiResRe.FindStringSubmatch(doc.HTML); len(m) > 1 {
		p.ImageURL = m[1]
	} else if tag := amazonLandingRe.FindString(doc.HTML); tag != "" {
		if hi := parseAttrs(tag)["data-old-hires"]; hi != "" {
			p.ImageURL = hi
		}
	}

	// Fiyat: generic boşsa buybox JSON → görünür a-offscreen metni
	if p.Price == "" {
		if m := amazonPriceAmountRe.FindStringSubmatch(doc.HTML); len(m) > 1 {
			p.Price = m[1]
		} else if m := amazonOffscreenRe.FindStringSubmatch(doc.HTML); len(m) > 1 {
			p.Price = m[1]
		}
	}
	if p.Price != "" && p.Currency == "" {
		p.Currency = "TRY"
	}
}

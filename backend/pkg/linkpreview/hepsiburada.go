package linkpreview

import (
	"regexp"
	"strings"
)

// hepsiburada Hepsiburada.
//
// Test (2026-10-12, ...-p-HBCV00009BDBJM):
//   - Akamai bot koruması: Accept-Encoding başlığı olmayan istek → 403 "Hepsiburada | Güvenlik".
//     Go'nun otomatik "Accept-Encoding: gzip" başlığı yeterli (fetch.go'daki nota bak).
//   - Sayfada OG etiketi YOK → her şey JSON-LD'den
//   - JSON-LD @graph[Product]: name KESİK ("...14 Uzunluk,"), tam ad WebPage.name'de
//   - Product.image: ".../s/777/375/<id>.jpg/format:webp" → 1500px JPG'ye yükseltilebilir
//   - Product.offers.price: "1345.00" TRY
//   - Product.description jenerik SEO metni ("...Hepsiburada'dan satın alın!") → atılır
type hepsiburada struct{}

func (hepsiburada) Name() string { return "hepsiburada" }

func (hepsiburada) Hosts() []string { return []string{"hepsiburada.com"} }

// productimages.hepsiburada.net/s/777/375/110001099889767.jpg/format:webp
var hbImageRe = regexp.MustCompile(`^(https://productimages\.hepsiburada\.net/s/\d+/)[^/]+(/[^/]+\.(?:jpg|jpeg|png))`)

func (hepsiburada) Refine(doc *Document, p *Preview) {
	product := doc.LD.bestProduct()

	// Başlık: WebPage.name (tam) → Product.name (kesik olabilir) → <title>
	p.Title = firstNonEmpty(doc.LD.PageName, p.Title)

	// SEO kalıp açıklamasını at; ürün hakkında bilgi vermiyor
	if strings.Contains(strings.ToLower(p.Description), "satın alın") {
		p.Description = ""
	}

	if p.ImageURL == "" {
		p.ImageURL = ldString(product["image"])
	}
	p.ImageURL = upgradeHBImage(p.ImageURL)
}

// upgradeHBImage küçük webp görseli 1500px JPG sürümüne çevirir
func upgradeHBImage(img string) string {
	if m := hbImageRe.FindStringSubmatch(img); len(m) > 2 {
		return m[1] + "1500" + m[2]
	}
	return img
}

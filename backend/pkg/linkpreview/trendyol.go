package linkpreview

import "regexp"

// trendyol paylaşım linkleri (ty.gl) Adjust tracker'ından (adj.st) geçer;
// tracker çözümlemesi fetch.go'da ortak yapılır.
//
// Test (2026-10-12, ty.gl/gkyyduf4wm8u3):
//   - og:title "... - Fiyatı" ekiyle gelir; JSON-LD WebPage.name ekisiz ve temiz
//   - og:image ürün sayfasında doğru (tracker sayfasındakiler jenerik)
//   - JSON-LD'de Product YOK (yalnızca WebPage) → fiyat gömülü state JSON'undan:
//     "winnerVariant":{..."price":{"currency":"TRY","discountedPrice":{"value":699,...}}}
//     (06.10'daki prototip fiyatı sayfadaki ilk "price" eşleşmesinden tesadüfen alıyordu)
type trendyol struct{}

func (trendyol) Name() string { return "trendyol" }

func (trendyol) Hosts() []string { return []string{"ty.gl", "trendyol.com"} }

var (
	// İndirimli (sepette görünen) fiyat: önce bu
	tyDiscountedRe = regexp.MustCompile(`"discountedPrice"\s*:\s*\{\s*"value"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)
	// Varyant fiyatı: "price":{"value":699,"text":"699 TL"}
	tyVariantPriceRe = regexp.MustCompile(`"price"\s*:\s*\{\s*"value"\s*:\s*([0-9]+(?:\.[0-9]+)?)`)
)

func (trendyol) Refine(doc *Document, p *Preview) {
	// Başlık: WebPage.name "- Fiyatı" eki içermiyor
	p.Title = firstNonEmpty(doc.LD.PageName, p.Title)

	// og:image bazen kampanya görseli olabiliyor; JSON-LD ürün görseli yedek
	if p.ImageURL == "" || isGenericImage(p.ImageURL) {
		if img := ldString(doc.LD.bestProduct()["image"]); img != "" && !isGenericImage(img) {
			p.ImageURL = img
		}
	}

	// Fiyat: generic (OG / JSON-LD Offer) boşsa gömülü state JSON'u
	if p.Price == "" {
		for _, re := range []*regexp.Regexp{tyDiscountedRe, tyVariantPriceRe} {
			if m := re.FindStringSubmatch(doc.HTML); len(m) > 1 {
				p.Price = m[1]
				break
			}
		}
	}
}

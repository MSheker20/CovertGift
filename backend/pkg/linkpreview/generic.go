package linkpreview

// applyGeneric kayıtlı olmayan siteler için de çalışan temel çıkarım.
// Öncelik sırası PROJECT_CONTEXT → İş Kuralı 5'e göre:
//
//	Başlık:   og:title → JSON-LD Product.name → <title>
//	Açıklama: og:description → JSON-LD Product.description → meta description
//	Görsel:   og:image → JSON-LD Product.image
//	Fiyat:    product:price:amount → JSON-LD Offer.price
func applyGeneric(doc *Document, p *Preview) {
	product := doc.LD.bestProduct()
	ldPrice, ldCurrency := ldOffer(product)

	p.Title = firstNonEmpty(doc.Meta["og:title"], ldString(product["name"]), doc.Title)
	p.Description = firstNonEmpty(doc.Meta["og:description"], ldString(product["description"]), doc.Meta["description"])
	p.ImageURL = firstNonEmpty(doc.Meta["og:image"], ldString(product["image"]))
	p.Price = firstNonEmpty(doc.Meta["product:price:amount"], ldPrice)
	p.Currency = firstNonEmpty(doc.Meta["product:price:currency"], ldCurrency)
}

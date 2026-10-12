package linkpreview

import (
	"context"
	"net/url"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{"Valid Trendyol short link", "https://ty.gl/gkyyduf4wm8u3", false},
		{"Valid Trendyol full link", "https://www.trendyol.com/brand/product-p-123", false},
		{"Valid Amazon short link", "https://amzn.eu/d/08B1hxDx", false},
		{"Valid Amazon TR link", "https://www.amazon.com.tr/dp/B07TEST123", false},
		{"Valid Hepsiburada link", "https://www.hepsiburada.com/urun-p-HBCV00009BDBJM", false},
		{"Valid Adjust tracker link", "https://app.adj.st/product?adjust_redirect=https://ty.gl", false},
		{"SSRF Block: Localhost IP", "http://127.0.0.1:8080/admin", true},
		{"SSRF Block: Localhost hostname", "http://localhost:3000/api", true},
		{"SSRF Block: External untrusted domain", "https://example.com/test", true},
		{"SSRF Block: Evil attacker site", "https://evil-site.com/exploit", true},
		{"Invalid Scheme: FTP", "ftp://ty.gl/gkyyduf4wm8u3", true},
		{"Invalid Scheme: File", "file:///etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("url.Parse(%q) error: %v", tt.rawURL, err)
			}
			err = validateURL(u)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURL(%q) err = %v, wantErr = %v", tt.rawURL, err, tt.wantErr)
			}
		})
	}
}

func TestBlockPrivateIPs(t *testing.T) {
	privateAddresses := []string{
		"127.0.0.1:80",
		"10.0.0.1:443",
		"192.168.1.1:8080",
		"172.16.0.1:80",
		"169.254.169.254:80", // AWS/Cloud metadata service IP
		"[::1]:80",
	}

	for _, addr := range privateAddresses {
		t.Run("Block "+addr, func(t *testing.T) {
			if err := blockPrivateIPs("", addr, nil); err == nil {
				t.Errorf("expected blockPrivateIPs to fail for private IP %q, got nil", addr)
			}
		})
	}
}

func TestNormalizePrice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.553,94 TL", "1553.94"},
		{"1345.00", "1345.00"},
		{"699", "699.00"},
		{"699.00 TL", "699.00"},
		{"1,553.94", "1553.94"},
		{" 2.499,50 ₺ ", "2499.50"},
		{"0", ""},
		{"invalid", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizePrice(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePrice(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"Xiaomi Band 8 Akıllı Bileklik - Fiyatı", "Xiaomi Band 8 Akıllı Bileklik"},
		{"Logitech G29 Driving Force Yarış Direksiyonu : Amazon.com.tr: Bilgisayar", "Logitech G29 Driving Force Yarış Direksiyonu"},
		{"Philips HD7461/20 Filtre Kahve Makinesi | Hepsiburada", "Philips HD7461/20 Filtre Kahve Makinesi"},
		{"Apple AirPods Pro 2. Nesil en uygun fiyat", "Apple AirPods Pro 2. Nesil"},
		{"Sony WH-1000XM5 Kulaklık", "Sony WH-1000XM5 Kulaklık"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := buildSearchQuery(tt.title)
			if got != tt.expected {
				t.Errorf("buildSearchQuery(%q) = %q, want %q", tt.title, got, tt.expected)
			}
		})
	}
}

func TestMetaSingleQuoteAndEntityDecode(t *testing.T) {
	htmlBody := `
		<html>
		<head>
			<meta property="og:title" content="Sony WH-1000XM5 Kulaklık &amp; Aksesuar">
			<meta property="og:description" content="Trendyol&#39;da en uygun fiyata satın alın &amp; hızlı kargo avantajını yakalayın!">
			<meta property="og:image" content="https://cdn.example.com/product.jpg">
		</head>
		</html>
	`

	u, _ := url.Parse("https://www.trendyol.com/sony-p-1")
	doc := newDocument(htmlBody, u)

	if doc.Meta["og:title"] != "Sony WH-1000XM5 Kulaklık & Aksesuar" {
		t.Errorf("unexpected og:title: %q", doc.Meta["og:title"])
	}
	expectedDesc := "Trendyol'da en uygun fiyata satın alın & hızlı kargo avantajını yakalayın!"
	if doc.Meta["og:description"] != expectedDesc {
		t.Errorf("unexpected og:description: %q, want %q", doc.Meta["og:description"], expectedDesc)
	}
}

func TestSiteRefinementsOffline(t *testing.T) {
	// 1. Trendyol Refine Test
	t.Run("Trendyol Refine", func(t *testing.T) {
		mockHTML := `
			<html>
			<head>
				<title>Trendyol Ürün Başlığı - Fiyatı</title>
				<meta property="og:title" content="Trendyol Ürün Başlığı - Fiyatı">
				<meta property="og:image" content="https://cdn.dsmcdn.com/product_main.jpg">
				<script type="application/ld+json">
				{
					"@context": "https://schema.org",
					"@type": "WebPage",
					"name": "Trendyol Temiz Ürün Başlığı"
				}
				</script>
			</head>
			<body>
				<script>
					window.__PRODUCT_DETAIL_APP_INITIAL_STATE__ = {
						"product": {
							"winnerVariant": {
								"price": {
									"currency": "TRY",
									"discountedPrice": { "value": 699.00 }
								}
							}
						}
					};
				</script>
			</body>
			</html>
		`
		u, _ := url.Parse("https://www.trendyol.com/test-p-123")
		doc := newDocument(mockHTML, u)
		p := &Preview{FinalURL: u.String(), Source: "generic"}
		applyGeneric(doc, p)

		ty := trendyol{}
		ty.Refine(doc, p)
		finalize(doc, p)

		if p.Title != "Trendyol Temiz Ürün Başlığı" {
			t.Errorf("expected clean title from WebPage.name, got %q", p.Title)
		}
		if p.Price != "699.00" {
			t.Errorf("expected price 699.00, got %q", p.Price)
		}
		if p.Partial {
			t.Errorf("expected partial=false, got true")
		}
	})

	// 2. Amazon Refine Test
	t.Run("Amazon Refine", func(t *testing.T) {
		mockHTML := `
			<html>
			<head>
				<meta property="og:title" content="Amazon Ürün Başlığı">
				<meta property="og:image" content="https://m.media-amazon.com/images/I/share_card_rating.jpg">
			</head>
			<body>
				<script>
					var obj = {
						"hiRes": "https://m.media-amazon.com/images/I/high_res_photo_1500.jpg",
						"priceAmount": 1553.94
					};
				</script>
			</body>
			</html>
		`
		u, _ := url.Parse("https://www.amazon.com.tr/dp/B07XYZ")
		doc := newDocument(mockHTML, u)
		p := &Preview{FinalURL: u.String(), Source: "generic"}
		applyGeneric(doc, p)

		amz := amazon{}
		amz.Refine(doc, p)
		finalize(doc, p)

		if p.ImageURL != "https://m.media-amazon.com/images/I/high_res_photo_1500.jpg" {
			t.Errorf("expected hiRes image, got %q", p.ImageURL)
		}
		if p.Price != "1553.94" {
			t.Errorf("expected price 1553.94, got %q", p.Price)
		}
	})

	// 3. Hepsiburada Refine Test
	t.Run("Hepsiburada Refine", func(t *testing.T) {
		mockHTML := `
			<html>
			<head>
				<script type="application/ld+json">
				{
					"@context": "https://schema.org",
					"@graph": [
						{
							"@type": "WebPage",
							"name": "Hepsiburada Tam Ürün Adı 14 cm"
						},
						{
							"@type": "Product",
							"name": "Hepsiburada Kesik Adı...",
							"image": "https://productimages.hepsiburada.net/s/777/375/110001099889767.jpg/format:webp",
							"offers": {
								"@type": "Offer",
								"price": "1345.00",
								"priceCurrency": "TRY"
							}
						}
					]
				}
				</script>
			</head>
			<body></body>
			</html>
		`
		u, _ := url.Parse("https://www.hepsiburada.com/test-p-HBCV123")
		doc := newDocument(mockHTML, u)
		p := &Preview{FinalURL: u.String(), Source: "generic"}
		applyGeneric(doc, p)

		hb := hepsiburada{}
		hb.Refine(doc, p)
		finalize(doc, p)

		if p.Title != "Hepsiburada Tam Ürün Adı 14 cm" {
			t.Errorf("expected title from WebPage.name, got %q", p.Title)
		}
		expectedImg := "https://productimages.hepsiburada.net/s/777/1500/110001099889767.jpg"
		if p.ImageURL != expectedImg {
			t.Errorf("expected upgraded 1500px image %q, got %q", expectedImg, p.ImageURL)
		}
		if p.Price != "1345.00" {
			t.Errorf("expected price 1345.00, got %q", p.Price)
		}
	})

	// 4. Generic Ad Banner Filter Test
	t.Run("Ad Banner Filter", func(t *testing.T) {
		mockHTML := `
			<html>
			<head>
				<meta property="og:title" content="Örnek Ürün">
				<meta property="og:image" content="https://cdn.example.com/mrktng/banner_ad_campaign.jpg">
				<meta property="product:price:amount" content="250">
			</head>
			</html>
		`
		u, _ := url.Parse("https://www.trendyol.com/test")
		doc := newDocument(mockHTML, u)
		p := &Preview{FinalURL: u.String(), Source: "generic"}
		applyGeneric(doc, p)
		finalize(doc, p)

		if p.ImageURL != "" {
			t.Errorf("expected ad banner image to be stripped, got %q", p.ImageURL)
		}
		if !p.Partial {
			t.Errorf("expected partial=true since image was stripped, got false")
		}
	})
}

func TestFetchSSRFBlockedGraceful(t *testing.T) {
	// Fetching an SSRF target should return Partial: true and an error, not panic
	p, err := Fetch(context.Background(), "http://127.0.0.1:9999/private")
	if err == nil {
		t.Errorf("expected SSRF error, got nil")
	}
	if p == nil || !p.Partial {
		t.Errorf("expected non-nil preview with partial=true, got %+v", p)
	}
}

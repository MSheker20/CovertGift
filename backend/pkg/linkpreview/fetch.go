package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// İstek kimliği ve güvenlik limitleri (PROJECT_CONTEXT → İş Kuralı 5)
const (
	userAgent      = "WhatsApp/2.21.12.21 A"
	acceptLanguage = "tr-TR,tr;q=0.9,en-US;q=0.8,en;q=0.7"
	requestTimeout = 10 * time.Second
	maxBodyBytes   = 2 << 20 // 2 MB
	maxRedirects   = 10
)

// trackerHosts gerçek hedefi sorgu parametresinde saklayan ara (takip) sayfaları
var trackerHosts = []string{"adj.st", "adjust.com"}

// allowedHosts SSRF allowlist'i: tracker'lar + kayıtlı tüm sitelerin domainleri
func allowedHosts() []string {
	hosts := append([]string{}, trackerHosts...)
	for _, e := range extractors {
		hosts = append(hosts, e.Hosts()...)
	}
	return hosts
}

// hostMatches host'un listedeki bir domain'in kendisi veya alt domaini olup olmadığını kontrol eder
func hostMatches(host string, domains []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// validateURL sadece http/https ve allowlist'teki hostlara izin verir
func validateURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("desteklenmeyen şema: %q", u.Scheme)
	}
	if !hostMatches(u.Hostname(), allowedHosts()) {
		return fmt.Errorf("izin verilmeyen alan adı: %s", u.Hostname())
	}
	return nil
}

// blockPrivateIPs bağlantı anında (DNS çözümlendikten sonra) özel/yerel IP'leri reddeder.
// Dial seviyesinde kontrol, DNS rebinding saldırılarına karşı da korur.
func blockPrivateIPs(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("özel/yerel IP adresine bağlantı engellendi: %s", host)
	}
	return nil
}

// newClient çerez kavanozu, redirect limiti ve SSRF korumalı HTTP istemcisi oluşturur
func newClient() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: blockPrivateIPs}
	transport := &http.Transport{
		Proxy:               nil, // ortam proxy'si allowlist'i atlatmasın
		DialContext:         dialer.DialContext,
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:       jar,
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("çok fazla yönlendirme (>%d)", maxRedirects)
			}
			if err := validateURL(req.URL); err != nil {
				return err
			}
			slog.Debug("linkpreview yönlendirme", "host", req.URL.Host, "path", req.URL.Path)
			return nil
		},
	}
}

// fetch verilen URL'yi çeker; HTML gövdesini ve son varılan URL'yi döner.
//
// NOT: Accept-Encoding başlığı bilerek elle set EDİLMEZ. Go bu durumda otomatik
// "Accept-Encoding: gzip" ekler ve yanıtı şeffafça açar. Hepsiburada (Akamai)
// Accept-Encoding içermeyen istekleri 403 ile engelliyor.
func fetch(ctx context.Context, client *http.Client, targetURL string) (string, *url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", acceptLanguage)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("bağlantı hatası: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("linkpreview yanıt", "status", resp.Status, "host", resp.Request.URL.Host)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", resp.Request.URL, fmt.Errorf("beklenmeyen HTTP durumu: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", nil, fmt.Errorf("sayfa içeriği okunamadı: %w", err)
	}
	return string(body), resp.Request.URL, nil
}

// resolveTrackerRedirect Adjust gibi takip sayfalarında gömülü gerçek hedef URL'yi bulur
func resolveTrackerRedirect(u *url.URL) string {
	if u == nil || !hostMatches(u.Hostname(), trackerHosts) {
		return ""
	}
	return u.Query().Get("adjust_redirect") // Query().Get zaten URL decode eder
}

// fetchDocument linki çeker, gerekirse tracker'ı çözer ve ayrıştırılmış Document döner
func fetchDocument(ctx context.Context, rawURL string) (*Document, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("geçersiz URL: %w", err)
	}
	if err := validateURL(u); err != nil {
		return nil, err
	}

	client := newClient()
	body, finalURL, err := fetch(ctx, client, u.String())
	if err != nil {
		return nil, err
	}

	// Takip (tracker) sayfasında kaldıysak gerçek ürün sayfasına ikinci istek at
	if real := resolveTrackerRedirect(finalURL); real != "" {
		slog.Debug("linkpreview takip sayfası çözüldü", "tracker_host", finalURL.Host, "target", real)
		ru, err := url.Parse(real)
		if err != nil {
			return nil, fmt.Errorf("tracker hedefi geçersiz: %w", err)
		}
		if err := validateURL(ru); err != nil {
			return nil, fmt.Errorf("tracker hedefi reddedildi: %w", err)
		}
		body, finalURL, err = fetch(ctx, client, ru.String())
		if err != nil {
			return nil, err
		}
	}

	if finalURL == nil {
		return nil, errors.New("son URL belirlenemedi")
	}

	// Hata ayıklama / test fixture'ı: LINKPREVIEW_DUMP=<klasör> ise HTML'i kaydet
	if dir := os.Getenv("LINKPREVIEW_DUMP"); dir != "" {
		path := filepath.Join(dir, finalURL.Hostname()+".html")
		if err := os.WriteFile(path, []byte(body), 0o644); err == nil {
			slog.Debug("linkpreview HTML kaydedildi", "path", path)
		}
	}
	return newDocument(body, finalURL), nil
}

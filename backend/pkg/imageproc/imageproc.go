// Package imageproc kullanıcıdan gelen görselleri güvenli ve küçük bir JPEG'e çevirir.
//
// Grup kapak fotoğrafları (4:3) ve profil fotoğrafları (kare) için kullanılır: tür bayt imzasından doğrulanır, çözünürlük
// çözmeden ÖNCE sınırlanır (decompression bomb), EXIF yönü uygulanır, görsel ortadan 4:3
// kırpılır, en çok 1600x1200'e küçültülür ve yeniden JPEG kodlanır. Yeniden kodlama tüm
// metadata'yı (EXIF, konum, renk profili, yorumlar) atar. Yalnızca saf Go; cgo yok.
//
// Büyük fotoğrafı küçültmek tarayıcının işidir (arayüz 4:3 kırpıp en çok 1600x1200 JPEG
// gönderir). Sunucu yalnızca küçük girdi kabul eder (en çok 2 MiB, uzun kenar 2000 piksel,
// 4 megapiksel); böylece tek bir görselin tepe belleği birkaç on MB'ı geçmez.
package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png" // PNG çözücüsünü kaydeder

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // WebP çözücüsünü kaydeder
)

const (
	// MaxInputBytes kabul edilen en büyük dosyadır (2 MiB). Tarayıcıda küçültülmüş
	// 1600x1200 JPEG bunun çok altında kalır.
	MaxInputBytes = 2 << 20
	// MaxDimension uzun kenarın, MaxPixels toplam piksel sayısının üst sınırıdır. Sınırlar
	// image.DecodeConfig ile başlıktan okunur; aşan görsel hiç çözülmez.
	MaxDimension = 2000
	MaxPixels    = 4_000_000
	// MinWidth ve MinHeight, 4:3 kırpmanın anlamlı kalması için en küçük kaynak boyutudur
	// (yön uygulandıktan sonra).
	MinWidth  = 64
	MinHeight = 48
	// CoverMaxWidth ve CoverMaxHeight çıktının üst sınırıdır (4:3).
	CoverMaxWidth  = 1600
	CoverMaxHeight = 1200
	// AvatarMaxSize profil fotoğrafının (kare) kenar üst sınırı, AvatarMinSize kaynakta
	// kırpılacak karenin en küçük kenarıdır (yön uygulandıktan sonra).
	AvatarMaxSize = 512
	AvatarMinSize = 128
	// JPEGQuality çıktı kalitesidir.
	JPEGQuality = 80
	// OutputContentType çıktının içerik türüdür.
	OutputContentType = "image/jpeg"
)

// Hatalar. Mesajlar kullanıcıya gösterilir.
var (
	ErrTooLarge          = errors.New("Fotoğraf en fazla 2 MB olabilir. Lütfen tarayıcıda küçültülmüş bir görsel yükleyin.")
	ErrUnsupportedType   = errors.New("yalnızca JPEG, PNG veya WebP görseller desteklenir")
	ErrHEICNotSupported  = errors.New("HEIC/HEIF görseller desteklenmiyor; JPEG, PNG veya WebP yükleyin")
	ErrCorrupt           = errors.New("görsel okunamadı; dosya bozuk olabilir")
	ErrDimensionsTooBig  = errors.New("Fotoğraf çok büyük. Lütfen tarayıcıda küçültülmüş (en çok 2000 piksel) bir görsel yükleyin.")
	ErrDimensionsTooTiny = errors.New("görsel çok küçük; en az 64x48 piksel olmalı")
	ErrAvatarTooTiny     = errors.New("görsel çok küçük; profil fotoğrafı en az 128x128 piksel olmalı")
)

// Result işlenmiş görseldir.
type Result struct {
	Data        []byte
	ContentType string
	Width       int
	Height      int
}

// Format bayt imzasından tanınan görsel türüdür.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatWebP Format = "webp"
)

// Sniff görselin türünü bayt imzasından belirler; dosya adı ve Content-Type dikkate alınmaz.
func Sniff(data []byte) (Format, error) {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return FormatJPEG, nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return FormatPNG, nil
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return FormatWebP, nil
	case len(data) >= 12 && string(data[4:8]) == "ftyp":
		switch string(data[8:12]) {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "heif", "avif", "avis":
			return "", ErrHEICNotSupported
		}
	}
	return "", ErrUnsupportedType
}

// spec bir çıktı biçimidir: oran (aw:ah), çıktı genişliğinin üst sınırı ve kaynakta
// kabul edilen en küçük boyut.
type spec struct {
	aw, ah     int
	maxWidth   int
	minW, minH int
	tooTiny    error
}

var (
	coverSpec  = spec{aw: 4, ah: 3, maxWidth: CoverMaxWidth, minW: MinWidth, minH: MinHeight, tooTiny: ErrDimensionsTooTiny}
	avatarSpec = spec{aw: 1, ah: 1, maxWidth: AvatarMaxSize, minW: AvatarMinSize, minH: AvatarMinSize, tooTiny: ErrAvatarTooTiny}
)

// ProcessCover ham yüklemeyi 4:3, en çok 1600x1200, metadata'sız bir JPEG'e çevirir.
func ProcessCover(data []byte) (*Result, error) { return process(data, coverSpec) }

// ProcessAvatar ham yüklemeyi kare, en çok 512x512, metadata'sız bir JPEG'e çevirir. Tarayıcı
// zaten kare kırpar; sunucu aynı kuralı savunma olarak yeniden uygular (ortadan kare).
func ProcessAvatar(data []byte) (*Result, error) { return process(data, avatarSpec) }

func process(data []byte, sp spec) (*Result, error) {
	if len(data) > MaxInputBytes {
		return nil, ErrTooLarge
	}
	format, err := Sniff(data)
	if err != nil {
		return nil, err
	}

	// Çözmeden önce yalnızca başlık okunur: bildirilen boyut sınırı aşıyorsa piksel
	// belleği hiç ayrılmaz.
	cfg, decoded, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || decoded != string(format) {
		return nil, ErrCorrupt
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrCorrupt
	}
	if cfg.Width > MaxDimension || cfg.Height > MaxDimension || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, ErrDimensionsTooBig
	}

	orientation := 1
	switch format {
	case FormatJPEG:
		orientation = jpegOrientation(data)
	case FormatWebP:
		orientation = webpOrientation(data)
	}
	transposed := orientation >= 5 // 5–8: genişlik ve yükseklik yer değiştirir

	shownW, shownH := cfg.Width, cfg.Height
	if transposed {
		shownW, shownH = shownH, shownW
	}
	cropW, cropH := centeredCrop(shownW, shownH, sp.aw, sp.ah)
	if shownW < sp.minW || shownH < sp.minH || cropW < sp.minW || cropH < sp.minH {
		return nil, sp.tooTiny
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrCorrupt
	}
	// Başlık ile çözülen görsel uyuşmuyorsa dosyaya güvenilmez.
	if src.Bounds().Dx() != cfg.Width || src.Bounds().Dy() != cfg.Height {
		return nil, ErrCorrupt
	}

	// Gösterilen (yönü uygulanmış) görselde ortadan oranlı kırpma ve çıktı boyutu. Genişlik
	// oranın katına yuvarlanır: yükseklik tam sayı ve oran birebir kalır.
	outW := min(cropW, sp.maxWidth) / sp.aw * sp.aw
	outH := outW / sp.aw * sp.ah

	// Ölçekleme kaynak yönünde yapılır, yön küçük çıktıya uygulanır (daha az bellek).
	srcCropW, srcCropH, scaledW, scaledH := cropW, cropH, outW, outH
	if transposed {
		srcCropW, srcCropH, scaledW, scaledH = cropH, cropW, outH, outW
	}
	b := src.Bounds()
	cropRect := image.Rect(0, 0, srcCropW, srcCropH).Add(image.Pt(
		b.Min.X+(b.Dx()-srcCropW)/2,
		b.Min.Y+(b.Dy()-srcCropH)/2,
	))

	// Beyaz zemin: PNG/WebP şeffaflığı JPEG'de siyaha dönmesin diye Over ile yaylandırılır.
	scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
	draw.Draw(scaled, scaled.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, cropRect, draw.Over, nil)

	final := applyOrientation(scaled, orientation)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, final, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, err
	}
	return &Result{
		Data:        buf.Bytes(),
		ContentType: OutputContentType,
		Width:       final.Bounds().Dx(),
		Height:      final.Bounds().Dy(),
	}, nil
}

// centeredCrop w x h içine sığan en büyük aw:ah alanın boyutlarını döner.
func centeredCrop(w, h, aw, ah int) (int, int) {
	if w*ah > h*aw { // fazla geniş: yüksekliği koru
		return h * aw / ah, h
	}
	return w, w * ah / aw // fazla uzun (ya da tam oran): genişliği koru
}

// applyOrientation EXIF yön değerine (1–8) göre görseli döndürür/çevirir.
func applyOrientation(src *image.RGBA, orientation int) *image.RGBA {
	if orientation <= 1 || orientation > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2: // yatay ayna
				dx, dy = w-1-x, y
			case 3: // 180°
				dx, dy = w-1-x, h-1-y
			case 4: // dikey ayna
				dx, dy = x, h-1-y
			case 5: // köşegen ayna (transpose)
				dx, dy = y, x
			case 6: // saat yönünde 90°
				dx, dy = h-1-y, x
			case 7: // ters köşegen ayna (transverse)
				dx, dy = h-1-y, w-1-x
			case 8: // saat yönünün tersine 90°
				dx, dy = y, w-1-x
			}
			si := src.PixOffset(x, y)
			di := dst.PixOffset(dx, dy)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

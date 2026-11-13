// Package imageproc kullanıcıdan gelen görselleri güvenli ve küçük bir JPEG'e çevirir.
//
// Grup kapak fotoğrafları için kullanılır: tür bayt imzasından doğrulanır, çözünürlük
// çözmeden ÖNCE sınırlanır (decompression bomb), EXIF yönü uygulanır, görsel ortadan 4:3
// kırpılır, en çok 1600x1200'e küçültülür ve yeniden JPEG kodlanır. Yeniden kodlama tüm
// metadata'yı (EXIF, konum, renk profili, yorumlar) atar. Yalnızca saf Go; cgo yok.
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
	// MaxInputBytes kabul edilen en büyük dosyadır (10 MiB).
	MaxInputBytes = 10 << 20
	// MaxDimension tek kenarın, MaxPixels toplam piksel sayısının üst sınırıdır. Sınırlar
	// başlıktan okunur; aşan görsel hiç çözülmez.
	MaxDimension = 12000
	MaxPixels    = 50_000_000
	// MinWidth ve MinHeight, 4:3 kırpmanın anlamlı kalması için en küçük kaynak boyutudur
	// (yön uygulandıktan sonra).
	MinWidth  = 64
	MinHeight = 48
	// CoverMaxWidth ve CoverMaxHeight çıktının üst sınırıdır (4:3).
	CoverMaxWidth  = 1600
	CoverMaxHeight = 1200
	// JPEGQuality çıktı kalitesidir.
	JPEGQuality = 80
	// OutputContentType çıktının içerik türüdür.
	OutputContentType = "image/jpeg"
)

// Hatalar. Mesajlar kullanıcıya gösterilir.
var (
	ErrTooLarge          = errors.New("görsel en fazla 10 MB olabilir")
	ErrUnsupportedType   = errors.New("yalnızca JPEG, PNG veya WebP görseller desteklenir")
	ErrHEICNotSupported  = errors.New("HEIC/HEIF görseller desteklenmiyor; JPEG, PNG veya WebP yükleyin")
	ErrCorrupt           = errors.New("görsel okunamadı; dosya bozuk olabilir")
	ErrDimensionsTooBig  = errors.New("görselin çözünürlüğü çok yüksek (en fazla 12000 piksel kenar ve 50 megapiksel)")
	ErrDimensionsTooTiny = errors.New("görsel çok küçük; en az 64x48 piksel olmalı")
)

// Result işlenmiş kapak görselidir.
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

// ProcessCover ham yüklemeyi 4:3, en çok 1600x1200, metadata'sız bir JPEG'e çevirir.
func ProcessCover(data []byte) (*Result, error) {
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
	if shownW < MinWidth || shownH < MinHeight {
		return nil, ErrDimensionsTooTiny
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrCorrupt
	}
	// Başlık ile çözülen görsel uyuşmuyorsa dosyaya güvenilmez.
	if src.Bounds().Dx() != cfg.Width || src.Bounds().Dy() != cfg.Height {
		return nil, ErrCorrupt
	}

	// Gösterilen (yönü uygulanmış) görselde ortadan 4:3 kırpma ve çıktı boyutu.
	cropW, cropH := centeredCrop(shownW, shownH)
	outW := min(cropW, CoverMaxWidth) &^ 3 // 4'ün katı: yükseklik tam 3/4 olur
	outH := outW / 4 * 3

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

// centeredCrop w x h içine sığan en büyük 4:3 alanın boyutlarını döner.
func centeredCrop(w, h int) (int, int) {
	if w*3 > h*4 { // fazla geniş: yüksekliği koru
		return h * 4 / 3, h
	}
	return w, w * 3 / 4 // fazla uzun (ya da tam 4:3): genişliği koru
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

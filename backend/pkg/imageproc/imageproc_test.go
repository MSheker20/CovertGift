package imageproc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

var (
	red    = color.RGBA{R: 220, G: 20, B: 20, A: 255}
	green  = color.RGBA{R: 20, G: 200, B: 20, A: 255}
	blue   = color.RGBA{R: 20, G: 20, B: 220, A: 255}
	yellow = color.RGBA{R: 230, G: 220, B: 20, A: 255}
)

// fill w x h boyutunda, pikselleri at(x, y) ile boyanmış bir görsel üretir.
func fill(w, h int, at func(x, y int) color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, at(x, y))
		}
	}
	return img
}

// photo sıkıştırması gerçek fotoğrafa benzeyen (yumuşak geçiş + doku) bir görsel üretir.
func photo(w, h int) *image.RGBA {
	return fill(w, h, func(x, y int) color.RGBA {
		n := uint8((x*7 + y*13) % 23)
		return color.RGBA{R: uint8(x*255/w) + n, G: uint8(y*255/h) + n, B: uint8((x+y)*255/(w+h)) + n, A: 255}
	})
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type byteOrder interface {
	binary.ByteOrder
	binary.AppendByteOrder
}

// withEXIF JPEG'in SOI'sinden hemen sonra yön etiketi ve sahte GPS verisi taşıyan bir
// APP1/EXIF bölümü ekler.
func withEXIF(t *testing.T, jpg []byte, orientation uint16, order byteOrder) []byte {
	t.Helper()
	tiff := make([]byte, 0, 96)
	if order == binary.LittleEndian {
		tiff = append(tiff, 'I', 'I')
	} else {
		tiff = append(tiff, 'M', 'M')
	}
	tiff = order.AppendUint16(tiff, 42)
	tiff = order.AppendUint32(tiff, 8) // IFD0 ofseti
	tiff = order.AppendUint16(tiff, 2) // 2 kayıt
	// GPS IFD işaretçisi (0x8825, LONG)
	tiff = order.AppendUint16(tiff, 0x8825)
	tiff = order.AppendUint16(tiff, 4)
	tiff = order.AppendUint32(tiff, 1)
	tiff = order.AppendUint32(tiff, 38)
	// Yön (0x0112, SHORT)
	tiff = order.AppendUint16(tiff, 0x0112)
	tiff = order.AppendUint16(tiff, 3)
	tiff = order.AppendUint32(tiff, 1)
	tiff = order.AppendUint16(tiff, orientation)
	tiff = order.AppendUint16(tiff, 0)
	tiff = order.AppendUint32(tiff, 0) // sonraki IFD yok
	tiff = append(tiff, []byte("GPSLatitude=41.0082;GPSLongitude=28.9784")...)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1}
	segment = binary.BigEndian.AppendUint16(segment, uint16(len(payload)+2))
	segment = append(segment, payload...)

	out := append([]byte{}, jpg[:2]...)
	out = append(out, segment...)
	return append(out, jpg[2:]...)
}

func mustProcess(t *testing.T, data []byte) (*Result, image.Image) {
	t.Helper()
	res, err := ProcessCover(data)
	if err != nil {
		t.Fatalf("ProcessCover: %v", err)
	}
	if res.ContentType != "image/jpeg" {
		t.Fatalf("içerik türü %q", res.ContentType)
	}
	img, format, err := image.Decode(bytes.NewReader(res.Data))
	if err != nil || format != "jpeg" {
		t.Fatalf("çıktı JPEG değil: %v (%s)", err, format)
	}
	if img.Bounds().Dx() != res.Width || img.Bounds().Dy() != res.Height {
		t.Fatalf("bildirilen boyut %dx%d, gerçek %v", res.Width, res.Height, img.Bounds())
	}
	return res, img
}

// near JPEG kaybını hoş görerek iki rengin yakın olduğunu söyler.
func near(got color.Color, want color.RGBA) bool {
	r, g, b, _ := got.RGBA()
	d := func(a uint32, b uint8) int {
		v := int(a>>8) - int(b)
		if v < 0 {
			v = -v
		}
		return v
	}
	return d(r, want.R) < 40 && d(g, want.G) < 40 && d(b, want.B) < 40
}

func TestProcessCoverSizeAndRatio(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"büyük 4:3 küçültülür", 4000, 3000, 1600, 1200},
		{"16:9 ortadan kırpılır", 1920, 1080, 1440, 1080},
		{"dikey görsel kırpılır", 1000, 2000, 1000, 750},
		{"küçük görsel büyütülmez", 400, 300, 400, 300},
		{"çok geniş panorama", 6000, 1000, 1332, 999},
		{"kare", 2400, 2400, 1600, 1200},
		{"4'e bölünmeyen genişlik", 801, 700, 800, 600},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, _ := mustProcess(t, encodeJPEG(t, photo(tc.w, tc.h)))
			if res.Width != tc.wantW || res.Height != tc.wantH {
				t.Fatalf("%dx%d, beklenen %dx%d", res.Width, res.Height, tc.wantW, tc.wantH)
			}
			if res.Width*3 != res.Height*4 {
				t.Fatalf("çıktı 4:3 değil: %dx%d", res.Width, res.Height)
			}
			if res.Width > CoverMaxWidth || res.Height > CoverMaxHeight {
				t.Fatalf("çıktı sınırı aşıyor: %dx%d", res.Width, res.Height)
			}
		})
	}
}

func TestProcessCoverOutputIsSmall(t *testing.T) {
	res, _ := mustProcess(t, encodeJPEG(t, photo(4000, 3000)))
	if len(res.Data) > 600<<10 {
		t.Fatalf("1600x1200 çıktı %d KB; 600 KB altında olmalı", len(res.Data)>>10)
	}
}

func TestProcessCoverCropsFromCenter(t *testing.T) {
	// 400x100: kenarlar kırmızı, orta bant (100–300) yeşil. 4:3 kırpma 133 piksellik orta
	// bölümü alır; kenarlardaki kırmızı çıktıda olmamalı.
	src := fill(400, 100, func(x, _ int) color.RGBA {
		if x >= 100 && x < 300 {
			return green
		}
		return red
	})
	_, img := mustProcess(t, encodePNG(t, src))
	b := img.Bounds()
	for _, p := range []image.Point{{2, 2}, {b.Dx() - 3, 2}, {b.Dx() / 2, b.Dy() / 2}, {2, b.Dy() - 3}} {
		if !near(img.At(p.X, p.Y), green) {
			t.Fatalf("%v noktasında yeşil bekleniyordu, %v", p, img.At(p.X, p.Y))
		}
	}
}

func TestProcessCoverAppliesEXIFOrientation(t *testing.T) {
	// Depolanan 400x300 görselin dört çeyreği: sol üst kırmızı, sağ üst yeşil, sol alt mavi,
	// sağ alt sarı. Her yön değeri için gösterilen görselin çeyrekleri (SÜ, SaÜ, SA, SaA).
	stored := fill(400, 300, func(x, y int) color.RGBA {
		switch {
		case x < 200 && y < 150:
			return red
		case y < 150:
			return green
		case x < 200:
			return blue
		default:
			return yellow
		}
	})
	want := map[uint16][4]color.RGBA{
		1: {red, green, blue, yellow},
		2: {green, red, yellow, blue},
		3: {yellow, blue, green, red},
		4: {blue, yellow, red, green},
		5: {red, blue, green, yellow},
		6: {blue, red, yellow, green},
		7: {yellow, green, blue, red},
		8: {green, yellow, red, blue},
	}
	base := encodeJPEG(t, stored)
	for orientation, quadrants := range want {
		for _, order := range []byteOrder{binary.BigEndian, binary.LittleEndian} {
			res, img := mustProcess(t, withEXIF(t, base, orientation, order))
			if res.Width*3 != res.Height*4 {
				t.Fatalf("yön %d: çıktı 4:3 değil (%dx%d)", orientation, res.Width, res.Height)
			}
			w, h := res.Width, res.Height
			points := [4]image.Point{{w / 8, h / 8}, {w - w/8, h / 8}, {w / 8, h - h/8}, {w - w/8, h - h/8}}
			for i, p := range points {
				if !near(img.At(p.X, p.Y), quadrants[i]) {
					t.Errorf("yön %d (%v): %d. çeyrekte %v bekleniyordu, %v", orientation, order, i, quadrants[i], img.At(p.X, p.Y))
				}
			}
		}
	}
}

func TestProcessCoverWithoutEXIFKeepsOrientation(t *testing.T) {
	if got := jpegOrientation(encodeJPEG(t, photo(100, 100))); got != 1 {
		t.Fatalf("EXIF'siz JPEG için yön %d", got)
	}
	// Bozuk EXIF yüklemeyi düşürmez; yön 1 kabul edilir.
	broken := withEXIF(t, encodeJPEG(t, photo(400, 300)), 6, binary.BigEndian)
	copy(broken[12:], []byte("XX")) // TIFF bayt sırası işaretini boz
	res, _ := mustProcess(t, broken)
	if res.Width != 400 || res.Height != 300 {
		t.Fatalf("bozuk EXIF ile %dx%d", res.Width, res.Height)
	}
}

// jpegMarkers çıktıdaki bölüm işaretlerini (SOS'a kadar) listeler.
func jpegMarkers(t *testing.T, data []byte) []byte {
	t.Helper()
	var markers []byte
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			t.Fatalf("bozuk JPEG bölümü: ofset %d", i)
		}
		marker := data[i+1]
		markers = append(markers, marker)
		if marker == 0xDA {
			break
		}
		i += 2 + int(binary.BigEndian.Uint16(data[i+2:i+4]))
	}
	return markers
}

func TestProcessCoverStripsMetadata(t *testing.T) {
	input := withEXIF(t, encodeJPEG(t, photo(800, 600)), 1, binary.BigEndian)
	if !bytes.Contains(input, []byte("GPSLatitude")) {
		t.Fatal("test girdisi GPS verisi içermiyor")
	}
	res, _ := mustProcess(t, input)
	for _, leaked := range []string{"Exif", "GPSLatitude", "41.0082", "GPSLongitude"} {
		if bytes.Contains(res.Data, []byte(leaked)) {
			t.Fatalf("çıktı metadata içeriyor: %q", leaked)
		}
	}
	for _, m := range jpegMarkers(t, res.Data) {
		// APP1–APP15 (EXIF, XMP, ICC, IPTC…) ve COM bölümleri bulunmamalı.
		if (m >= 0xE1 && m <= 0xEF) || m == 0xFE {
			t.Fatalf("çıktıda metadata bölümü var: 0xFF%02X", m)
		}
	}
}

// pngWithHeaderSize geçerli küçük bir PNG'nin IHDR boyutlarını (CRC'yi düzelterek) değiştirir.
func pngWithHeaderSize(t *testing.T, w, h uint32) []byte {
	t.Helper()
	data := encodePNG(t, photo(64, 64))
	// 8 imza + 4 uzunluk + 4 "IHDR" + 13 veri + 4 CRC
	binary.BigEndian.PutUint32(data[16:20], w)
	binary.BigEndian.PutUint32(data[20:24], h)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func TestProcessCoverRejectsDecompressionBombs(t *testing.T) {
	cases := []struct {
		name string
		w, h uint32
	}{
		{"dev kare", 20000, 20000},
		{"tek kenar sınırın üstünde", MaxDimension + 1, 100},
		{"kenarlar sınırda ama 50 MP üstünde", 10000, 6000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := pngWithHeaderSize(t, tc.w, tc.h)
			if len(data) > 64<<10 {
				t.Fatalf("bomba dosyası küçük olmalıydı: %d bayt", len(data))
			}
			if _, err := ProcessCover(data); !errors.Is(err, ErrDimensionsTooBig) {
				t.Fatalf("hata %v, beklenen ErrDimensionsTooBig", err)
			}
		})
	}
	// Taşma denemesi: PNG çözücüsü başlığı baştan reddeder; her durumda çözülmez.
	if _, err := ProcessCover(pngWithHeaderSize(t, 0x7FFFFFFF, 0x7FFFFFFF)); !errors.Is(err, ErrDimensionsTooBig) && !errors.Is(err, ErrCorrupt) {
		t.Fatalf("taşan boyut reddedilmedi: %v", err)
	}
	// Sınırdaki kenar reddedilmez (boyut kontrolü geçer; başlık yalan söylediği için çözme
	// aşamasında bozuk sayılır, ama "çok büyük" değildir).
	if _, err := ProcessCover(pngWithHeaderSize(t, MaxDimension, 100)); errors.Is(err, ErrDimensionsTooBig) {
		t.Fatalf("12000 piksellik kenar sınırın içinde olmalıydı: %v", err)
	}
}

func TestProcessCoverRejectsBadInput(t *testing.T) {
	valid := encodeJPEG(t, photo(800, 600))
	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...)
	gif := []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\xff\xff\xff\x00\x00\x00;")
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"boş", nil, ErrUnsupportedType},
		{"metin", []byte("merhaba, ben bir görsel değilim"), ErrUnsupportedType},
		{"gif", gif, ErrUnsupportedType},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), ErrUnsupportedType},
		{"heic", heic, ErrHEICNotSupported},
		{"yarım jpeg", valid[:len(valid)/3], ErrCorrupt},
		{"yalnızca jpeg imzası", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0}, ErrCorrupt},
		{"png imzası + çöp", append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0x41}, 64)...), ErrCorrupt},
		{"webp imzası + çöp", append([]byte("RIFF\x40\x00\x00\x00WEBP"), bytes.Repeat([]byte{0x41}, 64)...), ErrCorrupt},
		{"10 MB üstü", append(append([]byte{}, valid...), make([]byte, MaxInputBytes)...), ErrTooLarge},
		{"çok küçük", encodePNG(t, photo(10, 10)), ErrDimensionsTooTiny},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ProcessCover(tc.data)
			if !errors.Is(err, tc.want) {
				t.Fatalf("hata %v, beklenen %v", err, tc.want)
			}
			if res != nil {
				t.Fatal("hata durumunda sonuç dönmemeli")
			}
		})
	}
}

func TestProcessCoverFlattensPNGTransparencyOnWhite(t *testing.T) {
	// Sol yarı tam şeffaf, sağ yarı yarı saydam kırmızı.
	src := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 200; x < 400; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 128})
		}
	}
	_, img := mustProcess(t, encodePNG(t, src))
	if !near(img.At(50, 150), color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("şeffaf alan beyaz olmalıydı (siyah değil): %v", img.At(50, 150))
	}
	if !near(img.At(350, 150), color.RGBA{R: 255, G: 127, B: 127, A: 255}) {
		t.Fatalf("yarı saydam kırmızı beyazla karışmalıydı: %v", img.At(350, 150))
	}
}

func TestProcessCoverAcceptsWebP(t *testing.T) {
	alpha, err := os.ReadFile("testdata/alpha.webp") // 240x120; sol yarı şeffaf, sağ yarı kırmızı
	if err != nil {
		t.Fatal(err)
	}
	res, img := mustProcess(t, alpha)
	if res.Width != 160 || res.Height != 120 {
		t.Fatalf("%dx%d, beklenen 160x120", res.Width, res.Height)
	}
	if !near(img.At(20, 60), color.RGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("WebP şeffaflığı beyaz olmalıydı: %v", img.At(20, 60))
	}
	if !near(img.At(140, 60), red) {
		t.Fatalf("WebP opak alanı kırmızı olmalıydı: %v", img.At(140, 60))
	}

	// 160x120 depolanan, EXIF yönü 6: sol kırmızı yarı gösterimde üste gelir.
	rotated, err := os.ReadFile("testdata/orient6.webp")
	if err != nil {
		t.Fatal(err)
	}
	res, img = mustProcess(t, rotated)
	if res.Width != 120 || res.Height != 90 {
		t.Fatalf("%dx%d, beklenen 120x90", res.Width, res.Height)
	}
	if !near(img.At(100, 10), red) || !near(img.At(20, 80), blue) {
		t.Fatalf("WebP EXIF yönü uygulanmadı: üst %v, alt %v", img.At(100, 10), img.At(20, 80))
	}
	if bytes.Contains(res.Data, []byte("Exif")) {
		t.Fatal("WebP EXIF verisi çıktıya taşındı")
	}
}

func TestSniffIgnoresDeclaredType(t *testing.T) {
	// Uzantısı/başlığı ne olursa olsun tür bayttan okunur.
	if f, err := Sniff(encodePNG(t, photo(64, 64))); err != nil || f != FormatPNG {
		t.Fatalf("png: %v %v", f, err)
	}
	if f, err := Sniff(encodeJPEG(t, photo(64, 64))); err != nil || f != FormatJPEG {
		t.Fatalf("jpeg: %v %v", f, err)
	}
}

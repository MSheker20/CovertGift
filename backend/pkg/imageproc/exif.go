package imageproc

import "encoding/binary"

// jpegOrientation JPEG'in APP1/EXIF bölümündeki yön etiketini (0x0112) okur. Bulunamazsa
// ya da veri bozuksa 1 (normal) döner; yön okunamadığı için yükleme reddedilmez.
func jpegOrientation(data []byte) int {
	i := 2 // SOI'den sonra
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xFF { // dolgu baytı
			i++
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // SOS / EOI: başlık bölümleri bitti
			return 1
		}
		if marker >= 0xD0 && marker <= 0xD7 || marker == 0x01 { // uzunluksuz işaretler
			i += 2
			continue
		}
		size := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if size < 2 || i+2+size > len(data) {
			return 1
		}
		segment := data[i+4 : i+2+size]
		if marker == 0xE1 && len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
			return tiffOrientation(segment[6:])
		}
		i += 2 + size
	}
	return 1
}

// webpOrientation genişletilmiş WebP (VP8X) dosyasındaki EXIF parçasının yönünü okur.
func webpOrientation(data []byte) int {
	i := 12 // "RIFF" + boyut + "WEBP"
	for i+8 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		start := i + 8
		if size < 0 || start+size > len(data) {
			return 1
		}
		if string(data[i:i+4]) == "EXIF" {
			chunk := data[start : start+size]
			if len(chunk) >= 6 && string(chunk[:6]) == "Exif\x00\x00" {
				chunk = chunk[6:]
			}
			return tiffOrientation(chunk)
		}
		i = start + size + size%2 // parçalar çift bayta hizalanır
	}
	return 1
}

// tiffOrientation TIFF başlığıyla başlayan EXIF verisinin IFD0'ındaki yön değerini döner.
func tiffOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return 1
	}
	offset := int(order.Uint32(tiff[4:8]))
	if offset < 8 || offset+2 > len(tiff) {
		return 1
	}
	count := int(order.Uint16(tiff[offset : offset+2]))
	for n := 0; n < count; n++ {
		entry := offset + 2 + n*12
		if entry+12 > len(tiff) {
			return 1
		}
		if order.Uint16(tiff[entry:entry+2]) != 0x0112 {
			continue
		}
		value := int(order.Uint16(tiff[entry+8 : entry+10]))
		if value < 1 || value > 8 {
			return 1
		}
		return value
	}
	return 1
}

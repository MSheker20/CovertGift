package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/MSheker20/CovertGift/backend/internal/models"
	"github.com/MSheker20/CovertGift/backend/internal/repository"
	"github.com/MSheker20/CovertGift/backend/internal/services"
	"gorm.io/gorm"
)

// seedAvatars Can ve Elif'e kodla üretilmiş profil fotoğrafı ekler; Zeynep ve Ahmet baş harfle
// kalır (fotoğraflı ve fotoğrafsız hâl yan yana görünsün). Fotoğraf gerçek yükleme yolundan
// (AvatarService.Set → imageproc.ProcessAvatar) geçer. Fotoğrafı olan kullanıcıya dokunulmaz;
// hiçbirine eklenmediyse errAlreadySeeded döner.
func seedAvatars(tx *gorm.DB, users map[string]*models.User) error {
	svc := services.NewAvatarService(repository.NewAvatarRepository(tx))
	added := 0
	for _, a := range []struct {
		email     string
		from, to  color.RGBA
		ringColor color.RGBA
	}{
		{demoAdmin.Email, color.RGBA{47, 91, 255, 255}, color.RGBA{134, 160, 255, 255}, color.RGBA{255, 201, 60, 255}},
		{demoFriend.Email, color.RGBA{31, 122, 61, 255}, color.RGBA{95, 211, 138, 255}, color.RGBA{255, 255, 255, 255}},
	} {
		u := users[a.email]
		var current models.User
		if err := tx.Select("avatar_version").First(&current, "id = ?", u.ID).Error; err != nil {
			return err
		}
		if current.AvatarVersion != "" {
			continue
		}
		data, err := avatarPNG(a.from, a.to, a.ringColor)
		if err != nil {
			return err
		}
		if _, err := svc.Set(u.ID, data); err != nil {
			return fmt.Errorf("%s profil fotoğrafı: %w", a.email, err)
		}
		added++
	}
	if added == 0 {
		return errAlreadySeeded
	}
	fmt.Printf("Profil fotoğrafı: %s ve %s (üretilmiş görsel); %s ve %s baş harfle.\n",
		demoAdmin.Name, demoFriend.Name, demoMember.Name, demoTarget.Name)
	return nil
}

// avatarPNG 512x512'lik çapraz degrade ve ortasında halka olan bir görsel üretir.
func avatarPNG(from, to, ring color.RGBA) ([]byte, error) {
	const size = 512
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	lerp := func(a, b uint8, t float64) uint8 { return uint8(float64(a) + (float64(b)-float64(a))*t) }
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			t := float64(x+y) / float64(2*size)
			c := color.RGBA{lerp(from.R, to.R, t), lerp(from.G, to.G, t), lerp(from.B, to.B, t), 255}
			if d := math.Hypot(float64(x-size/2), float64(y-size/2)); d > 120 && d < 150 {
				c = ring
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

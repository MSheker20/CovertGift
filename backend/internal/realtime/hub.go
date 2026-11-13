// Package realtime "bir şey değişti" olayını açık SSE akışlarına iletir. Hub bir anahtara
// (UUID) göre abone tutar; uygulama iki ayrı Hub kullanır: grup sayfaları için grup
// kimliğiyle, bildirim zili için kullanıcı kimliğiyle.
//
// Olaylar veri taşımaz; istemci yalnızca ilgili sorguları yeniden çeker ve veriyi her
// zamanki Anti-Spoiler korumalı uçlardan alır. Hub bellek içidir ve tek bir API
// kopyası için tasarlanmıştır (Azure'da en fazla 1 replica). Birden çok kopyada
// olayların kopyalar arasında taşınması için PostgreSQL LISTEN/NOTIFY gibi bir aracı gerekir.
package realtime

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

// ErrTooManySubscribers aynı anda açık bağlantı sınırı aşıldığında döner.
var ErrTooManySubscribers = errors.New("realtime: too many open streams")

// Hub bir anahtara (grup ya da kullanıcı kimliği) göre aboneleri tutar.
type Hub struct {
	mu    sync.Mutex
	subs  map[uuid.UUID]map[chan struct{}]struct{}
	total int
	max   int
}

// NewHub en fazla max açık aboneliğe izin veren bir Hub oluşturur.
func NewHub(max int) *Hub {
	return &Hub{subs: map[uuid.UUID]map[chan struct{}]struct{}{}, max: max}
}

// Subscribe anahtara abone olur. Dönen kanal bir değişiklikte sinyal alır; art arda gelen
// değişiklikler tek sinyalde birleşir (kanal 1 kapasitelidir). İş bitince cancel çağrılmalıdır.
func (h *Hub) Subscribe(key uuid.UUID) (<-chan struct{}, func(), error) {
	return h.SubscribeMany([]uuid.UUID{key})
}

// SubscribeMany tek bir kanalı birden çok anahtara bağlar (ör. MULTI grup sayfası: grup anahtarı
// ve çağıranın görebildiği havuzlar). Açık bağlantı sınırına bir kez sayılır. Anahtarlardan
// herhangi birine gelen sinyal aynı kanala düşer. İş bitince cancel çağrılmalıdır.
func (h *Hub) SubscribeMany(keys []uuid.UUID) (<-chan struct{}, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.total >= h.max {
		return nil, nil, ErrTooManySubscribers
	}
	ch := make(chan struct{}, 1)
	unique := make([]uuid.UUID, 0, len(keys))
	seen := map[uuid.UUID]bool{}
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, key)
		if h.subs[key] == nil {
			h.subs[key] = map[chan struct{}]struct{}{}
		}
		h.subs[key][ch] = struct{}{}
	}
	h.total++
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			for _, key := range unique {
				delete(h.subs[key], ch)
				if len(h.subs[key]) == 0 {
					delete(h.subs, key)
				}
			}
			h.total--
		})
	}
	return ch, cancel, nil
}

// Publish anahtara abone olan herkese (aynı kullanıcının birden çok sekmesi dahil) sinyal gönderir; hiçbir zaman bloklamaz.
func (h *Hub) Publish(key uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[key] {
		select {
		case ch <- struct{}{}:
		default: // bekleyen bir sinyal zaten var
		}
	}
}

// Count açık abonelik sayısını döner (testler ve izleme için).
func (h *Hub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.total
}

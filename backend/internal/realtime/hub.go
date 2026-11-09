// Package realtime grup sayfalarına "bu grupta bir şey değişti" olayını iletir (SSE).
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

// Hub grup kimliğine göre aboneleri tutar.
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

// Subscribe gruba abone olur. Dönen kanal bir değişiklikte sinyal alır; art arda gelen
// değişiklikler tek sinyalde birleşir (kanal 1 kapasitelidir). İş bitince cancel çağrılmalıdır.
func (h *Hub) Subscribe(groupID uuid.UUID) (<-chan struct{}, func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.total >= h.max {
		return nil, nil, ErrTooManySubscribers
	}
	ch := make(chan struct{}, 1)
	if h.subs[groupID] == nil {
		h.subs[groupID] = map[chan struct{}]struct{}{}
	}
	h.subs[groupID][ch] = struct{}{}
	h.total++
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.subs[groupID], ch)
			if len(h.subs[groupID]) == 0 {
				delete(h.subs, groupID)
			}
			h.total--
		})
	}
	return ch, cancel, nil
}

// Publish gruba abone olan herkese sinyal gönderir; hiçbir zaman bloklamaz.
func (h *Hub) Publish(groupID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[groupID] {
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

package realtime

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestHubDeliversCoalescesAndUnsubscribes(t *testing.T) {
	h := NewHub(2)
	g1, g2 := uuid.New(), uuid.New()
	a, cancelA, err := h.Subscribe(g1)
	if err != nil {
		t.Fatal(err)
	}
	b, cancelB, _ := h.Subscribe(g2)

	h.Publish(g1)
	h.Publish(g1) // birleşir, bloklamaz
	select {
	case <-a:
	default:
		t.Fatal("abone sinyal almadı")
	}
	select {
	case <-a:
		t.Fatal("art arda sinyaller birleşmedi")
	default:
	}
	select {
	case <-b:
		t.Fatal("başka grubun abonesi sinyal aldı")
	default:
	}

	if _, _, err := h.Subscribe(g1); !errors.Is(err, ErrTooManySubscribers) {
		t.Fatalf("sınır uygulanmadı: %v", err)
	}
	cancelA()
	cancelA() // iki kez çağırmak güvenli
	cancelB()
	if h.Count() != 0 {
		t.Fatalf("abonelik sızdı: %d", h.Count())
	}
	h.Publish(g1) // abonesiz grup
}

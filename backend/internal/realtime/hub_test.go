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

// Bildirim zili: Hub kullanıcı kimliğiyle anahtarlanır. Aynı kullanıcının iki sekmesi de
// sinyal alır, başka kullanıcı almaz; bir sekme kapanınca diğeri almaya devam eder.
func TestHubPerUserFanOut(t *testing.T) {
	h := NewHub(10)
	alice, bob := uuid.New(), uuid.New()
	tab1, cancel1, err := h.Subscribe(alice)
	if err != nil {
		t.Fatal(err)
	}
	tab2, cancel2, _ := h.Subscribe(alice)
	other, cancelOther, _ := h.Subscribe(bob)
	defer cancel2()
	defer cancelOther()

	got := func(ch <-chan struct{}) bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
	h.Publish(alice)
	if !got(tab1) || !got(tab2) {
		t.Fatal("kullanıcının tüm sekmeleri sinyal almadı")
	}
	if got(other) {
		t.Fatal("başka kullanıcı sinyal aldı")
	}

	cancel1()
	h.Publish(alice)
	if got(tab1) {
		t.Fatal("kapanan sekme sinyal aldı")
	}
	if !got(tab2) {
		t.Fatal("açık kalan sekme sinyal almadı")
	}
	if h.Count() != 2 {
		t.Fatalf("abonelik sayısı %d, beklenen 2", h.Count())
	}
}

// Eşzamanlı abone ol / yayınla / bırak yarışında kilitlenme ve sızıntı olmamalı (-race ile).
func TestHubConcurrentUse(t *testing.T) {
	h := NewHub(1000)
	key := uuid.New()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			_, cancel, err := h.Subscribe(key)
			if err != nil {
				t.Error(err)
				return
			}
			h.Publish(key)
			cancel()
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	if h.Count() != 0 {
		t.Fatalf("abonelik sızdı: %d", h.Count())
	}
}

func TestHubSubscribeManyCountsOnceAndReceivesFromEveryKey(t *testing.T) {
	h := NewHub(2)
	group, pool1, pool2, hidden := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	ch, cancel, err := h.SubscribeMany([]uuid.UUID{group, pool1, pool2, pool1})
	if err != nil {
		t.Fatal(err)
	}
	if h.Count() != 1 {
		t.Fatalf("çok anahtarlı abonelik %d kez sayıldı", h.Count())
	}
	for _, key := range []uuid.UUID{group, pool1, pool2} {
		h.Publish(key)
		select {
		case <-ch:
		default:
			t.Fatalf("%s sinyali gelmedi", key)
		}
	}
	h.Publish(hidden)
	select {
	case <-ch:
		t.Fatal("abone olunmayan anahtardan sinyal geldi")
	default:
	}
	cancel()
	cancel() // idempotent
	if h.Count() != 0 || len(h.subs) != 0 {
		t.Fatalf("iptal sonrası kalan: count=%d subs=%d", h.Count(), len(h.subs))
	}
}

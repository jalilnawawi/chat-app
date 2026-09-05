package hub

import (
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// fakeSink meniru koneksi WebSocket dengan buffer berukuran tetap.
type fakeSink struct {
	id   uuid.UUID
	mu   sync.Mutex
	got  [][]byte
	cap_ int
}

func newFakeSink(id uuid.UUID, capacity int) *fakeSink {
	return &fakeSink{id: id, cap_: capacity}
}

func (f *fakeSink) UserID() uuid.UUID { return f.id }

func (f *fakeSink) Enqueue(p []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) >= f.cap_ {
		return false // buffer penuh, seperti client yang tidak menyusul
	}
	f.got = append(f.got, p)
	return true
}

func (f *fakeSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.got)
}

func testHub() *Hub { return New(slog.New(slog.NewTextHandler(io.Discard, nil))) }

func TestPublishReachesEveryTargetOnce(t *testing.T) {
	h := testHub()
	alice, bob, carol := uuid.New(), uuid.New(), uuid.New()

	sa, sb, sc := newFakeSink(alice, 10), newFakeSink(bob, 10), newFakeSink(carol, 10)
	h.Register(sa)
	h.Register(sb)
	h.Register(sc)

	// carol bukan target: siaran tidak boleh bocor ke luar anggota ruang.
	h.Publish([]uuid.UUID{alice, bob}, Event{Type: EventMessageNew, Payload: "halo"})

	if sa.count() != 1 || sb.count() != 1 {
		t.Fatalf("target harus menerima tepat 1 pesan, dapat alice=%d bob=%d", sa.count(), sb.count())
	}
	if sc.count() != 0 {
		t.Fatalf("non-target menerima %d pesan, harusnya 0", sc.count())
	}
}

func TestPublishReachesAllTabsOfSameUser(t *testing.T) {
	h := testHub()
	alice := uuid.New()

	tab1, tab2 := newFakeSink(alice, 10), newFakeSink(alice, 10)
	if first := h.Register(tab1); !first {
		t.Fatal("koneksi pertama harus dilaporkan sebagai perubahan presence")
	}
	if first := h.Register(tab2); first {
		t.Fatal("koneksi kedua user yang sama bukan perubahan presence")
	}

	h.Publish([]uuid.UUID{alice}, Event{Type: EventMessageNew, Payload: "halo"})

	if tab1.count() != 1 || tab2.count() != 1 {
		t.Fatalf("kedua tab harus menerima, dapat %d dan %d", tab1.count(), tab2.count())
	}
}

func TestSlowClientIsDroppedAndDoesNotBlockOthers(t *testing.T) {
	h := testHub()
	slow, fast := uuid.New(), uuid.New()

	// Kapasitas 1: penerima ini penuh setelah satu pesan.
	slowSink := newFakeSink(slow, 1)
	fastSink := newFakeSink(fast, 10)
	h.Register(slowSink)
	h.Register(fastSink)

	targets := []uuid.UUID{slow, fast}
	for range 3 {
		h.Publish(targets, Event{Type: EventMessageNew, Payload: "spam"})
	}

	// Client lambat dilepas setelah gagal sekali, dan tidak menahan yang lain.
	if fastSink.count() != 3 {
		t.Fatalf("client cepat harus tetap menerima 3 pesan, dapat %d", fastSink.count())
	}
	if slowSink.count() != 1 {
		t.Fatalf("client lambat harus berhenti di 1 pesan, dapat %d", slowSink.count())
	}
	if got := h.OnlineAmong([]uuid.UUID{slow}); len(got) != 0 {
		t.Fatal("client lambat seharusnya sudah dilepas dari hub")
	}
}

func TestUnregisterReportsLastConnection(t *testing.T) {
	h := testHub()
	alice := uuid.New()
	tab1, tab2 := newFakeSink(alice, 10), newFakeSink(alice, 10)
	h.Register(tab1)
	h.Register(tab2)

	if last := h.Unregister(tab1); last {
		t.Fatal("masih ada tab lain, belum offline")
	}
	if last := h.Unregister(tab2); !last {
		t.Fatal("tab terakhir keluar harus dilaporkan sebagai offline")
	}
	if got := h.OnlineAmong([]uuid.UUID{alice}); len(got) != 0 {
		t.Fatal("user tanpa koneksi tidak boleh dianggap online")
	}
}

func TestEventIsEncodedOnceAndValid(t *testing.T) {
	h := testHub()
	alice := uuid.New()
	sink := newFakeSink(alice, 5)
	h.Register(sink)

	h.Publish([]uuid.UUID{alice}, Event{
		Type:    EventTyping,
		Payload: map[string]any{"conversationId": "abc", "typing": true},
	})

	var decoded struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(sink.got[0], &decoded); err != nil {
		t.Fatalf("payload bukan JSON valid: %v", err)
	}
	if decoded.Type != EventTyping || decoded.Payload["typing"] != true {
		t.Fatalf("isi event tidak sesuai: %+v", decoded)
	}
}

package hub

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/metrics"
)

// fakeSink meniru koneksi WebSocket dengan buffer berukuran tetap.
type fakeSink struct {
	id      uuid.UUID
	session []byte
	mu      sync.Mutex
	got     [][]byte
	cap_    int
	closed  bool
}

func newFakeSink(id uuid.UUID, capacity int) *fakeSink {
	return &fakeSink{id: id, cap_: capacity}
}

// newFakeSession meniru koneksi milik SATU perangkat: dua sink dengan id user
// yang sama tapi hash sesi yang berbeda adalah dua perangkat orang yang sama.
func newFakeSession(id uuid.UUID, session string, capacity int) *fakeSink {
	return &fakeSink{id: id, session: []byte(session), cap_: capacity}
}

func (f *fakeSink) UserID() uuid.UUID { return f.id }

func (f *fakeSink) SessionHash() []byte { return f.session }

// Kick meniru ws.Client: payload-nya ditulis LEBIH DULU, baru koneksinya
// berakhir. Urutan itu yang diuji di TestPencabutanSesi.
func (f *fakeSink) Kick(p []byte) {
	f.mu.Lock()
	if !f.closed {
		f.got = append(f.got, p)
		f.closed = true
	}
	f.mu.Unlock()
}

// Enqueue meniru ws.Client, termasuk membedakan koneksi yang sudah tutup dari
// buffer yang penuh.
func (f *fakeSink) Enqueue(p []byte) Delivery {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return Gone
	}
	if len(f.got) >= f.cap_ {
		return Backpressure // client yang tidak menyusul
	}
	f.got = append(f.got, p)
	return Delivered
}

func (f *fakeSink) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

// last adalah payload terakhir yang diterima sink ini.
func (f *fakeSink) last() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.got) == 0 {
		return nil
	}
	return f.got[len(f.got)-1]
}

func (f *fakeSink) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.got)
}

func (f *fakeSink) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

func (f *fakeSink) at(i int) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.got[i]
}

func testHub(t *testing.T) *Memory {
	t.Helper()
	return NewMemory(slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.New())
}

func register(t *testing.T, h *Memory, s Sink) bool {
	t.Helper()
	first, err := h.Register(context.Background(), s)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return first
}

func unregister(t *testing.T, h *Memory, s Sink) bool {
	t.Helper()
	last, err := h.Unregister(context.Background(), s)
	if err != nil {
		t.Fatalf("unregister: %v", err)
	}
	return last
}

func onlineCount(t *testing.T, h *Memory, ids ...uuid.UUID) int {
	t.Helper()
	got, err := h.OnlineAmong(context.Background(), ids)
	if err != nil {
		t.Fatalf("online among: %v", err)
	}
	return len(got)
}

func TestPublishReachesEveryTargetOnce(t *testing.T) {
	h := testHub(t)
	alice, bob, carol := uuid.New(), uuid.New(), uuid.New()

	sa, sb, sc := newFakeSink(alice, 10), newFakeSink(bob, 10), newFakeSink(carol, 10)
	register(t, h, sa)
	register(t, h, sb)
	register(t, h, sc)

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
	h := testHub(t)
	alice := uuid.New()

	tab1, tab2 := newFakeSink(alice, 10), newFakeSink(alice, 10)
	if !register(t, h, tab1) {
		t.Fatal("koneksi pertama harus dilaporkan sebagai perubahan presence")
	}
	if register(t, h, tab2) {
		t.Fatal("koneksi kedua user yang sama bukan perubahan presence")
	}

	h.Publish([]uuid.UUID{alice}, Event{Type: EventMessageNew, Payload: "halo"})

	if tab1.count() != 1 || tab2.count() != 1 {
		t.Fatalf("kedua tab harus menerima, dapat %d dan %d", tab1.count(), tab2.count())
	}
}

func TestSlowClientIsClosedAndDoesNotBlockOthers(t *testing.T) {
	h := testHub(t)
	slow, fast := uuid.New(), uuid.New()

	// Kapasitas 1: penerima ini penuh setelah satu pesan.
	slowSink := newFakeSink(slow, 1)
	fastSink := newFakeSink(fast, 10)
	register(t, h, slowSink)
	register(t, h, fastSink)

	targets := []uuid.UUID{slow, fast}
	for range 3 {
		h.Publish(targets, Event{Type: EventMessageNew, Payload: "spam"})
	}

	if fastSink.count() != 3 {
		t.Fatalf("client cepat harus tetap menerima 3 pesan, dapat %d", fastSink.count())
	}
	if slowSink.count() != 1 {
		t.Fatalf("client lambat harus berhenti di 1 pesan, dapat %d", slowSink.count())
	}
	if !slowSink.isClosed() {
		t.Fatal("client lambat harus diminta menutup koneksinya")
	}
}

// Hub sengaja TIDAK melepas sendiri client lambat dari registry. Pelepasan
// dikerjakan satu tempat — defer di ws.Handler.Serve — supaya presence
// "offline" tetap disiarkan. Kalau hub ikut melepas, Unregister dari ws akan
// mengembalikan false dan status offline tidak pernah sampai ke lawan bicara.
func TestSlowClientStillReportsOfflineWhenUnregistered(t *testing.T) {
	h := testHub(t)
	alice := uuid.New()
	sink := newFakeSink(alice, 1)
	register(t, h, sink)

	h.Publish([]uuid.UUID{alice}, Event{Type: EventMessageNew, Payload: "satu"})
	h.Publish([]uuid.UUID{alice}, Event{Type: EventMessageNew, Payload: "dua"}) // memenuhi buffer

	if !sink.isClosed() {
		t.Fatal("prasyarat: client harus sudah diminta menutup")
	}
	if !unregister(t, h, sink) {
		t.Fatal("pelepasan koneksi lambat harus tetap dilaporkan sebagai offline")
	}
	if onlineCount(t, h, alice) != 0 {
		t.Fatal("user tanpa koneksi tidak boleh dianggap online")
	}
}

func TestUnregisterReportsLastConnection(t *testing.T) {
	h := testHub(t)
	alice := uuid.New()
	tab1, tab2 := newFakeSink(alice, 10), newFakeSink(alice, 10)
	register(t, h, tab1)
	register(t, h, tab2)

	if unregister(t, h, tab1) {
		t.Fatal("masih ada tab lain, belum offline")
	}
	if !unregister(t, h, tab2) {
		t.Fatal("tab terakhir keluar harus dilaporkan sebagai offline")
	}
	if onlineCount(t, h, alice) != 0 {
		t.Fatal("user tanpa koneksi tidak boleh dianggap online")
	}
}

// Melepas koneksi yang sama dua kali tidak boleh melaporkan "offline" lagi;
// kalau iya, satu putus koneksi bisa menyiarkan offline padahal tab lain milik
// user itu masih terbuka.
func TestUnregisterIsIdempotent(t *testing.T) {
	h := testHub(t)
	alice := uuid.New()
	tab1, tab2 := newFakeSink(alice, 10), newFakeSink(alice, 10)
	register(t, h, tab1)
	register(t, h, tab2)

	unregister(t, h, tab1)
	if unregister(t, h, tab1) {
		t.Fatal("pelepasan ulang koneksi yang sama tidak boleh dianggap offline")
	}
	if onlineCount(t, h, alice) != 1 {
		t.Fatal("tab kedua masih terbuka, user harus tetap online")
	}
}

func TestEventIsEncodedOnceAndValid(t *testing.T) {
	h := testHub(t)
	alice := uuid.New()
	sink := newFakeSink(alice, 5)
	register(t, h, sink)

	h.Publish([]uuid.UUID{alice}, Event{
		Type:    EventTyping,
		Payload: map[string]any{"conversationId": "abc", "typing": true},
	})

	var decoded struct {
		Type    string         `json:"type"`
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(sink.at(0), &decoded); err != nil {
		t.Fatalf("payload bukan JSON valid: %v", err)
	}
	if decoded.Type != EventTyping || decoded.Payload["typing"] != true {
		t.Fatalf("isi event tidak sesuai: %+v", decoded)
	}
}

// Drain harus memberi tahu SEBELUM menutup. Urutan terbalik berarti client
// hanya melihat koneksi putus, lalu mundur perlahan seperti menghadapi gangguan
// jaringan — persis yang tidak diinginkan saat instance pengganti sudah siap.
func TestDrainNotifiesBeforeClosing(t *testing.T) {
	h := testHub(t)
	sinks := make([]*fakeSink, 5)
	for i := range sinks {
		sinks[i] = newFakeSink(uuid.New(), 5)
		register(t, h, sinks[i])
	}

	h.Drain(context.Background(), 50*time.Millisecond)

	for i, s := range sinks {
		if !s.isClosed() {
			t.Fatalf("koneksi %d harus ditutup setelah drain", i)
		}
		if s.count() != 1 {
			t.Fatalf("koneksi %d harus menerima 1 pemberitahuan, dapat %d", i, s.count())
		}

		var ev struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(s.at(0), &ev); err != nil {
			t.Fatalf("pemberitahuan bukan JSON valid: %v", err)
		}
		if ev.Type != EventServerShutdown {
			t.Fatalf("koneksi %d menerima %q, harusnya %q", i, ev.Type, EventServerShutdown)
		}
	}
}

// Drain menyebar penutupan sepanjang periode, bukan menutup serentak. Inilah
// yang mengubah badai reconnect jadi aliran saat rolling deploy.
func TestDrainSpreadsClosuresOverPeriod(t *testing.T) {
	h := testHub(t)
	for range 4 {
		register(t, h, newFakeSink(uuid.New(), 5))
	}

	start := time.Now()
	h.Drain(context.Background(), 200*time.Millisecond)
	elapsed := time.Since(start)

	if elapsed < 100*time.Millisecond {
		t.Fatalf("penutupan terlalu serentak: selesai dalam %s", elapsed)
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("penutupan jauh melebihi periode: %s", elapsed)
	}
}

// Batas waktu shutdown harus menang atas penyebaran: lebih baik semua client
// reconnect berbarengan daripada proses ditembak paksa di tengah penulisan.
func TestDrainRespectsDeadline(t *testing.T) {
	h := testHub(t)
	sinks := make([]*fakeSink, 20)
	for i := range sinks {
		sinks[i] = newFakeSink(uuid.New(), 5)
		register(t, h, sinks[i])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	h.Drain(ctx, 10*time.Second)

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("drain mengabaikan batas waktu, butuh %s", elapsed)
	}
	for i, s := range sinks {
		if !s.isClosed() {
			t.Fatalf("koneksi %d harus tetap ditutup saat waktu habis", i)
		}
	}
}

func TestLocalConnectionsCountsEveryTab(t *testing.T) {
	h := testHub(t)
	alice := uuid.New()
	tab1, tab2 := newFakeSink(alice, 5), newFakeSink(alice, 5)
	bobSink := newFakeSink(uuid.New(), 5)

	register(t, h, tab1)
	register(t, h, tab2)
	register(t, h, bobSink)

	if got := h.LocalConnections(); got != 3 {
		t.Fatalf("jumlah koneksi = %d, harusnya 3", got)
	}

	unregister(t, h, tab1)
	if got := h.LocalConnections(); got != 2 {
		t.Fatalf("setelah satu tab keluar = %d, harusnya 2", got)
	}
}

// Koneksi yang sudah ditutup bukan client lambat. Sebelum keduanya dibedakan,
// setiap orang yang menutup tab saat sebuah siaran kebetulan sedang berjalan
// ikut tercatat sebagai backpressure — dan metrik yang berbunyi tanpa sebab
// adalah metrik yang akhirnya diabaikan.
func TestClosedConnectionIsNotCountedAsBackpressure(t *testing.T) {
	h := testHub(t)
	alice := uuid.New()
	sink := newFakeSink(alice, 10)
	register(t, h, sink)

	sink.Close() // seperti user yang menutup tab
	h.Publish([]uuid.UUID{alice}, Event{Type: EventMessageNew, Payload: "halo"})

	if got := sink.Enqueue(nil); got != Gone {
		t.Fatalf("koneksi tertutup harus melapor Gone, dapat %v", got)
	}
	if sink.count() != 0 {
		t.Fatal("koneksi tertutup tidak boleh menerima apa pun")
	}
}

func TestDeliveryStatesAreDistinct(t *testing.T) {
	sink := newFakeSink(uuid.New(), 1)

	if got := sink.Enqueue([]byte("satu")); got != Delivered {
		t.Fatalf("antrean kosong harus Delivered, dapat %v", got)
	}
	if got := sink.Enqueue([]byte("dua")); got != Backpressure {
		t.Fatalf("antrean penuh harus Backpressure, dapat %v", got)
	}

	sink.Close()
	if got := sink.Enqueue([]byte("tiga")); got != Gone {
		t.Fatalf("setelah ditutup harus Gone, dapat %v", got)
	}
}

// ---------- pencabutan sesi (Fase 10) ----------

// Mengganti password harus menutup semua perangkat KECUALI yang sedang dipakai
// menekan tombolnya.
//
// Kalau penyaringnya memakai id pengguna saja — satu-satunya nama yang dimiliki
// hub sebelum Fase 10 — orang yang baru saja mengamankan akunnya akan langsung
// dikeluarkan dari layar yang sedang dia buka.
func TestGantiPasswordMenutupSemuaSesiKecualiYangDipakai(t *testing.T) {
	h := testHub(t)
	me, orangLain := uuid.New(), uuid.New()

	ini := newFakeSession(me, "sesi-ini", 4)
	ponsel := newFakeSession(me, "sesi-ponsel", 4)
	asing := newFakeSession(me, "sesi-asing", 4)
	tetangga := newFakeSession(orangLain, "sesi-tetangga", 4)

	for _, s := range []*fakeSink{ini, ponsel, asing, tetangga} {
		register(t, h, s)
	}

	h.RevokeSessionsExcept(me, []byte("sesi-ini"))

	if ini.isClosed() {
		t.Error("sesi yang sedang dipakai ikut ditutup")
	}
	if !ponsel.isClosed() || !asing.isClosed() {
		t.Error("ada sesi lain yang tidak ditutup")
	}
	if tetangga.isClosed() {
		t.Error("koneksi milik orang lain ikut ditutup")
	}
}

// Yang ditutup menerima keterangannya lebih dulu. Tanpa itu, satu-satunya yang
// dilihat orang adalah aplikasi yang tiba-tiba memutus sambungan tanpa sebab —
// dan aplikasi yang memutus sambungan tanpa sebab terlihat persis seperti
// aplikasi yang rusak.
func TestSesiYangDicabutMenerimaKabarnyaLebihDulu(t *testing.T) {
	h := testHub(t)
	me := uuid.New()

	c := newFakeSession(me, "sesi-lama", 4)
	register(t, h, c)

	h.RevokeSessionsExcept(me, nil)

	if !c.isClosed() {
		t.Fatal("koneksi tidak ditutup")
	}
	if c.count() != 1 {
		t.Fatalf("mau satu pesan perpisahan, dapat %d", c.count())
	}

	var ev Event
	if err := json.Unmarshal(c.last(), &ev); err != nil {
		t.Fatalf("pesan perpisahan bukan JSON yang sah: %v", err)
	}
	if ev.Type != EventSessionRevoked {
		t.Fatalf("mau %q, dapat %q", EventSessionRevoked, ev.Type)
	}
}

// Mencabut satu perangkat adalah penyaring yang BERLAWANAN: satu ditutup,
// sisanya tidak disentuh. Dua method terpisah justru supaya tidak ada pemanggil
// yang harus mengingat bendera — dan bendera yang salah di sini berarti
// mengeluarkan orang dari semua perangkatnya saat dia cuma ingin mengeluarkan
// satu.
func TestMencabutSatuPerangkatTidakMenyentuhYangLain(t *testing.T) {
	h := testHub(t)
	me := uuid.New()

	laptop := newFakeSession(me, "sesi-laptop", 4)
	ponsel := newFakeSession(me, "sesi-ponsel", 4)
	register(t, h, laptop)
	register(t, h, ponsel)

	h.RevokeSession(me, []byte("sesi-ponsel"))

	if !ponsel.isClosed() {
		t.Error("perangkat yang dicabut tidak ditutup")
	}
	if laptop.isClosed() {
		t.Error("perangkat lain ikut ditutup")
	}
}

// Dua tab pada perangkat yang sama berbagi satu cookie, jadi keduanya memenuhi
// syarat pencabutan yang sama. Mencabut perangkat itu harus menutup keduanya —
// menyisakan satu tab hidup berarti sesi yang sudah dicabut masih menerima
// setiap pesan yang masuk.
func TestMencabutPerangkatMenutupSemuaTabnya(t *testing.T) {
	h := testHub(t)
	me := uuid.New()

	tabSatu := newFakeSession(me, "sesi-laptop", 4)
	tabDua := newFakeSession(me, "sesi-laptop", 4)
	register(t, h, tabSatu)
	register(t, h, tabDua)

	h.RevokeSession(me, []byte("sesi-laptop"))

	if !tabSatu.isClosed() || !tabDua.isClosed() {
		t.Error("ada tab yang tertinggal hidup setelah perangkatnya dicabut")
	}
}

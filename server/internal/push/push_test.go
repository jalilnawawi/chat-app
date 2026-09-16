package push

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/metrics"
	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

type fakePresence struct {
	online []uuid.UUID
	// semuaOnline membuat tiap penerima dianggap sedang terhubung, sehingga
	// tidak ada kabar yang berlanjut sampai menyentuh database.
	semuaOnline bool
}

func (f fakePresence) OnlineAmong(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	if f.semuaOnline {
		return ids, nil
	}
	out := []uuid.UUID{}
	for _, id := range ids {
		if slices.Contains(f.online, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

// fakeStore memenuhi Subscriptions tanpa database. Hanya BusyAmong yang pernah
// dipanggil dari jalur penyaringan penerima; tiga sisanya baru terpakai setelah
// penyaringan itu memutuskan ada yang layak dibangunkan.
type fakeStore struct {
	busy []uuid.UUID
}

func (f fakeStore) BusyAmong(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	out := []uuid.UUID{}
	for _, id := range ids {
		if slices.Contains(f.busy, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (fakeStore) SubscriptionsOf(context.Context, []uuid.UUID) ([]store.PushSubscription, error) {
	return nil, nil
}
func (fakeStore) DeleteSubscription(context.Context, string) error        { return nil }
func (fakeStore) MarkSubscriptionDelivered(context.Context, string) error { return nil }

// newTestDispatcher merakit Dispatcher tanpa menyalakan pekerjanya. Yang diuji
// di sini adalah penyaringan penerima, dan itu tidak menyentuh database maupun
// jaringan.
func newTestDispatcher(presence Presence, rule ratelimit.Rule, busy ...uuid.UUID) *Dispatcher {
	return &Dispatcher{
		presence: presence,
		store:    fakeStore{busy: busy},
		limiter:  ratelimit.NewMemory(),
		rule:     rule,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		m:        metrics.New(),
	}
}

// Aturan pertama notifikasi: orang yang tabnya terbuka tidak perlu dibangunkan.
func TestYangSedangOnlineTidakDibangunkan(t *testing.T) {
	online, offline := uuid.New(), uuid.New()
	d := newTestDispatcher(fakePresence{online: []uuid.UUID{online}}, ratelimit.PerMinute(5, 60))

	got, err := d.awake(t.Context(), Notification{
		Recipients:     []uuid.UUID{online, offline},
		ConversationID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("awake: %v", err)
	}

	if len(got) != 1 || got[0] != offline {
		t.Fatalf("penerima = %v, mau hanya %v", got, offline)
	}
}

// Aturan kedua: dua puluh pesan beruntun adalah satu kabar, bukan dua puluh.
func TestPercakapanYangSamaTidakBerderingBerkali(t *testing.T) {
	orang := uuid.New()
	conv := uuid.New()

	// Burst 2: dua kabar pertama lewat, sisanya diredam sampai token terisi.
	d := newTestDispatcher(fakePresence{}, ratelimit.PerMinute(2, 2))

	lolos := 0
	for range 10 {
		got, err := d.awake(t.Context(), Notification{
			Recipients:     []uuid.UUID{orang},
			ConversationID: conv,
		})
		if err != nil {
			t.Fatalf("awake: %v", err)
		}
		lolos += len(got)
	}

	if lolos != 2 {
		t.Errorf("%d kabar lolos, mau 2 — peredam dering tidak bekerja", lolos)
	}
}

// Peredam berlaku per percakapan, bukan per orang: pesan dari ruang lain
// adalah kabar yang benar-benar baru dan tetap harus sampai.
func TestPeredamTidakMenutupPercakapanLain(t *testing.T) {
	orang := uuid.New()
	d := newTestDispatcher(fakePresence{}, ratelimit.PerMinute(1, 1))

	for i := range 3 {
		got, err := d.awake(t.Context(), Notification{
			Recipients:     []uuid.UUID{orang},
			ConversationID: uuid.New(),
		})
		if err != nil {
			t.Fatalf("awake: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("percakapan ke-%d diredam padahal baru", i+1)
		}
	}
}

// Dispatcher nil adalah bentuk "fitur ini dimatikan". Semua jalurnya harus
// aman, karena alternatifnya adalah menyebar pemeriksaan nil ke tiap pemanggil.
func TestDispatcherNilAmanDipakai(t *testing.T) {
	var d *Dispatcher

	if d.Enabled() {
		t.Error("Dispatcher nil melaporkan dirinya aktif")
	}
	if d.PublicKey() != "" {
		t.Error("Dispatcher nil mengembalikan kunci")
	}
	d.Enqueue(Notification{Recipients: []uuid.UUID{uuid.New()}})
	d.Close()
}

// Menitipkan kabar SETELAH pengirimnya ditutup harus diam-diam dibuang, bukan
// panik.
//
// Celahnya nyata: Close dipanggil saat instance pamit, sedangkan permintaan HTTP
// yang sedang berjalan bisa saja baru sampai ke baris Enqueue-nya saat itu juga.
// Mengirim ke channel yang sudah ditutup selalu panik, dan panik di sana
// menjatuhkan seluruh proses di tengah rolling deploy.
func TestEnqueueSetelahCloseTidakPanik(t *testing.T) {
	d := New(nil, fakePresence{semuaOnline: true}, ratelimit.NewMemory(), ratelimit.PerMinute(5, 60),
		"kunci-publik", "kunci-privat", "mailto:a@b.c",
		slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.New())
	if d == nil {
		t.Fatal("Dispatcher tidak dibuat padahal kunci lengkap")
	}

	d.Close()
	// Close kedua juga harus aman — shutdown bisa dipicu dari lebih dari satu
	// jalur.
	d.Close()

	d.Enqueue(Notification{Recipients: []uuid.UUID{uuid.New()}, ConversationID: uuid.New()})
}

// Beberapa pengirim yang bertabrakan dengan penutupan adalah bentuk paling
// mungkin dari celah itu di dunia nyata. Dijalankan dengan -race.
func TestEnqueueBersamaanDenganCloseAman(t *testing.T) {
	// Semua penerima dianggap online, jadi pekerja berhenti di penyaringan dan
	// tidak pernah menyentuh database. Yang diuji di sini adalah jalur
	// antreannya, bukan pengirimannya.
	d := New(nil, fakePresence{semuaOnline: true}, ratelimit.NewMemory(), ratelimit.PerMinute(5, 60),
		"kunci-publik", "kunci-privat", "mailto:a@b.c",
		slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.New())

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Enqueue(Notification{Recipients: []uuid.UUID{uuid.New()}, ConversationID: uuid.New()})
		}()
	}

	d.Close()
	wg.Wait()
}

// Kunci yang tidak lengkap berarti fitur mati, bukan fitur yang separuh jalan.
func TestNewTanpaKunciMengembalikanNil(t *testing.T) {
	if d := New(nil, fakePresence{}, nil, ratelimit.Rule{}, "", "", "", nil, nil); d != nil {
		t.Error("Dispatcher dibuat padahal kunci VAPID kosong")
	}
}

// Status "busy" adalah peredam KEDUA, di samping token bucket per percakapan.
// Inilah yang membuat status bukan sekadar hiasan: dia menyambung ke peredam
// dering yang sudah ada sejak Fase 7.
func TestYangSedangSibukTidakDibangunkan(t *testing.T) {
	sibuk, santai := uuid.New(), uuid.New()
	d := newTestDispatcher(fakePresence{}, ratelimit.PerMinute(5, 60), sibuk)

	got, err := d.awake(t.Context(), Notification{
		Recipients:     []uuid.UUID{sibuk, santai},
		ConversationID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("awake: %v", err)
	}
	if len(got) != 1 || got[0] != santai {
		t.Fatalf("mau hanya yang tidak sibuk, dapat %v", got)
	}
}

// Sebutan menembus KEDUA peredam. Aturannya sama persis dengan keputusan Fase
// 9: dering biasa boleh diredam, panggilan yang menyebut nama seseorang tidak.
// Orang yang memasang "sedang rapat" justru yang paling butuh tahu kalau
// namanya dipanggil di tengah rapat itu.
func TestSebutanMenembusStatusSibuk(t *testing.T) {
	sibuk := uuid.New()
	d := newTestDispatcher(fakePresence{}, ratelimit.PerMinute(5, 60), sibuk)

	got, err := d.awake(t.Context(), Notification{
		Recipients:     []uuid.UUID{sibuk},
		Mentioned:      []uuid.UUID{sibuk},
		ConversationID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("awake: %v", err)
	}
	if len(got) != 1 || got[0] != sibuk {
		t.Fatalf("sebutan tidak menembus status sibuk: %v", got)
	}
}

// Yang sibuk TAPI sedang online tetap tidak dibangunkan, dan urutan
// pemeriksaannya yang menjamin itu: presence disaring lebih dulu, sendiri,
// sebelum status ditanyakan sama sekali.
func TestYangSibukDanOnlineTidakMenyentuhPemeriksaanStatus(t *testing.T) {
	sibuk := uuid.New()
	d := newTestDispatcher(fakePresence{online: []uuid.UUID{sibuk}}, ratelimit.PerMinute(5, 60), sibuk)

	got, err := d.awake(t.Context(), Notification{
		Recipients:     []uuid.UUID{sibuk},
		Mentioned:      []uuid.UUID{sibuk},
		ConversationID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("awake: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("yang sedang online ikut dibangunkan: %v", got)
	}
}

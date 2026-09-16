package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/metrics"
)

// registry adalah tabel koneksi milik SATU proses.
//
// Baik Memory maupun Redis memakainya: berapa pun jumlah instance, byte pada
// akhirnya harus ditulis ke socket yang dipegang proses ini. Yang membedakan
// kedua implementasi hanyalah bagaimana event sampai ke registry — langsung,
// atau lewat langganan Redis.
type registry struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[Sink]struct{} // satu user bisa punya banyak tab
	log     *slog.Logger
	m       *metrics.Metrics
}

func newRegistry(log *slog.Logger, m *metrics.Metrics) *registry {
	return &registry{clients: make(map[uuid.UUID]map[Sink]struct{}), log: log, m: m}
}

// add mengembalikan true bila ini koneksi pertama milik user DI INSTANCE INI.
func (r *registry) add(c Sink) (firstLocal bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conns, ok := r.clients[c.UserID()]
	if !ok {
		conns = make(map[Sink]struct{})
		r.clients[c.UserID()] = conns
	}
	conns[c] = struct{}{}
	return len(conns) == 1
}

// remove mengembalikan true bila itu koneksi terakhir milik user DI INSTANCE INI.
func (r *registry) remove(c Sink) (lastLocal bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conns, ok := r.clients[c.UserID()]
	if !ok {
		return false
	}
	if _, tracked := conns[c]; !tracked {
		return false
	}
	delete(conns, c)
	if len(conns) == 0 {
		delete(r.clients, c.UserID())
		return true
	}
	return false
}

func (r *registry) hasLocal(id uuid.UUID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients[id]) > 0
}

func (r *registry) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n := 0
	for _, conns := range r.clients {
		n += len(conns)
	}
	return n
}

// deliver menaruh payload di antrean semua koneksi lokal milik targets.
//
// Backpressure: bila buffer sebuah koneksi penuh, koneksi itu diminta menutup
// diri, bukan ditunggu. Menunggu client lambat akan menahan seluruh siaran dan
// menular ke semua orang di ruang yang sama. Client yang terputus akan
// reconnect dan menyusul lewat mekanisme resume.
//
// Yang TIDAK dilakukan di sini: melepas koneksi dari registry. Pelepasan
// dikerjakan satu tempat saja — defer di ws.Handler.Serve — supaya siaran
// presence "offline" tetap terkirim. Melepas di dua tempat pernah membuat
// Unregister kedua mengembalikan false dan status offline tidak pernah
// disiarkan.
func (r *registry) deliver(targets []uuid.UUID, payload []byte) (receivers int) {
	r.mu.RLock()
	var slow []Sink
	for _, id := range targets {
		for c := range r.clients[id] {
			switch c.Enqueue(payload) {
			case Delivered:
				receivers++
			case Backpressure:
				slow = append(slow, c)
			case Gone:
				// Koneksi sudah ditutup di tempat lain. Diam saja — pelepasan
				// dari registry sudah dalam perjalanan.
			}
		}
	}
	r.mu.RUnlock()

	for _, c := range slow {
		r.log.Warn("client lambat, koneksi ditutup", "user", c.UserID())
		r.m.WSSlowDropped.Inc()
		c.Close()
	}
	return receivers
}

// revoke menutup koneksi lokal milik userID, disaring oleh salah satu dari dua
// penyaring yang berlawanan: `keep` menyisakan satu sesi, `only` menutup satu
// sesi. Keduanya nil berarti seluruh koneksi milik user itu.
//
// Payload perpisahannya disusun di sini, sekali, lalu dibagikan — dan dia
// dikirim lewat Kick, bukan Enqueue lalu Close: yang kedua kehilangan pesannya
// lebih sering daripada tidak. Lihat catatan di Sink.Kick.
//
// Yang TIDAK dilakukan: melepas koneksi dari registry. Pelepasan tetap satu
// tempat saja — defer di ws.Handler.Serve — dengan alasan yang sama persis
// seperti pada deliver di atas.
func (r *registry) revoke(userID uuid.UUID, keep, only []byte) (closed int) {
	notice, err := json.Marshal(Event{
		Type:    EventSessionRevoked,
		Payload: map[string]any{"reason": "session_revoked"},
	})
	if err != nil {
		return 0
	}

	r.mu.RLock()
	var doomed []Sink
	for c := range r.clients[userID] {
		if keep != nil && bytes.Equal(c.SessionHash(), keep) {
			continue
		}
		if only != nil && !bytes.Equal(c.SessionHash(), only) {
			continue
		}
		doomed = append(doomed, c)
	}
	r.mu.RUnlock()

	for _, c := range doomed {
		c.Kick(notice)
	}
	if len(doomed) > 0 {
		r.log.Info("sesi dicabut, koneksi ditutup", "user", userID, "koneksi", len(doomed))
	}
	return len(doomed)
}

// onlineLocal menyaring ids ke yang punya koneksi di instance ini.
func (r *registry) onlineLocal(ids []uuid.UUID) []uuid.UUID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	online := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if len(r.clients[id]) > 0 {
			online = append(online, id)
		}
	}
	return online
}

// localUsers adalah daftar user yang punya koneksi di sini. Dipakai penyegar
// presence Redis.
func (r *registry) localUsers() []uuid.UUID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]uuid.UUID, 0, len(r.clients))
	for id := range r.clients {
		ids = append(ids, id)
	}
	return ids
}

func (r *registry) snapshot() []Sink {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Sink, 0, len(r.clients))
	for _, conns := range r.clients {
		for c := range conns {
			out = append(out, c)
		}
	}
	return out
}

// drain memberi tahu tiap koneksi bahwa server pamit, lalu menutupnya tersebar
// merata sepanjang period.
//
// Penyebarannya yang penting. Kalau 1000 koneksi ditutup dalam milidetik yang
// sama, 1000 browser akan mencoba reconnect dalam milidetik yang sama juga, dan
// instance pengganti menerima badai handshake + query sync sekaligus — persis
// saat dia paling belum panas. Menyebar penutupan mengubah lonjakan itu jadi
// aliran. Jitter kecil per koneksi mencegah client tetap berbaris rapi.
func (r *registry) drain(ctx context.Context, period time.Duration) {
	conns := r.snapshot()
	if len(conns) == 0 {
		return
	}

	notice, err := json.Marshal(Event{
		Type:    EventServerShutdown,
		Payload: map[string]any{"reason": "deploy"},
	})
	if err == nil {
		for _, c := range conns {
			c.Enqueue(notice)
		}
	}

	gap := period / time.Duration(len(conns))
	r.log.Info("menguras koneksi websocket",
		"jumlah", len(conns), "periode", period.String(), "jeda", gap.String())

	for _, c := range conns {
		// Jeda diacak di sekitar gap, bukan ditambahkan ke gap, supaya total
		// waktu kuras tetap ~period alih-alih membengkak jadi dua kalinya.
		wait := gap/2 + time.Duration(rand.Int64N(int64(gap)+1))
		select {
		case <-ctx.Done():
			// Waktu habis: sisanya ditutup sekaligus. Lebih baik client
			// reconnect berbarengan daripada proses ditembak paksa di tengah
			// penulisan.
			for _, rest := range conns {
				rest.Close()
			}
			return
		case <-time.After(wait):
			c.Close()
		}
	}
}

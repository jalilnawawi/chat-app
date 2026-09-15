// Package push membangunkan orang yang sedang tidak membuka aplikasi.
//
// Ini fitur yang gampang dibuat menyebalkan, jadi aturannya dipasang di satu
// tempat dan sengaja ketat:
//
//  1. Hanya untuk yang BENAR-BENAR tidak terhubung. Presence sudah tahu
//     jawabannya lintas instance; orang yang tabnya terbuka di sebelah tidak
//     perlu diberi tahu dua kali.
//  2. Satu percakapan tidak boleh berbunyi berkali-kali. Dua puluh pesan
//     beruntun dari satu orang adalah SATU kabar, bukan dua puluh.
//  3. Pengiriman tidak pernah menahan pengirim pesan. Layanan push milik
//     vendor browser bisa memakan ratusan milidetik, dan orang yang menekan
//     "kirim" tidak sedang menunggu itu.
//
// Poin ketiga yang menentukan bentuk paket ini: sebuah antrean dengan beberapa
// pekerja, dan kiriman yang DIBUANG kalau antreannya penuh. Notifikasi adalah
// kenyamanan; pesannya sendiri sudah tersimpan aman dan akan terlihat begitu
// aplikasi dibuka.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/metrics"
	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// ttl adalah umur notifikasi di layanan push. Lewat dari ini, kabar tentang
// pesan chat sudah basi — orangnya akan melihat sendiri saat membuka aplikasi.
const ttl = 6 * time.Hour

// sendTimeout membatasi satu kiriman ke layanan push. Endpoint vendor sesekali
// menggantung, dan satu pekerja yang menunggu selamanya berarti satu pekerja
// yang hilang.
const sendTimeout = 10 * time.Second

// queueSize adalah kedalaman antrean kiriman. Cukup untuk meredam lonjakan
// wajar, cukup dangkal untuk ketahuan kalau ada yang salah.
const queueSize = 1024

// workers dipilih dari sifat pekerjaannya: menunggu jaringan, bukan memakai
// CPU. Delapan kiriman bersamaan sudah jauh melebihi laju pesan yang pantas
// membangunkan orang.
const workers = 8

// Presence adalah bagian dari hub yang dibutuhkan paket ini — dan hanya itu.
// Mendeklarasikannya di sini, bukan menerima hub.Broadcaster utuh, membuat
// ketergantungannya jujur dan gampang dipalsukan saat pengujian.
type Presence interface {
	OnlineAmong(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
}

// Notification adalah satu kabar yang mungkin layak dikirim. "Mungkin", karena
// penyaringannya baru terjadi di pekerja: siapa yang online dan siapa yang baru
// saja dibangunkan hanya relevan pada detik pengirimannya.
type Notification struct {
	// Recipients adalah kandidat: anggota percakapan selain pengirim.
	Recipients     []uuid.UUID
	ConversationID uuid.UUID
	MessageID      uuid.UUID
	Title          string
	Body           string
}

type Dispatcher struct {
	store    *store.Store
	presence Presence
	limiter  ratelimit.Limiter
	rule     ratelimit.Rule
	log      *slog.Logger
	m        *metrics.Metrics

	publicKey  string
	privateKey string
	subject    string

	queue chan Notification
	wg    sync.WaitGroup

	// mu menjaga queue dari penutupan yang bertabrakan dengan pengiriman.
	//
	// select-dengan-default TIDAK cukup: dia melindungi dari antrean penuh,
	// bukan dari antrean yang sudah ditutup — mengirim ke channel tertutup
	// selalu panik, dan panik itu menjatuhkan seluruh proses. Celahnya nyata:
	// Close dipanggil setelah koneksi dikuras, sedangkan permintaan HTTP yang
	// sedang berjalan bisa saja baru sampai ke baris Enqueue-nya saat itu.
	mu     sync.RWMutex
	closed bool
}

// New menyalakan pengirim notifikasi. Dikembalikan nil bila kunci VAPID belum
// diisi — lihat catatan di Dispatcher.Enqueue soal kenapa nil aman dipakai.
func New(
	st *store.Store,
	presence Presence,
	limiter ratelimit.Limiter,
	rule ratelimit.Rule,
	publicKey, privateKey, subject string,
	log *slog.Logger,
	m *metrics.Metrics,
) *Dispatcher {
	if publicKey == "" || privateKey == "" {
		return nil
	}

	d := &Dispatcher{
		store:      st,
		presence:   presence,
		limiter:    limiter,
		rule:       rule,
		log:        log,
		m:          m,
		publicKey:  publicKey,
		privateKey: privateKey,
		subject:    subject,
		queue:      make(chan Notification, queueSize),
	}

	d.wg.Add(workers)
	for range workers {
		go d.work()
	}
	return d
}

// PublicKey adalah kunci yang diminta browser saat mendaftar. Aman disebarkan;
// pasangannya yang rahasia tidak pernah meninggalkan server.
func (d *Dispatcher) PublicKey() string {
	if d == nil {
		return ""
	}
	return d.publicKey
}

// Enabled melaporkan apakah notifikasi menyala di instance ini.
func (d *Dispatcher) Enabled() bool { return d != nil }

// Enqueue menitipkan kabar, tanpa pernah memblokir.
//
// Aman dipanggil pada Dispatcher nil — itulah bentuk "fitur ini dimatikan".
// Alternatifnya adalah menyebar `if push != nil` ke setiap pemanggil, dan
// pemeriksaan seperti itu selalu ada satu yang terlupa.
func (d *Dispatcher) Enqueue(n Notification) {
	if d == nil || len(n.Recipients) == 0 {
		return
	}

	// RLock, bukan Lock: banyak pengirim boleh masuk bersamaan, dan hanya
	// Close yang perlu menyingkirkan mereka semua.
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		// Instance sedang pamit. Pesannya sudah tersimpan dan sudah disiarkan;
		// kabar untuk yang offline akan menyusul dari instance lain saat
		// pesan berikutnya masuk.
		d.m.PushDropped.Inc()
		return
	}

	select {
	case d.queue <- n:
	default:
		// Antrean penuh berarti layanan push sedang lambat atau ada lonjakan
		// besar. Pesannya sendiri sudah tersimpan dan sudah disiarkan ke yang
		// online; yang hilang di sini cuma deringnya.
		d.m.PushDropped.Inc()
		d.log.Warn("antrean notifikasi penuh, kabar dibuang",
			"conversation", n.ConversationID)
	}
}

func (d *Dispatcher) work() {
	defer d.wg.Done()
	for n := range d.queue {
		d.deliver(n)
	}
}

func (d *Dispatcher) deliver(n Notification) {
	// Konteksnya milik pekerja, bukan milik permintaan HTTP yang memicunya —
	// permintaan itu sudah lama selesai dan konteksnya sudah dibatalkan.
	//
	// Batas ini hanya menutupi pencarian di database. Kirimannya sendiri
	// TIDAK ikut di bawahnya: sebuah kabar bisa punya banyak penerima, tiap
	// kiriman berjalan berurutan dengan batas sepuluh detik, dan satu anggaran
	// bersama akan membuat penerima terakhir gagal hanya karena dia yang paling
	// belakang antre.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	targets, err := d.awake(ctx, n)
	if err != nil {
		d.log.Error("menyaring penerima notifikasi", "err", err)
		return
	}
	if len(targets) == 0 {
		return
	}

	subs, err := d.store.SubscriptionsOf(ctx, targets)
	if err != nil {
		d.log.Error("mengambil langganan push", "err", err)
		return
	}

	payload, err := json.Marshal(map[string]any{
		"title":          n.Title,
		"body":           n.Body,
		"conversationId": n.ConversationID,
		"messageId":      n.MessageID,
	})
	if err != nil {
		return
	}

	for _, sub := range subs {
		d.send(sub, payload, n.ConversationID)
	}
}

// awake mengembalikan penerima yang pantas dibangunkan: tidak sedang terhubung
// di instance mana pun, dan belum dibangunkan untuk percakapan ini belakangan.
func (d *Dispatcher) awake(ctx context.Context, n Notification) ([]uuid.UUID, error) {
	online, err := d.presence.OnlineAmong(ctx, n.Recipients)
	if err != nil {
		return nil, err
	}
	connected := make(map[uuid.UUID]struct{}, len(online))
	for _, id := range online {
		connected[id] = struct{}{}
	}

	out := make([]uuid.UUID, 0, len(n.Recipients))
	for _, id := range n.Recipients {
		if _, ok := connected[id]; ok {
			d.m.PushSkipped.WithLabelValues("online").Inc()
			continue
		}

		// Peredam dering memakai token bucket yang sama dengan kuota lain —
		// termasuk versi Redis-nya. Itu bukan penghematan kode belaka: tanpa
		// penghitung yang dibagi, dua instance akan membangunkan orang yang
		// sama dua kali untuk percakapan yang sama.
		key := "push:" + id.String() + ":" + n.ConversationID.String()
		res, err := d.limiter.Allow(ctx, key, d.rule)
		if err != nil {
			// Peredam yang bermasalah tidak boleh membungkam notifikasi:
			// terlalu berisik lebih baik daripada diam-diam tidak sampai.
			d.log.Warn("peredam notifikasi gagal", "err", err)
		} else if !res.Allowed {
			d.m.PushSkipped.WithLabelValues("debounce").Inc()
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

func (d *Dispatcher) send(sub store.PushSubscription, payload []byte, convID uuid.UUID) {
	sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	start := time.Now()
	res, err := webpush.SendNotificationWithContext(sendCtx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		Subscriber:      d.subject,
		VAPIDPublicKey:  d.publicKey,
		VAPIDPrivateKey: d.privateKey,
		TTL:             int(ttl.Seconds()),
		Urgency:         webpush.UrgencyHigh,

		// Topic membuat layanan push MENGGANTI kabar lama yang belum sempat
		// diantar dengan yang baru, selama keduanya untuk percakapan yang sama.
		// Ponsel yang baru menyala setelah lama mati jadi menampilkan kabar
		// terakhir tiap percakapan, bukan menumpahkan seluruh riwayat dering.
		Topic: strings.ReplaceAll(convID.String(), "-", ""),
	})
	d.m.PushDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		// Konteks habis saat shutdown bukan kegagalan yang perlu ditakuti.
		if !errors.Is(err, context.Canceled) {
			d.m.PushSent.WithLabelValues("error").Inc()
			d.log.Warn("kirim notifikasi gagal", "err", err)
		}
		return
	}
	defer res.Body.Close() //nolint:errcheck

	// Konteks terpisah untuk tindak lanjut di database. Memakai sisa anggaran
	// kiriman berarti kiriman yang lambat menyisakan waktu hampir nol untuk
	// membersihkan langganan mati — persis kasus yang paling butuh dibersihkan.
	dbCtx, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDB()

	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		// Browser sudah mencabut langganan ini — orangnya menghapus data situs,
		// mencopot aplikasi, atau mematikan izin. Menyimpannya berarti mengirim
		// ke alamat mati selamanya.
		d.m.PushSent.WithLabelValues("expired").Inc()
		if err := d.store.DeleteSubscription(dbCtx, sub.Endpoint); err != nil {
			d.log.Warn("membuang langganan mati", "err", err)
		}

	case res.StatusCode >= 200 && res.StatusCode < 300:
		d.m.PushSent.WithLabelValues("ok").Inc()
		if err := d.store.MarkSubscriptionDelivered(dbCtx, sub.Endpoint); err != nil {
			d.log.Debug("menandai kiriman berhasil", "err", err)
		}

	default:
		d.m.PushSent.WithLabelValues("error").Inc()
		d.log.Warn("layanan push menolak", "status", res.Status)
	}
}

// Close menutup antrean dan menunggu kiriman yang sedang berjalan selesai.
// Dipanggil saat shutdown, setelah koneksi WebSocket dikuras: kabar terakhir
// sebelum instance pamit tetap layak diantar.
func (d *Dispatcher) Close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.closed = true
	close(d.queue)
	d.mu.Unlock()

	// Menunggu di LUAR kunci: pekerja yang sedang mengirim tidak memegang
	// kunci ini, tapi menahannya selama menunggu akan membuat Enqueue yang
	// kebetulan bersamaan ikut terblokir sampai kiriman terakhir selesai.
	d.wg.Wait()
}

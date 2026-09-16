// Package hub menyiarkan event realtime ke koneksi WebSocket yang aktif.
//
// Semua siaran melewati SATU pintu: interface Broadcaster. Ada dua implementasi
// yang memenuhinya:
//
//   - Memory — koneksi disimpan di memori proses. Cukup untuk satu instance,
//     dan tetap jadi jalur default saat REDIS_URL kosong supaya `go run` lokal
//     tidak menuntut infrastruktur tambahan.
//   - Redis — siaran lewat pub/sub dan presence lewat key ber-TTL, sehingga
//     beberapa instance bisa melayani user yang sama.
//
// Handler HTTP dan WebSocket tidak tahu mana yang sedang dipakai. Itu bukan
// kebetulan: pilihan transport adalah keputusan deployment, bukan keputusan
// yang boleh bocor ke logika percakapan.
package hub

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Tipe event yang dikirim server ke client.
const (
	EventMessageNew      = "message.new"
	EventMessageUpdated  = "message.updated"
	EventConversationNew = "conversation.new"
	EventReadUpdated     = "read.updated"
	EventTyping          = "typing"
	EventPresence        = "presence"
	EventSyncComplete    = "sync.complete"
	// EventSyncBatch membawa banyak pesan susulan dalam satu frame. Lihat
	// alasannya di ws.Handler.handleSync.
	EventSyncBatch        = "sync.batch"
	EventPresenceSnapshot = "presence.snapshot"
	EventError            = "error"

	// Dua event untuk satu tombol, karena keduanya membawa SATU perubahan:
	// siapa, emoji apa, pada pesan mana.
	//
	// Yang disiarkan sengaja bukan ringkasan jadi ("👍 3"), melainkan
	// perubahannya. Ringkasan mengandung "apakah AKU ikut", dan siaran adalah
	// satu payload yang sama untuk semua orang — memasukkan jawaban yang
	// berbeda per pembaca ke dalamnya berarti mengirim jawaban milik orang lain
	// ke setiap orang. Client menerapkan selisihnya pada hitungan yang sudah
	// dia punya.
	EventReactionAdded   = "reaction.added"
	EventReactionRemoved = "reaction.removed"

	// EventReactionBatch adalah jalur menyusul setelah reconnect, dan ini
	// dikirim ke SATU koneksi — jadi ringkasan lengkap beserta "apakah aku
	// ikut" memang boleh ada di sini. Lihat store/reactions.go.
	EventReactionBatch = "reaction.batch"

	// EventServerShutdown dikirim tepat sebelum instance menutup koneksi saat
	// rolling deploy. Client memakainya untuk membedakan "server pamit, sambung
	// lagi sekarang dengan jitter" dari "jaringan putus, mundur perlahan".
	EventServerShutdown = "server.shutdown"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Broadcaster adalah satu-satunya jalan keluar event ke client.
type Broadcaster interface {
	// Register mencatat koneksi. firstConnection true bila setelah pendaftaran
	// ini user berubah dari offline menjadi online SECARA GLOBAL — bukan cuma
	// di instance ini. Itu sinyal untuk menyiarkan presence.
	Register(ctx context.Context, c Sink) (firstConnection bool, err error)

	// Unregister melepas koneksi. lastConnection true bila user tidak lagi
	// punya koneksi di instance mana pun.
	Unregister(ctx context.Context, c Sink) (lastConnection bool, err error)

	// Publish mengirim event ke semua koneksi milik user pada daftar target.
	// Sengaja tanpa error: siaran bersifat best-effort dan pemanggilnya adalah
	// handler yang sudah menyelesaikan pekerjaan utamanya (pesan sudah tersimpan
	// di database). Kegagalan dicatat ke log dan metrik, tidak dilempar ke user,
	// karena client akan menyusul lewat resume saat reconnect.
	Publish(targets []uuid.UUID, ev Event)

	// OnlineAmong menyaring daftar user, mengembalikan yang sedang terhubung.
	OnlineAmong(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)

	// LocalConnections adalah jumlah koneksi di instance ini — dipakai probe
	// kesiapan dan metrik.
	LocalConnections() int

	// Drain menutup semua koneksi lokal secara bertahap selama period.
	// Lihat catatan penyebaran beban di registry.drain.
	Drain(ctx context.Context, period time.Duration)

	Close() error
}

// Delivery membedakan DUA sebab sebuah event tidak jadi terkirim.
//
// Keduanya pernah dilaporkan sebagai satu nilai false yang sama, dan itu
// membuat metrik backpressure ikut menghitung setiap koneksi yang menutup
// normal — satu uji beban dengan seribu koneksi menghasilkan puluhan
// "client lambat" palsu. Alarm yang berbunyi saat tidak ada apa-apa adalah
// alarm yang akhirnya dimatikan orang, jadi kedua keadaan ini dipisahkan.
type Delivery int

const (
	// Delivered: payload masuk antrean kirim koneksi.
	Delivered Delivery = iota
	// Backpressure: buffer penuh, client benar-benar tidak menyusul.
	Backpressure
	// Gone: koneksi sudah ditutup. Bukan masalah, cuma balapan biasa antara
	// siaran yang sedang berjalan dan orang yang menutup tab.
	Gone
)

// Sink adalah sisi hub yang dilihat sebuah koneksi: tempat menaruh byte keluar.
type Sink interface {
	UserID() uuid.UUID

	// Enqueue menaruh payload di antrean kirim tanpa pernah memblokir.
	// Lihat catatan backpressure di registry.deliver.
	Enqueue(payload []byte) Delivery

	// Close meminta koneksi ditutup. Aman dipanggil berkali-kali.
	Close()
}

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

	// EventConversationUpdated membawa keadaan grup setelah dikelola: judulnya
	// dan daftar anggotanya.
	//
	// Terpisah dari catatan sistem yang menyertainya, dan keduanya memang
	// dikirim berpasangan. Catatan itu adalah KEJADIAN — dia masuk ke riwayat,
	// punya seq, dan tetap terbaca besok. Ini adalah KEADAAN sekarang, dan dia
	// tidak punya tempat di riwayat: daftar anggota hari ini bukan sesuatu yang
	// layak diulang di setiap halaman percakapan.
	EventConversationUpdated = "conversation.updated"

	// EventConversationRemoved dikirim HANYA kepada orang yang baru saja
	// berhenti jadi anggota. Tanpa ini, percakapan yang sudah bukan miliknya
	// tetap menggantung di sidebar sampai halamannya dimuat ulang — dan
	// mengkliknya menghasilkan 404 yang tidak bisa dijelaskan.
	EventConversationRemoved = "conversation.removed"
	EventReadUpdated         = "read.updated"
	EventTyping              = "typing"
	EventPresence            = "presence"
	EventSyncComplete        = "sync.complete"
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

	// EventStatus membawa status yang baru dipasang seseorang, dan
	// EventStatusSnapshot membawa keadaan awal saat koneksi dibuka.
	//
	// Keduanya menempuh jalur yang PERSIS SAMA dengan presence — siaran ke
	// ContactIDs, snapshot ke satu koneksi — dan itu disengaja: tidak ada
	// mekanisme fan-out kedua yang harus ikut benar. Yang berbeda cuma sumber
	// kebenarannya: presence dari koneksi yang hidup, status dari Postgres.
	EventStatus         = "status"
	EventStatusSnapshot = "status.snapshot"

	// EventUserUpdated membawa bentuk publik seseorang setelah nama tampilan
	// atau fotonya berubah.
	//
	// Tanpa dia, foto baru memang punya alamat baru — tapi tidak seorang pun
	// tahu alamat itu sampai halamannya dimuat ulang, dan cache setahun yang
	// dipasang pada byte avatar jadi benar dan tidak berguna sekaligus:
	// byte-nya tidak basi, penunjuknya yang basi.
	//
	// Terpisah dari EventStatus walau keduanya menempuh jalur yang sama, karena
	// keduanya berubah pada saat yang sangat berbeda: nama beberapa kali seumur
	// akun, status beberapa kali sehari.
	EventUserUpdated = "user.updated"

	// EventSessionRevoked dikirim ke koneksi yang sesinya baru saja dicabut,
	// tepat sebelum koneksi itu ditutup dari sisi server.
	//
	// Ini satu-satunya event yang arahnya kebalikan dari semua yang lain: dia
	// tidak menyampaikan sesuatu yang terjadi di percakapan, melainkan menyuruh
	// instance mana pun yang memegang sebuah sesi untuk mengakhirinya. Lihat
	// Broadcaster.RevokeSessions soal kenapa itu tidak cukup dikerjakan dengan
	// menghapus barisnya di database.
	EventSessionRevoked = "session.revoked"
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

	// RevokeSessionsExcept menutup setiap koneksi milik userID yang sesinya
	// BUKAN `keep`, di instance mana pun. `keep` nil berarti tidak ada yang
	// dikecualikan — bentuk yang dipakai pemulihan password.
	//
	// Ada karena menghapus baris di `sessions` saja tidak cukup. Koneksi
	// WebSocket yang sudah terlanjur terbuka dipegang di memori proses, dan
	// sejak Fase 6 proses itu bisa instance LAIN — baris sesi yang hilang
	// sementara koneksinya tetap hidup berarti orang yang password-nya baru saja
	// dicuri tetap terhubung, melihat setiap pesan yang masuk, sampai dia sendiri
	// yang memutuskan untuk reconnect.
	//
	// Jalurnya sama dengan siaran biasa, hanya arahnya kebalikan: ini tidak
	// menyampaikan sesuatu untuk dibaca client, melainkan perintah untuk instance
	// yang kebetulan memegang koneksinya.
	//
	// Sengaja tanpa error, seperti Publish: pekerjaan utamanya — mencabut sesi di
	// database — sudah selesai sebelum ini dipanggil, dan koneksi yang lolos
	// karenanya akan mati sendiri pada percobaan reconnect berikutnya.
	RevokeSessionsExcept(userID uuid.UUID, keep []byte)

	// RevokeSession menutup koneksi milik SATU sesi saja — bentuk yang dipakai
	// saat seseorang mengeluarkan satu perangkat dari daftar sesi aktifnya.
	//
	// Kebalikan dari yang di atas, dan keduanya memang dibutuhkan: yang satu
	// menyisakan satu, yang satu menutup satu. Menyatukannya jadi satu method
	// dengan bendera berarti setiap pemanggil harus mengingat benderanya, dan
	// bendera yang salah di sini berarti mengeluarkan orang dari semua
	// perangkatnya saat dia cuma ingin mengeluarkan satu.
	RevokeSession(userID uuid.UUID, hash []byte)

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

	// SessionHash adalah hash token sesi yang dipakai membuka koneksi ini —
	// nama yang membedakan dua tab milik ORANG YANG SAMA di perangkat yang
	// berbeda.
	//
	// UserID saja tidak cukup untuk RevokeSessions: mencabut satu perangkat
	// tanpa ini berarti mengeluarkan orangnya dari semua perangkatnya sekaligus,
	// termasuk yang sedang dia pakai untuk menekan tombolnya.
	SessionHash() []byte

	// Enqueue menaruh payload di antrean kirim tanpa pernah memblokir.
	// Lihat catatan backpressure di registry.deliver.
	Enqueue(payload []byte) Delivery

	// Kick mengirim satu payload terakhir lalu menutup koneksi SETELAH payload
	// itu benar-benar tertulis.
	//
	// Berbeda dari Enqueue lalu Close, yang tampak sama tapi tidak: Close
	// membatalkan konteks tulis seketika, dan pesan yang baru saja diantrekan
	// kalah balapan dengan pembatalan itu lebih sering daripada tidak. Yang
	// hilang justru satu-satunya keterangan yang dimiliki orang tentang kenapa
	// aplikasinya tiba-tiba mengeluarkan dia.
	Kick(payload []byte)

	// Close meminta koneksi ditutup. Aman dipanggil berkali-kali.
	Close()
}

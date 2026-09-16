// Package ws mengelola satu koneksi WebSocket: umur koneksi, heartbeat,
// backpressure, dan sinkronisasi setelah reconnect.
package ws

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/metrics"
)

const (
	// Buffer per koneksi. Cukup untuk menyerap lonjakan pendek; kalau sampai
	// penuh berarti client benar-benar tidak menyusul dan lebih baik diputus
	// daripada menahan siaran ke semua orang.
	sendBuffer = 64

	// Batas ukuran pesan masuk — pertahanan sederhana dari client nakal.
	readLimit = 32 * 1024

	pingInterval = 30 * time.Second
	pingTimeout  = 10 * time.Second
	writeTimeout = 10 * time.Second
)

// Client adalah satu koneksi WebSocket. Memenuhi hub.Sink.
type Client struct {
	userID uuid.UUID

	// sessionHash adalah hash token sesi yang dipakai membuka koneksi ini.
	//
	// Dibawa sampai ke sini karena pencabutan sesi bekerja per PERANGKAT, bukan
	// per orang: mengganti password harus menutup semua koneksi KECUALI yang
	// sedang dipakai menekan tombolnya. Tanpa nama yang membedakan dua tab milik
	// orang yang sama, satu-satunya pilihan adalah mengeluarkan orangnya dari
	// layar yang sedang dia buka.
	sessionHash []byte

	conn *websocket.Conn
	send chan []byte
	m    *metrics.Metrics

	// kick membawa satu pesan perpisahan. Berkapasitas satu dan hanya pernah
	// diisi sekali — koneksi yang sudah diminta pergi tidak punya pesan
	// perpisahan kedua.
	kick chan []byte

	// cancel membatalkan konteks yang dipakai readLoop dan writeLoop.
	cancel context.CancelFunc

	closeOnce sync.Once
	done      chan struct{}
}

func newClient(
	userID uuid.UUID,
	sessionHash []byte,
	conn *websocket.Conn,
	cancel context.CancelFunc,
	m *metrics.Metrics,
) *Client {
	return &Client{
		userID:      userID,
		sessionHash: sessionHash,
		conn:        conn,
		send:        make(chan []byte, sendBuffer),
		kick:        make(chan []byte, 1),
		m:           m,
		cancel:      cancel,
		done:        make(chan struct{}),
	}
}

func (c *Client) UserID() uuid.UUID { return c.userID }

func (c *Client) SessionHash() []byte { return c.sessionHash }

// Kick menitipkan pesan perpisahan lalu membiarkan writeLoop yang menutup.
//
// Urutannya yang penting, dan dia kebalikan dari Close: di sini payload-nya
// ditulis LEBIH DULU, baru koneksinya berakhir. Enqueue lalu Close tampak sama
// tapi tidak — Close membatalkan konteks tulis seketika, dan pesan yang baru
// diantrekan kalah balapan dengan pembatalan itu lebih sering daripada tidak.
// Yang hilang justru satu-satunya keterangan yang dimiliki orang tentang kenapa
// aplikasinya tiba-tiba mengeluarkan dia.
//
// Tidak pernah memblokir, dan aman dipanggil berkali-kali: yang kedua jatuh ke
// default karena kanalnya sudah terisi.
func (c *Client) Kick(payload []byte) {
	select {
	case c.kick <- payload:
	default:
	}
}

// Enqueue tidak pernah memblokir.
//
// Kedalaman antrean dicatat setiap kali, bukan hanya saat penuh. Antrean yang
// biasanya nol lalu mulai sering menyentuh belasan adalah peringatan dini bahwa
// siaran mulai lebih cepat daripada kemampuan client menyerapnya — jauh sebelum
// ada satu pun koneksi yang benar-benar diputus.
func (c *Client) Enqueue(payload []byte) hub.Delivery {
	select {
	case <-c.done:
		return hub.Gone
	case c.send <- payload:
		c.m.WSSendQueue.Observe(float64(len(c.send)))
		return hub.Delivered
	default:
		c.m.WSSendQueue.Observe(float64(cap(c.send)))
		return hub.Backpressure
	}
}

// EnqueueWait menunggu sampai ada ruang di antrean kirim.
//
// Ini kebalikan sengaja dari Enqueue, dan bedanya penting. Enqueue tidak pernah
// menunggu karena dia dipanggil dari jalur SIARAN: menahan satu client lambat
// di sana akan menahan semua orang di ruang yang sama. EnqueueWait dipakai untuk
// pengiriman yang hanya menyangkut koneksi ini sendiri — pengiriman ulang
// setelah reconnect — di mana menunggu tidak merugikan siapa pun kecuali yang
// sedang menunggu.
//
// Tanpa pemisahan ini, client yang tertinggal jauh akan diputus karena dianggap
// lambat justru saat sedang menyusul, lalu menyambung lagi, tertinggal lagi,
// dan terjebak di lingkaran yang tidak pernah selesai.
func (c *Client) EnqueueWait(ctx context.Context, payload []byte) bool {
	select {
	case <-c.done:
		return false
	case c.send <- payload:
		c.m.WSSendQueue.Observe(float64(len(c.send)))
		return true
	default:
	}

	select {
	case <-ctx.Done():
		return false
	case <-c.done:
		return false
	case c.send <- payload:
		c.m.WSSendQueue.Observe(float64(len(c.send)))
		return true
	}
}

// Close meminta koneksi ditutup. Aman dipanggil berkali-kali dan dari goroutine
// mana pun — inilah cara hub, drain, dan read loop sama-sama bisa mengakhiri
// koneksi tanpa saling menunggu.
//
// Menutup channel done saja TIDAK cukup. readLoop sedang memblokir di
// conn.Read, dan pembacaan itu hanya berakhir kalau lawan bicaranya yang
// menutup — client yang diam (tab di latar belakang, HP terkunci) akan
// menggantung sampai prosesnya dimatikan paksa. Membatalkan konteks memutus
// pembacaan itu dari sisi sini, dan itulah yang membuat drain saat rolling
// deploy benar-benar selesai alih-alih kehabisan waktu tunggu.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// writeLoop adalah SATU-SATUNYA goroutine yang menulis ke koneksi. WebSocket
// tidak mengizinkan dua penulis bersamaan, dan menyalurkan semuanya lewat satu
// channel membuat aturan itu terjaga tanpa kunci tersebar di banyak tempat.
//
// Loop yang sama juga mengirim ping berkala. Tanpa ini, koneksi yang mati diam-
// diam (HP masuk terowongan, wifi putus) akan menempel di memori server berjam-
// jam dan user terlihat online padahal sudah pergi.
func (c *Client) writeLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return

		case payload := <-c.send:
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				c.Close()
				return
			}

		case payload := <-c.kick:
			// Ditulis dengan konteks yang BELUM dibatalkan — itu seluruh alasan
			// pencabutan sesi lewat jalur ini, bukan lewat Close. Berhasil atau
			// tidak, koneksinya berakhir setelah baris ini.
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			_ = c.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			c.Close()
			return

		case <-ticker.C:
			// Ping memblokir sampai pong diterima. Kalau lewat batas waktu,
			// koneksi dianggap mati.
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				c.Close()
				return
			}
		}
	}
}

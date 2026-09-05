// Package ws mengelola satu koneksi WebSocket: umur koneksi, heartbeat,
// backpressure, dan sinkronisasi setelah reconnect.
package ws

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
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
	conn   *websocket.Conn
	send   chan []byte

	closeOnce sync.Once
	done      chan struct{}
}

func newClient(userID uuid.UUID, conn *websocket.Conn) *Client {
	return &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, sendBuffer),
		done:   make(chan struct{}),
	}
}

func (c *Client) UserID() uuid.UUID { return c.userID }

// Enqueue tidak pernah memblokir. Mengembalikan false bila buffer penuh atau
// koneksi sudah ditutup, dan hub akan melepas koneksi ini.
func (c *Client) Enqueue(payload []byte) bool {
	select {
	case <-c.done:
		return false
	case c.send <- payload:
		return true
	default:
		return false
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() { close(c.done) })
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
				c.close()
				return
			}

		case <-ticker.C:
			// Ping memblokir sampai pong diterima. Kalau lewat batas waktu,
			// koneksi dianggap mati.
			pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				c.close()
				return
			}
		}
	}
}

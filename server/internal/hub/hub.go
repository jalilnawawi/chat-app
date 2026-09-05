// Package hub menyiarkan event realtime ke koneksi WebSocket yang aktif.
//
// Semua siaran melewati SATU pintu: interface Broadcaster. Implementasi saat ini
// menyimpan koneksi di memori proses — cukup untuk satu instance. Saat nanti
// butuh lebih dari satu instance, yang ditulis hanyalah implementasi kedua
// (Redis pub/sub) yang memenuhi interface yang sama; handler HTTP dan WebSocket
// tidak perlu berubah sama sekali.
package hub

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// Tipe event yang dikirim server ke client.
const (
	EventMessageNew       = "message.new"
	EventMessageUpdated   = "message.updated"
	EventConversationNew  = "conversation.new"
	EventReadUpdated      = "read.updated"
	EventTyping           = "typing"
	EventPresence         = "presence"
	EventSyncComplete     = "sync.complete"
	EventPresenceSnapshot = "presence.snapshot"
	EventError            = "error"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Broadcaster adalah satu-satunya jalan keluar event ke client.
type Broadcaster interface {
	// Publish mengirim event ke semua koneksi milik user pada daftar target.
	Publish(targets []uuid.UUID, ev Event)
	// OnlineAmong menyaring daftar user, mengembalikan yang sedang terhubung.
	OnlineAmong(ids []uuid.UUID) []uuid.UUID
}

// Sink adalah sisi hub yang dilihat sebuah koneksi: tempat menaruh byte keluar.
type Sink interface {
	UserID() uuid.UUID
	// Enqueue mengembalikan false bila buffer penuh — pemanggil harus menutup
	// koneksi. Lihat catatan backpressure di Publish.
	Enqueue(payload []byte) bool
}

type Hub struct {
	mu      sync.RWMutex
	clients map[uuid.UUID]map[Sink]struct{} // satu user bisa punya banyak tab
	log     *slog.Logger
}

func New(log *slog.Logger) *Hub {
	return &Hub{clients: make(map[uuid.UUID]map[Sink]struct{}), log: log}
}

// Register menambahkan koneksi. Mengembalikan true bila ini koneksi PERTAMA
// milik user tersebut, yaitu saat presence berubah menjadi online.
func (h *Hub) Register(c Sink) (firstConnection bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[c.UserID()]
	if !ok {
		conns = make(map[Sink]struct{})
		h.clients[c.UserID()] = conns
	}
	conns[c] = struct{}{}
	return len(conns) == 1
}

// Unregister melepas koneksi. Mengembalikan true bila itu koneksi TERAKHIR
// milik user, yaitu saat presence berubah menjadi offline.
func (h *Hub) Unregister(c Sink) (lastConnection bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[c.UserID()]
	if !ok {
		return false
	}
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.clients, c.UserID())
		return true
	}
	return false
}

// Publish menyiarkan satu event ke semua koneksi milik target.
//
// JSON di-encode SEKALI lalu byte yang sama dibagikan ke semua penerima —
// di ruang grup, ini beda antara satu encode dan puluhan.
//
// Backpressure: bila buffer sebuah koneksi penuh, koneksi itu ditandai untuk
// ditutup, bukan ditunggu. Menunggu client lambat akan menahan seluruh siaran
// dan menular ke semua orang di ruang yang sama. Client yang terputus akan
// reconnect dan menyusul lewat mekanisme resume.
func (h *Hub) Publish(targets []uuid.UUID, ev Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		h.log.Error("encode event", "type", ev.Type, "err", err)
		return
	}

	h.mu.RLock()
	var slow []Sink
	for _, id := range targets {
		for c := range h.clients[id] {
			if !c.Enqueue(payload) {
				slow = append(slow, c)
			}
		}
	}
	h.mu.RUnlock()

	for _, c := range slow {
		h.log.Warn("client lambat, koneksi ditutup", "user", c.UserID(), "event", ev.Type)
		h.Unregister(c)
	}
}

func (h *Hub) OnlineAmong(ids []uuid.UUID) []uuid.UUID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	online := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if len(h.clients[id]) > 0 {
			online = append(online, id)
		}
	}
	return online
}

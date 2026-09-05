package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Batas pesan yang dikirim ulang saat resume. Kalau client tertinggal lebih
// jauh dari ini, dia diminta memuat ulang riwayat lewat REST — mengalirkan
// ribuan pesan lewat WebSocket saat reconnect justru bikin macet.
const maxResume = 200

type Handler struct {
	store       *store.Store
	hub         *hub.Hub
	log         *slog.Logger
	originHosts []string
}

func NewHandler(st *store.Store, h *hub.Hub, log *slog.Logger, allowedOrigins []string) *Handler {
	// websocket.Accept membandingkan HOST, bukan origin lengkap, jadi skema
	// dibuang di sini.
	hosts := []string{}
	for _, origin := range allowedOrigins {
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			hosts = append(hosts, u.Host)
		}
	}
	return &Handler{store: st, hub: h, log: log, originHosts: hosts}
}

// clientMessage adalah amplop untuk pesan dari client.
type clientMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Serve menangani satu koneksi sampai putus. User sudah terautentikasi oleh
// middleware lewat cookie sesi — itulah alasan memilih cookie ketimbang token
// di header: browser mengirim cookie otomatis saat handshake WebSocket,
// sedangkan custom header tidak bisa dipasang dari WebSocket API.
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request, user store.User) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.originHosts,
	})
	if err != nil {
		h.log.Warn("ws accept gagal", "err", err)
		return
	}
	conn.SetReadLimit(readLimit)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := newClient(user.ID, conn)
	defer c.close()

	first := h.hub.Register(c)
	defer func() {
		if last := h.hub.Unregister(c); last {
			h.broadcastPresence(context.WithoutCancel(ctx), user.ID, false)
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	if first {
		h.broadcastPresence(ctx, user.ID, true)
	}

	go c.writeLoop(ctx)

	h.sendPresenceSnapshot(ctx, c, user.ID)
	h.readLoop(ctx, c, user)
}

func (h *Handler) readLoop(ctx context.Context, c *Client, user store.User) {
	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, data, err := c.conn.Read(ctx)
		if err != nil {
			status := websocket.CloseStatus(err)
			if status == -1 && !errors.Is(err, context.Canceled) {
				h.log.Debug("ws read berhenti", "user", user.ID, "err", err)
			}
			c.close()
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.sendTo(c, hub.Event{Type: hub.EventError, Payload: map[string]string{
				"message": "format pesan tidak valid",
			}})
			continue
		}

		switch msg.Type {
		case "sync":
			h.handleSync(ctx, c, user, msg.Payload)
		case "typing":
			h.handleTyping(ctx, user, msg.Payload)
		case "read":
			h.handleRead(ctx, user, msg.Payload)
		default:
			h.sendTo(c, hub.Event{Type: hub.EventError, Payload: map[string]string{
				"message": "tipe pesan tidak dikenal: " + msg.Type,
			}})
		}
	}
}

// handleSync mengirim ulang pesan yang terlewat selama koneksi putus.
// Client mengirim `seq` terakhir yang dia punya per percakapan; server
// membalas hanya selisihnya. Inilah yang bikin chat terasa tidak pernah putus.
func (h *Handler) handleSync(ctx context.Context, c *Client, user store.User, raw json.RawMessage) {
	var payload struct {
		Cursors map[string]int64 `json:"cursors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	for idStr, seq := range payload.Cursors {
		convID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		ok, err := h.store.IsMember(ctx, convID, user.ID)
		if err != nil || !ok {
			continue
		}

		missed, err := h.store.MessagesSince(ctx, convID, seq, maxResume)
		if err != nil {
			h.log.Error("resume gagal", "conversation", convID, "err", err)
			continue
		}
		for _, m := range missed {
			h.sendTo(c, hub.Event{Type: hub.EventMessageNew, Payload: m})
		}
	}

	h.sendTo(c, hub.Event{Type: hub.EventSyncComplete, Payload: struct{}{}})
}

func (h *Handler) handleTyping(ctx context.Context, user store.User, raw json.RawMessage) {
	var payload struct {
		ConversationID uuid.UUID `json:"conversationId"`
		Typing         bool      `json:"typing"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	ok, err := h.store.IsMember(ctx, payload.ConversationID, user.ID)
	if err != nil || !ok {
		return
	}

	members, err := h.store.MemberIDs(ctx, payload.ConversationID)
	if err != nil {
		return
	}

	// Typing bersifat sementara: tidak pernah menyentuh database, dan pengirim
	// tidak perlu melihat indikator dirinya sendiri.
	targets := make([]uuid.UUID, 0, len(members))
	for _, id := range members {
		if id != user.ID {
			targets = append(targets, id)
		}
	}

	h.hub.Publish(targets, hub.Event{Type: hub.EventTyping, Payload: map[string]any{
		"conversationId": payload.ConversationID,
		"userId":         user.ID,
		"displayName":    user.DisplayName,
		"typing":         payload.Typing,
	}})
}

func (h *Handler) handleRead(ctx context.Context, user store.User, raw json.RawMessage) {
	var payload struct {
		ConversationID uuid.UUID `json:"conversationId"`
		Seq            int64     `json:"seq"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	current, err := h.store.MarkRead(ctx, payload.ConversationID, user.ID, payload.Seq)
	if err != nil {
		return
	}

	members, err := h.store.MemberIDs(ctx, payload.ConversationID)
	if err != nil {
		return
	}

	h.hub.Publish(members, hub.Event{Type: hub.EventReadUpdated, Payload: map[string]any{
		"conversationId": payload.ConversationID,
		"userId":         user.ID,
		"lastReadSeq":    current,
	}})
}

// broadcastPresence memberi tahu hanya kontak user — orang yang berbagi
// percakapan dengannya — bukan seluruh pengguna aplikasi.
func (h *Handler) broadcastPresence(ctx context.Context, userID uuid.UUID, online bool) {
	contacts, err := h.store.ContactIDs(ctx, userID)
	if err != nil {
		h.log.Error("ambil kontak gagal", "user", userID, "err", err)
		return
	}
	h.hub.Publish(contacts, hub.Event{Type: hub.EventPresence, Payload: map[string]any{
		"userId": userID,
		"online": online,
	}})
}

// sendPresenceSnapshot memberi client keadaan awal: siapa saja yang sedang
// online sekarang. Tanpa ini, client baru tahu status seseorang saat orang itu
// kebetulan berganti status.
func (h *Handler) sendPresenceSnapshot(ctx context.Context, c *Client, userID uuid.UUID) {
	contacts, err := h.store.ContactIDs(ctx, userID)
	if err != nil {
		return
	}
	h.sendTo(c, hub.Event{
		Type:    hub.EventPresenceSnapshot,
		Payload: map[string]any{"online": h.hub.OnlineAmong(contacts)},
	})
}

func (h *Handler) sendTo(c *Client, ev hub.Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	if !c.Enqueue(payload) {
		c.close()
	}
}

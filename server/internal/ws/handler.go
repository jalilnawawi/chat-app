package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/metrics"
	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Batas pesan yang dikirim ulang saat resume. Kalau client tertinggal lebih
// jauh dari ini, dia diminta memuat ulang riwayat lewat REST — mengalirkan
// ribuan pesan lewat WebSocket saat reconnect justru bikin macet.
const maxResume = 200

// resumeTimeout membatasi seluruh proses menyusul untuk satu koneksi. Menunggu
// itu boleh; menunggu selamanya tidak — satu client yang berhenti membaca tanpa
// menutup koneksi akan menahan goroutine-nya sampai proses mati.
const resumeTimeout = 30 * time.Second

// resumeBatch adalah jumlah pesan susulan per frame WebSocket.
//
// Angka ini yang menyelamatkan reconnect massal. Antrean kirim tiap koneksi
// dalamnya 64 frame, dan itu ukuran untuk MEREDAM LEDAKAN SIARAN — bukan untuk
// menampung riwayat. Mengirim 200 pesan susulan satu per satu memenuhi antrean
// itu sampai luber, dan siaran pertama yang kebetulan datang saat itu membuat
// koneksinya diputus karena dikira lambat. Client menyambung lagi, menyusul
// lagi, diputus lagi.
//
// Yang membuatnya berbahaya: kejadiannya justru saat SEMUA orang menyusul
// bersamaan — setelah rolling deploy atau setelah jaringan pulih.
//
// Dengan 50 pesan per frame, riwayat sepenuh apa pun muat dalam beberapa frame.
const resumeBatch = 50

type Handler struct {
	store       *store.Store
	hub         hub.Broadcaster
	limiter     ratelimit.Limiter
	typingRate  ratelimit.Rule
	log         *slog.Logger
	m           *metrics.Metrics
	originHosts []string
}

func NewHandler(
	st *store.Store,
	h hub.Broadcaster,
	limiter ratelimit.Limiter,
	typingRate ratelimit.Rule,
	log *slog.Logger,
	m *metrics.Metrics,
	allowedOrigins []string,
) *Handler {
	// websocket.Accept membandingkan HOST, bukan origin lengkap, jadi skema
	// dibuang di sini.
	hosts := []string{}
	for _, origin := range allowedOrigins {
		if u, err := url.Parse(origin); err == nil && u.Host != "" {
			hosts = append(hosts, u.Host)
		}
	}
	return &Handler{
		store:       st,
		hub:         h,
		limiter:     limiter,
		typingRate:  typingRate,
		log:         log,
		m:           m,
		originHosts: hosts,
	}
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
func (h *Handler) Serve(w http.ResponseWriter, r *http.Request, user store.User, sessionHash []byte) {
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

	// Hash sesinya ikut masuk ke koneksi: itu nama yang membedakan tab ini dari
	// tab orang yang sama di perangkat lain, dan tanpa dia pencabutan satu
	// perangkat berarti mengeluarkan orangnya dari semua perangkatnya.
	c := newClient(user.ID, sessionHash, conn, cancel, h.m)
	defer c.Close()

	first, err := h.hub.Register(ctx, c)
	if err != nil {
		// Pendaftaran gagal berarti koneksi ini tidak akan menerima siaran.
		// Menutupnya sekarang membuat client langsung mencoba lagi, alih-alih
		// duduk di ruang chat yang diam-diam mati.
		h.log.Error("registrasi koneksi gagal", "user", user.ID, "err", err)
		h.m.WSClosed.WithLabelValues("register_error").Inc()
		conn.Close(websocket.StatusInternalError, "gagal mendaftar")
		return
	}

	h.m.WSAccepted.Inc()
	h.m.WSActive.Inc()

	defer func() {
		h.m.WSActive.Dec()

		// Konteks permintaan sudah mati begitu koneksi putus, sedangkan
		// pelepasan presence justru harus tetap jalan — karena itu konteks
		// terpisah dengan batas waktunya sendiri.
		cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelCleanup()

		last, err := h.hub.Unregister(cleanupCtx, c)
		if err != nil {
			h.log.Error("pelepasan koneksi gagal", "user", user.ID, "err", err)
		}
		if last {
			h.broadcastPresence(cleanupCtx, user.ID, false)
		}
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	if first {
		h.broadcastPresence(ctx, user.ID, true)
	}

	go c.writeLoop(ctx)

	h.sendSnapshots(ctx, c, user.ID)
	h.readLoop(ctx, c, user)
	h.m.WSClosed.WithLabelValues("client").Inc()
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
			c.Close()
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
		// ReactionCursors adalah jam KEDUA, dan client mengirimnya terpisah
		// karena dia menghitung hal yang berbeda. Reaksi menempel pada pesan
		// lama, yang seq-nya sudah lama berhenti bergerak — cursor pesan tidak
		// akan pernah menyusulkannya. Lihat store/reactions.go.
		//
		// Client lama yang tidak mengirim peta ini tetap dilayani: petanya
		// kosong, dan bagian reaksi dilewati tanpa merusak apa pun.
		ReactionCursors map[string]int64 `json:"reactionCursors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	syncCtx, cancel := context.WithTimeout(ctx, resumeTimeout)
	defer cancel()

	for idStr, seq := range payload.Cursors {
		convID, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		ok, err := h.store.IsMember(syncCtx, convID, user.ID)
		if err != nil || !ok {
			continue
		}

		missed, err := h.store.MessagesSince(syncCtx, convID, user.ID, seq, maxResume)
		if err != nil {
			h.log.Error("resume gagal", "conversation", convID, "err", err)
			continue
		}

		for chunk := range slices.Chunk(missed, resumeBatch) {
			// Menunggu, bukan membuang. Lihat catatan di Client.EnqueueWait:
			// menyusul adalah urusan koneksi ini sendiri, jadi tekanan baliknya
			// ditanggung sendiri juga.
			ev := hub.Event{Type: hub.EventSyncBatch, Payload: map[string]any{
				"conversationId": convID,
				"messages":       chunk,
			}}
			if !h.sendWaiting(syncCtx, c, ev) {
				h.m.WSResumeFail.Inc()
				h.log.Warn("resume terputus di tengah jalan",
					"user", user.ID, "conversation", convID, "tersisa", len(chunk))
				return
			}
			h.m.WSResumeSent.Add(float64(len(chunk)))
		}

		// Reaksi menyusul SETELAH pesannya, dan urutan itu penting: sebuah
		// ringkasan reaksi menunjuk id pesan, dan client yang belum punya
		// pesannya tidak punya tempat untuk menaruhnya.
		//
		// Pesan yang baru saja disusulkan di atas sudah membawa ringkasannya
		// sendiri, jadi yang tersisa di sini justru yang paling mudah
		// terlewat: reaksi pada pesan LAMA yang isinya tidak berubah sama
		// sekali.
		if !h.resumeReactions(syncCtx, c, user, convID, payload.ReactionCursors[idStr]) {
			return
		}
	}

	h.sendTo(c, hub.Event{Type: hub.EventSyncComplete, Payload: struct{}{}})
}

// resumeReactions menyusulkan keadaan reaksi yang berubah selagi client putus.
// Mengembalikan false bila koneksinya tidak sanggup lagi menerima — pemanggil
// berhenti, sama seperti pada susulan pesan.
func (h *Handler) resumeReactions(ctx context.Context, c *Client, user store.User, convID uuid.UUID, after int64) bool {
	changed, err := h.store.ReactionsSince(ctx, convID, user.ID, after, maxResume)
	if err != nil {
		h.log.Error("resume reaksi gagal", "conversation", convID, "err", err)
		// Bukan alasan menghentikan seluruh resume: pesannya yang penting sudah
		// sampai, dan reaksi yang tertinggal akan terlihat saat riwayatnya
		// dimuat ulang.
		return true
	}
	if len(changed) == 0 {
		return true
	}

	for chunk := range slices.Chunk(changed, resumeBatch) {
		ev := hub.Event{Type: hub.EventReactionBatch, Payload: map[string]any{
			"conversationId": convID,
			"messages":       chunk,
		}}
		if !h.sendWaiting(ctx, c, ev) {
			h.m.WSResumeFail.Inc()
			h.log.Warn("susulan reaksi terputus", "user", user.ID, "conversation", convID)
			return false
		}
	}
	return true
}

func (h *Handler) handleTyping(ctx context.Context, user store.User, raw json.RawMessage) {
	var payload struct {
		ConversationID uuid.UUID `json:"conversationId"`
		Typing         bool      `json:"typing"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	// Typing dibatasi SEBELUM menyentuh database. Tiap event memicu dua query
	// (cek keanggotaan + daftar anggota), dan event ini yang paling gampang
	// dikirim beruntun — baik oleh keyboard yang cepat maupun oleh client
	// nakal. Tanpa gerbang di sini, satu koneksi bisa memakai kuota database
	// milik semua orang.
	if allowed, err := h.limiter.Allow(ctx, "typing:"+user.ID.String(), h.typingRate); err == nil && !allowed.Allowed {
		h.m.RateLimited.WithLabelValues("typing").Inc()
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

// sendSnapshots memberi client DUA keadaan awal: siapa yang sedang terhubung,
// dan apa yang sedang orang-orang nyatakan tentang dirinya.
//
// Keduanya dikirim dari satu tempat karena keduanya butuh daftar kontak yang
// sama — dan mengambilnya dua kali adalah dua query identik pada jalur yang
// dilewati setiap koneksi yang dibuka, termasuk ribuan yang menyambung
// bersamaan setelah rolling deploy.
//
// Tetap DUA event, bukan satu. Presence dan status berubah pada saat yang
// berbeda dan datang dari sumber yang berbeda — yang satu dari koneksi yang
// hidup, yang satu dari Postgres — jadi menggabungkan keadaan awalnya berarti
// menyatukan dua hal yang justru sengaja dipisah di sisa aplikasi ini.
func (h *Handler) sendSnapshots(ctx context.Context, c *Client, userID uuid.UUID) {
	contacts, err := h.store.ContactIDs(ctx, userID)
	if err != nil {
		h.log.Error("ambil kontak untuk snapshot gagal", "user", userID, "err", err)
		return
	}

	if online, err := h.hub.OnlineAmong(ctx, contacts); err != nil {
		h.log.Error("ambil presence awal gagal", "user", userID, "err", err)
	} else {
		h.sendTo(c, hub.Event{
			Type:    hub.EventPresenceSnapshot,
			Payload: map[string]any{"online": online},
		})
	}

	// Tanpa ini, status seseorang baru terlihat saat dia kebetulan
	// menggantinya — dan "sedang rapat sampai 13.00" yang dipasang sebelum
	// kita membuka aplikasi adalah justru yang paling ingin kita lihat.
	//
	// Yang dikirim hanya yang punya sesuatu untuk diceritakan; 'available'
	// tanpa teks adalah keadaan bawaan yang sudah diasumsikan client. Lihat
	// store.StatusesOf.
	statuses, err := h.store.StatusesOf(ctx, contacts)
	if err != nil {
		h.log.Error("ambil status awal gagal", "user", userID, "err", err)
		return
	}
	h.sendTo(c, hub.Event{
		Type:    hub.EventStatusSnapshot,
		Payload: map[string]any{"statuses": statuses},
	})
}

// sendWaiting menaruh event di antrean kirim, menunggu bila perlu.
func (h *Handler) sendWaiting(ctx context.Context, c *Client, ev hub.Event) bool {
	payload, err := json.Marshal(ev)
	if err != nil {
		return false
	}
	return c.EnqueueWait(ctx, payload)
}

func (h *Handler) sendTo(c *Client, ev hub.Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		return
	}
	switch c.Enqueue(payload) {
	case hub.Backpressure:
		h.m.WSSlowDropped.Inc()
		c.Close()
	case hub.Gone:
		// Koneksi sudah ditutup; tidak ada yang perlu dilakukan.
	}
}

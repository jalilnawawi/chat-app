// Package api menyusun seluruh permukaan HTTP: REST untuk aksi, WebSocket
// untuk siaran.
//
// Kenapa kirim pesan lewat REST, bukan WebSocket? Karena HTTP memberi status
// sukses/gagal yang jelas per pesan, sehingga retry punya arti pasti. WebSocket
// dipakai untuk yang memang jadi kekuatannya: menyebar hasilnya ke semua orang.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
	"github.com/jalilnawawi/chat-app/server/internal/config"
	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/metrics"
	"github.com/jalilnawawi/chat-app/server/internal/push"
	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
	"github.com/jalilnawawi/chat-app/server/internal/store"
	"github.com/jalilnawawi/chat-app/server/internal/ws"
)

type Server struct {
	cfg     config.Config
	store   *store.Store
	hub     hub.Broadcaster
	limiter ratelimit.Limiter
	ws      *ws.Handler
	log     *slog.Logger
	m       *metrics.Metrics

	// blobs nil berarti lampiran dimatikan; push nil berarti notifikasi
	// dimatikan. Keduanya fitur yang butuh infrastruktur di luar proses ini,
	// dan aplikasi harus tetap utuh sebagai chat tanpa keduanya — itu yang
	// membuat `go run ./cmd/server` cukup untuk mengembangkan sisanya.
	blobs blob.Store
	push  *push.Dispatcher

	// draining dibaca tiap kali /readyz dipanggil, dari goroutine mana pun.
	draining atomic.Bool
}

func NewServer(
	cfg config.Config,
	st *store.Store,
	h hub.Broadcaster,
	limiter ratelimit.Limiter,
	blobs blob.Store,
	pusher *push.Dispatcher,
	log *slog.Logger,
	m *metrics.Metrics,
) *Server {
	return &Server{
		cfg:     cfg,
		store:   st,
		hub:     h,
		limiter: limiter,
		ws:      ws.NewHandler(st, h, limiter, cfg.TypingRate, log, m, cfg.AllowedOrigins),
		log:     log,
		m:       m,
		blobs:   blobs,
		push:    pusher,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// healthz menjawab "proses ini hidup", readyz menjawab "kirim trafik ke
	// sini". Bedanya baru terasa saat rolling deploy: instance yang sedang
	// dikuras masih sangat hidup — dia justru sedang merawat koneksi yang ada —
	// tapi tidak boleh lagi menerima yang baru.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	// Endpoint auth memverifikasi argon2id yang sengaja mahal, jadi kuotanya
	// per alamat IP — belum ada user yang bisa dijadikan kunci.
	mux.Handle("POST /api/auth/register", s.rateLimitByIP("auth", http.HandlerFunc(s.handleRegister)))
	mux.Handle("POST /api/auth/login", s.rateLimitByIP("auth", http.HandlerFunc(s.handleLogin)))
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/users", s.requireAuth(http.HandlerFunc(s.handleSearchUsers)))

	mux.Handle("GET /api/conversations", s.requireAuth(http.HandlerFunc(s.handleListConversations)))
	mux.Handle("POST /api/conversations/direct", s.requireAuth(http.HandlerFunc(s.handleCreateDirect)))
	mux.Handle("POST /api/conversations/group", s.requireAuth(http.HandlerFunc(s.handleCreateGroup)))
	mux.Handle("GET /api/conversations/{id}/messages", s.requireAuth(http.HandlerFunc(s.handleListMessages)))
	mux.Handle("POST /api/conversations/{id}/messages",
		s.requireAuth(s.rateLimitByUser("message", s.cfg.MessageRate, http.HandlerFunc(s.handleSendMessage))))
	mux.Handle("GET /api/conversations/{id}/members", s.requireAuth(http.HandlerFunc(s.handleMembers)))
	mux.Handle("POST /api/conversations/{id}/read", s.requireAuth(http.HandlerFunc(s.handleMarkRead)))

	// Unggahan punya kuotanya sendiri, jauh lebih ketat dari kirim pesan: satu
	// berkas bisa sepuluh megabyte yang melewati proses ini dua kali.
	mux.Handle("POST /api/attachments",
		s.requireAuth(s.rateLimitByUser("upload", s.cfg.UploadRate, http.HandlerFunc(s.handleUploadAttachment))))
	mux.Handle("GET /api/attachments/{id}", s.requireAuth(http.HandlerFunc(s.handleDownloadAttachment)))

	mux.Handle("GET /api/push/config", s.requireAuth(http.HandlerFunc(s.handlePushConfig)))
	mux.Handle("POST /api/push/subscribe", s.requireAuth(http.HandlerFunc(s.handlePushSubscribe)))
	mux.Handle("POST /api/push/unsubscribe", s.requireAuth(http.HandlerFunc(s.handlePushUnsubscribe)))

	mux.Handle("PATCH /api/messages/{id}", s.requireAuth(http.HandlerFunc(s.handleEditMessage)))
	mux.Handle("DELETE /api/messages/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteMessage)))

	// Membuka koneksi juga dibatasi. Handshake WebSocket jauh lebih mahal dari
	// permintaan biasa — cek sesi, upgrade, dua goroutine, satu langganan
	// Redis — dan client yang reconnect-loop bisa mengulanginya ratusan kali
	// per menit tanpa sadar.
	mux.Handle("GET /ws", s.requireAuth(s.rateLimitByUser("connect", s.cfg.ConnectRate, http.HandlerFunc(s.handleWS))))

	return s.recoverer(s.requestID(s.observe(s.cors(mux))))
}

// MetricsRoutes dilayani di listener terpisah. /metrics membocorkan bentuk
// internal sistem — nama rute, jumlah user online, versi build — dan tidak ada
// alasan menempelkannya di port yang sama dengan API publik.
func (s *Server) MetricsRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.m.Registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	}))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	return mux
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Instance yang sedang dikuras menolak koneksi BARU, walau tetap merawat
	// yang sudah ada sampai gilirannya ditutup.
	//
	// Idealnya load balancer sudah berhenti mengirim ke sini begitu /readyz
	// menjawab 503. Tapi tanpa penjagaan di sini, client yang baru saja diputus
	// oleh drain bisa langsung menyambung kembali ke instance yang sama —
	// justru instance yang sedang pamit — dan pengurasan berputar tanpa pernah
	// selesai. Itu bukan skenario teoretis: tanpa gerbang ini, drain lokal
	// selalu menyisakan koneksi yang baru lahir setelah snapshot diambil.
	if s.draining.Load() {
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "instance sedang dikuras, sambungkan ke instance lain")
		return
	}
	s.ws.Serve(w, r, userFrom(r.Context()))
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// Fitur yang bisa dimatikan ikut dilaporkan supaya "kenapa lampirannya
	// tidak jalan di staging" bisa dijawab dengan satu permintaan, bukan dengan
	// membaca variabel lingkungan di mesin orang lain.
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"instance":    s.cfg.InstanceID,
		"attachments": s.blobs != nil,
		"push":        s.push.Enabled(),
	})
}

// handleReadyz ikut memeriksa database. Instance yang kehilangan Postgres masih
// bisa menjawab HTTP, tapi tiap pesan yang masuk ke sana akan gagal — lebih baik
// load balancer mengarahkan trafik ke instance lain sampai koneksinya pulih.
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.draining.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":      "draining",
			"instance":    s.cfg.InstanceID,
			"connections": s.hub.LocalConnections(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.store.Ping(ctx); err != nil {
		s.log.Warn("readyz: database tidak siap", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "database tidak siap",
			"instance": s.cfg.InstanceID,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ready",
		"instance":    s.cfg.InstanceID,
		"connections": s.hub.LocalConnections(),
	})
}

// StartDraining membuat /readyz menjawab 503 tanpa memutus apa pun.
// Dipanggil di awal shutdown supaya load balancer punya waktu berhenti
// mengirim koneksi baru sebelum yang lama ditutup.
func (s *Server) StartDraining() {
	s.draining.Store(true)
	s.m.Draining.Set(1)
}

// ---------- helper respons ----------

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

// writeStoreError menerjemahkan error domain ke status HTTP di satu tempat,
// supaya tiap handler tidak mengulang pemetaan yang sama.
func (s *Server) writeStoreError(w http.ResponseWriter, err error, context string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "tidak ditemukan")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "tidak diizinkan")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "sudah ada atau bentrok")
	default:
		s.log.Error(context, "err", err)
		writeError(w, http.StatusInternalServerError, "terjadi kesalahan di server")
	}
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

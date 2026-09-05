// Package api menyusun seluruh permukaan HTTP: REST untuk aksi, WebSocket
// untuk siaran.
//
// Kenapa kirim pesan lewat REST, bukan WebSocket? Karena HTTP memberi status
// sukses/gagal yang jelas per pesan, sehingga retry punya arti pasti. WebSocket
// dipakai untuk yang memang jadi kekuatannya: menyebar hasilnya ke semua orang.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jalilnawawi/chat-app/server/internal/config"
	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/store"
	"github.com/jalilnawawi/chat-app/server/internal/ws"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	hub   *hub.Hub
	ws    *ws.Handler
	log   *slog.Logger
}

func NewServer(cfg config.Config, st *store.Store, h *hub.Hub, log *slog.Logger) *Server {
	return &Server{
		cfg:   cfg,
		store: st,
		hub:   h,
		ws:    ws.NewHandler(st, h, log, cfg.AllowedOrigins),
		log:   log,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/users", s.requireAuth(http.HandlerFunc(s.handleSearchUsers)))

	mux.Handle("GET /api/conversations", s.requireAuth(http.HandlerFunc(s.handleListConversations)))
	mux.Handle("POST /api/conversations/direct", s.requireAuth(http.HandlerFunc(s.handleCreateDirect)))
	mux.Handle("POST /api/conversations/group", s.requireAuth(http.HandlerFunc(s.handleCreateGroup)))
	mux.Handle("GET /api/conversations/{id}/messages", s.requireAuth(http.HandlerFunc(s.handleListMessages)))
	mux.Handle("POST /api/conversations/{id}/messages", s.requireAuth(http.HandlerFunc(s.handleSendMessage)))
	mux.Handle("GET /api/conversations/{id}/members", s.requireAuth(http.HandlerFunc(s.handleMembers)))
	mux.Handle("POST /api/conversations/{id}/read", s.requireAuth(http.HandlerFunc(s.handleMarkRead)))

	mux.Handle("PATCH /api/messages/{id}", s.requireAuth(http.HandlerFunc(s.handleEditMessage)))
	mux.Handle("DELETE /api/messages/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteMessage)))

	mux.Handle("GET /ws", s.requireAuth(http.HandlerFunc(s.handleWS)))

	return s.recoverer(s.requestLog(s.cors(mux)))
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	s.ws.Serve(w, r, userFrom(r.Context()))
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

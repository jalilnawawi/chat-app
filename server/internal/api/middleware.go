package api

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/jalilnawawi/chat-app/server/internal/auth"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

const (
	sessionCookie = "chat_session"
	sessionTTL    = 30 * 24 * time.Hour
)

type ctxKey int

const userKey ctxKey = iota

func userFrom(ctx context.Context) store.User {
	u, _ := ctx.Value(userKey).(store.User)
	return u
}

// requireAuth memvalidasi cookie sesi dan menaruh user di context.
//
// Sesi memakai cookie httpOnly, bukan JWT di localStorage. Dua alasannya:
// script berbahaya tidak bisa membaca cookie httpOnly, dan browser mengirim
// cookie otomatis saat handshake WebSocket — WebSocket API tidak mengizinkan
// custom header, jadi pendekatan token akan memaksa menaruh kredensial di
// query string, tempat yang gampang bocor ke log.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "belum login")
			return
		}

		user, err := s.store.UserBySession(r.Context(), auth.HashToken(cookie.Value))
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.log.Error("lookup sesi", "err", err)
			}
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "sesi tidak berlaku")
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionTTL),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// cors memantulkan kembali origin yang cocok dengan daftar izin. Kredensial
// cookie hanya boleh dikirim ke origin yang disebut eksplisit — wildcard "*"
// tidak sah bersama credentials.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); s.cfg.AllowsOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap dan Hijack membuat wrapper ini tembus ke writer asli — tanpa keduanya,
// upgrade WebSocket gagal karena koneksi tidak bisa diambil alih dari server HTTP.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("koneksi tidak mendukung hijack")
	}
	return hj.Hijack()
}

func (s *Server) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}

// recoverer menjaga satu handler panik tidak menjatuhkan seluruh server —
// termasuk memutus semua koneksi WebSocket yang sedang aktif.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "path", r.URL.Path, "recover", rec)
				writeError(w, http.StatusInternalServerError, "terjadi kesalahan di server")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

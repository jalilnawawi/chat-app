package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jalilnawawi/chat-app/server/internal/auth"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

type credentials struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}

	if len(req.Username) < 3 || len(req.Username) > 32 {
		writeError(w, http.StatusBadRequest, "username harus 3-32 karakter")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password minimal 8 karakter")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.writeStoreError(w, err, "hash password")
		return
	}

	user, err := s.store.CreateUser(r.Context(), req.Username, req.DisplayName, hash)
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "username sudah dipakai")
			return
		}
		s.writeStoreError(w, err, "create user")
		return
	}

	if err := s.startSession(w, r, store.Me{User: user}); err != nil {
		s.writeStoreError(w, err, "start session")
		return
	}

	// Dibungkus Me, bukan dikirim sebagai User. Keduanya terlihat sama untuk
	// akun yang baru lahir — dia memang belum punya email — tapi bentuknya
	// harus sama dengan yang dijawab /api/auth/me, karena client menyimpan
	// keduanya di tempat yang sama. Yang beda bentuk adalah panel akun yang
	// kehilangan keadaan verifikasinya sampai halamannya dimuat ulang.
	writeJSON(w, http.StatusCreated, store.Me{User: user})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credentials
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "body tidak valid")
		return
	}

	user, hash, err := s.store.UserByUsername(r.Context(), strings.TrimSpace(req.Username))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Pesan sengaja sama dengan kasus password salah, supaya tidak
			// membocorkan username mana yang terdaftar.
			writeError(w, http.StatusUnauthorized, "username atau password salah")
			return
		}
		s.writeStoreError(w, err, "login lookup")
		return
	}

	if err := auth.VerifyPassword(hash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "username atau password salah")
		return
	}

	if err := s.startSession(w, r, user); err != nil {
		s.writeStoreError(w, err, "start session")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := s.store.DeleteSession(r.Context(), auth.HashToken(cookie.Value)); err != nil {
			s.log.Error("hapus sesi", "err", err)
		}
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user store.Me) error {
	token, hash, err := auth.NewSessionToken()
	if err != nil {
		return err
	}
	// Keterangan perangkat disalin dari permintaan yang membuat sesinya, dan
	// dipotong: User-Agent datang dari client dan tidak ada batas panjangnya di
	// standar mana pun. Batas yang sama dipakai jalur langganan push.
	agent := truncate(r.UserAgent(), 300)
	if err := s.store.CreateSession(r.Context(), hash, user.ID, agent, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	s.setSessionCookie(w, token)
	return nil
}

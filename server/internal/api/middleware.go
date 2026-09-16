package api

import (
	"bufio"
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/auth"
	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

const (
	sessionCookie = "chat_session"
	sessionTTL    = 30 * 24 * time.Hour
)

type ctxKey int

const (
	userKey ctxKey = iota
	sessionKey
	requestIDKey
)

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func userFrom(ctx context.Context) store.Me {
	u, _ := ctx.Value(userKey).(store.Me)
	return u
}

// sessionRef adalah sesi yang dipakai permintaan ini, dalam dua bentuk yang
// dipakai untuk dua hal berbeda.
//
// ID adalah nama publiknya: yang muncul di daftar sesi aktif dan yang disebut
// saat mencabut salah satunya. Hash adalah nama internalnya: yang dipakai
// menandai koneksi WebSocket, dan karena itu satu-satunya yang bisa menjawab
// "koneksi mana yang BOLEH tetap hidup" saat semua sesi lain dicabut.
type sessionRef struct {
	ID   uuid.UUID
	Hash []byte
}

func sessionFrom(ctx context.Context) sessionRef {
	s, _ := ctx.Value(sessionKey).(sessionRef)
	return s
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

		hash := auth.HashToken(cookie.Value)
		user, sessionID, err := s.store.UserBySession(r.Context(), hash)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				s.log.Error("lookup sesi", "err", err)
			}
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "sesi tidak berlaku")
			return
		}

		// Sesinya ikut masuk ke context, bukan hanya penggunanya.
		//
		// Yang membutuhkannya adalah jalur-jalur Fase 10: mengganti password
		// mencabut semua sesi KECUALI yang ini, dan daftar sesi aktif harus
		// menandai mana yang sedang dipakai membacanya. Keduanya mustahil
		// dijawab dari identitas penggunanya saja — dua tab milik orang yang
		// sama tidak bisa dibedakan olehnya.
		ctx := context.WithValue(r.Context(), userKey, user)
		ctx = context.WithValue(ctx, sessionKey, sessionRef{ID: sessionID, Hash: hash})
		next.ServeHTTP(w, r.WithContext(ctx))
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

// requestID memberi setiap permintaan satu identitas yang ikut ke log dan
// dipantulkan ke header respons.
//
// Dengan beberapa instance di belakang load balancer, "coba lihat lognya"
// berhenti berarti tanpa ini: keluhan satu user tersebar di beberapa proses.
// ID yang datang dari proxy dipakai apa adanya supaya satu permintaan punya
// nomor yang sama sepanjang perjalanannya.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// observe mencatat log terstruktur dan metrik dari satu tempat, supaya
// keduanya tidak pernah bercerita berbeda tentang permintaan yang sama.
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		elapsed := time.Since(start)
		route := routeLabel(r.URL.Path)

		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"status", sw.status,
			"dur", elapsed.Round(time.Millisecond).String(),
			"request_id", requestIDFrom(r.Context()),
			"instance", s.cfg.InstanceID,
		)

		// Koneksi WebSocket berumur menit sampai jam. Memasukkannya ke
		// histogram durasi permintaan akan menenggelamkan p99 REST yang justru
		// ingin dipantau, jadi dia hanya dihitung, tidak diukur waktunya —
		// umur koneksi sudah punya metriknya sendiri di paket ws.
		s.m.HTTPRequests.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Inc()
		if route != "/ws" {
			s.m.HTTPDuration.WithLabelValues(r.Method, route).Observe(elapsed.Seconds())
		}
	})
}

// routeLabel mengubah path menjadi label berkardinalitas rendah dengan
// mengganti tiap segmen UUID jadi "{id}".
//
// Tanpa ini, satu label metrik akan lahir untuk setiap percakapan yang pernah
// dibuka, dan Prometheus akan menyimpan ratusan ribu deret waktu yang tidak
// pernah ada yang membacanya — cara klasik sebuah sistem monitoring menjatuhkan
// sistem yang dipantaunya.
func routeLabel(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if _, err := uuid.Parse(p); err == nil {
			parts[i] = "{id}"
		}
	}
	return strings.Join(parts, "/")
}

// clientIP membaca X-Forwarded-For lebih dulu karena di belakang load balancer
// RemoteAddr selalu berisi alamat proxy — satu nilai untuk semua orang, yang
// akan membuat kuota per-IP menjadi kuota global.
//
// Catatan penting: header ini bisa dipalsukan bila server bisa dijangkau
// langsung dari internet. Aman dipakai hanya kalau proxy di depan menimpanya,
// dan itu memang yang dilakukan load balancer pada umumnya.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if first, _, ok := strings.Cut(fwd, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(fwd)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitByUser membatasi per identitas user. Harus dipasang DI DALAM
// requireAuth supaya user sudah ada di context.
func (s *Server) rateLimitByUser(kind string, rule ratelimit.Rule, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := kind + ":" + userFrom(r.Context()).ID.String()
		if !s.allow(w, r, kind, key, rule) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimitByIP membatasi per alamat IP — bentuk yang dipakai endpoint yang
// BELUM punya user untuk dijadikan kunci.
//
// Aturannya ikut sebagai parameter sejak Fase 10, dan itu bukan kelenturan
// yang dicari-cari: jalur pemulihan password menyuruh server mengirim surat ke
// kotak masuk orang sungguhan, dan yang perlu dibatasi di sana bukan biaya
// argon2 melainkan kemampuan seseorang membanjiri alamat orang lain. Dua
// masalah yang berbeda pantas punya dua angka yang berbeda.
func (s *Server) rateLimitByIP(kind string, rule ratelimit.Rule, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := kind + ":ip:" + clientIP(r)
		if !s.allow(w, r, kind, key, rule) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allow menjalankan kuota dan menulis 429 bila habis.
//
// Kalau limiter sendiri yang error, permintaan DILOLOSKAN. Rate limit adalah
// pelindung, bukan bagian dari kebenaran aplikasi; membuat chat berhenti total
// gara-gara penghitung kuota bermasalah adalah menukar gangguan kecil dengan
// gangguan besar.
func (s *Server) allow(w http.ResponseWriter, r *http.Request, kind, key string, rule ratelimit.Rule) bool {
	res, err := s.limiter.Allow(r.Context(), key, rule)
	if err != nil {
		s.log.Error("rate limiter gagal", "kind", kind, "err", err)
		return true
	}
	if res.Allowed {
		return true
	}

	s.m.RateLimited.WithLabelValues(kind).Inc()
	retry := max(int(res.RetryAfter.Round(time.Second).Seconds()), 1)
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	writeError(w, http.StatusTooManyRequests, "terlalu banyak permintaan, coba lagi sebentar lagi")
	return false
}

// recoverer menjaga satu handler panik tidak menjatuhkan seluruh server —
// termasuk memutus semua koneksi WebSocket yang sedang aktif.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic",
					"path", r.URL.Path,
					"request_id", requestIDFrom(r.Context()),
					"recover", rec,
				)
				writeError(w, http.StatusInternalServerError, "terjadi kesalahan di server")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

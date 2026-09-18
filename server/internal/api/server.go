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
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jalilnawawi/chat-app/server/internal/blob"
	"github.com/jalilnawawi/chat-app/server/internal/config"
	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/mail"
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
	// dimatikan; mail nil berarti verifikasi email dan pemulihan password
	// dimatikan. Ketiganya fitur yang butuh infrastruktur di luar proses ini,
	// dan aplikasi harus tetap utuh sebagai chat tanpa ketiganya — itu yang
	// membuat `go run ./cmd/server` cukup untuk mengembangkan sisanya.
	blobs blob.Store
	push  *push.Dispatcher
	mail  *mail.Sender

	// thumbSem membatasi berapa gambar boleh dibentangkan di memori bersamaan.
	//
	// Bukan antrean: yang tidak kebagian tempat dalam beberapa detik TIDAK
	// menunggu, melainkan melanjutkan tanpa turunan. Alasannya sama dengan
	// antrean push yang membuang saat penuh — turunannya boleh tidak ada,
	// lampirannya tidak boleh gagal. Nil bila turunan dimatikan.
	thumbSem chan struct{}

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
	mailer *mail.Sender,
	log *slog.Logger,
	m *metrics.Metrics,
) *Server {
	var thumbSem chan struct{}
	if cfg.Thumbnails() {
		thumbSem = make(chan struct{}, cfg.ThumbConcurrency)
	}

	return &Server{
		cfg:      cfg,
		store:    st,
		hub:      h,
		limiter:  limiter,
		ws:       ws.NewHandler(st, h, limiter, cfg.TypingRate, log, m, cfg.AllowedOrigins),
		log:      log,
		m:        m,
		blobs:    blobs,
		push:     pusher,
		mail:     mailer,
		thumbSem: thumbSem,
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

	// Apa yang bisa dilakukan server ini. SENGAJA tanpa sesi.
	//
	// Halaman masuk sudah membutuhkannya sebelum siapa pun login: tombol "Lupa
	// password?" yang ditampilkan pada server tanpa SMTP mengantar orang ke
	// jalan buntu, dan orang yang lupa password adalah orang yang paling tidak
	// punya cadangan kesabaran.
	//
	// Isinya bukan rahasia baru: /healthz sudah melaporkan bagian opsional mana
	// yang menyala, di port yang sama, sejak Fase 7 — dan keduanya sekarang
	// membacanya dari SATU method supaya tidak bisa berselisih.
	mux.HandleFunc("GET /api/config", s.handleConfig)

	// Endpoint auth memverifikasi argon2id yang sengaja mahal, jadi kuotanya
	// per alamat IP — belum ada user yang bisa dijadikan kunci.
	mux.Handle("POST /api/auth/register",
		s.rateLimitByIP("auth", s.cfg.AuthRate, http.HandlerFunc(s.handleRegister)))
	mux.Handle("POST /api/auth/login",
		s.rateLimitByIP("auth", s.cfg.AuthRate, http.HandlerFunc(s.handleLogin)))
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Verifikasi dan pemulihan TIDAK menuntut sesi: keduanya diklik dari kotak
	// masuk, sering di perangkat yang berbeda dari tempat akunnya dipakai.
	// Kuotanya per alamat IP karena belum ada user yang bisa dijadikan kunci —
	// sama seperti login dan register.
	mux.Handle("POST /api/auth/verify-email",
		s.rateLimitByIP("auth", s.cfg.AuthRate, http.HandlerFunc(s.handleVerifyEmail)))

	// Pemulihan memakai kuota EMAIL, bukan kuota auth, dan dia satu-satunya
	// endpoint tanpa sesi yang begitu: yang perlu dibatasi di sini bukan biaya
	// argon2 melainkan kemampuan seseorang membanjiri kotak masuk orang lain
	// dengan tautan yang tidak mereka minta.
	mux.Handle("POST /api/auth/forgot-password",
		s.rateLimitByIP("email", s.cfg.EmailRate, http.HandlerFunc(s.handleForgotPassword)))

	mux.Handle("POST /api/auth/reset-password",
		s.rateLimitByIP("auth", s.cfg.AuthRate, http.HandlerFunc(s.handleResetPassword)))

	mux.Handle("GET /api/auth/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/users", s.requireAuth(http.HandlerFunc(s.handleSearchUsers)))

	// ---- kelola akun & profil (Fase 10) ----
	//
	// Kuotanya sendiri, dan sengaja ketat: tiap perubahan status disiarkan ke
	// SELURUH kontak, jadi yang dibatasi bukan beban server melainkan kemampuan
	// satu orang mengisi layar orang lain dengan perubahan yang tidak berarti.
	mux.Handle("PATCH /api/account",
		s.requireAuth(s.rateLimitByUser("profile", s.cfg.ProfileRate, http.HandlerFunc(s.handleUpdateProfile))))
	mux.Handle("PUT /api/account/status",
		s.requireAuth(s.rateLimitByUser("profile", s.cfg.ProfileRate, http.HandlerFunc(s.handleSetStatus))))
	mux.Handle("POST /api/account/password",
		s.requireAuth(s.rateLimitByUser("profile", s.cfg.ProfileRate, http.HandlerFunc(s.handleChangePassword))))

	// Jalur yang menyuruh server MENGIRIM SURAT punya kuotanya sendiri, jauh
	// lebih ketat: satu permintaan di sini berakhir di kotak masuk orang
	// sungguhan.
	mux.Handle("POST /api/account/email",
		s.requireAuth(s.rateLimitByUser("email", s.cfg.EmailRate, http.HandlerFunc(s.handleSetEmail))))
	mux.Handle("POST /api/account/email/verify",
		s.requireAuth(s.rateLimitByUser("email", s.cfg.EmailRate, http.HandlerFunc(s.handleResendVerification))))

	mux.Handle("GET /api/account/sessions", s.requireAuth(http.HandlerFunc(s.handleListSessions)))

	// Mencabut sesi TIDAK dibatasi kuota, dengan alasan yang sama dengan keluar
	// dari grup di Fase 9b: ini tindakan seseorang atas dirinya sendiri, dan
	// menahan orang yang sedang berusaha mengeluarkan perangkat asing dari
	// akunnya adalah bentuk penolakan yang tidak pernah pantas.
	mux.Handle("DELETE /api/account/sessions/{id}",
		s.requireAuth(http.HandlerFunc(s.handleRevokeSession)))

	// Mencabut SEMUA sesi lain sekaligus, dengan kelonggaran kuota yang sama:
	// ini jalur orang yang baru saja kehilangan ponselnya.
	mux.Handle("DELETE /api/account/sessions",
		s.requireAuth(http.HandlerFunc(s.handleRevokeOtherSessions)))

	// Avatar memakai kuota unggahan yang sama dengan lampiran — keduanya
	// mengalirkan berkas lewat proses ini.
	mux.Handle("POST /api/account/avatar",
		s.requireAuth(s.rateLimitByUser("upload", s.cfg.UploadRate, http.HandlerFunc(s.handleUploadAvatar))))
	mux.Handle("DELETE /api/account/avatar", s.requireAuth(http.HandlerFunc(s.handleRemoveAvatar)))

	// Izin bacanya BERBEDA dari lampiran, dan itu disengaja: siapa pun yang
	// sudah login boleh melihat avatar siapa pun. Lihat store.AvatarForRead.
	mux.Handle("GET /api/avatars/{id}", s.requireAuth(http.HandlerFunc(s.handleDownloadAvatar)))

	mux.Handle("GET /api/conversations", s.requireAuth(http.HandlerFunc(s.handleListConversations)))
	mux.Handle("POST /api/conversations/direct", s.requireAuth(http.HandlerFunc(s.handleCreateDirect)))
	mux.Handle("POST /api/conversations/group", s.requireAuth(http.HandlerFunc(s.handleCreateGroup)))
	mux.Handle("GET /api/conversations/{id}/messages", s.requireAuth(http.HandlerFunc(s.handleListMessages)))
	mux.Handle("POST /api/conversations/{id}/messages",
		s.requireAuth(s.rateLimitByUser("message", s.cfg.MessageRate, http.HandlerFunc(s.handleSendMessage))))
	mux.Handle("GET /api/conversations/{id}/members", s.requireAuth(http.HandlerFunc(s.handleMembers)))
	mux.Handle("POST /api/conversations/{id}/read", s.requireAuth(http.HandlerFunc(s.handleMarkRead)))

	// Pengelolaan grup. Kuotanya sendiri, dan sengaja ketat: tiap tindakan
	// menulis catatan sistem ke riwayat DAN menyiarkan dua event ke seluruh
	// anggota, jadi menambah lalu mengeluarkan orang berulang-ulang adalah cara
	// memenuhi percakapan orang lain dengan baris yang tidak mereka minta.
	mux.Handle("PATCH /api/conversations/{id}",
		s.requireAuth(s.rateLimitByUser("group", s.cfg.GroupRate, http.HandlerFunc(s.handleRenameGroup))))
	mux.Handle("POST /api/conversations/{id}/members",
		s.requireAuth(s.rateLimitByUser("group", s.cfg.GroupRate, http.HandlerFunc(s.handleAddMembers))))
	mux.Handle("DELETE /api/conversations/{id}/members/{userId}",
		s.requireAuth(s.rateLimitByUser("group", s.cfg.GroupRate, http.HandlerFunc(s.handleRemoveMember))))
	mux.Handle("POST /api/conversations/{id}/owner",
		s.requireAuth(s.rateLimitByUser("group", s.cfg.GroupRate, http.HandlerFunc(s.handleTransferOwnership))))

	// Keluar TIDAK dibatasi kuota. Ini satu-satunya tindakan yang dilakukan
	// seseorang atas dirinya sendiri, dan menahan orang di dalam grup karena dia
	// terlalu sering menekan tombol adalah bentuk penolakan yang tidak pernah
	// pantas.
	mux.Handle("POST /api/conversations/{id}/leave",
		s.requireAuth(http.HandlerFunc(s.handleLeaveGroup)))

	// Penanda sebutan punya endpoint SENDIRI, bukan menumpang /read. Keduanya
	// bergerak pada saat yang berbeda: terbaca saat ruangnya dibuka, sebutan
	// saat pesan yang memanggil namanya benar-benar terlihat. Satu endpoint
	// untuk keduanya berarti membuka ruang sekilas sudah cukup untuk melupakan
	// bahwa ada yang memanggil.
	mux.Handle("POST /api/conversations/{id}/mentions/ack",
		s.requireAuth(http.HandlerFunc(s.handleAckMentions)))

	// Unggahan punya kuotanya sendiri, jauh lebih ketat dari kirim pesan: satu
	// berkas bisa sepuluh megabyte yang melewati proses ini dua kali.
	mux.Handle("POST /api/attachments",
		s.requireAuth(s.rateLimitByUser("upload", s.cfg.UploadRate, http.HandlerFunc(s.handleUploadAttachment))))
	mux.Handle("GET /api/attachments/{id}", s.requireAuth(http.HandlerFunc(s.handleDownloadAttachment)))

	// Turunan punya jalurnya sendiri, bukan parameter query pada jalur di atas.
	// Keduanya adalah byte yang berbeda dan keduanya disimpan selamanya, jadi
	// keduanya layak punya alamat sendiri yang bisa di-cache sendiri — dan
	// alamat yang berbeda tidak bisa saling meracuni cache.
	mux.Handle("GET /api/attachments/{id}/thumb", s.requireAuth(http.HandlerFunc(s.handleDownloadThumbnail)))

	mux.Handle("POST /api/push/subscribe", s.requireAuth(http.HandlerFunc(s.handlePushSubscribe)))
	mux.Handle("POST /api/push/unsubscribe", s.requireAuth(http.HandlerFunc(s.handlePushUnsubscribe)))

	// ---- menemukan pesan (Fase 12) ----
	//
	// Pencarian punya kuotanya sendiri: dia satu-satunya kueri yang biayanya
	// sebanding dengan jumlah kecocokan, dan dia dijalankan sambil mengetik.
	mux.Handle("GET /api/search",
		s.requireAuth(s.rateLimitByUser("search", s.cfg.SearchRate, http.HandlerFunc(s.handleSearch))))
	mux.Handle("GET /api/conversations/{id}/pins", s.requireAuth(http.HandlerFunc(s.handleListPins)))

	// Sematan memakai kuota pengelolaan grup, dengan alasan yang sama persis:
	// tiap tindakan menulis catatan sistem ke riwayat semua anggota.
	mux.Handle("PUT /api/messages/{id}/pin",
		s.requireAuth(s.rateLimitByUser("group", s.cfg.GroupRate, http.HandlerFunc(s.handlePin))))
	mux.Handle("DELETE /api/messages/{id}/pin",
		s.requireAuth(s.rateLimitByUser("group", s.cfg.GroupRate, http.HandlerFunc(s.handleUnpin))))

	mux.Handle("PATCH /api/messages/{id}", s.requireAuth(http.HandlerFunc(s.handleEditMessage)))
	mux.Handle("DELETE /api/messages/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteMessage)))

	// Reaksi punya kuotanya sendiri: memasang dan mencabut adalah dua
	// permintaan yang bisa diulang secepat jari bergerak, dan tiap satunya
	// menulis ke database.
	mux.Handle("POST /api/messages/{id}/reactions",
		s.requireAuth(s.rateLimitByUser("reaction", s.cfg.ReactionRate, http.HandlerFunc(s.handleAddReaction))))
	mux.Handle("DELETE /api/messages/{id}/reactions",
		s.requireAuth(s.rateLimitByUser("reaction", s.cfg.ReactionRate, http.HandlerFunc(s.handleRemoveReaction))))

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
	s.ws.Serve(w, r, userFrom(r.Context()).User, sessionFrom(r.Context()).Hash)
}

// features melaporkan bagian opsional mana yang menyala di instance ini.
//
// SATU tempat, dibaca oleh /healthz maupun /api/config. Keduanya menjawab
// pertanyaan yang sama untuk dua pembaca yang berbeda — operator dan aplikasi —
// dan dua daftar yang disusun sendiri-sendiri adalah dua daftar yang suatu hari
// akan berbeda pendapat tentang apa yang sedang menyala.
func (s *Server) features() map[string]any {
	return map[string]any{
		"attachments": s.blobs != nil,
		"thumbnails":  s.thumbSem != nil,

		// Avatar menumpang blob.Store yang sama dengan lampiran DAN menuntut
		// pengolahan gambar, karena berkas aslinya dibuang setelah diperkecil.
		// Keduanya harus menyala; tanpa salah satunya tidak ada foto profil.
		"avatars": s.blobs != nil && s.thumbSem != nil,

		"push": s.push.Enabled(),
		"mail": s.mail.Enabled(),
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	// Fitur yang bisa dimatikan ikut dilaporkan supaya "kenapa lampirannya
	// tidak jalan di staging" bisa dijawab dengan satu permintaan, bukan dengan
	// membaca variabel lingkungan di mesin orang lain.
	body := s.features()
	body["status"] = "ok"
	body["instance"] = s.cfg.InstanceID
	writeJSON(w, http.StatusOK, body)
}

// handleConfig memberi tahu client apa yang bisa dilakukan server ini, supaya
// tombol yang pasti ditolak tidak pernah ditampilkan.
//
// Itu aturan yang sudah dipakai di tempat lain — panel kelola grup tidak
// menampilkan tombol yang bukan hak seseorang, bukan menampilkannya lalu
// membiarkan server menolak. Tombol yang selalu ada tapi kadang gagal membuat
// orang belajar bahwa pesan kesalahan di aplikasi ini boleh diabaikan, dan
// pelajaran itu terbawa ke pesan kesalahan yang benar-benar penting.
//
// Kunci publik VAPID ikut di sini, dan hanya di sini — /healthz tidak
// membawanya. Client TIDAK boleh menyimpannya di kodenya sendiri: kunci adalah
// urusan deployment, dan browser mengunci langganannya pada kunci yang dipakai
// saat mendaftar, sehingga kunci yang tertinggal di bundel frontend akan jadi
// kunci yang salah begitu server diganti.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	body := s.features()
	body["vapidPublicKey"] = s.push.PublicKey()

	// Dibiarkan boleh di-cache sebentar: jawabannya hanya berubah saat server
	// dijalankan ulang dengan variabel lingkungan yang berbeda, dan setiap
	// halaman yang dibuka memintanya satu kali.
	w.Header().Set("Cache-Control", "private, max-age=60")
	writeJSON(w, http.StatusOK, body)
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
	case errors.Is(err, store.ErrInvalid):
		writeError(w, http.StatusBadRequest, storeDetail(err, store.ErrInvalid, "masukan tidak valid"))
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "tidak ditemukan")
	case errors.Is(err, store.ErrForbidden):
		writeError(w, http.StatusForbidden, "tidak diizinkan")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, storeDetail(err, store.ErrConflict, "sudah ada atau bentrok"))
	default:
		s.log.Error(context, "err", err)
		writeError(w, http.StatusInternalServerError, "terjadi kesalahan di server")
	}
}

// storeDetail mengambil keterangan yang ditempelkan store pada sebuah error
// domain — "invalid: judul grup wajib diisi" jadi "judul grup wajib diisi".
//
// Hanya bentuk `fmt.Errorf("%w: keterangan", base)` yang dipercaya, karena
// hanya bentuk itu yang ditulis dengan sengaja untuk dibaca orang. Error yang
// membungkus base dari arah lain bisa saja membawa isi kueri atau nama tabel,
// dan yang itu dijawab dengan kalimat umum.
//
// Sebelumnya 409 selalu dijawab "sudah ada atau bentrok", dan "paling banyak 20
// pesan disematkan" adalah persis jenis penolakan yang tidak bisa ditebak dari
// kalimat itu.
func storeDetail(err, base error, fallback string) string {
	prefix := base.Error() + ": "
	if msg := err.Error(); strings.HasPrefix(msg, prefix) {
		return strings.TrimPrefix(msg, prefix)
	}
	return fallback
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

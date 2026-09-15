package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
)

type Config struct {
	DatabaseURL string
	HTTPAddr    string
	// AllowedOrigins berisi lebih dari satu entri karena "localhost" dan
	// "127.0.0.1" adalah origin yang BERBEDA di mata browser, walau menunjuk
	// mesin yang sama. Menyebut keduanya membuat aplikasi tetap jalan apa pun
	// yang diketik di address bar.
	AllowedOrigins []string
	SecureCookie   bool

	// RedisURL kosong berarti mode satu instance: hub dan rate limit memakai
	// memori proses. Ini bukan mode "kurang lengkap" — untuk pengembangan
	// lokal dia justru yang paling jujur, karena `go run` cukup untuk
	// menjalankan seluruh aplikasi.
	RedisURL string

	// InstanceID membedakan proses saat beberapa instance melayani bersamaan:
	// jadi anggota klaim presence di Redis, dan label di metrik supaya grafik
	// bisa dipisah per instance saat rolling deploy.
	InstanceID string

	// PresenceTTL adalah umur klaim presence sebuah instance di Redis.
	// Menentukan seberapa cepat user yang instance-nya mati mendadak berubah
	// jadi offline.
	PresenceTTL time.Duration

	MetricsAddr string

	DBMaxConns int32

	// ---- lampiran (Fase 7) ----
	// FilerURL kosong berarti lampiran dimatikan: aplikasi tetap jalan penuh
	// sebagai chat teks. Pola yang sama dengan REDIS_URL — satu variabel
	// menentukan apakah sebuah bagian infrastruktur ikut dipakai, dan tidak ada
	// jalur kode yang setengah hidup.
	FilerURL    string
	FilerPrefix string

	// MaxUploadBytes membatasi satu berkas. Ini batas KERAS di sisi server;
	// client memeriksa lebih dulu hanya supaya orang tidak menunggu unggahan
	// yang sudah pasti ditolak.
	MaxUploadBytes int64

	// OrphanTTL adalah umur lampiran yang sudah diunggah tapi tidak pernah jadi
	// dikirim, sebelum dibuang. Cukup panjang untuk orang yang menulis pesan
	// panjang sambil melampirkan foto, cukup pendek supaya sampah tidak
	// menumpuk berbulan-bulan.
	OrphanTTL time.Duration

	// ---- push notification (Fase 7) ----
	// Kunci kosong berarti notifikasi dimatikan. Kunci dibuat sekali lalu
	// disimpan: menggantinya membatalkan SEMUA langganan yang sudah ada,
	// karena browser mengunci langganannya pada kunci publik yang dipakai saat
	// mendaftar. Buat sepasang dengan `go run ./cmd/vapid`.
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	// VAPIDSubject adalah kontak yang bisa dihubungi operator layanan push
	// bila server ini bermasalah. Wajib berupa mailto: atau https:.
	VAPIDSubject string

	// ---- shutdown bertahap ----
	// DrainDelay: jeda antara /readyz mulai menjawab 503 dan koneksi mulai
	// ditutup. Load balancer butuh beberapa detik untuk berhenti mengirim
	// trafik baru; menutup lebih awal berarti memutus koneksi yang baru saja
	// diantar ke sini.
	DrainDelay time.Duration
	// DrainPeriod: rentang penyebaran penutupan WebSocket.
	DrainPeriod time.Duration
	// ShutdownTimeout: batas keras seluruh proses shutdown.
	ShutdownTimeout time.Duration

	// ---- kuota ----
	MessageRate ratelimit.Rule
	ConnectRate ratelimit.Rule
	TypingRate  ratelimit.Rule
	AuthRate    ratelimit.Rule
	UploadRate  ratelimit.Rule
	// PushRate bukan kuota melawan penyalahgunaan, melainkan peredam dering:
	// berapa kali sebuah percakapan boleh membangunkan satu orang. Bentuknya
	// token bucket yang sama karena masalahnya memang sama — dan versi
	// Redis-nya membuat peredam itu berlaku lintas instance.
	PushRate ratelimit.Rule
}

const defaultOrigins = "http://localhost:5174,http://127.0.0.1:5174,http://[::1]:5174"

func Load() (Config, error) {
	c := Config{
		DatabaseURL:    env("DATABASE_URL", "postgres://chat:chat@localhost:5433/chatapp?sslmode=disable"),
		HTTPAddr:       env("HTTP_ADDR", ":8090"),
		AllowedOrigins: splitOrigins(env("ALLOWED_ORIGIN", defaultOrigins)),
		RedisURL:       os.Getenv("REDIS_URL"),
		InstanceID:     env("INSTANCE_ID", defaultInstanceID()),
		MetricsAddr:    env("METRICS_ADDR", ":9091"),

		FilerURL:    os.Getenv("SEAWEED_FILER_URL"),
		FilerPrefix: env("SEAWEED_PREFIX", "/chat/attachments"),

		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    env("VAPID_SUBJECT", "mailto:admin@example.com"),
	}

	if len(c.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("ALLOWED_ORIGIN kosong")
	}

	var err error
	if c.SecureCookie, err = envBool("SECURE_COOKIE", false); err != nil {
		return Config{}, err
	}
	if c.PresenceTTL, err = envDuration("PRESENCE_TTL", 45*time.Second); err != nil {
		return Config{}, err
	}
	if c.DrainDelay, err = envDuration("DRAIN_DELAY", 5*time.Second); err != nil {
		return Config{}, err
	}
	if c.DrainPeriod, err = envDuration("DRAIN_PERIOD", 20*time.Second); err != nil {
		return Config{}, err
	}
	if c.ShutdownTimeout, err = envDuration("SHUTDOWN_TIMEOUT", 60*time.Second); err != nil {
		return Config{}, err
	}

	if c.OrphanTTL, err = envDuration("ATTACHMENT_ORPHAN_TTL", 6*time.Hour); err != nil {
		return Config{}, err
	}

	maxConns, err := envInt("DB_MAX_CONNS", 0)
	if err != nil {
		return Config{}, err
	}
	c.DBMaxConns = int32(maxConns)

	maxUpload, err := envInt("MAX_UPLOAD_BYTES", 10<<20)
	if err != nil {
		return Config{}, err
	}
	if maxUpload < 1 {
		return Config{}, fmt.Errorf("MAX_UPLOAD_BYTES harus positif")
	}
	c.MaxUploadBytes = int64(maxUpload)

	// Nilai default dipilih dari perilaku manusia, bukan angka bulat yang enak
	// dilihat: 120 pesan per menit adalah dua pesan per detik terus-menerus —
	// jauh di atas orang mengetik sungguhan, tapi cukup untuk menghentikan
	// skrip yang membanjiri ruang grup.
	if c.MessageRate, err = envRule("RATE_MESSAGES", 20, 120); err != nil {
		return Config{}, err
	}
	if c.ConnectRate, err = envRule("RATE_CONNECTS", 10, 60); err != nil {
		return Config{}, err
	}
	if c.TypingRate, err = envRule("RATE_TYPING", 20, 120); err != nil {
		return Config{}, err
	}
	// Login dan register jauh lebih ketat: keduanya memverifikasi argon2id,
	// yang sengaja mahal. Tanpa batas, satu penyerang bisa menghabiskan CPU
	// seluruh instance hanya dengan menebak password.
	if c.AuthRate, err = envRule("RATE_AUTH", 10, 30); err != nil {
		return Config{}, err
	}
	// Unggahan jauh lebih mahal dari pesan teks — satu berkas bisa sepuluh
	// megabyte yang melewati proses ini dua kali, masuk dan keluar.
	if c.UploadRate, err = envRule("RATE_UPLOADS", 10, 60); err != nil {
		return Config{}, err
	}
	// Satu dering per percakapan tiap 30 detik, dengan kelonggaran dua di awal
	// supaya kabar pertama tidak pernah tertelan. Angkanya dipilih dari cara
	// orang membalas pesan, bukan dari kemampuan server mengirim.
	if c.PushRate, err = envRule("RATE_PUSH", 2, 2); err != nil {
		return Config{}, err
	}

	// Satu kunci terisi dan satu kosong hampir selalu berarti salah salin, dan
	// akibatnya adalah fitur yang diam-diam mati padahal terlihat dikonfigurasi.
	// Lebih baik gagal saat start daripada baru ketahuan saat seseorang
	// mengeluh notifikasinya tidak pernah datang.
	if (c.VAPIDPublicKey == "") != (c.VAPIDPrivateKey == "") {
		return Config{}, fmt.Errorf("VAPID_PUBLIC_KEY dan VAPID_PRIVATE_KEY harus diisi berdua atau dikosongkan berdua")
	}
	if c.Push() && !strings.HasPrefix(c.VAPIDSubject, "mailto:") && !strings.HasPrefix(c.VAPIDSubject, "https://") {
		return Config{}, fmt.Errorf("VAPID_SUBJECT harus diawali mailto: atau https://")
	}

	if c.ShutdownTimeout <= c.DrainDelay+c.DrainPeriod {
		return Config{}, fmt.Errorf(
			"SHUTDOWN_TIMEOUT (%s) harus lebih besar dari DRAIN_DELAY + DRAIN_PERIOD (%s)",
			c.ShutdownTimeout, c.DrainDelay+c.DrainPeriod)
	}

	return c, nil
}

// MultiInstance melaporkan apakah konfigurasi ini siap dijalankan lebih dari
// satu proses sekaligus.
func (c Config) MultiInstance() bool { return c.RedisURL != "" }

// Attachments melaporkan apakah unggahan lampiran menyala.
func (c Config) Attachments() bool { return c.FilerURL != "" }

// Push melaporkan apakah notifikasi menyala.
func (c Config) Push() bool { return c.VAPIDPublicKey != "" && c.VAPIDPrivateKey != "" }

// AllowsOrigin melaporkan apakah origin permintaan termasuk yang diizinkan.
func (c Config) AllowsOrigin(origin string) bool {
	for _, o := range c.AllowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

func defaultInstanceID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "server"
	}
	// Hostname saja tidak cukup: dua proses di satu mesin (persis yang terjadi
	// saat menguji rolling deploy lokal) akan berbagi ID dan saling menimpa
	// klaim presence.
	return host + "-" + uuid.NewString()[:8]
}

func splitOrigins(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func envInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return v, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("%s harus lebih dari nol", key)
	}
	return v, nil
}

// envRule membaca kuota dari dua variabel: <name>_BURST dan <name>_PER_MIN.
func envRule(name string, burst int, perMinute float64) (ratelimit.Rule, error) {
	b, err := envInt(name+"_BURST", burst)
	if err != nil {
		return ratelimit.Rule{}, err
	}
	raw := os.Getenv(name + "_PER_MIN")
	if raw != "" {
		perMinute, err = strconv.ParseFloat(raw, 64)
		if err != nil {
			return ratelimit.Rule{}, fmt.Errorf("%s_PER_MIN: %w", name, err)
		}
	}
	if b < 1 || perMinute <= 0 {
		return ratelimit.Rule{}, fmt.Errorf("%s: burst dan laju harus positif", name)
	}
	return ratelimit.PerMinute(b, perMinute), nil
}

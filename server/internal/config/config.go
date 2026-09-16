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

	// ---- turunan gambar (Fase 8) ----
	// ThumbMaxDim adalah panjang sisi terpanjang turunan. Nol mematikan
	// pembuatan turunan sama sekali, dan aplikasi kembali menyajikan berkas
	// asli untuk semua gambar — pola satu-variabel yang sama dengan REDIS_URL
	// dan SEAWEED_FILER_URL.
	ThumbMaxDim int

	// ThumbMaxPixels adalah batas jumlah piksel yang boleh didekode, dan
	// satuannya PIKSEL, bukan byte. Berkas PNG seratus kilobyte bisa berisi
	// kanvas 40.000 x 40.000 — sah menurut standar, dan enam gigabyte begitu
	// dibentangkan di memori. MAX_UPLOAD_BYTES tidak menolongnya sama sekali.
	ThumbMaxPixels int

	// ThumbConcurrency membatasi berapa gambar boleh didekode BERSAMAAN.
	//
	// Satu foto dua belas megapiksel menjadi sekitar lima puluh megabyte piksel
	// mentah saat dibentangkan, dan angka itu tidak ada hubungannya dengan
	// ukuran berkasnya. Tanpa batas ini, dua puluh unggahan yang kebetulan
	// bersamaan adalah satu gigabyte yang tidak pernah direncanakan siapa pun.
	ThumbConcurrency int

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

	// ---- akun & profil (Fase 10) ----

	// SMTPURL kosong berarti fitur email MATI dan aplikasinya tetap utuh: orang
	// masih bisa mendaftar, login, dan memakai seluruh chat — yang tidak ada
	// cuma verifikasi alamat dan pemulihan password lewat tautan.
	//
	// Pola yang sama persis dengan REDIS_URL, SEAWEED_FILER_URL, dan kunci
	// VAPID. Bentuknya "smtp://user:sandi@host:587" (STARTTLS) atau
	// "smtps://..." (TLS sejak byte pertama).
	SMTPURL string

	// MailFrom adalah alamat pengirim yang akan dilihat orang. Wajib diisi bila
	// SMTP_URL diisi — surat tanpa pengirim yang jelas adalah surat yang
	// berakhir di folder spam.
	MailFrom string

	// AppURL adalah awalan alamat yang dipakai menyusun tautan di dalam email.
	// Tidak bisa diturunkan dari permintaan HTTP: surat verifikasi dikirim dari
	// proses yang sama, tapi tautannya harus tetap benar kalau suatu hari
	// dikirim oleh job yang tidak punya permintaan sama sekali.
	AppURL string

	// AvatarMaxDim adalah panjang sisi terpanjang foto profil setelah
	// diperkecil. Kecil dengan sengaja: avatar tidak pernah ditampilkan lebih
	// lebar dari beberapa puluh piksel, dan yang aslinya DIBUANG — menyimpan
	// foto dua belas megapiksel berarti membayar selamanya untuk sesuatu yang
	// tidak akan pernah dilihat pada ukuran itu.
	AvatarMaxDim int

	// AvatarMaxBytes membatasi berkas yang diterima jalur avatar. Jauh lebih
	// kecil dari MAX_UPLOAD_BYTES karena yang masuk ke sini SELALU dibentangkan
	// di memori untuk diperkecil — tidak ada jalur "simpan apa adanya" seperti
	// pada lampiran.
	AvatarMaxBytes int64

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
	// ReactionRate melindungi jalur yang paling mudah diulang dengan jari:
	// menekan dan melepas emoji adalah dua permintaan yang bisa datang secepat
	// tombolnya bisa diklik, dan tiap satunya menulis ke database.
	ReactionRate ratelimit.Rule
	// MentionAllRate dihitung per user PER PERCAKAPAN. Satu orang yang
	// membangunkan dua ratus orang sekaligus adalah hal yang harus dibatasi,
	// bukan dilarang — yang tidak boleh adalah mengulanginya tiap beberapa
	// detik.
	MentionAllRate ratelimit.Rule
	// ProfileRate membatasi perubahan akun: nama, status, email, dan password.
	// Ketat bukan karena mahal di server, melainkan karena tiap perubahan status
	// disiarkan ke SELURUH kontak — dan orang yang menggeser statusnya
	// bolak-balik tiap detik sedang mengisi layar orang lain.
	ProfileRate ratelimit.Rule

	// EmailRate membatasi jalur yang menyuruh server MENGIRIM SURAT. Satu
	// permintaan di sini berakhir di kotak masuk orang sungguhan, dan yang
	// dibatasi bukan beban server melainkan kemampuan satu orang membanjiri
	// alamat orang lain dengan tautan yang tidak dia minta.
	EmailRate ratelimit.Rule

	// GroupRate membatasi pengelolaan grup. Tiap tindakan menulis catatan
	// sistem ke riwayat semua anggota sekaligus menyiarkan dua event, jadi yang
	// dibatasi di sini bukan beban server melainkan kemampuan satu orang
	// memenuhi percakapan orang lain dengan baris yang tidak mereka minta.
	GroupRate ratelimit.Rule
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

		SMTPURL:  os.Getenv("SMTP_URL"),
		MailFrom: os.Getenv("MAIL_FROM"),
	}

	if len(c.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("ALLOWED_ORIGIN kosong")
	}

	// Bawaannya origin pertama yang diizinkan. Itu hampir selalu benar — dia
	// memang alamat tempat aplikasi ini dibuka — dan membuat pengembangan lokal
	// tidak menuntut satu variabel lagi hanya untuk menghasilkan tautan yang
	// sudah jelas.
	c.AppURL = strings.TrimRight(env("APP_URL", c.AllowedOrigins[0]), "/")

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

	// 480 piksel: cukup untuk layar yang rapat pada gelembung pesan selebar
	// 300 piksel, dan masih sekitar seperseratus ukuran foto aslinya.
	if c.ThumbMaxDim, err = envInt("THUMBNAIL_MAX_DIM", 480); err != nil {
		return Config{}, err
	}
	if c.ThumbMaxDim != 0 && (c.ThumbMaxDim < 16 || c.ThumbMaxDim > 4096) {
		return Config{}, fmt.Errorf("THUMBNAIL_MAX_DIM harus 0 (mati) atau antara 16 dan 4096")
	}
	// 50 megapiksel kira-kira dua kali kamera ponsel kelas atas — longgar untuk
	// foto sungguhan, jauh di bawah kanvas yang dibuat untuk meledakkan memori.
	if c.ThumbMaxPixels, err = envInt("THUMBNAIL_MAX_PIXELS", 50_000_000); err != nil {
		return Config{}, err
	}
	if c.ThumbMaxPixels < 1 {
		return Config{}, fmt.Errorf("THUMBNAIL_MAX_PIXELS harus positif")
	}
	// Bawaannya sengaja kecil dan TIDAK mengikuti jumlah inti mesin: yang
	// dibatasi di sini memori, bukan CPU, dan mesin berinti banyak justru yang
	// paling mudah kehabisan memori karena batasnya ikut membesar.
	if c.ThumbConcurrency, err = envInt("THUMBNAIL_CONCURRENCY", 4); err != nil {
		return Config{}, err
	}
	if c.ThumbConcurrency < 1 {
		return Config{}, fmt.Errorf("THUMBNAIL_CONCURRENCY harus positif")
	}

	// 256 piksel: dua kali lipat ukuran avatar terbesar yang ditampilkan
	// aplikasi ini, jadi layar ber-DPI tinggi tetap tajam, dan tetap sekitar
	// dua ribu kali lebih kecil daripada foto kamera ponsel.
	if c.AvatarMaxDim, err = envInt("AVATAR_MAX_DIM", 256); err != nil {
		return Config{}, err
	}
	if c.AvatarMaxDim < 32 || c.AvatarMaxDim > 1024 {
		return Config{}, fmt.Errorf("AVATAR_MAX_DIM harus antara 32 dan 1024")
	}
	maxAvatar, err := envInt("AVATAR_MAX_BYTES", 5<<20)
	if err != nil {
		return Config{}, err
	}
	if maxAvatar < 1 {
		return Config{}, fmt.Errorf("AVATAR_MAX_BYTES harus positif")
	}
	c.AvatarMaxBytes = int64(maxAvatar)

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
	// Reaksi jauh lebih longgar dari pesan: menekan lima emoji beruntun pada
	// satu pesan lucu adalah perilaku manusia yang wajar, dan tiap penekanan
	// jauh lebih murah daripada satu pesan.
	if c.ReactionRate, err = envRule("RATE_REACTIONS", 30, 240); err != nil {
		return Config{}, err
	}
	// Dua @semua beruntun untuk satu percakapan, lalu satu tiap dua menit.
	// Angkanya dipilih dari cara rapat sungguhan diumumkan, bukan dari
	// kemampuan server mengirim.
	if c.MentionAllRate, err = envRule("RATE_MENTION_ALL", 2, 0.5); err != nil {
		return Config{}, err
	}
	// Mengelola grup adalah tindakan sesekali, bukan sesuatu yang dilakukan
	// terus-menerus: sepuluh beruntun sudah cukup untuk menyusun grup baru
	// dalam sekali duduk, dan tiga puluh per menit jauh di atas kecepatan orang
	// membaca daftar namanya sendiri.
	if c.GroupRate, err = envRule("RATE_GROUP", 10, 30); err != nil {
		return Config{}, err
	}
	// Mengganti nama atau status adalah tindakan sesekali. Dua puluh beruntun
	// cukup untuk mencoba-coba pilihan status dalam sekali duduk, enam puluh per
	// menit jauh di atas kecepatan orang membaca akibatnya di layar sendiri.
	if c.ProfileRate, err = envRule("RATE_PROFILE", 20, 60); err != nil {
		return Config{}, err
	}
	// Tiga surat beruntun, lalu satu tiap dua menit. Angkanya dipilih dari
	// perilaku orang yang tautannya belum sampai dan menekan "kirim ulang"
	// beberapa kali — bukan dari kemampuan server mengirim.
	if c.EmailRate, err = envRule("RATE_EMAIL", 3, 0.5); err != nil {
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
	// Satu variabel terisi dan pasangannya kosong hampir selalu berarti salah
	// salin — dan akibatnya adalah tautan verifikasi yang tidak pernah sampai,
	// dengan server yang terlihat baik-baik saja.
	if c.SMTPURL != "" && c.MailFrom == "" {
		return Config{}, fmt.Errorf("SMTP_URL diisi tapi MAIL_FROM kosong")
	}

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

// Thumbnails melaporkan apakah turunan gambar dibuat. Ikut mati sendiri bila
// lampirannya mati — tidak ada gambar untuk diperkecil.
func (c Config) Thumbnails() bool { return c.Attachments() && c.ThumbMaxDim > 0 }

// Mail melaporkan apakah pengiriman email menyala. Yang mati bukan aplikasinya,
// melainkan verifikasi alamat dan pemulihan password lewat tautan.
func (c Config) Mail() bool { return c.SMTPURL != "" }

// Avatars melaporkan apakah foto profil bisa diunggah. Ikut mati sendiri bila
// lampirannya mati: byte-nya lewat blob.Store yang sama, dan tanpa penyimpanan
// tidak ada tempat menaruhnya.
func (c Config) Avatars() bool { return c.Attachments() }

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

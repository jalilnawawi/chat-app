package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/jalilnawawi/chat-app/server/internal/api"
	"github.com/jalilnawawi/chat-app/server/internal/config"
	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/metrics"
	"github.com/jalilnawawi/chat-app/server/internal/migrations"
	"github.com/jalilnawawi/chat-app/server/internal/ratelimit"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

func main() {
	cfg, cfgErr := config.Load()

	// Log JSON begitu ada lebih dari satu instance: log gabungan dari beberapa
	// proses hanya bisa disaring per field kalau field-nya memang terstruktur.
	// Untuk satu instance, teks tetap lebih enak dibaca di terminal.
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if cfgErr == nil && cfg.MultiInstance() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	log := slog.New(handler)

	if cfgErr != nil {
		log.Error("konfigurasi tidak valid", "err", cfgErr)
		os.Exit(1)
	}
	log = log.With("instance", cfg.InstanceID)

	if err := run(cfg, log); err != nil {
		log.Error("server berhenti", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	m := metrics.New()
	m.BuildInfo.WithLabelValues(cfg.InstanceID, runtime.Version()).Set(1)
	m.Draining.Set(0)

	// ---------- database ----------

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if cfg.DBMaxConns > 0 {
		poolCfg.MaxConns = cfg.DBMaxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	m.MustRegister(metrics.NewPoolCollector(pool))

	pingCtx, cancelPing := context.WithTimeout(ctx, 10*time.Second)
	err = pool.Ping(pingCtx)
	cancelPing()
	if err != nil {
		return errors.New("tidak bisa terhubung ke Postgres — sudah jalan `docker compose up -d`? (" + err.Error() + ")")
	}

	if err := migrations.Run(ctx, pool); err != nil {
		return err
	}
	log.Info("migrasi database selesai")

	st := store.New(pool)

	// ---------- hub & rate limit ----------

	broadcaster, limiter, closeRedis, err := buildBackplane(ctx, cfg, log, m)
	if err != nil {
		return err
	}
	defer func() {
		if err := broadcaster.Close(); err != nil {
			log.Warn("menutup hub", "err", err)
		}
		_ = limiter.Close()
		closeRedis()
	}()

	// ---------- server ----------

	srv := api.NewServer(cfg, st, broadcaster, limiter, log, m)

	httpSrv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: srv.Routes(),

		ReadHeaderTimeout: 10 * time.Second,
		// Sengaja TIDAK memasang WriteTimeout: koneksi WebSocket berumur panjang
		// dan akan diputus paksa olehnya. Batas waktu untuk WebSocket ditangani
		// per operasi tulis di paket ws.
		IdleTimeout: 120 * time.Second,
	}

	metricsSrv := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           srv.MetricsRoutes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	janitorDone := startSessionJanitor(ctx, st, log)

	errCh := make(chan error, 2)
	go func() {
		log.Info("server siap",
			"addr", cfg.HTTPAddr,
			"origins", strings.Join(cfg.AllowedOrigins, ", "),
			"multi_instance", cfg.MultiInstance(),
		)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		log.Info("metrics siap", "addr", cfg.MetricsAddr, "path", "/metrics")
		if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdown(cfg, srv, httpSrv, metricsSrv, broadcaster, log)
	<-janitorDone
	return nil
}

// buildBackplane memilih antara mode satu instance dan multi-instance.
//
// Keputusannya hanya satu variabel lingkungan, dan itu disengaja: yang berubah
// adalah cara event menyeberang antar proses, bukan aturan percakapannya. Kalau
// menambah instance menuntut perubahan kode, orang akan menundanya sampai
// terlambat.
func buildBackplane(
	ctx context.Context,
	cfg config.Config,
	log *slog.Logger,
	m *metrics.Metrics,
) (hub.Broadcaster, ratelimit.Limiter, func(), error) {
	if !cfg.MultiInstance() {
		log.Info("mode satu instance: hub dan rate limit memakai memori proses")
		return hub.NewMemory(log, m), ratelimit.NewMemory(), func() {}, nil
	}

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, nil, nil, err
	}
	rdb := redis.NewClient(opts)

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	h, err := hub.NewRedis(connectCtx, rdb, cfg.InstanceID, cfg.PresenceTTL, log, m)
	if err != nil {
		_ = rdb.Close()
		return nil, nil, nil, errors.New("tidak bisa terhubung ke Redis: " + err.Error())
	}

	log.Info("mode multi-instance: siaran lewat Redis pub/sub",
		"presence_ttl", cfg.PresenceTTL.String())

	return h, ratelimit.NewRedis(rdb, log), func() { _ = rdb.Close() }, nil
}

// shutdown menurunkan instance secara bertahap.
//
// Urutannya yang membuat rolling deploy tidak terasa oleh user:
//
//  1. /readyz mulai menjawab 503. Load balancer berhenti mengirim koneksi baru,
//     tapi yang sudah tersambung tidak diganggu sama sekali.
//  2. Jeda. LB butuh beberapa detik untuk benar-benar berhenti mengirim; menutup
//     lebih awal justru memutus koneksi yang baru saja diantar ke sini.
//  3. Koneksi WebSocket ditutup TERSEBAR, bukan serentak, setelah tiap client
//     diberi tahu bahwa ini pamit terencana. Client menyambung ke instance lain
//     dan melanjutkan lewat resume — pesan yang lewat di sela itu tetap sampai.
//  4. Baru HTTP dimatikan.
//
// Langkah 3 tidak bisa diwakilkan ke http.Server.Shutdown: koneksi WebSocket
// sudah di-hijack dari server HTTP, jadi Shutdown tidak melihatnya dan akan
// selesai seolah semuanya beres sementara ribuan koneksi masih terbuka.
func shutdown(
	cfg config.Config,
	srv *api.Server,
	httpSrv, metricsSrv *http.Server,
	broadcaster hub.Broadcaster,
	log *slog.Logger,
) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	log.Info("mulai menguras instance",
		"koneksi", broadcaster.LocalConnections(),
		"jeda_lb", cfg.DrainDelay.String(),
		"periode_kuras", cfg.DrainPeriod.String(),
	)

	srv.StartDraining()

	select {
	case <-ctx.Done():
	case <-time.After(cfg.DrainDelay):
	}

	broadcaster.Drain(ctx, cfg.DrainPeriod)
	waitDrained(ctx, broadcaster, 5*time.Second)

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Warn("shutdown http belum tuntas", "err", err)
	}
	if err := metricsSrv.Shutdown(ctx); err != nil {
		log.Warn("shutdown metrics belum tuntas", "err", err)
	}

	log.Info("instance berhenti", "koneksi_tersisa", broadcaster.LocalConnections())
}

// waitDrained menunggu koneksi benar-benar melepaskan diri dari registry.
//
// Drain hanya MEMINTA tiap koneksi menutup; yang mencatat pelepasan adalah
// goroutine koneksi itu sendiri, beberapa milidetik kemudian. Tanpa jeda ini,
// baris log terakhir melaporkan sisa koneksi yang sebenarnya sedang pamit baik-
// baik — angka yang terbaca seolah instance memutus orang secara paksa, persis
// hal yang ingin dibuktikan TIDAK terjadi.
func waitDrained(ctx context.Context, broadcaster hub.Broadcaster, limit time.Duration) {
	// Batas waktunya sendiri, jauh lebih pendek dari anggaran shutdown
	// keseluruhan: kalau ada koneksi yang tetap membandel, sisa waktu lebih
	// berguna untuk menuntaskan permintaan HTTP yang sedang berjalan daripada
	// dihabiskan menunggu satu socket.
	waitCtx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for broadcaster.LocalConnections() > 0 {
		select {
		case <-waitCtx.Done():
			return
		case <-ticker.C:
		}
	}
}

// startSessionJanitor membuang sesi kedaluwarsa secara berkala.
//
// Barisnya tidak pernah dibaca lagi setelah lewat expires_at, tapi tanpa
// pembersihan tabelnya tumbuh selamanya — dan index yang ikut membengkak
// memperlambat pemeriksaan sesi, yang dijalankan pada SETIAP permintaan.
func startSessionJanitor(ctx context.Context, st *store.Store, log *slog.Logger) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				n, err := st.DeleteExpiredSessions(opCtx)
				cancel()

				if err != nil {
					log.Warn("bersih-bersih sesi gagal", "err", err)
					continue
				}
				if n > 0 {
					log.Info("sesi kedaluwarsa dibuang", "jumlah", n)
				}
			}
		}
	}()

	return done
}

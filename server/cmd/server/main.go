package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jalilnawawi/chat-app/server/internal/api"
	"github.com/jalilnawawi/chat-app/server/internal/config"
	"github.com/jalilnawawi/chat-app/server/internal/hub"
	"github.com/jalilnawawi/chat-app/server/internal/migrations"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(log); err != nil {
		log.Error("server berhenti", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return errors.New("tidak bisa terhubung ke Postgres — sudah jalan `docker compose up -d`? (" + err.Error() + ")")
	}

	if err := migrations.Run(ctx, pool); err != nil {
		return err
	}
	log.Info("migrasi database selesai")

	st := store.New(pool)
	h := hub.New(log)
	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: api.NewServer(cfg, st, h, log).Routes(),

		ReadHeaderTimeout: 10 * time.Second,
		// Sengaja TIDAK memasang WriteTimeout: koneksi WebSocket berumur panjang
		// dan akan diputus paksa olehnya. Batas waktu untuk WebSocket ditangani
		// per operasi tulis di paket ws.
		IdleTimeout: 120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server siap", "addr", cfg.HTTPAddr, "origins", strings.Join(cfg.AllowedOrigins, ", "))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("mematikan server...")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	return srv.Shutdown(shutdownCtx)
}

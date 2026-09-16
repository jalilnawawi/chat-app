package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/jalilnawawi/chat-app/server/internal/metrics"
)

// Memory menyiarkan event di dalam satu proses.
//
// Ini implementasi default saat REDIS_URL kosong. Presence-nya adalah tabel
// koneksi itu sendiri: satu-satunya sumber kebenaran, dan selalu akurat karena
// tidak ada instance lain yang perlu ditanya.
type Memory struct {
	reg *registry
	m   *metrics.Metrics
}

var _ Broadcaster = (*Memory)(nil)

func NewMemory(log *slog.Logger, m *metrics.Metrics) *Memory {
	return &Memory{reg: newRegistry(log, m), m: m}
}

func (h *Memory) Register(_ context.Context, c Sink) (bool, error) {
	return h.reg.add(c), nil
}

func (h *Memory) Unregister(_ context.Context, c Sink) (bool, error) {
	return h.reg.remove(c), nil
}

// Publish menyiarkan satu event ke semua koneksi milik target.
//
// JSON di-encode SEKALI lalu byte yang sama dibagikan ke semua penerima —
// di ruang grup, ini beda antara satu encode dan puluhan.
func (h *Memory) Publish(targets []uuid.UUID, ev Event) {
	payload, err := json.Marshal(ev)
	if err != nil {
		h.reg.log.Error("encode event", "type", ev.Type, "err", err)
		return
	}

	start := time.Now()
	receivers := h.reg.deliver(targets, payload)

	h.m.EventsPublished.WithLabelValues(ev.Type).Inc()
	h.m.BroadcastDuration.WithLabelValues("memory").Observe(time.Since(start).Seconds())
	h.m.BroadcastFanout.Observe(float64(receivers))
}

func (h *Memory) OnlineAmong(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	return h.reg.onlineLocal(ids), nil
}

// Keduanya langsung menyentuh tabel koneksi: dalam mode satu instance, setiap
// koneksi milik user ini memang dipegang oleh proses ini.
func (h *Memory) RevokeSessionsExcept(userID uuid.UUID, keep []byte) {
	h.reg.revoke(userID, keep, nil)
}

func (h *Memory) RevokeSession(userID uuid.UUID, hash []byte) {
	h.reg.revoke(userID, nil, hash)
}

func (h *Memory) LocalConnections() int { return h.reg.count() }

func (h *Memory) Drain(ctx context.Context, period time.Duration) {
	h.reg.drain(ctx, period)
}

func (h *Memory) Close() error { return nil }

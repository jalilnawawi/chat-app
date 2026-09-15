package ratelimit

import (
	"context"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Memory menyimpan bucket di proses. Dipakai saat REDIS_URL kosong, dan juga
// sebagai jaring pengaman: kalau Redis mendadak tidak bisa dihubungi, lebih
// baik membatasi per instance daripada tidak membatasi sama sekali.
type Memory struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	stop chan struct{}
	once sync.Once
}

var _ Limiter = (*Memory)(nil)

func NewMemory() *Memory {
	m := &Memory{buckets: make(map[string]*bucket), stop: make(chan struct{})}
	go m.sweep()
	return m
}

func (m *Memory) Allow(_ context.Context, key string, rule Rule) (Result, error) {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.buckets[key]
	if !ok {
		b = &bucket{tokens: float64(rule.Burst), last: now}
		m.buckets[key] = b
	}

	b.tokens = min(float64(rule.Burst), b.tokens+now.Sub(b.last).Seconds()*rule.Refill)
	b.last = now

	if b.tokens < 1 {
		return Result{RetryAfter: retryAfter(b.tokens, rule)}, nil
	}

	b.tokens--
	return Result{Allowed: true, Remaining: int(b.tokens)}, nil
}

func (m *Memory) Close() error {
	m.once.Do(func() { close(m.stop) })
	return nil
}

// sweep membuang bucket yang sudah lama tidak disentuh. Tanpa ini, map-nya
// tumbuh seukuran jumlah user yang pernah datang, bukan yang sedang aktif —
// kebocoran memori yang pelan dan baru terlihat setelah berminggu-minggu.
func (m *Memory) sweep() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stop:
			return
		case now := <-ticker.C:
			m.mu.Lock()
			for key, b := range m.buckets {
				if now.Sub(b.last) > 10*time.Minute {
					delete(m.buckets, key)
				}
			}
			m.mu.Unlock()
		}
	}
}

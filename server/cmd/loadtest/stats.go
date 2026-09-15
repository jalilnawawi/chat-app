package main

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

// samples mengumpulkan durasi mentah, bukan rata-rata berjalan.
//
// Rata-rata adalah angka paling menyesatkan untuk sistem realtime: satu persen
// user yang menunggu tiga detik tenggelam sempurna di balik rata-rata 40 ms,
// padahal justru merekalah yang merasa aplikasinya rusak. Menyimpan semuanya
// membuat p95 dan p99 bisa dihitung apa adanya — dan pada beban yang diuji di
// sini, jumlah sampelnya masih puluhan ribu, bukan sesuatu yang perlu dihemat.
type samples struct {
	mu     sync.Mutex
	values []time.Duration
}

func newSamples(capacity int) *samples {
	return &samples{values: make([]time.Duration, 0, capacity)}
}

func (s *samples) add(d time.Duration) {
	s.mu.Lock()
	s.values = append(s.values, d)
	s.mu.Unlock()
}

func (s *samples) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.values)
}

// summary mengurutkan salinan dan mengambil persentilnya.
func (s *samples) summary() stat {
	s.mu.Lock()
	sorted := slices.Clone(s.values)
	s.mu.Unlock()

	if len(sorted) == 0 {
		return stat{}
	}
	slices.Sort(sorted)

	var total time.Duration
	for _, v := range sorted {
		total += v
	}

	return stat{
		N:    len(sorted),
		Min:  sorted[0],
		P50:  percentile(sorted, 0.50),
		P95:  percentile(sorted, 0.95),
		P99:  percentile(sorted, 0.99),
		Max:  sorted[len(sorted)-1],
		Mean: total / time.Duration(len(sorted)),
	}
}

type stat struct {
	N                       int
	Min, P50, P95, P99, Max time.Duration
	Mean                    time.Duration
}

func (s stat) String() string {
	if s.N == 0 {
		return "tidak ada sampel"
	}
	return fmt.Sprintf("n=%-6d p50=%-9s p95=%-9s p99=%-9s maks=%-9s",
		s.N, round(s.P50), round(s.P95), round(s.P99), round(s.Max))
}

// percentile memakai nearest-rank: tanpa interpolasi, nilai yang dilaporkan
// selalu benar-benar pernah terjadi.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p*float64(len(sorted))+0.5) - 1
	return sorted[max(min(idx, len(sorted)-1), 0)]
}

func round(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(10 * time.Millisecond)
	case d >= time.Millisecond:
		return d.Round(100 * time.Microsecond)
	default:
		return d.Round(time.Microsecond)
	}
}

package metrics

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// PoolCollector membaca statistik pool pgx saat scrape, bukan lewat ticker.
//
// Pool yang kehabisan koneksi adalah salah satu cara paling sunyi sebuah chat
// server melambat: WebSocket tetap terbuka, presence tetap hijau, tapi tiap
// kirim pesan menunggu giliran koneksi. `acquire_wait_seconds` yang naik adalah
// tanda paling awal bahwa DATABASE_MAX_CONNS perlu dinaikkan.
type PoolCollector struct {
	pool *pgxpool.Pool

	acquired     *prometheus.Desc
	idle         *prometheus.Desc
	total        *prometheus.Desc
	max          *prometheus.Desc
	constructing *prometheus.Desc
	acquireCount *prometheus.Desc
	acquireWait  *prometheus.Desc
	emptyAcquire *prometheus.Desc
	canceled     *prometheus.Desc
}

func NewPoolCollector(pool *pgxpool.Pool) *PoolCollector {
	d := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc(namespace+"_db_pool_"+name, help, nil, nil)
	}
	return &PoolCollector{
		pool:         pool,
		acquired:     d("acquired_conns", "Koneksi yang sedang dipakai."),
		idle:         d("idle_conns", "Koneksi menganggur yang siap pakai."),
		total:        d("total_conns", "Total koneksi dalam pool."),
		max:          d("max_conns", "Batas atas koneksi pool."),
		constructing: d("constructing_conns", "Koneksi yang sedang dibangun."),
		acquireCount: d("acquires_total", "Total pengambilan koneksi."),
		acquireWait:  d("acquire_wait_seconds_total", "Akumulasi waktu menunggu koneksi."),
		emptyAcquire: d("empty_acquires_total", "Pengambilan yang harus menunggu karena pool kosong."),
		canceled:     d("canceled_acquires_total", "Pengambilan yang dibatalkan sebelum dapat koneksi."),
	}
}

func (c *PoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquired
	ch <- c.idle
	ch <- c.total
	ch <- c.max
	ch <- c.constructing
	ch <- c.acquireCount
	ch <- c.acquireWait
	ch <- c.emptyAcquire
	ch <- c.canceled
}

func (c *PoolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	g := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v)
	}
	cnt := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
	}

	g(c.acquired, float64(s.AcquiredConns()))
	g(c.idle, float64(s.IdleConns()))
	g(c.total, float64(s.TotalConns()))
	g(c.max, float64(s.MaxConns()))
	g(c.constructing, float64(s.ConstructingConns()))
	cnt(c.acquireCount, float64(s.AcquireCount()))
	cnt(c.acquireWait, s.AcquireDuration().Seconds())
	cnt(c.emptyAcquire, float64(s.EmptyAcquireCount()))
	cnt(c.canceled, float64(s.CanceledAcquireCount()))
}

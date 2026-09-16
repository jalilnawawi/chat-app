// Package metrics mengumpulkan seluruh instrumen Prometheus di satu tempat.
//
// Alasan dikumpulkan, bukan disebar sebagai variabel global di tiap paket:
// nama metrik adalah API yang dilihat dashboard dan alert. Menaruhnya dalam
// satu struct membuat daftar itu bisa dibaca sekali jalan, dan membuat paket
// lain menerimanya lewat parameter — sehingga test bisa memakai registry
// sendiri tanpa bentrok "duplicate metrics collector registration".
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

const namespace = "chat"

type Metrics struct {
	Registry *prometheus.Registry

	// ---- HTTP ----
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec

	// ---- WebSocket ----
	WSActive      prometheus.Gauge
	WSAccepted    prometheus.Counter
	WSClosed      *prometheus.CounterVec
	WSSlowDropped prometheus.Counter
	WSSendQueue   prometheus.Histogram
	WSResumeSent  prometheus.Counter
	WSResumeFail  prometheus.Counter

	// ---- Siaran ----
	EventsPublished *prometheus.CounterVec
	// BroadcastDuration adalah lag siaran: waktu dari "event siap dikirim"
	// sampai byte-nya masuk antrean semua penerima. Ini angka yang paling cepat
	// membusuk saat sistem mulai kewalahan, jadi dia yang dipantau.
	BroadcastDuration *prometheus.HistogramVec
	BroadcastFanout   prometheus.Histogram

	// ---- Redis ----
	RedisPublishDuration prometheus.Histogram
	RedisErrors          *prometheus.CounterVec
	RedisSubscriptions   prometheus.Gauge

	// ---- Lampiran ----
	AttachmentUploads   *prometheus.CounterVec
	AttachmentBytes     prometheus.Counter
	AttachmentDownloads *prometheus.CounterVec
	AttachmentSwept     prometheus.Counter

	// ---- Turunan gambar (Fase 8) ----
	// Thumbnails berlabel hasil karena "dilewati" BUKAN kegagalan: gambar yang
	// sudah kecil memang tidak perlu turunan. Yang layak membangunkan operator
	// hanyalah "gagal" dan "sibuk" — yang kedua berarti batas dekode bersamaan
	// terlalu ketat untuk beban yang sedang berjalan.
	Thumbnails       *prometheus.CounterVec
	ThumbnailBytes   prometheus.Counter
	ThumbnailSeconds prometheus.Histogram

	// RangeRequests menghitung permintaan sepotong. Naiknya angka ini adalah
	// bukti bahwa orang benar-benar melompati video, bukan mengunduhnya utuh.
	RangeRequests *prometheus.CounterVec

	// ---- Push notification ----
	PushSent    *prometheus.CounterVec
	PushSkipped *prometheus.CounterVec
	PushDropped prometheus.Counter
	// PushMentionBypass menghitung kabar yang MELEWATI peredam dering karena
	// menyebut nama penerimanya. Dipisahkan dari skipped_total supaya
	// "peredamnya bekerja" dan "peredamnya ditembus" tidak pernah terbaca
	// sebagai satu angka.
	PushMentionBypass prometheus.Counter
	PushDuration      prometheus.Histogram

	// ---- Rate limit ----
	RateLimited *prometheus.CounterVec

	// ---- Siklus hidup ----
	Draining  prometheus.Gauge
	BuildInfo *prometheus.GaugeVec
}

// New membangun set metrik lengkap di registry-nya sendiri.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{Registry: reg}

	m.HTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "http", Name: "requests_total",
		Help: "Jumlah permintaan HTTP per rute dan status.",
	}, []string{"method", "route", "status"})

	m.HTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "http", Name: "request_duration_seconds",
		Help:    "Durasi permintaan HTTP.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"method", "route"})

	m.WSActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: "ws", Name: "connections_active",
		Help: "Koneksi WebSocket yang sedang terbuka di instance ini.",
	})

	m.WSAccepted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "ws", Name: "connections_accepted_total",
		Help: "Total koneksi WebSocket yang pernah diterima.",
	})

	m.WSClosed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "ws", Name: "connections_closed_total",
		Help: "Total koneksi WebSocket yang ditutup, per sebab.",
	}, []string{"reason"})

	m.WSSlowDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "ws", Name: "slow_clients_dropped_total",
		Help: "Koneksi yang diputus karena buffer kirimnya penuh (backpressure).",
	})

	m.WSSendQueue = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "ws", Name: "send_queue_depth",
		Help:    "Isi antrean kirim per koneksi saat event dimasukkan.",
		Buckets: []float64{0, 1, 2, 4, 8, 16, 32, 48, 64},
	})

	m.WSResumeSent = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "ws", Name: "resume_messages_total",
		Help: "Pesan yang dikirim ulang ke client setelah reconnect.",
	})

	m.WSResumeFail = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "ws", Name: "resume_aborted_total",
		Help: "Sinkronisasi ulang yang berhenti sebelum selesai (client terlalu lambat atau putus).",
	})

	m.EventsPublished = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "events_published_total",
		Help: "Event yang disiarkan, per tipe.",
	}, []string{"type"})

	m.BroadcastDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace, Name: "broadcast_duration_seconds",
		Help:    "Lag siaran: dari event siap sampai masuk antrean semua penerima.",
		Buckets: []float64{.0001, .00025, .0005, .001, .0025, .005, .01, .025, .05, .1, .25, 1},
	}, []string{"transport"})

	m.BroadcastFanout = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Name: "broadcast_fanout_receivers",
		Help:    "Jumlah koneksi penerima per siaran.",
		Buckets: []float64{1, 2, 5, 10, 25, 50, 100, 250, 500},
	})

	m.RedisPublishDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "redis", Name: "publish_duration_seconds",
		Help:    "Durasi PUBLISH ke Redis (pipeline satu siaran).",
		Buckets: []float64{.0001, .00025, .0005, .001, .0025, .005, .01, .025, .05, .1},
	})

	m.RedisErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "redis", Name: "errors_total",
		Help: "Kegagalan operasi Redis, per operasi.",
	}, []string{"op"})

	m.RedisSubscriptions = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Subsystem: "redis", Name: "subscriptions_active",
		Help: "Channel Redis yang sedang dilanggan instance ini (satu per user online lokal).",
	})

	m.AttachmentUploads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "uploads_total",
		Help: "Unggahan lampiran, per hasil (ok, ditolak, gagal simpan).",
	}, []string{"result"})

	m.AttachmentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "uploaded_bytes_total",
		Help: "Total byte lampiran yang masuk ke penyimpanan.",
	})

	m.AttachmentDownloads = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "downloads_total",
		Help: "Pengambilan lampiran, per hasil.",
	}, []string{"result"})

	m.AttachmentSwept = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "swept_total",
		Help: "Lampiran yatim yang dibuang: diunggah tapi tidak pernah jadi dikirim.",
	})

	m.Thumbnails = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "thumbnails_total",
		Help: "Pembuatan turunan gambar, per hasil (ok, dilewati, sibuk, gagal).",
	}, []string{"result"})

	m.ThumbnailBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "thumbnail_bytes_total",
		Help: "Total byte turunan yang ditulis ke penyimpanan.",
	})

	m.ThumbnailSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "thumbnail_seconds",
		Help: "Lama mendekode dan memperkecil satu gambar.",
		// Rentangnya jauh lebih lebar dari histogram lain di berkas ini karena
		// yang diukur memang bukan I/O melainkan kerja CPU pada gambar yang
		// ukurannya berbeda seratus kali lipat.
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	})

	m.RangeRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "attachments", Name: "range_requests_total",
		Help: "Permintaan sepotong berkas, per hasil (sepotong, utuh, tidak_terpenuhi).",
	}, []string{"result"})

	m.PushSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "push", Name: "sent_total",
		Help: "Notifikasi yang dikirim ke layanan push, per hasil.",
	}, []string{"result"})

	m.PushSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "push", Name: "skipped_total",
		Help: "Notifikasi yang sengaja TIDAK dikirim, per alasan (online, debounce).",
	}, []string{"reason"})

	m.PushMentionBypass = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "push", Name: "mention_bypass_total",
		Help: "Notifikasi yang menembus peredam dering karena menyebut nama penerimanya.",
	})

	m.PushDropped = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace, Subsystem: "push", Name: "dropped_total",
		Help: "Notifikasi yang dibuang karena antrean pengirim penuh.",
	})

	m.PushDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace, Subsystem: "push", Name: "send_duration_seconds",
		Help:    "Durasi satu kiriman ke layanan push milik vendor browser.",
		Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	})

	m.RateLimited = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace, Name: "rate_limited_total",
		Help: "Permintaan yang ditolak rate limiter, per jenis kuota.",
	}, []string{"kind"})

	m.Draining = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace, Name: "draining",
		Help: "1 bila instance sedang dikuras menjelang shutdown, 0 bila melayani normal.",
	})

	m.BuildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace, Name: "build_info",
		Help: "Selalu 1; labelnya yang berguna untuk membedakan instance saat rolling deploy.",
	}, []string{"instance", "go_version"})

	reg.MustRegister(
		m.HTTPRequests, m.HTTPDuration,
		m.WSActive, m.WSAccepted, m.WSClosed, m.WSSlowDropped, m.WSSendQueue,
		m.WSResumeSent, m.WSResumeFail,
		m.EventsPublished, m.BroadcastDuration, m.BroadcastFanout,
		m.RedisPublishDuration, m.RedisErrors, m.RedisSubscriptions,
		m.AttachmentUploads, m.AttachmentBytes, m.AttachmentDownloads, m.AttachmentSwept,
		m.Thumbnails, m.ThumbnailBytes, m.ThumbnailSeconds, m.RangeRequests,
		m.PushSent, m.PushSkipped, m.PushDropped, m.PushMentionBypass, m.PushDuration,
		m.RateLimited, m.Draining, m.BuildInfo,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return m
}

// MustRegister menambahkan collector pihak ketiga (mis. statistik pool pgx)
// ke registry yang sama.
func (m *Metrics) MustRegister(cs ...prometheus.Collector) {
	m.Registry.MustRegister(cs...)
}

// Command loadtest membebani server chat dengan ribuan koneksi WebSocket
// sungguhan dan mengukur berapa lama sebuah pesan sampai ke layar orang lain.
//
// Kenapa ditulis sendiri, bukan memakai k6 atau sejenisnya? Karena angka yang
// paling menentukan rasa sebuah aplikasi chat adalah latensi FAN-OUT: jeda
// antara A menekan kirim dan B melihat pesannya. Mengukurnya menuntut satu
// proses memegang kedua sisi percakapan sekaligus. Perkakas load test umumnya
// mengisolasi tiap virtual user justru supaya mereka tidak saling memengaruhi —
// isolasi yang membuat pengukuran lintas-user jadi mustahil.
//
// Jalankan lawan server yang kuota autentikasinya dilonggarkan, karena tahap
// penyiapan membuat ribuan sesi dari satu alamat IP:
//
//	RATE_AUTH_PER_MIN=100000 RATE_AUTH_BURST=1000 go run ./cmd/server
//	go run ./cmd/loadtest -users 1000 -rate 50 -duration 60s
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type options struct {
	bases       []string
	metricsURLs []string
	users       int
	groupSize   int
	rate        float64
	duration    time.Duration
	ramp        time.Duration
	bodySize    int
	prefix      string
	password    string
	provision   int
	sendWorkers int
	settle      time.Duration
}

func main() {
	var o options
	// Daftar, bukan satu alamat: user disebar bergiliran ke tiap instance,
	// sehingga pengirim dan penerima sebuah pesan sering berada di proses yang
	// BERBEDA. Itulah satu-satunya cara membuktikan jalur Redis pub/sub benar-
	// benar bekerja, bukan cuma tidak error.
	bases := flag.String("base", "http://127.0.0.1:8090", "alamat server, pisahkan dengan koma untuk multi-instance")
	metricsURLs := flag.String("metrics", "http://127.0.0.1:9091/metrics", "endpoint metrik, pisahkan dengan koma (kosongkan untuk melewati)")
	flag.IntVar(&o.users, "users", 1000, "jumlah user dan koneksi WebSocket")
	flag.IntVar(&o.groupSize, "group-size", 10, "anggota per ruang grup")
	flag.Float64Var(&o.rate, "rate", 50, "pesan per detik secara keseluruhan (0 = hanya koneksi menganggur)")
	flag.DurationVar(&o.duration, "duration", 60*time.Second, "lama fase pengiriman")
	flag.DurationVar(&o.ramp, "ramp", 30*time.Second, "rentang waktu membuka koneksi")
	flag.IntVar(&o.bodySize, "size", 80, "panjang badan pesan dalam karakter")
	flag.StringVar(&o.prefix, "prefix", "loadtest", "awalan username")
	flag.StringVar(&o.password, "password", "loadtest-password", "password akun uji")
	flag.IntVar(&o.provision, "provision-concurrency", 4, "penyiapan akun paralel (argon2id memakai ~64MB per hash)")
	flag.IntVar(&o.sendWorkers, "send-workers", 64, "pengirim paralel")
	flag.DurationVar(&o.settle, "settle", 3*time.Second, "jeda menunggu pesan terakhir sampai sebelum laporan")
	flag.Parse()

	o.bases = splitList(*bases)
	o.metricsURLs = splitList(*metricsURLs)
	if len(o.bases) == 0 {
		log.Fatal("-base tidak boleh kosong")
	}

	if err := run(o); err != nil {
		log.Fatalf("load test gagal: %v", err)
	}
}

func run(o options) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// runID memisahkan pesan uji kali ini dari sisa uji sebelumnya yang masih
	// tersimpan di database. Tanpa itu, riwayat lama yang ikut terkirim saat
	// sinkronisasi akan terhitung sebagai pengiriman dengan latensi berjam-jam.
	runID := uuid.NewString()[:8]
	r := &runner{
		opts:     o,
		runID:    runID,
		connect:  newSamples(o.users),
		post:     newSamples(4096),
		delivery: newSamples(1 << 16),
		http:     newHTTPClient(o.users),
	}

	fmt.Printf("Load test chat — %d user, grup %d orang, %.0f pesan/detik, %s\n",
		o.users, o.groupSize, o.rate, o.duration)
	fmt.Printf("Target %s (run %s)\n\n", strings.Join(o.bases, ", "), runID)

	if err := r.provision(ctx); err != nil {
		return err
	}
	if err := r.buildGroups(ctx); err != nil {
		return err
	}
	if err := r.connectAll(ctx); err != nil {
		return err
	}
	defer r.closeAll()

	r.sendPhase(ctx)

	fmt.Printf("Menunggu %s untuk pesan yang masih di jalan...\n", o.settle)
	select {
	case <-ctx.Done():
	case <-time.After(o.settle):
	}

	r.report(ctx)
	return nil
}

type runner struct {
	opts  options
	runID string
	http  *http.Client

	clients []*apiClient
	groups  []group

	connect  *samples
	post     *samples
	delivery *samples

	sent        atomic.Int64
	expected    atomic.Int64
	received    atomic.Int64
	sendErrors  atomic.Int64
	rateLimited atomic.Int64
	connErrors  atomic.Int64
	connDropped atomic.Int64

	wg sync.WaitGroup
}

type group struct {
	convID  uuid.UUID
	members []int // indeks ke runner.clients
}

// newHTTPClient menyetel pool koneksi seukuran beban.
//
// Bawaan Go hanya menyimpan 2 koneksi menganggur per host. Dengan ratusan
// pengirim paralel, sisanya membuka dan menutup TCP terus-menerus — dan yang
// terukur akhirnya adalah biaya handshake milik alat ujinya sendiri, bukan
// kemampuan servernya.
func newHTTPClient(users int) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			MaxIdleConns:        users + 256,
			MaxIdleConnsPerHost: users + 256,
			MaxConnsPerHost:     0,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// ---------- fase 1: akun ----------

func (r *runner) provision(ctx context.Context) error {
	start := time.Now()
	r.clients = make([]*apiClient, r.opts.users)

	sem := make(chan struct{}, r.opts.provision)
	var wg sync.WaitGroup
	var failed atomic.Int64
	var firstErr atomic.Value

	for i := range r.opts.users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			base := r.opts.bases[i%len(r.opts.bases)]
			c := &apiClient{
				base:     base,
				wsBase:   wsScheme(base),
				http:     r.http,
				username: fmt.Sprintf("%s-%05d", r.opts.prefix, i),
				password: r.opts.password,
			}
			if err := r.withRetry(ctx, func() error { return c.ensureAccount(ctx) }); err != nil {
				failed.Add(1)
				firstErr.CompareAndSwap(nil, err)
				return
			}
			r.clients[i] = c
		}()
	}
	wg.Wait()

	if n := failed.Load(); n > 0 {
		err, _ := firstErr.Load().(error)
		return fmt.Errorf("%d dari %d akun gagal disiapkan (contoh: %v)", n, r.opts.users, err)
	}

	fmt.Printf("  akun siap    : %d dalam %s (tersebar ke %d instance)\n",
		r.opts.users, time.Since(start).Round(time.Millisecond), len(r.opts.bases))
	return nil
}

// withRetry menghormati Retry-After.
//
// Kuota yang menendang balik BUKAN kegagalan uji — justru bukti rate limiter
// bekerja. Yang salah adalah alat uji yang menerjemahkannya jadi error dan
// menghentikan seluruh pengukuran.
func (r *runner) withRetry(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := range 6 {
		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		var he *httpError
		if !asHTTPError(err, &he) || he.Status != http.StatusTooManyRequests {
			return err
		}

		wait := he.RetryAfter
		if wait <= 0 {
			wait = time.Duration(1<<attempt) * time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return lastErr
}

// ---------- fase 2: ruang percakapan ----------

// buildGroups memakai ulang ruang dari uji sebelumnya bila judulnya cocok.
// Membuat ruang baru tiap kali akan menumpuk percakapan mati yang ikut
// memperlambat query daftar percakapan — memperburuk hasil uji berikutnya
// karena alasan yang tidak ada hubungannya dengan yang sedang diukur.
func (r *runner) buildGroups(ctx context.Context) error {
	start := time.Now()

	size := min(r.opts.groupSize, r.opts.users)
	count := r.opts.users / size
	if count == 0 {
		return errors.New("jumlah user lebih kecil dari satu grup")
	}

	r.groups = make([]group, count)
	created, reused := 0, 0

	for g := range count {
		members := make([]int, 0, size)
		for j := range size {
			members = append(members, g*size+j)
		}

		owner := r.clients[members[0]]
		// Ukuran grup ikut masuk judul. Tanpa itu, uji dengan -group-size
		// berbeda akan memakai ulang ruang lama yang anggotanya lebih sedikit,
		// dan user yang sebenarnya bukan anggota mendapat 404 saat mengirim —
		// kegagalan yang terlihat seperti masalah server padahal berasal dari
		// alat ujinya sendiri.
		title := fmt.Sprintf("%s-g%02d-%04d", r.opts.prefix, size, g)

		existing, err := owner.listConversations(ctx)
		if err != nil {
			return fmt.Errorf("daftar percakapan: %w", err)
		}

		var convID uuid.UUID
		for _, conv := range existing {
			if conv.Title != nil && *conv.Title == title {
				convID = conv.ID
				reused++
				break
			}
		}

		if convID == uuid.Nil {
			ids := make([]uuid.UUID, 0, len(members)-1)
			for _, idx := range members[1:] {
				ids = append(ids, r.clients[idx].id)
			}
			if convID, err = owner.createGroup(ctx, title, ids); err != nil {
				return fmt.Errorf("buat grup: %w", err)
			}
			created++
		}

		r.groups[g] = group{convID: convID, members: members}
		for _, idx := range members {
			r.clients[idx].convs = append(r.clients[idx].convs, convID)
		}
	}

	fmt.Printf("  ruang siap   : %d (%d baru, %d dipakai ulang) dalam %s\n",
		count, created, reused, time.Since(start).Round(time.Millisecond))
	return nil
}

// ---------- fase 3: koneksi ----------

// connectAll membuka koneksi tersebar sepanjang ramp.
//
// Membuka seribu koneksi sekaligus mengukur hal yang salah: yang terlihat
// adalah antrean accept, bukan kemampuan server melayani seribu koneksi yang
// sudah mapan. Naik bertahap juga lebih mirip kenyataan — user datang satu per
// satu, bukan serentak.
func (r *runner) connectAll(ctx context.Context) error {
	start := time.Now()

	gap := time.Duration(0)
	if r.opts.ramp > 0 && len(r.clients) > 0 {
		gap = r.opts.ramp / time.Duration(len(r.clients))
	}

	var wg sync.WaitGroup
	for i, c := range r.clients {
		if ctx.Err() != nil {
			break
		}
		if gap > 0 && i > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(gap):
			}
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			dialStart := time.Now()
			dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			if err := c.connect(dialCtx); err != nil {
				r.connErrors.Add(1)
				return
			}
			r.connect.add(time.Since(dialStart))

			if err := c.sync(dialCtx); err != nil {
				r.connErrors.Add(1)
				return
			}

			r.wg.Add(1)
			go r.readLoop(ctx, c)
		}()
	}
	wg.Wait()

	active := len(r.clients) - int(r.connErrors.Load())
	fmt.Printf("  koneksi      : %d/%d terbuka dalam %s\n\n",
		active, len(r.clients), time.Since(start).Round(time.Millisecond))

	if active == 0 {
		return errors.New("tidak ada koneksi yang berhasil dibuka")
	}
	return nil
}

// readLoop mencatat kapan tiap pesan uji tiba.
func (r *runner) readLoop(ctx context.Context, c *apiClient) {
	defer r.wg.Done()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if ctx.Err() == nil {
				r.connDropped.Add(1)
			}
			return
		}

		// Waktu dicatat SEBELUM parsing. Membaca jam setelah decode berarti
		// biaya alat ujinya sendiri ikut terhitung sebagai latensi server.
		now := time.Now()

		sentAt, ok := parseProbe(data, r.runID)
		if !ok {
			continue
		}
		r.received.Add(1)
		r.delivery.add(now.Sub(sentAt))
	}
}

func (r *runner) closeAll() {
	for _, c := range r.clients {
		if c != nil && c.conn != nil {
			c.conn.Close(websocket.StatusNormalClosure, "")
		}
	}
	r.wg.Wait()
}

// ---------- fase 4: pengiriman ----------

func (r *runner) sendPhase(ctx context.Context) {
	if r.opts.rate <= 0 {
		fmt.Printf("Fase menganggur: %d koneksi ditahan selama %s\n", len(r.clients), r.opts.duration)
		select {
		case <-ctx.Done():
		case <-time.After(r.opts.duration):
		}
		return
	}

	fmt.Printf("Fase kirim: %.0f pesan/detik selama %s\n", r.opts.rate, r.opts.duration)

	sendCtx, cancel := context.WithTimeout(ctx, r.opts.duration)
	defer cancel()

	jobs := make(chan int, r.opts.sendWorkers*2)
	var workers sync.WaitGroup
	for range r.opts.sendWorkers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for g := range jobs {
				r.sendOne(sendCtx, g)
			}
		}()
	}

	// Ticker, bukan sleep di dalam loop: sleep menambahkan waktu kerja ke
	// jeda, sehingga laju yang benar-benar terjadi selalu di bawah target dan
	// selisihnya tumbuh persis saat server melambat — sumber angka yang
	// menyesatkan.
	interval := time.Duration(float64(time.Second) / r.opts.rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	start := time.Now()
loop:
	for {
		select {
		case <-sendCtx.Done():
			break loop
		case <-ticker.C:
			select {
			case jobs <- rand.IntN(len(r.groups)):
			default:
				// Antrean pengirim penuh: server tidak menyusul. Dicatat
				// sebagai kehilangan laju, bukan diam-diam ditunda.
				r.sendErrors.Add(1)
			}
		}
	}

	close(jobs)
	workers.Wait()

	elapsed := time.Since(start)
	fmt.Printf("  terkirim     : %d dalam %s (%.1f/detik)\n\n",
		r.sent.Load(), elapsed.Round(time.Millisecond),
		float64(r.sent.Load())/elapsed.Seconds())
}

func (r *runner) sendOne(ctx context.Context, groupIdx int) {
	g := r.groups[groupIdx]
	sender := r.clients[g.members[rand.IntN(len(g.members))]]

	body := makeProbe(r.runID, time.Now(), r.opts.bodySize)

	start := time.Now()
	err := sender.sendMessage(ctx, g.convID, body)
	elapsed := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			return
		}
		var he *httpError
		if asHTTPError(err, &he) && he.Status == http.StatusTooManyRequests {
			r.rateLimited.Add(1)
			return
		}
		r.sendErrors.Add(1)
		return
	}

	r.post.add(elapsed)
	r.sent.Add(1)
	// Setiap anggota ruang, termasuk pengirim, seharusnya menerima satu salinan.
	r.expected.Add(int64(len(g.members)))
}

// ---------- laporan ----------

func (r *runner) report(ctx context.Context) {
	sent := r.sent.Load()
	expected := r.expected.Load()
	received := r.received.Load()

	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println(" HASIL")
	fmt.Println("════════════════════════════════════════════════════════════")

	fmt.Printf(" Koneksi        %d dibuka · %d gagal · %d putus di tengah jalan\n",
		r.connect.count(), r.connErrors.Load(), r.connDropped.Load())
	fmt.Printf(" Waktu connect  %s\n", r.connect.summary())

	if sent > 0 {
		fmt.Printf(" Pesan terkirim %d · POST %s\n", sent, r.post.summary())

		ratio := float64(received) / float64(expected) * 100
		fmt.Printf(" Pengiriman     %d/%d salinan sampai (%.2f%%)\n", received, expected, ratio)
		fmt.Printf(" Latensi fanout %s\n", r.delivery.summary())
	}

	if n := r.sendErrors.Load(); n > 0 {
		fmt.Printf(" Gagal kirim    %d\n", n)
	}
	if n := r.rateLimited.Load(); n > 0 {
		fmt.Printf(" Kena kuota     %d (rate limiter bekerja — bukan kegagalan)\n", n)
	}

	for _, url := range r.opts.metricsURLs {
		fmt.Printf("\n Dari sisi server (%s):\n", url)
		r.printServerMetrics(ctx, url)
	}
	fmt.Println("════════════════════════════════════════════════════════════")
}

// printServerMetrics mengambil beberapa angka dari sisi server.
//
// Yang diukur alat ini adalah apa yang DIRASAKAN client. Metrik server
// menjawab pertanyaan berikutnya — kalau lambat, lambatnya di mana — dan
// menaruh keduanya berdampingan menghemat satu babak tebak-tebakan.
func (r *runner) printServerMetrics(ctx context.Context, url string) {
	wanted := []string{
		"chat_ws_connections_active",
		"chat_ws_slow_clients_dropped_total",
		"chat_ws_connections_closed_total",
		"chat_rate_limited_total",
		"chat_redis_subscriptions_active",
		"chat_events_published_total",
		"chat_redis_errors_total",
		"chat_db_pool_acquired_conns",
		"chat_db_pool_max_conns",
		"chat_db_pool_acquire_wait_seconds_total",
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	resp, err := r.http.Do(req)
	if err != nil {
		fmt.Printf("   (metrik tidak terbaca: %v)\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	for line := range strings.Lines(string(body)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, name := range wanted {
			if strings.HasPrefix(line, name) {
				fmt.Printf("   %s\n", line)
				break
			}
		}
	}
}

func splitList(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func wsScheme(base string) string {
	base = strings.Replace(base, "https://", "wss://", 1)
	return strings.Replace(base, "http://", "ws://", 1)
}

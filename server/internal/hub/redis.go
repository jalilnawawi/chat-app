package hub

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/jalilnawawi/chat-app/server/internal/metrics"
)

const (
	channelPrefix  = "chat:u:"
	presencePrefix = "chat:presence:"
)

// Redis menyiarkan event lintas instance lewat pub/sub, dan menyimpan presence
// sebagai key ber-TTL alih-alih di memori proses.
//
// Satu channel per user (`chat:u:<uuid>`), bukan satu channel global. Bedanya
// terasa saat instance bertambah: dengan channel global setiap instance
// menerima dan men-decode SEMUA event aplikasi lalu membuang yang bukan
// miliknya; dengan channel per user, Redis yang menyaring, dan sebuah instance
// hanya menerima byte yang memang akan dia tulis ke socket.
//
// Instance juga menerima kembali event yang dia publish sendiri, dan itu
// disengaja: hanya ada SATU jalur pengiriman, dari langganan ke registry.
// Menambah jalan pintas lokal berarti dua jalur yang harus sama-sama benar dan
// tidak boleh mengirim dobel — biaya yang tidak sepadan dengan hematnya satu
// perjalanan ke Redis (di bawah satu milidetik) dibanding total waktu sebuah
// pesan sampai ke layar.
type Redis struct {
	reg        *registry
	rdb        *redis.Client
	pubsub     *redis.PubSub
	instanceID string
	ttl        time.Duration
	log        *slog.Logger
	m          *metrics.Metrics

	subsMu sync.Mutex
	subs   map[uuid.UUID]struct{}

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

var _ Broadcaster = (*Redis)(nil)

// Skrip presence dijalankan sebagai Lua supaya "bersihkan yang basi, catat
// diriku, lalu hitung" jadi satu operasi atomik. Tanpa itu, dua instance yang
// connect/disconnect bersamaan bisa sama-sama menyimpulkan dirinya yang pertama
// (atau sama-sama bukan yang terakhir), dan presence jadi bohong.
var (
	// Anggota sorted set adalah instance ID, skornya waktu kedaluwarsa.
	// Instance yang mati tidak sempat membersihkan dirinya; skor basi itulah
	// yang membuatnya tersapu sendiri pada operasi berikutnya.
	presenceJoin = redis.NewScript(`
		redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[3])
		redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
		redis.call('EXPIRE', KEYS[1], ARGV[4])
		return redis.call('ZCARD', KEYS[1])
	`)

	presenceLeave = redis.NewScript(`
		redis.call('ZREM', KEYS[1], ARGV[1])
		redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[2])
		local n = redis.call('ZCARD', KEYS[1])
		if n == 0 then redis.call('DEL', KEYS[1]) end
		return n
	`)
)

// NewRedis menyiapkan hub multi-instance. presenceTTL adalah berapa lama sebuah
// instance dianggap masih memegang user tanpa kabar; penyegaran dikirim pada
// sepertiga nilai itu supaya satu penyegaran yang meleset tidak langsung
// membuat user terlihat offline.
func NewRedis(
	ctx context.Context,
	rdb *redis.Client,
	instanceID string,
	presenceTTL time.Duration,
	log *slog.Logger,
	m *metrics.Metrics,
) (*Redis, error) {
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	h := &Redis{
		reg:        newRegistry(log, m),
		rdb:        rdb,
		pubsub:     rdb.Subscribe(runCtx),
		instanceID: instanceID,
		ttl:        presenceTTL,
		log:        log,
		m:          m,
		subs:       make(map[uuid.UUID]struct{}),
		cancel:     cancel,
	}

	h.wg.Add(2)
	go h.receiveLoop(runCtx)
	go h.refreshLoop(runCtx, presenceTTL/3)

	return h, nil
}

func userChannel(id uuid.UUID) string { return channelPrefix + id.String() }
func presenceKey(id uuid.UUID) string { return presencePrefix + id.String() }

func (h *Redis) Register(ctx context.Context, c Sink) (bool, error) {
	firstLocal := h.reg.add(c)
	if !firstLocal {
		// Tab kedua user yang sama di instance ini: langganan sudah ada dan
		// presence-nya sudah tercatat. Tidak ada yang berubah secara global.
		return false, nil
	}

	if err := h.subscribe(ctx, c.UserID()); err != nil {
		// Registry sudah terisi tapi langganan gagal: koneksi ini tidak akan
		// menerima siaran. Lebih jujur menolaknya daripada membiarkannya
		// terbuka dan tampak sehat sambil kehilangan pesan.
		h.reg.remove(c)
		return false, err
	}

	holders, err := h.presenceJoin(ctx, c.UserID())
	if err != nil {
		return false, err
	}
	return holders == 1, nil
}

func (h *Redis) Unregister(ctx context.Context, c Sink) (bool, error) {
	lastLocal := h.reg.remove(c)
	if !lastLocal {
		return false, nil
	}

	h.unsubscribe(ctx, c.UserID())

	holders, err := h.presenceLeave(ctx, c.UserID())
	if err != nil {
		return false, err
	}
	return holders == 0, nil
}

// Publish mengirim satu event ke channel milik tiap target dalam satu pipeline
// — sekali perjalanan ke Redis, bukan sekali per anggota grup.
func (h *Redis) Publish(targets []uuid.UUID, ev Event) {
	if len(targets) == 0 {
		return
	}

	payload, err := json.Marshal(ev)
	if err != nil {
		h.log.Error("encode event", "type", ev.Type, "err", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	pipe := h.rdb.Pipeline()
	for _, id := range targets {
		pipe.Publish(ctx, userChannel(id), payload)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		h.m.RedisErrors.WithLabelValues("publish").Inc()
		h.log.Error("publish ke redis gagal", "type", ev.Type, "targets", len(targets), "err", err)
		return
	}

	h.m.RedisPublishDuration.Observe(time.Since(start).Seconds())
	h.m.EventsPublished.WithLabelValues(ev.Type).Inc()
}

// OnlineAmong menghitung pemegang presence yang belum kedaluwarsa untuk tiap
// user, seluruhnya dalam satu pipeline.
func (h *Redis) OnlineAmong(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return []uuid.UUID{}, nil
	}

	now := strconv.FormatInt(time.Now().Unix(), 10)
	pipe := h.rdb.Pipeline()
	cmds := make([]*redis.IntCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.ZCount(ctx, presenceKey(id), now, "+inf")
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		h.m.RedisErrors.WithLabelValues("presence_query").Inc()
		return nil, err
	}

	online := make([]uuid.UUID, 0, len(ids))
	for i, cmd := range cmds {
		if n, err := cmd.Result(); err == nil && n > 0 {
			online = append(online, ids[i])
		}
	}
	return online, nil
}

func (h *Redis) LocalConnections() int { return h.reg.count() }

func (h *Redis) Drain(ctx context.Context, period time.Duration) {
	h.reg.drain(ctx, period)
}

func (h *Redis) Close() error {
	h.cancel()
	err := h.pubsub.Close()
	h.wg.Wait()
	return err
}

// ---------- langganan ----------

func (h *Redis) subscribe(ctx context.Context, id uuid.UUID) error {
	h.subsMu.Lock()
	defer h.subsMu.Unlock()

	if _, ok := h.subs[id]; ok {
		return nil
	}
	if err := h.pubsub.Subscribe(ctx, userChannel(id)); err != nil {
		h.m.RedisErrors.WithLabelValues("subscribe").Inc()
		return err
	}
	h.subs[id] = struct{}{}
	h.m.RedisSubscriptions.Set(float64(len(h.subs)))
	return nil
}

func (h *Redis) unsubscribe(ctx context.Context, id uuid.UUID) {
	h.subsMu.Lock()
	defer h.subsMu.Unlock()

	if _, ok := h.subs[id]; !ok {
		return
	}
	if err := h.pubsub.Unsubscribe(ctx, userChannel(id)); err != nil {
		h.m.RedisErrors.WithLabelValues("unsubscribe").Inc()
		h.log.Warn("unsubscribe gagal", "user", id, "err", err)
	}
	delete(h.subs, id)
	h.m.RedisSubscriptions.Set(float64(len(h.subs)))
}

// receiveLoop memakai ReceiveMessage, bukan Channel(), karena Channel() punya
// buffer internal yang MEMBUANG pesan diam-diam saat penuh. Pesan chat yang
// hilang tanpa jejak adalah kegagalan paling mahal untuk didiagnosis; menahan
// aliran di Redis jauh lebih baik. Aman dilakukan karena pengiriman ke koneksi
// tidak pernah memblokir — buffer koneksi yang penuh berujung putus, bukan
// tunggu.
func (h *Redis) receiveLoop(ctx context.Context) {
	defer h.wg.Done()

	for {
		msg, err := h.pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			h.m.RedisErrors.WithLabelValues("receive").Inc()
			h.log.Error("terima dari redis gagal", "err", err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		id, ok := userFromChannel(msg.Channel)
		if !ok {
			continue
		}

		start := time.Now()
		receivers := h.reg.deliver([]uuid.UUID{id}, []byte(msg.Payload))
		h.m.BroadcastDuration.WithLabelValues("redis").Observe(time.Since(start).Seconds())
		h.m.BroadcastFanout.Observe(float64(receivers))
	}
}

func userFromChannel(channel string) (uuid.UUID, bool) {
	raw, ok := strings.CutPrefix(channel, channelPrefix)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// ---------- presence ----------

func (h *Redis) presenceJoin(ctx context.Context, id uuid.UUID) (int64, error) {
	now := time.Now()
	n, err := presenceJoin.Run(ctx, h.rdb,
		[]string{presenceKey(id)},
		h.instanceID,
		strconv.FormatInt(now.Add(h.ttl).Unix(), 10),
		strconv.FormatInt(now.Unix(), 10),
		strconv.Itoa(int(h.ttl.Seconds())),
	).Int64()
	if err != nil {
		h.m.RedisErrors.WithLabelValues("presence_join").Inc()
		return 0, err
	}
	return n, nil
}

func (h *Redis) presenceLeave(ctx context.Context, id uuid.UUID) (int64, error) {
	n, err := presenceLeave.Run(ctx, h.rdb,
		[]string{presenceKey(id)},
		h.instanceID,
		strconv.FormatInt(time.Now().Unix(), 10),
	).Int64()
	if err != nil {
		h.m.RedisErrors.WithLabelValues("presence_leave").Inc()
		return 0, err
	}
	return n, nil
}

// refreshLoop memperpanjang klaim presence instance ini.
//
// Ini yang membuat instance yang mati mendadak tidak meninggalkan user
// "online selamanya": klaimnya berhenti diperpanjang dan lenyap sendiri dalam
// satu TTL. SADD lewat skrip yang sama juga menyembuhkan kasus sebaliknya —
// key yang telanjur kedaluwarsa saat Redis sempat tersendat akan ditulis ulang
// pada penyegaran berikutnya.
func (h *Redis) refreshLoop(ctx context.Context, every time.Duration) {
	defer h.wg.Done()

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			users := h.reg.localUsers()
			if len(users) == 0 {
				continue
			}

			opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			now := time.Now()
			cutoff := strconv.FormatInt(now.Unix(), 10)
			ttl := strconv.Itoa(int(h.ttl.Seconds()))

			// Perintah biasa, bukan skrip: dalam pipeline, Script.Run
			// mengantre EVALSHA dan tidak bisa jatuh balik ke EVAL saat Redis
			// baru restart dan cache skripnya kosong. Penyegaran juga tidak
			// membaca hasil hitungan, jadi atomisitas tidak diperlukan di sini.
			ttlSec, _ := strconv.Atoi(ttl)
			pipe := h.rdb.Pipeline()
			for _, id := range users {
				key := presenceKey(id)
				pipe.ZRemRangeByScore(opCtx, key, "-inf", cutoff)
				pipe.ZAdd(opCtx, key, redis.Z{Score: float64(now.Add(h.ttl).Unix()), Member: h.instanceID})
				pipe.Expire(opCtx, key, time.Duration(ttlSec)*time.Second)
			}
			if _, err := pipe.Exec(opCtx); err != nil {
				h.m.RedisErrors.WithLabelValues("presence_refresh").Inc()
				h.log.Warn("penyegaran presence gagal", "users", len(users), "err", err)
			}
			cancel()
		}
	}
}

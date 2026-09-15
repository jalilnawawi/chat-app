package ratelimit

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// tokenBucket menjalankan seluruh "isi ulang, ambil satu, simpan" sebagai satu
// operasi atomik. Kalau dipecah jadi GET lalu SET dari sisi Go, dua permintaan
// bersamaan dari user yang sama akan membaca sisa token yang sama dan keduanya
// lolos — persis pada saat batas itu paling dibutuhkan.
//
// Waktu diambil dari perintah TIME milik Redis, bukan jam masing-masing
// instance. Dengan beberapa instance, jam yang meleset beberapa detik membuat
// bucket seolah terisi ulang lebih cepat; satu sumber waktu menghapus seluruh
// kelas bug itu.
var tokenBucket = redis.NewScript(`
	local burst  = tonumber(ARGV[1])
	local refill = tonumber(ARGV[2])
	local ttl    = tonumber(ARGV[3])

	local t   = redis.call('TIME')
	local now = tonumber(t[1]) + tonumber(t[2]) / 1000000

	local data   = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
	local tokens = tonumber(data[1])
	local ts     = tonumber(data[2])
	if tokens == nil or ts == nil then
		tokens = burst
		ts = now
	end

	tokens = math.min(burst, tokens + (now - ts) * refill)

	local allowed = 0
	if tokens >= 1 then
		tokens = tokens - 1
		allowed = 1
	end

	redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
	redis.call('EXPIRE', KEYS[1], ttl)

	return {allowed, tostring(tokens)}
`)

// Redis membagi satu kuota untuk seluruh instance.
//
// fallback dipakai saat Redis tidak bisa dihubungi. Pilihannya bukan antara
// "batasi" dan "jangan batasi": membiarkan lolos semua saat Redis tumbang
// justru membuka pintu di saat sistem paling rapuh, sedangkan menolak semua
// berarti aplikasi ikut mati bersama Redis. Membatasi per instance adalah
// jalan tengah yang jujur — batas efektifnya jadi longgar sebanyak jumlah
// instance, dan itu tercatat di metrik.
type Redis struct {
	rdb      *redis.Client
	prefix   string
	fallback *Memory
	log      *slog.Logger
}

var _ Limiter = (*Redis)(nil)

func NewRedis(rdb *redis.Client, log *slog.Logger) *Redis {
	return &Redis{rdb: rdb, prefix: "chat:rl:", fallback: NewMemory(), log: log}
}

func (r *Redis) Allow(ctx context.Context, key string, rule Rule) (Result, error) {
	ttl := 60
	if rule.Refill > 0 {
		if full := int(float64(rule.Burst)/rule.Refill) + 10; full > ttl {
			ttl = full
		}
	}

	res, err := tokenBucket.Run(ctx, r.rdb, []string{r.prefix + key},
		rule.Burst, rule.Refill, ttl).Slice()
	if err != nil {
		r.log.Warn("rate limit jatuh ke mode lokal", "err", err)
		return r.fallback.Allow(ctx, key, rule)
	}

	allowed, _ := res[0].(int64)
	tokens, _ := strconv.ParseFloat(toString(res[1]), 64)

	if allowed == 0 {
		return Result{RetryAfter: retryAfter(tokens, rule)}, nil
	}
	return Result{Allowed: true, Remaining: int(tokens)}, nil
}

func (r *Redis) Close() error { return r.fallback.Close() }

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return "0"
	}
}

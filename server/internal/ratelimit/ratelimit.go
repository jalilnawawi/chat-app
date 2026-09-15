// Package ratelimit membatasi laju per user dengan token bucket.
//
// Kenapa token bucket, bukan "maksimal N per menit"? Karena chat itu meledak-
// ledak secara wajar: orang mengetik tujuh pesan pendek beruntun lalu diam satu
// menit. Jendela tetap akan menolak ledakan yang tidak berbahaya itu, sementara
// token bucket mengizinkannya selama rata-rata jangka panjangnya masih sopan.
//
// Ada dua implementasi di balik satu interface, sejalan dengan hub: Memory
// untuk satu instance, Redis supaya kuota seorang user tetap satu walau
// koneksinya tersebar ke beberapa instance. Tanpa yang kedua, menambah instance
// diam-diam ikut mengalikan batasnya.
package ratelimit

import (
	"context"
	"math"
	"time"
)

// Rule adalah satu kuota: Burst token yang terisi ulang Refill token per detik.
type Rule struct {
	Burst  int
	Refill float64
}

// PerMinute menyusun aturan dari cara orang biasa membicarakannya.
func PerMinute(burst int, sustained float64) Rule {
	return Rule{Burst: burst, Refill: sustained / 60}
}

type Result struct {
	Allowed   bool
	Remaining int
	// RetryAfter adalah perkiraan kapan satu token berikutnya tersedia.
	// Dipakai untuk header Retry-After supaya client tahu harus menunggu
	// berapa lama, bukan mencoba lagi secepat mungkin dan memperparah keadaan.
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, key string, rule Rule) (Result, error)
	Close() error
}

// retryAfter menghitung waktu sampai bucket punya satu token utuh.
func retryAfter(tokens float64, rule Rule) time.Duration {
	if rule.Refill <= 0 {
		return time.Minute
	}
	need := 1 - tokens
	if need <= 0 {
		return 0
	}
	return time.Duration(math.Ceil(need/rule.Refill*1000)) * time.Millisecond
}

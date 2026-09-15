package ratelimit

import (
	"context"
	"testing"
	"time"
)

func allow(t *testing.T, l Limiter, key string, rule Rule) Result {
	t.Helper()
	res, err := l.Allow(context.Background(), key, rule)
	if err != nil {
		t.Fatalf("allow: %v", err)
	}
	return res
}

// Ledakan pendek harus lolos utuh. Inilah alasan memilih token bucket: orang
// mengirim beberapa pesan pendek beruntun, dan menolaknya akan terasa seperti
// aplikasi yang rusak, bukan aplikasi yang aman.
func TestBurstPassesThenBlocks(t *testing.T) {
	l := NewMemory()
	defer l.Close()

	rule := Rule{Burst: 5, Refill: 1}
	for i := range 5 {
		if !allow(t, l, "user", rule).Allowed {
			t.Fatalf("permintaan ke-%d dalam burst seharusnya lolos", i+1)
		}
	}

	res := allow(t, l, "user", rule)
	if res.Allowed {
		t.Fatal("permintaan setelah burst habis seharusnya ditolak")
	}
	if res.RetryAfter <= 0 {
		t.Fatal("penolakan harus memberi tahu kapan boleh mencoba lagi")
	}
}

// Kuota satu user tidak boleh ikut memakan kuota user lain.
func TestQuotasAreIndependentPerKey(t *testing.T) {
	l := NewMemory()
	defer l.Close()

	rule := Rule{Burst: 2, Refill: 0.1}
	allow(t, l, "alice", rule)
	allow(t, l, "alice", rule)

	if allow(t, l, "alice", rule).Allowed {
		t.Fatal("alice sudah menghabiskan kuotanya")
	}
	if !allow(t, l, "bob", rule).Allowed {
		t.Fatal("bob punya kuota sendiri dan seharusnya lolos")
	}
}

func TestTokensRefillOverTime(t *testing.T) {
	l := NewMemory()
	defer l.Close()

	// 100 token per detik: satu token pulih dalam 10 milidetik.
	rule := Rule{Burst: 1, Refill: 100}
	if !allow(t, l, "user", rule).Allowed {
		t.Fatal("token pertama seharusnya tersedia")
	}
	if allow(t, l, "user", rule).Allowed {
		t.Fatal("bucket seharusnya kosong tepat setelah dipakai")
	}

	time.Sleep(30 * time.Millisecond)

	if !allow(t, l, "user", rule).Allowed {
		t.Fatal("token seharusnya sudah terisi ulang")
	}
}

// Isi ulang tidak boleh menumpuk melebihi burst — kalau bisa, user yang diam
// seharian akan mengumpulkan kuota berjam-jam lalu melepasnya sekaligus.
func TestRefillIsCappedAtBurst(t *testing.T) {
	l := NewMemory()
	defer l.Close()

	rule := Rule{Burst: 3, Refill: 1000}
	allow(t, l, "user", rule)

	time.Sleep(20 * time.Millisecond) // cukup untuk 20 token kalau tidak dibatasi

	passed := 0
	for range 10 {
		if allow(t, l, "user", rule).Allowed {
			passed++
		}
	}
	if passed != 3 {
		t.Fatalf("hanya %d token boleh menumpuk, dapat %d", rule.Burst, passed)
	}
}

func TestPerMinuteConvertsToPerSecond(t *testing.T) {
	rule := PerMinute(20, 120)
	if rule.Burst != 20 {
		t.Fatalf("burst = %d, harusnya 20", rule.Burst)
	}
	if rule.Refill != 2 {
		t.Fatalf("120 per menit = %v per detik, harusnya 2", rule.Refill)
	}
}

func TestRetryAfterShrinksAsTokensRecover(t *testing.T) {
	rule := Rule{Burst: 10, Refill: 2} // satu token tiap 500ms

	if got := retryAfter(0, rule); got != 500*time.Millisecond {
		t.Fatalf("dari kosong = %s, harusnya 500ms", got)
	}
	if got := retryAfter(0.5, rule); got != 250*time.Millisecond {
		t.Fatalf("dari setengah token = %s, harusnya 250ms", got)
	}
	if got := retryAfter(1, rule); got != 0 {
		t.Fatalf("token sudah cukup, harusnya 0, dapat %s", got)
	}
}

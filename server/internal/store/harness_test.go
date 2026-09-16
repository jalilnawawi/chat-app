package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jalilnawawi/chat-app/server/internal/migrations"
	"github.com/jalilnawawi/chat-app/server/internal/store"
)

// Harness Postgres untuk test paket store.
//
// Paket ini adalah satu-satunya yang sampai Fase 8 tidak punya satu test pun,
// dan justru dia lapisan tempat satu salah ketik dalam SQL berubah jadi
// kehilangan data — bukan jadi error compile. `go vet` tidak membaca isi string
// SQL, dan tiga dari empat kesalahan yang pernah ditemukan di fase-fase
// sebelumnya hanya ketahuan dengan menyalakan sesuatu.
//
// Dua keputusan menentukan bentuk berkas ini:
//
//  1. Database SUNGGUHAN, bukan tiruan. Yang diuji di sini adalah SQL —
//     self-join, agregasi, ON CONFLICT, dan CHECK constraint. Tiruan dari
//     lapisan database hanya akan menguji tiruan itu sendiri.
//
//  2. Dilewati, bukan gagal, bila databasenya tidak ada. `go test ./...` harus
//     tetap hijau di mesin yang belum menyalakan docker — kalau tidak, orang
//     berhenti menjalankannya sama sekali, dan test yang tidak pernah
//     dijalankan tidak melindungi apa pun.
//
// Tiap kali dijalankan, seluruh skema dibuat baru dalam schema Postgres
// tersendiri lalu dibuang di akhir. Bukan transaksi yang di-rollback: yang
// diuji di sini termasuk kode yang membuka transaksinya SENDIRI, dan transaksi
// bersarang bukan hal yang sama dengan transaksi.

var (
	testPool  *pgxpool.Pool
	skipCause string
)

func TestMain(m *testing.M) {
	code := func() int {
		url := os.Getenv("TEST_DATABASE_URL")
		if url == "" {
			url = os.Getenv("DATABASE_URL")
		}
		if url == "" {
			url = "postgres://chat:chat@localhost:5433/chatapp?sslmode=disable"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cleanup, err := setup(ctx, url)
		if err != nil {
			skipCause = err.Error()
			return m.Run()
		}
		defer cleanup()

		return m.Run()
	}()
	os.Exit(code)
}

func setup(ctx context.Context, url string) (func(), error) {
	// Nama schema dibuat acak supaya dua `go test` yang berjalan bersamaan —
	// dan `go test ./...` memang menjalankan paket secara paralel — tidak saling
	// menghapus tabel.
	schema := "test_store_" + uuid.NewString()[:8]

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("database tidak tersedia: %w", err)
	}
	if err := admin.Ping(ctx); err != nil {
		admin.Close()
		return nil, fmt.Errorf("database tidak tersedia: %w", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		return nil, fmt.Errorf("buat schema uji: %w", err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		admin.Close()
		return nil, err
	}
	// public tetap ikut di jalur pencarian — bukan untuk tabel, melainkan untuk
	// extension. `gin_trgm_ops` di migrasi 0002 tinggal di schema tempat
	// pg_trgm dipasang, dan tanpa public di sini seluruh migrasi berhenti di
	// baris itu.
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ", public"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		admin.Close()
		return nil, err
	}
	if err := migrations.Run(ctx, pool); err != nil {
		pool.Close()
		admin.Close()
		return nil, fmt.Errorf("migrasi gagal: %w", err)
	}

	testPool = pool
	return func() {
		pool.Close()
		dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// CASCADE: bila mesin ini belum pernah menjalankan migrasi sebelumnya,
		// pg_trgm ikut terpasang di dalam schema ini.
		_, _ = admin.Exec(dropCtx, "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	}, nil
}

// newStore menyiapkan store untuk satu test, atau melewatinya bila database
// memang tidak ada di mesin ini.
func newStore(t *testing.T) (*store.Store, context.Context) {
	t.Helper()
	if testPool == nil {
		t.Skip("lewati: " + skipCause)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return store.New(testPool), ctx
}

// ---------- pembantu penyusun keadaan ----------

// newUser membuat user dengan nama yang pasti unik, supaya test yang berbagi
// schema tidak saling bentrok di unique index username.
func newUser(t *testing.T, s *store.Store, ctx context.Context, name string) store.User {
	t.Helper()
	username := name + "_" + uuid.NewString()[:8]
	u, err := s.CreateUser(ctx, username, name, "hash-palsu")
	if err != nil {
		t.Fatalf("buat user %s: %v", name, err)
	}
	return u
}

func newDirect(t *testing.T, s *store.Store, ctx context.Context, a, b store.User) uuid.UUID {
	t.Helper()
	id, err := s.GetOrCreateDirect(ctx, a.ID, b.ID)
	if err != nil {
		t.Fatalf("buat DM: %v", err)
	}
	return id
}

func newGroup(t *testing.T, s *store.Store, ctx context.Context, owner store.User, members ...store.User) uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	id, err := s.CreateGroup(ctx, owner.ID, "Grup Uji", ids)
	if err != nil {
		t.Fatalf("buat grup: %v", err)
	}
	return id
}

// send mengirim pesan teks biasa dan menganggap kegagalan apa pun fatal.
func send(t *testing.T, s *store.Store, ctx context.Context, convID uuid.UUID, sender store.User, body string) store.Message {
	t.Helper()
	m, created, err := s.SendMessage(ctx, store.SendParams{
		ID: uuid.New(), ConversationID: convID, SenderID: sender.ID, Body: body,
	})
	if err != nil {
		t.Fatalf("kirim pesan: %v", err)
	}
	if !created {
		t.Fatalf("pesan baru dilaporkan sebagai duplikat")
	}
	return m
}

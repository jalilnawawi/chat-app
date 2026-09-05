# Chat App

Aplikasi chat realtime. Go + PostgreSQL di backend, Bun + React di frontend.
Mendukung DM 1-on-1 dan grup, presence, typing indicator, read receipt, serta
edit & hapus pesan.

Status: **MVP jalan end-to-end.** Rencana menuju 800+ user ada di [TASKLIST.md](TASKLIST.md) Fase 6.

## Menjalankan

Butuh Go 1.26+, Bun, dan Docker.

```bash
# 1. Database
docker compose up -d

# 2. Backend (migrasi jalan otomatis saat start)
cd server && go run ./cmd/server        # http://localhost:8090

# 3. Frontend
cd web && bun install && bun run dev    # http://localhost:5174
```

Buka dua jendela browser berbeda (satu normal, satu incognito), daftar dua akun,
lalu mulai percakapan.

Frontend bisa dibuka lewat `localhost:5174`, `127.0.0.1:5174`, maupun `[::1]:5174`
— ketiganya sudah diuji. Backend tidak perlu dibuka langsung: dev server Vite
meneruskan `/api` dan `/ws` ke sana.

Port default 8090 dan 5174 — bukan 8080/5173 — supaya tidak bentrok dengan
project lain yang biasanya sudah memakai port itu. Ubah lewat `.env`
(lihat `.env.example` di root dan di `web/`).

## Struktur

```
server/
  cmd/server/          entry point, graceful shutdown
  internal/config/     konfigurasi dari env
  internal/migrations/ skema SQL + migrator embedded (tanpa tool eksternal)
  internal/store/      akses database (pgx, SQL manual)
  internal/auth/       argon2id + token sesi
  internal/hub/        fan-out event realtime  <- titik tukar ke Redis nanti
  internal/ws/         satu koneksi WebSocket: heartbeat, backpressure, resume
  internal/api/        routing HTTP, middleware, handler
web/
  src/api.ts           client REST
  src/useSocket.ts     koneksi WS: reconnect + resume
  src/store.ts         state global (zustand), termasuk kiriman optimistik
  src/components/      UI
```

## Keputusan desain

Enam hal berikut murah dikerjakan di awal dan mahal ditambahkan belakangan.
Semuanya sudah terpasang:

**1. ID pesan dibuat client (UUIDv7).** ID datang dari browser dan langsung jadi
primary key. Efeknya: UI optimistik punya id final sejak render pertama, dan
kirim ulang setelah timeout mengembalikan pesan yang sama alih-alih duplikat.
Sudah diuji: POST dua kali dengan id sama menghasilkan satu baris.

**2. `seq` monotonic per percakapan.** Timestamp tidak cukup untuk mengurutkan —
dua pesan bisa punya waktu identik dan jam server bisa mundur. `seq` dialokasikan
di dalam `SELECT ... FOR UPDATE` sehingga berurutan tanpa lompatan, dan menjadi
dasar pagination sekaligus resume.

**3. Heartbeat ping/pong.** Tanpa ini, koneksi yang mati diam-diam (HP masuk
terowongan) menempel di memori server berjam-jam dan user terlihat online padahal
sudah pergi. Ping tiap 30 detik, timeout 10 detik.

**4. Backpressure.** Tiap koneksi punya buffer 64 pesan. Kalau penuh, koneksi
ditutup — bukan ditunggu. Menunggu satu client lambat akan menahan siaran ke
seluruh anggota ruang. Client yang terputus akan reconnect dan menyusul lewat
resume.

**5. Cursor pagination.** `WHERE seq < $cursor ORDER BY seq DESC`, bukan `OFFSET`.
Offset makin dalam makin lambat; cursor tidak.

**6. Resume setelah reconnect.** Client menyimpan `seq` terakhir per percakapan
dan mengirimnya saat WebSocket terbuka; server membalas hanya selisihnya.

Selain itu:

- **Sesi pakai cookie httpOnly, bukan JWT di localStorage.** WebSocket API di
  browser tidak bisa mengirim custom header, jadi pendekatan token memaksa
  menaruh kredensial di query string — tempat yang gampang bocor ke log. Cookie
  terkirim otomatis saat handshake, dan tidak bisa dibaca script.
- **DM dan grup memakai satu abstraksi `conversation`.** Semua logic pesan,
  read receipt, dan typing seragam; tidak ada dua jalur kode.
- **Kirim pesan lewat REST, siaran lewat WebSocket.** HTTP memberi status
  sukses/gagal per pesan sehingga retry punya arti pasti; WebSocket dipakai untuk
  yang jadi kekuatannya — menyebarkan hasilnya.
- **Hapus pesan bersifat soft delete.** Baris tetap ada supaya `seq` tidak bolong
  dan client yang sedang offline tetap bisa menyinkronkan status "dihapus".
- **Frontend dan API dilayani lewat satu origin.** Dev server Vite mem-proxy
  `/api` dan `/ws` ke backend. Alasannya bukan kenyamanan: kalau halaman dibuka
  di `127.0.0.1:5174` sementara API dipanggil di `localhost:8090`, browser
  menganggapnya lintas *site* — `localhost` dan `127.0.0.1` adalah site berbeda
  walau menunjuk mesin yang sama — sehingga cookie sesi `SameSite=Lax` tidak
  ikut terkirim dan semua permintaan setelah login menjadi 401. Dengan satu
  origin, alamat apa pun yang diketik di address bar tetap jalan.
- **Vite mengikat `::` (dual-stack).** Default-nya hanya IPv6 loopback, dan
  browser yang memilih IPv4 lebih dulu gagal tersambung ke `127.0.0.1:5174`.
- **Semua siaran lewat satu interface `hub.Broadcaster`.** Saat butuh lebih dari
  satu instance, yang ditulis hanya implementasi kedua berbasis Redis pub/sub —
  handler tidak berubah.

## Protokol WebSocket

Client -> server:

| type | payload |
|---|---|
| `sync` | `{ cursors: { [conversationId]: lastSeq } }` |
| `typing` | `{ conversationId, typing }` |
| `read` | `{ conversationId, seq }` |

Server -> client:

| type | payload |
|---|---|
| `message.new` | objek Message |
| `message.updated` | objek Message (hasil edit atau hapus) |
| `conversation.new` | objek Conversation |
| `read.updated` | `{ conversationId, userId, lastReadSeq }` |
| `typing` | `{ conversationId, userId, displayName, typing }` |
| `presence` | `{ userId, online }` |
| `presence.snapshot` | `{ online: string[] }` |
| `sync.complete` | `{}` |
| `error` | `{ message }` |

## Tes

```bash
cd server && go test -race ./...
cd web && bun run typecheck
```

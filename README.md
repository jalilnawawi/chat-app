# Chat App

Aplikasi chat realtime. Go + PostgreSQL di backend, Bun + React di frontend.
Mendukung DM 1-on-1 dan grup, presence, typing indicator, read receipt, edit &
hapus pesan, lampiran berkas, serta push notification untuk yang sedang tidak
membuka aplikasi.

Status: **MVP jalan end-to-end, dan sudah diuji untuk 1000 koneksi bersamaan.**
Catatan operasional multi-instance ada di [docs/scaling.md](docs/scaling.md);
keputusan seputar lampiran dan notifikasi di
[docs/lampiran-dan-push.md](docs/lampiran-dan-push.md); turunan gambar dan
permintaan sepotong di
[docs/thumbnail-dan-range.md](docs/thumbnail-dan-range.md).

## Menjalankan

Butuh Go 1.26+, Bun, dan Docker.

```bash
# 1. Infrastruktur: Postgres, Redis (multi-instance), SeaweedFS (lampiran)
docker compose up -d

# 2. Backend (migrasi jalan otomatis saat start)
cd server && go run ./cmd/server        # http://localhost:8090

# 3. Frontend
cd web && bun install && bun run dev    # http://localhost:5174
```

Tiga bagian bisa dimatikan lewat satu variabel lingkungan, dan aplikasi tetap
utuh sebagai chat tanpa ketiganya:

| Kosongkan | Akibatnya |
|---|---|
| `REDIS_URL` | Satu instance: hub dan rate limit memakai memori proses |
| `SEAWEED_FILER_URL` | Lampiran mati, tombolnya hilang dari UI |
| `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` | Notifikasi mati, tombolnya hilang |

Itu mode yang paling enak untuk pengembangan: `go run` cukup untuk menjalankan
seluruh aplikasi. Untuk beberapa instance sekaligus, lihat
[docs/scaling.md](docs/scaling.md).

Untuk menyalakan notifikasi, buat sepasang kunci SEKALI lalu simpan di `.env`:

```bash
cd server && go run ./cmd/vapid
```

Jangan membuatnya ulang: browser mengunci langganannya pada kunci publik yang
dipakai saat mendaftar, jadi kunci baru membuat seluruh langganan yang sudah ada
berhenti bekerja diam-diam.

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
  cmd/server/          entry point, shutdown bertahap untuk rolling deploy
  cmd/loadtest/        alat uji beban: 1000 koneksi + ukur latensi fan-out
  cmd/vapid/           pembuat kunci VAPID, dijalankan sekali saat menyiapkan
  internal/config/     konfigurasi dari env
  internal/migrations/ skema SQL + migrator embedded (tanpa tool eksternal)
  internal/store/      akses database (pgx, SQL manual)
  internal/auth/       argon2id + token sesi
  internal/hub/        fan-out event realtime — memori proses atau Redis pub/sub
  internal/ratelimit/  token bucket per user, lokal atau dibagi lewat Redis
  internal/blob/       penyimpanan isi lampiran (SeaweedFS lewat filer-nya)
  internal/imaging/    turunan kecil untuk gambar: masukannya byte, keluarannya byte
  internal/push/       notifikasi Web Push: siapa yang layak dibangunkan, kapan
  internal/metrics/    seluruh instrumen Prometheus di satu tempat
  internal/ws/         satu koneksi WebSocket: heartbeat, backpressure, resume
  internal/api/        routing HTTP, middleware, handler
web/
  public/sw.js         service worker: hanya notifikasi, tanpa cache sama sekali
  src/api.ts           client REST
  src/useSocket.ts     koneksi WS: reconnect + resume
  src/push.ts          izin notifikasi + langganan Web Push
  src/store.ts         state global (zustand), kiriman optimistik + unggahan
  src/components/      UI
docs/scaling.md        catatan operasional multi-instance
docs/lampiran-dan-push.md  keputusan seputar lampiran dan notifikasi
docs/thumbnail-dan-range.md  turunan gambar dan permintaan sepotong
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
- **Semua siaran lewat satu interface `hub.Broadcaster`.** Menambah instance
  hanya mengganti implementasinya dengan yang berbasis Redis pub/sub; handler
  HTTP dan WebSocket tidak berubah sebaris pun. Pilihan transport adalah
  keputusan deployment, bukan sesuatu yang boleh bocor ke logika percakapan.
- **`Enqueue` membedakan "buffer penuh" dari "koneksi sudah tutup".** Keduanya
  pernah dilaporkan sebagai satu nilai `false`, dan itu membuat metrik
  backpressure ikut menghitung setiap orang yang menutup tab. Alarm yang
  berbunyi saat tidak ada apa-apa adalah alarm yang akhirnya dimatikan orang.
- **Menutup koneksi ikut membatalkan konteksnya.** Menutup channel sinyal saja
  tidak membangunkan `conn.Read` yang sedang memblokir, dan client yang diam
  akan menggantung sampai prosesnya dimatikan paksa — yang membuat pengurasan
  saat rolling deploy tidak pernah selesai.
- **Menyusul setelah reconnect punya jalur kirimnya sendiri.** Susulan dikirim
  berkelompok (50 pesan per frame) lewat jalur yang MENUNGGU ruang antrean,
  bukan membuang. Aturan "putus client lambat" ada untuk melindungi siaran dari
  satu orang yang lambat; menyusul adalah urusan koneksi itu sendiri. Tanpa
  pemisahan ini, client yang tertinggal jauh diputus justru saat sedang menyusul
  — dan itu terjadi persis setelah rolling deploy, saat semua orang menyusul
  bersamaan.

- **Isi lampiran hanya bisa diambil lewat server ini.** Penyimpanan objek tidak
  tahu apa-apa tentang keanggotaan percakapan, jadi satu-satunya tempat yang bisa
  menjawab "boleh tidak orang ini membaca berkas ini" adalah server yang
  menyimpan keanggotaannya. SeaweedFS tidak pernah terjangkau dari internet, dan
  yang tidak berhak mendapat 404 — bukan 403, karena membedakan keduanya berarti
  memberi tahu orang asing bahwa berkas itu ada.
- **Tipe lampiran ditentukan dari isinya, dan hanya empat tipe gambar yang
  disajikan inline.** Lampiran keluar dari origin yang sama dengan aplikasi, jadi
  berkas yang bisa dieksekusi browser — HTML, dan terutama SVG — akan berjalan
  sebagai bagian dari aplikasi ini kalau dibuka inline. Selebihnya dipaksa
  terunduh, dengan `nosniff` dan CSP sebagai lapis berikutnya.
- **Unggah dipisah dari kirim pesan.** Berkas sepuluh megabyte butuh waktu dan
  bisa putus di tengah; mengirim pesan harus tetap satu tindakan cepat yang
  jawabannya pasti. Kalau digabung, jaringan yang putus pada detik terakhir ikut
  membatalkan teks yang sudah diketik orang.
- **Notifikasi hanya untuk yang benar-benar offline, dan satu percakapan tidak
  berbunyi berkali-kali.** Peredamnya memakai token bucket yang sama dengan kuota
  — termasuk versi Redis-nya, sehingga dua instance tidak membangunkan orang yang
  sama dua kali untuk percakapan yang sama.

Uraian lengkap keduanya di [docs/lampiran-dan-push.md](docs/lampiran-dan-push.md).

## Lampiran

```
POST   /api/attachments?w=&h=       multipart, field "file" → objek Attachment
GET    /api/attachments/{id}        isi berkasnya, hanya untuk yang berhak
GET    /api/attachments/{id}/thumb  turunan kecil, izin yang sama persis
```

Id yang dikembalikan disebut di `attachmentIds` saat mengirim pesan. Satu pesan
boleh membawa sampai sepuluh lampiran, dan boleh tanpa teks sama sekali.

Gambar yang lebih besar dari `THUMBNAIL_MAX_DIM` dibuatkan turunan saat diunggah,
dan `thumbUrl` ikut di objek Attachment. Kosongnya field itu berarti pakai `url`
biasa — bukan gambar, sudah cukup kecil, atau pembuatannya gagal; client tidak
perlu membedakan ketiganya. Foto 4000 x 3000 turun dari 6,2 MB jadi 13 KB.

Kedua alamat menerima `Range`, menjawab `Accept-Ranges: bytes`, dan membalas 206
dengan `Content-Range` untuk potongan yang benar-benar dilayani penyimpanan —
itu yang membuat video panjang bisa dilompati tanpa mengunduh bagian sebelumnya.
Rentang yang bentuknya tidak dikenali diabaikan (kirim utuh); yang menunjuk ke
luar berkas dijawab 416. Uraiannya di
[docs/thumbnail-dan-range.md](docs/thumbnail-dan-range.md).

## Notifikasi

```
GET    /api/push/config           { enabled, publicKey }
POST   /api/push/subscribe        langganan dari browser
POST   /api/push/unsubscribe      { endpoint }
```

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
| `sync.batch` | `{ conversationId, messages: Message[] }` — susulan setelah reconnect, berkelompok |
| `sync.complete` | `{}` |
| `server.shutdown` | `{ reason }` — instance pamit terencana; sambung lagi sekarang, jangan mundur bertahap |
| `error` | `{ message }` |

## Kuota

Setiap user punya token bucket sendiri untuk kirim pesan, buka koneksi, dan
event typing; login/register dibatasi per alamat IP karena argon2id sengaja
mahal. Semua batas diatur lewat env (lihat `.env.example`).

Server yang menolak membalas **429** dengan header `Retry-After`. Client menunggu
selama itu lalu mencoba sekali lagi diam-diam — aman justru karena id pesan
dibuat client, sehingga pengiriman ulang menghasilkan pesan yang sama, bukan
pesan kedua.

## Tes

```bash
cd server && go test -race ./...
cd web && bun run typecheck

# Uji beban (kuota auth dilonggarkan karena menyiapkan ribuan sesi dari satu IP)
RATE_AUTH_PER_MIN=100000 RATE_AUTH_BURST=2000 go run ./cmd/server
go run ./cmd/loadtest -users 1000 -rate 50 -duration 60s
```

Test unit menutup hal-hal yang keliru diam-diam: fan-out dan backpressure hub,
token bucket, percakapan dengan filer SeaweedFS, penyaringan penerima notifikasi
(online dan peredam dering), serta penyusunan kunci penyimpanan lampiran — yang
terakhir memastikan nama berkas kiriman tidak pernah ikut menentukan lokasi.

Fase 8 menambah empat lagi yang sifatnya sama: penguraian header `Range`
(termasuk bentuk sufiks `bytes=-500` yang dipakai pemutar untuk membaca indeks
MP4 di ujung berkas), penyajian 200/206/416 termasuk saat penyimpanan MENGABAIKAN
rentang yang diminta, pembuatan turunan beserta penolakan gambar yang terlalu
banyak pikselnya, dan pembacaan orientasi EXIF pada berkas yang sengaja
dipotong — yang terakhir menjaga berkas cacat berakhir sebagai error, bukan
sebagai panic.

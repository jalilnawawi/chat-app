# TASKLIST — Chat App

Stack: **Go 1.26** (net/http + coder/websocket + pgx) · **PostgreSQL 18** · **Bun + Vite + React 19 + TS**

Target sekarang: MVP jalan end-to-end di 1 mesin. **SELESAI** (5 Sep 2026)
Target 800+ concurrent user: **SELESAI** (5 Sep 2026) — diuji 1000 koneksi,
50 pesan/detik, 100% pesan sampai, latensi fan-out p99 10,8 ms lintas dua instance.
Lihat [docs/scaling.md](docs/scaling.md).

Lampiran & push notification: **SELESAI** (15 Sep 2026) — lihat
[docs/lampiran-dan-push.md](docs/lampiran-dan-push.md).

Legend: `[ ]` belum · `[~]` jalan · `[x]` selesai

---

## Fase 0 — Fondasi
- [x] Struktur monorepo (`server/`, `web/`)
- [x] `docker-compose.yml` (Postgres 18)
- [x] `.gitignore` + `.env.example`
- [x] `README.md` (cara run)

## Fase 1 — Database
- [x] Skema: users, sessions, conversations, conversation_members, messages
- [x] Tipe conversation: `direct` | `group` (satu abstraksi)
- [x] Unique pair untuk DM (biar nggak dobel conversation antar 2 orang)
- [x] `seq` monotonic per conversation (ordering & resume deterministik)
- [x] Kolom siap-pakai untuk fitur nanti: `edited_at`, `deleted_at`, `attachments`
- [x] Migrator embedded (`//go:embed`, nggak butuh goose)

## Fase 2 — Backend: core
- [x] Config dari env
- [x] Koneksi pgx pool + healthcheck
- [x] Auth: register / login / logout (argon2id + session cookie httpOnly)
- [x] Middleware auth + request logging
- [x] REST: list conversations, create/open conversation, history (cursor pagination)
- [x] REST: send message (idempotent via client_msg_id)
- [x] REST: edit / delete message
- [x] REST: mark read

## Fase 3 — Backend: realtime
- [x] Hub interface (in-memory dulu, siap ditukar Redis pub/sub)
- [x] WS endpoint + auth via cookie saat handshake
- [x] Protokol event (client→server & server→client) terdokumentasi
- [x] Heartbeat ping/pong + read/write deadline
- [x] Backpressure: buffered channel per client, penuh = disconnect
- [x] Resume: client kirim `last_seq`, server kirim yang ketinggalan
- [x] Presence (online/offline) + typing indicator (ephemeral, nggak nyentuh DB)
- [x] Fan-out read receipt & edit/delete ke member lain

## Fase 4 — Frontend
- [x] Scaffold Bun + Vite + React 19 + TS + Tailwind
- [x] API client + tipe TS yang match event backend
- [x] Halaman auth (login/register)
- [x] Layout chat: sidebar conversation + panel pesan
- [x] WS hook: auto-reconnect + exponential backoff + resume
- [x] Optimistic send (client_msg_id) + status pending/sent/failed
- [x] Infinite scroll history (cursor)
- [x] Typing indicator + presence dot + unread badge
- [x] Edit & delete pesan

## Fase 5 — Verifikasi
- [x] `go vet` + build bersih
- [x] `tsc --noEmit` bersih
- [x] Test manual 2 browser: kirim, terima, typing, read, reconnect
      (diotomatiskan lewat puppeteer-core + Brave, 10/10 lulus)
- [x] Test unit: hub fan-out + backpressure

---

## Fase 6 — Jalan ke 800+ user
Diuji sungguhan, bukan diperkirakan: `server/cmd/loadtest` memegang 1000 koneksi
dalam satu proses supaya latensi fan-out (A kirim -> B lihat) bisa diukur
langsung. Angka lengkap dan catatan operasional di [docs/scaling.md](docs/scaling.md).

- [x] Redis pub/sub → ganti implementasi Hub, multi-instance
      (satu channel per user; `REDIS_URL` kosong = tetap jalan satu instance)
- [x] Presence pindah ke Redis (sorted set ber-TTL, instance mati tersapu sendiri)
- [x] Rate limit per user: kirim pesan, buka koneksi, typing, dan auth per IP
      (token bucket; dibagi lintas instance lewat Lua di Redis)
- [x] Load test: 1000 WS + 50 msg/detik, tiap koneksi juga menyusul ~120 pesan
      riwayat → 100% sampai, p99 10,0 ms (1 instance) / 10,8 ms (2 instance)
- [x] Index tuning: trigram GIN untuk pencarian user, index-only scan untuk
      daftar percakapan, fillfactor + autovacuum di jalur tulis terpanas
- [x] Graceful shutdown + rolling deploy: readyz 503 → jeda LB → tolak koneksi
      baru → tutup WS tersebar 20 detik → matikan HTTP
- [x] Observability: log terstruktur + request id, `/metrics` di listener
      terpisah, metrik koneksi/lag siaran/antrean kirim/pool database
- [x] Verifikasi browser (Brave, dua konteks terpisah): daftar, buka DM, kirim
      realtime, putus jaringan, sambung lagi, dan pesan yang terlewat menyusul
      lewat `sync.batch` — 5/5 lulus
- [x] Push notification untuk user offline — ditunda dari fase ini karena
      memang fitur produk, bukan infrastruktur. **Dikerjakan di Fase 7.**

### Tiga bug yang ditemukan OLEH uji beban ini
Ketiganya sudah diperbaiki, dan ketiganya tidak terlihat pada dua browser di meja
— hanya pada seribu koneksi yang menyusul bersamaan. Uraian lengkap di
[docs/scaling.md](docs/scaling.md).

1. `Enqueue` mengembalikan `false` untuk dua sebab berbeda — buffer penuh dan
   koneksi sudah tutup — sehingga tiap orang yang menutup tab saat siaran
   kebetulan berjalan tercatat sebagai "client lambat". Sekarang dibedakan jadi
   `Delivered` / `Backpressure` / `Gone`.
2. `Client.Close()` tidak membangunkan `conn.Read` yang sedang memblokir, jadi
   client yang diam menggantung sampai prosesnya dimatikan paksa — dan drain
   rolling deploy selalu kehabisan waktu tunggu (55 detik untuk kerja 20 detik).
3. **Yang terpenting:** client yang tertinggal jauh diputus justru saat sedang
   menyusul. Resume mengirim sampai 200 pesan satu frame per pesan ke antrean
   yang dalamnya 64, lalu siaran berikutnya memutusnya karena dikira lambat —
   dan itu terjadi persis setelah rolling deploy, saat semua orang menyusul
   bersamaan. 464 dari 1000 koneksi terjebak di lingkaran itu. Diperbaiki dengan
   `sync.batch` (50 pesan per frame) + jalur kirim yang menunggu, bukan membuang.

## Fase 7 — Lampiran & push notification
Keduanya bisa dimatikan lewat satu variabel lingkungan, dan aplikasi tetap utuh
sebagai chat tanpa keduanya — pola yang sama dengan `REDIS_URL` di Fase 6.
Keputusan lengkap di [docs/lampiran-dan-push.md](docs/lampiran-dan-push.md).

### Lampiran
- [x] SeaweedFS (`chrislusf/seaweedfs:4.40`) di docker-compose, diakses lewat
      filer-nya — HTTP biasa, tanpa SDK tambahan
- [x] `blob.Store` sebagai satu-satunya pintu ke penyimpanan, sejalan dengan
      `hub.Broadcaster` dan `ratelimit.Limiter`
- [x] Unggah mengalir tanpa pernah utuh di memori (`MultipartReader` +
      `io.Pipe`), batas ukuran dipasang di body lewat `MaxBytesReader`
- [x] Isi lampiran HANYA lewat server ini, izinnya dievaluasi di dalam query —
      penyimpanan tidak tahu apa-apa tentang keanggotaan percakapan
- [x] Tipe ditentukan dari isi berkas, bukan dari yang diakui client; hanya
      empat tipe gambar disajikan inline, selebihnya dipaksa terunduh
- [x] Tabel `attachments` sebagai otoritas, kolom jsonb Fase 1 sebagai salinan
      baca — riwayat tetap satu query
- [x] Penyapu lampiran yatim (diunggah tapi tidak pernah jadi dikirim), dengan
      index parsial yang hanya memuat baris yang sedang menganggur
- [x] UI: pilih/seret/tempel, pratinjau lokal seketika, bilah kemajuan
      sungguhan, kirim ulang per berkas

### Push notification
- [x] Web Push (VAPID) — dipilih di atas FCM karena tidak mengikat aplikasi ke
      satu vendor dan tidak menuntut SDK di frontend
- [x] `go run ./cmd/vapid` membuat kunci sekali; kunci publiknya diminta client
      ke server, tidak ditanam di bundel frontend
- [x] Hanya membangunkan yang benar-benar offline (presence lintas instance)
- [x] Peredam dering per percakapan memakai token bucket yang sama dengan kuota,
      sehingga berlaku lintas instance juga
- [x] Antrean dengan delapan pekerja yang MEMBUANG saat penuh — deringnya boleh
      hilang, pesannya tidak
- [x] Langganan yang dijawab 404/410 dibuang sendiri, dan dicabut saat logout
      supaya komputer yang dipakai bergantian tidak menampilkan pratinjau pesan
      orang sebelumnya di layar kunci
- [x] Service worker tanpa cache sama sekali, dan klik notifikasi memakai
      postMessage alih-alih navigasi

### Verifikasi
- [x] `go vet` + `go test -race ./...` bersih; test baru untuk blob store,
      penyaringan penerima notifikasi, dan penyusunan kunci penyimpanan
- [x] `tsc --noEmit` + `vite build` bersih
- [x] Uji HTTP langsung: unggah, kirim, unduh oleh anggota (200), oleh orang
      luar (404), tanpa login (401), pakai ulang lampiran (403), berkas 12 MB
      (413), HTML dan SVG berisi script (keduanya jadi `attachment`)
- [x] Verifikasi browser (Brave, dua konteks terpisah, puppeteer-core):
      20/20 lulus — gambar sampai realtime dan benar-benar termuat, pesan tanpa
      teks, pratinjau sidebar, batas ukuran, tombol kirim yang tidak terkunci
      oleh unggahan gagal
- [ ] Jabat tangan langganan push dengan layanan push vendor — **tidak bisa
      diselesaikan di lingkungan uji ini.** Brave mematikan relai Google secara
      bawaan dan tidak bisa dinyalakan dari baris perintah. Yang sudah terbukti:
      service worker aktif, kunci publik sampai ke client, tombolnya muncul, dan
      seluruh sisi server (simpan langganan, penyaringan offline, peredam
      dering, kiriman keluar, pembuangan langganan mati) diuji lewat HTTP.

### Sembilan temuan dari review, semuanya diperbaiki
Yang paling serius: `Enqueue` bisa mengirim ke channel yang sudah ditutup dan
memanikkan seluruh proses — `select` dengan `default` melindungi dari antrean
penuh, bukan dari antrean tertutup, dan celahnya terbuka persis saat rolling
deploy. Selebihnya: langganan push yang tertinggal setelah logout, lampiran yang
tidak pernah terbuang saat pesannya dihapus, objek `File` yang menempel di memori
setelah terkirim, URL lampiran yang melewati awalan API, satu anggaran waktu yang
dibagi seluruh penerima satu kabar, tombol "coba lagi" untuk penolakan yang pasti
gagal lagi, pengodean `filename*` yang belum menutup titik koma, dan ikon
notifikasi yang dirujuk tanpa berkasnya ada.

### Satu bug yang ditemukan OLEH menjalankannya di browser
Selector zustand ditulis `s.uploads[id] ?? []`. Array kosong itu BARU pada setiap
pembacaan, dan zustand membandingkan hasil selector dengan `Object.is` — jadi
snapshotnya tidak pernah dianggap sama dengan sebelumnya, dan komponennya render
ulang tanpa henti sampai React menyerah dengan "Maximum update depth exceeded".

`tsc` bersih, `go test` bersih, dan tidak ada satu pun test yang bisa
menangkapnya. Yang menangkapnya adalah membuka halamannya.

---

## Fase 8 — Kandidat berikutnya
- [ ] Partisi `messages` — baru relevan di puluhan juta baris; rencana lengkap
      beserta pemicunya sudah ditulis di [docs/scaling.md](docs/scaling.md)
- [ ] Thumbnail untuk gambar — foto 12 megapiksel dari ponsel sekarang diunduh
      utuh untuk ditampilkan selebar 300 piksel
- [ ] Range request untuk lampiran, supaya video panjang bisa dilompati

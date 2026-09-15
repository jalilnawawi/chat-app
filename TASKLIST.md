# TASKLIST — Chat App

Stack: **Go 1.26** (net/http + coder/websocket + pgx) · **PostgreSQL 18** · **Bun + Vite + React 19 + TS**

Target sekarang: MVP jalan end-to-end di 1 mesin. **SELESAI** (5 Sep 2026)
Target 800+ concurrent user: **SELESAI** (5 Sep 2026) — diuji 1000 koneksi,
50 pesan/detik, 100% pesan sampai, latensi fan-out p99 10,8 ms lintas dua instance.
Lihat [docs/scaling.md](docs/scaling.md).

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
- [ ] Push notification untuk user offline — **ditunda, sengaja.** Ini fitur
      produk, bukan infrastruktur: butuh keputusan Web Push (VAPID) vs FCM,
      service worker, UI izin notifikasi, dan aturan kapan sebuah pesan pantas
      membangunkan orang.

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

## Fase 7 — Kandidat berikutnya
- [ ] Push notification untuk user offline (dipindahkan dari Fase 6)
- [ ] Partisi `messages` — baru relevan di puluhan juta baris; rencana lengkap
      beserta pemicunya sudah ditulis di [docs/scaling.md](docs/scaling.md)
- [ ] Upload lampiran (kolom `attachments` sudah disiapkan sejak Fase 1)

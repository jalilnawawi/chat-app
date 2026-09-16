# TASKLIST — Chat App

Stack: **Go 1.26** (net/http + coder/websocket + pgx) · **PostgreSQL 18** · **Bun + Vite + React 19 + TS**

Target sekarang: MVP jalan end-to-end di 1 mesin. **SELESAI** (5 Sep 2026)
Target 800+ concurrent user: **SELESAI** (5 Sep 2026) — diuji 1000 koneksi,
50 pesan/detik, 100% pesan sampai, latensi fan-out p99 10,8 ms lintas dua instance.
Lihat [docs/scaling.md](docs/scaling.md).

Lampiran & push notification: **SELESAI** (15 Sep 2026) — lihat
[docs/lampiran-dan-push.md](docs/lampiran-dan-push.md).

Turunan gambar & permintaan sepotong: **SELESAI** (15 Sep 2026) — foto 12
megapiksel turun 474 kali, dan video panjang bisa dilompati. Lihat
[docs/thumbnail-dan-range.md](docs/thumbnail-dan-range.md).

Membalas, menyebut, dan bereaksi: **SELESAI** (16 Sep 2026) — plus test
pertama untuk `internal/store`, di atas harness Postgres yang dilewati sendiri
bila databasenya tidak ada. Lihat [docs/balas-sebut-reaksi.md](docs/balas-sebut-reaksi.md).

Berikutnya: **Fase 10 — kelola akun & profil.**

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

## Fase 8 — Turunan gambar & permintaan sepotong
Keduanya menjawab pertanyaan yang sama dari dua arah: berapa byte yang sebenarnya
perlu dikirim supaya orang melihat apa yang dia minta. Sebelum fase ini
jawabannya selalu "semuanya". Keputusan lengkap di
[docs/thumbnail-dan-range.md](docs/thumbnail-dan-range.md).

### Turunan gambar
- [x] Dibuat saat UNGGAH, sebelum barisnya tercatat — supaya salinan jsonb Fase 7
      tidak pernah basi dan tidak butuh jalur sinkronisasi baru
- [x] Paket `imaging` yang masukannya byte dan keluarannya byte: bisa diuji tanpa
      database maupun penyimpanan objek
- [x] Batas decompression bomb dihitung dalam PIKSEL lewat `image.DecodeConfig`,
      sebelum dekode penuh — `MAX_UPLOAD_BYTES` tidak menolong sama sekali di sini
- [x] Dekode bersamaan dibatasi semaphore; yang tidak kebagian giliran dalam tiga
      detik lanjut TANPA turunan, tidak menunggu
- [x] Format turunan ditentukan ada-tidaknya alpha (`Opaque()`), bukan format
      aslinya — dan `thumb_mime` disimpan, bukan ditebak dari ekstensi
- [x] CatmullRom, bukan bilinear: pengecilan 3000→400 piksel dengan penapis
      bilinear memecah rambut, teks, dan garis halus jadi bintik
- [x] Orientasi EXIF dibaca dan diterapkan — tanpa ini setiap foto potret dari
      ponsel menghasilkan turunan yang MIRING sementara aslinya tampil tegak
- [x] Ukuran gambar tidak lagi dipercayakan ke client; `?w=`/`?h=` jadi cadangan
      untuk saat turunan dimatikan
- [x] Turunan ikut disapu bersama lampiran yatim, dan ikut dibuang saat baris
      lampirannya gagal ditulis
- [x] Bisa dimatikan penuh lewat `THUMBNAIL_MAX_DIM=0`

### Permintaan sepotong (Range)
- [x] `blob.Store.Get` menerima rentang; filer SeaweedFS dilayani lewat header
      HTTP biasa
- [x] Ukuran datang dari DATABASE, jadi rentang di luar berkas dijawab 416 tanpa
      menyentuh penyimpanan sama sekali
- [x] Status jawaban ditentukan oleh apa yang BENAR-BENAR dijawab penyimpanan
      (`obj.Partial`), bukan oleh apa yang kita minta darinya
- [x] Bentuk sufiks (`bytes=-500`) — yang dipakai pemutar untuk membaca indeks
      MP4 di ujung berkas
- [x] Bentuk yang tidak dikenali diabaikan (kirim utuh); yang di luar ukuran
      ditolak 416 dengan `Content-Range: bytes */<ukuran>`
- [x] Video dan rekaman suara disajikan `inline` — daftar izin, bukan larangan;
      SVG dan HTML tetap dipaksa terunduh
- [x] `Accept-Ranges: bytes` untuk semua tipe
- [x] UI: `<video controls preload="metadata">` dan `<audio>`; gambar memakai
      turunan untuk ditampilkan dan berkas asli saat diklik

### Verifikasi
- [x] `go vet` + `go test -race ./...` bersih; test baru untuk penguraian Range,
      penyajian 200/206/416, pembuatan turunan, dan pembacaan orientasi EXIF
- [x] `tsc --noEmit` + `vite build` bersih
- [x] Uji HTTP langsung: turunan hanya untuk gambar yang perlu, izin turunan sama
      persis dengan aslinya (401 tanpa login, 404 untuk orang luar, 200 untuk
      anggota), "bom" PNG 33 byte berisi kanvas 40.000 x 40.000 ditolak tapi
      lampirannya tetap utuh, dan CHECK database menolak kolom turunan yang
      separuh terisi
- [x] Verifikasi browser (Brave, dua konteks terpisah, puppeteer-core): 15/15
      lulus — gambar sampai realtime dan benar-benar termuat pada 480 piksel,
      13 KB alih-alih 6 MB, video memberi durasi dari metadata saja, dan melompat
      ke detik 170 menghasilkan `bytes 4587520-4871455/4871456`

### Angkanya
- Foto 4000 x 3000: **6.169.144 B → 13.023 B**, 474 kali lebih kecil, per anggota
  per kali percakapannya dibuka
- Video 4.871.456 B: **13.454 byte** benar-benar lewat kabel sebelum orang
  melompat — diukur dari `encodedDataLength`, bukan dari `Content-Length`

### Dua hal yang ditemukan OLEH menjalankannya
1. Migrasi `ALTER TABLE ... ADD COLUMN a, b, c` gagal di Postgres: tiap kolom
   butuh `ADD COLUMN` sendiri. `go vet` tidak melihat isi berkas `.sql`, dan
   satu-satunya yang menangkapnya adalah menyalakan servernya.
2. Nama turunan masih memakai ekstensi berkas aslinya — "logo.webp" untuk byte
   yang sebenarnya PNG. Baru terlihat saat header `Content-Disposition`
   sungguhan dibaca berdampingan dengan `Content-Type`-nya.

## Fase 9 — Membalas, menyebut, dan bereaksi
Tiga fitur yang diambil bersamaan karena bentuknya sama: **metadata yang
menempel pada SATU pesan tertentu.** Ketiganya butuh kolom atau tabel baru yang
menunjuk ke `messages`, ketiganya butuh event WS baru, dan ketiganya harus ikut
terbawa saat riwayat dibaca tanpa menambah satu query per pesan.

Mengerjakannya terpisah berarti menyelesaikan masalah "bagaimana metadata
per-pesan sampai ke client" tiga kali dengan tiga jawaban yang berbeda.

Referensinya SeaTalk ([seatalk.io](https://seatalk.io/features/communication)),
yang aplikasinya tertutup — yang bisa dilihat cuma daftar fiturnya.

Keputusan lengkap di [docs/balas-sebut-reaksi.md](docs/balas-sebut-reaksi.md).

### Balas / kutip
- [x] Kolom `reply_to_id` di `messages`, menunjuk ke tabelnya sendiri
- [x] Isi pesan yang dibalas diambil lewat SATU self-join per halaman riwayat,
      **bukan disalin** ke dalam barisnya.

      Ini berbeda dari keputusan lampiran di Fase 7, dan perbedaannya disengaja:
      salinan lampiran boleh ada karena lampiran tidak pernah berubah setelah
      terpasang. Isi pesan BERUBAH — diedit dan dihapus — jadi salinannya pasti
      basi. Self-join pada primary key murah, dan satu kali per halaman, bukan
      per pesan.
- [x] Pesan yang dibalas sudah dihapus → tampil sebagai "pesan dihapus", bukan
      hilang. Yang diedit → tampil versi terbarunya. Keduanya konsekuensi wajar
      dari tidak menyalin, dan keduanya perilaku yang benar.
- [x] **Pesan yang dibalas WAJIB berada di percakapan yang sama.** Tanpa
      pemeriksaan ini, mengutip id pesan dari percakapan yang tidak kita ikuti
      akan menampilkan isinya — kebocoran yang bentuknya persis seperti fitur.
- [x] UI: gelembung kutipan yang bisa diklik untuk melompat ke pesan aslinya

### Mention
- [x] Client mengirim `mentionedUserIds` eksplisit, BUKAN server mengurai
      `@nama` dari teks. Nama tampilan boleh mengandung spasi, dan penguraian
      teks akan selalu punya kasus tepi; yang lebih penting, siapa yang
      dibangunkan tidak boleh ditentukan oleh cara sebuah string ditulis.
- [x] Server memvalidasi tiap id: harus anggota percakapan itu. Client tidak
      pernah dipercaya soal siapa yang berhak dibangunkan.
- [x] Mention **menembus peredam dering** Fase 7 — dering biasa tetap diredam
      token bucket per percakapan, yang menyebut nama seseorang tidak. Inilah
      alasan fitur ini layak digabung dengan push yang sudah ada.
- [x] `@semua` untuk grup, dengan kuotanya sendiri — satu orang yang membangunkan
      dua ratus orang sekaligus adalah hal yang harus dibatasi, bukan dilarang
- [x] Penanda unread terpisah: "ada yang menyebut kamu" berbeda dari "ada pesan
      baru", dan bertahan walau percakapannya sudah dibuka sekilas

### Reaksi
- [x] Tabel `message_reactions` dengan primary key `(message_id, user_id, emoji)`
      — satu orang boleh memberi beberapa emoji berbeda, tapi tidak bisa
      memberi emoji yang sama dua kali. Bentuk kuncinya yang memaksakan itu,
      bukan kode aplikasi.
- [x] Diringkas per halaman riwayat dalam satu query beragregasi, bukan satu
      query per pesan
- [x] Event WS `reaction.added` / `reaction.removed`, dan ikut menyusul lewat
      jalur resume seperti event lain — reaksi yang muncul saat kita offline
      tetap harus terlihat saat menyambung lagi
- [x] Emoji divalidasi panjangnya dan harus berupa satu grafem; kolom teks bebas
      di sini berarti pesan kedua yang menyamar jadi reaksi
- [x] Kuota sendiri: menekan dan melepas reaksi adalah dua permintaan yang bisa
      diulang secepat jari bergerak
- [x] Reaksi TIDAK membangunkan notifikasi push

### Test untuk `internal/store`, disisipkan sambil jalan
Bukan fase tersendiri. Fase ini menambah SQL baru ke satu-satunya paket yang
sampai sekarang tidak punya satu test pun — dan itu justru lapisan tempat satu
salah ketik berubah jadi kehilangan data.

- [x] Harness Postgres untuk test (container sekali pakai atau skema sementara),
      dilewati otomatis bila database tidak tersedia supaya `go test` tetap
      hijau di mesin yang belum menyalakan docker
- [x] Test untuk SQL yang ditulis di fase ini
- [x] Test untuk yang paling mudah rusak diam-diam dari fase sebelumnya:
      idempotensi kirim pesan, pemasangan lampiran, dan izin baca lampiran

### Verifikasi
- [x] `go vet` + `go test -race ./...` bersih; 20 test baru untuk store, plus
      test validasi emoji yang ikut menjaga batas panjangnya tetap lebih ketat
      daripada CHECK di database
- [x] `tsc --noEmit` + `vite build` bersih
- [x] Uji HTTP langsung: kutipan lintas percakapan (403), sebutan orang luar
      (403), reaksi berupa kalimat dan dua emoji sekaligus (400), reaksi orang
      luar (404), penekanan kembar yang tidak memajukan jam, kuota @semua (429
      dengan `Retry-After: 120`), dan @semua di DM yang tersimpan sebagai false
- [x] Uji WebSocket langsung: client yang cursor pesannya sudah lengkap tapi
      cursor reaksinya nol menerima `reaction.batch` berisi keadaan terkini —
      termasuk pencabutan yang terjadi selagi dia offline
- [x] Verifikasi browser (Brave, dua konteks terpisah, puppeteer-core): 36/36
      lulus dalam dua putaran — balas, kutipan yang mengikuti edit, reaksi
      realtime yang hitungannya tidak pernah ganda, pemilih sebutan yang
      menyisipkan nama bersisipan spasi, penanda @ yang bertahan setelah
      ruangnya dibaca lalu padam setelah pesannya terlihat, dan lompat ke pesan
      yang dikutip yang halamannya belum termuat

### Tiga hal yang ditemukan OLEH menjalankannya di browser
Ketiganya lolos `tsc`, lolos `go test`, dan tidak satu pun bisa ditangkap test
mana pun yang tidak membuka halamannya. Uraian lengkap di
[docs/balas-sebut-reaksi.md](docs/balas-sebut-reaksi.md).

1. **Seluruh tombol aksi tidak bisa dijangkau dari ponsel.** `group-hover:` di
   Tailwind v4 dibungkus `@media (hover: hover)`, dan perangkat tanpa penunjuk
   melaporkan `hover: none` — jadi balas, edit, hapus, dan tombol tambah reaksi
   bukan "sulit ditemukan" melainkan tidak ada sama sekali. Berlaku sejak Fase 4
   untuk edit dan hapus; yang menemukannya adalah browser headless, yang
   kebetulan berperilaku persis seperti ponsel.
2. **Read receipt menanam larik kosong di daftar anggota.** `applyRead` menulis
   `(members[id] ?? []).map(...)` kembali ke state, dan larik kosong itu tidak
   bisa dibedakan dari "sudah dimuat, memang kosong". Read receipt datang jauh
   lebih sering daripada orang membuka percakapan, jadi keadaannya nyaris
   permanen: judul grup "0 anggota", pengirim "Seseorang", sebutan tidak pernah
   tersorot. Tidak terlihat di DM — dan seluruh verifikasi browser Fase 6-8
   memakai DM.
3. **Daftar anggota menumpang syarat milik riwayat.** `openConversation` memuat
   keduanya di balik `!messages[id]`, padahal `messages[id]` bisa terisi tanpa
   riwayat pernah dimuat: satu pesan yang datang lewat WebSocket sudah cukup.
   Bentuk kegagalannya sama persis dengan nomor 2, dan itu yang membuatnya
   sempat tersembunyi di baliknya.

### Yang sengaja TIDAK dikerjakan di fase ini
- **Panggilan suara/video.** Yang terbesar dari daftar SeaTalk, dan jalurnya
  jelas — repo mereka memuat fork `pion/webrtc` dan `pion/ice`, jadi backend Go
  kita ada di jalur yang sama. Tetap ditunda: dia satu fase penuh sendiri, dan
  dia menyeret kembali urusan deployment lewat kebutuhan server TURN.
- **Terjemahan, pesan terjadwal, self-destruct.** Menarik, tapi tidak satu pun
  mengubah bentuk aplikasi seperti tiga fitur di atas.

## Fase 10 — Kelola akun & profil
Sampai Fase 9 sebuah akun hanya punya username, nama tampilan, dan password.
Tidak ada cara mengubah apa pun setelah mendaftar, tidak ada email, dan tidak ada
foto. Fase ini menutup itu.

Satu keputusan menaungi seluruh fase ini, dan kalau salah akan terasa di
mana-mana: **status BUKAN presence.**

Presence (online/offline) sudah ada sejak Fase 6 — diturunkan dari koneksi yang
hidup, disimpan di Redis dengan TTL, dan sengaja fana: instance yang mati
tersapu sendiri. Status ("available", "busy", "sedang rapat sampai 13.00")
adalah pernyataan yang dibuat orang dengan sengaja, dan harus bertahan melewati
tutup laptop, ganti perangkat, dan logout.

Menyatukan keduanya berarti status "busy sampai jam 1" yang baru saja seseorang
pasang lenyap begitu dia menutup tab. Jadi presence tetap di Redis, status
tinggal di Postgres, dan client menampilkan gabungan keduanya.

### Foto profil
- [ ] Kolom `avatar_key` di `users`; byte-nya lewat `blob.Store` seperti lampiran
- [ ] Ukurannya diperkecil lewat paket `imaging` Fase 8 saat diunggah, dan
      **berkas aslinya dibuang** — tidak ada yang butuh avatar dua belas
      megapiksel, dan menyimpannya berarti membayar selamanya untuk sesuatu yang
      selalu ditampilkan selebar 40 piksel
- [ ] **Alamatnya harus berubah setiap fotonya berubah.**

      Fase 8 memasang `Cache-Control: private, max-age=31536000, immutable` pada
      semua isi lampiran, dan itu benar untuk byte yang memang tidak pernah
      berubah. Avatar BERUBAH. Alamat tetap seperti `/api/users/{id}/avatar`
      berarti browser menyimpan foto lama selama setahun dan tidak pernah lagi
      bertanya — orang mengganti fotonya, dan tidak seorang pun melihatnya.

      Jadi tiap unggahan menghasilkan id baru, persis seperti lampiran, dan
      alamatnya ikut berubah. Dengan begitu cache setahun kembali menjadi benar,
      bukan menjadi jebakan.
- [ ] Izin bacanya BERBEDA dari lampiran: avatar boleh dilihat siapa pun yang
      sudah login, karena orangnya memang sudah bisa ditemukan lewat pencarian
      pengguna. Ditulis eksplisit supaya tidak ada yang menyalin aturan lampiran
      ke sini dan mengira itu kebetulan lebih aman.
- [ ] Avatar lama disapu setelah diganti, lewat penyapu yang sudah ada

### Email & password
- [ ] Kolom `email` di `users` — unik, dan boleh kosong untuk akun lama
- [ ] Mengubah email atau password **menuntut password saat ini**, bukan sekadar
      sesi yang masih hidup. Sesi bisa saja milik laptop yang ditinggal terbuka.
- [ ] Verifikasi email lewat tautan bertoken, dan **email yang belum terverifikasi
      tidak boleh dipakai memulihkan akun sama sekali** — kalau boleh, memulihkan
      akun cuma butuh mengaku memiliki sebuah alamat
- [ ] SMTP jadi dependensi baru, dan mengikuti pola yang sama dengan `REDIS_URL`,
      `SEAWEED_FILER_URL`, dan kunci VAPID: `SMTP_URL` kosong berarti fitur
      email mati dan aplikasinya tetap utuh
- [ ] Reset password lewat email — token sekali pakai, berumur pendek, disimpan
      sebagai hash seperti token sesi
- [ ] **Ganti password mencabut semua sesi lain, dan benar-benar memutus
      koneksinya.**

      Menghapus baris di `sessions` saja tidak cukup. Koneksi WebSocket yang
      sudah terlanjur terbuka dipegang di memori proses, dan sejak Fase 6 proses
      itu bisa instance LAIN. Baris sesi hilang sementara koneksinya tetap hidup
      berarti orang yang password-nya baru saja dicuri tetap terhubung.

      Perlu satu event baru di `hub` yang menyuruh instance mana pun yang
      memegang sesi itu menutup koneksinya — jalur yang sama dengan siaran
      biasa, hanya arah kebalikannya.
- [ ] Daftar sesi aktif beserta cara mencabutnya satu per satu

### Status
- [ ] Kolom `status` (`available` / `busy` / `away`), `status_text`, dan
      `status_expires_at` di `users`
- [ ] `status_text` dibatasi panjangnya. Ini teks bebas yang ditampilkan ke orang
      lain, dan tanpa batas dia jadi pesan kedua yang menyamar jadi status.
- [ ] **Durasi tidak dijaga timer di server.**

      "10.00 – 13.00" disimpan sebagai `status_expires_at`, dan tidak ada job
      yang membersihkannya. Pembacaan menyaring sendiri
      (`CASE WHEN status_expires_at > now()`), dan client menerima `expiresAt`
      lalu menghitung mundur sendiri.

      Alasannya sama dengan typing indicator di Fase 3 yang sengaja tidak
      menyentuh database: keadaan yang kedaluwarsa dengan sendirinya tidak
      butuh sesuatu yang berjalan. Penyapu lintas instance justru menambah
      masalah — butuh penguncian, dan tiap instance akan menyiarkan kabar
      kedaluwarsa yang sama.
- [ ] Waktunya disimpan sebagai instan absolut (`timestamptz`), dan client yang
      merendernya ke jam lokal. "Sampai jam 13.00" di jam siapa adalah
      pertanyaan yang harus punya jawaban sebelum baris pertama ditulis.
- [ ] Perubahan status disiarkan ke `ContactIDs` lewat `hub.Publish` — jalur yang
      persis sama dengan presence, jadi tidak ada mekanisme fan-out kedua
- [ ] Snapshot status ikut dikirim saat client menyambung, seperti
      `presence.snapshot` — tanpa itu status seseorang baru terlihat saat dia
      kebetulan menggantinya
- [ ] **`busy` meredam push.** Inilah yang membuat status bukan sekadar hiasan:
      dia menyambung ke peredam dering Fase 7. Mention tetap menembus, sama
      seperti keputusan di Fase 9.
- [ ] UI: pemilih status, kolom teks bebas, pemilih durasi dengan pilihan cepat
      (30 menit, 1 jam, sampai akhir hari), dan titik presence yang menampilkan
      gabungan online + status

### Verifikasi
- [ ] Test store untuk jalur baru, di atas harness yang dibuat di Fase 9
- [ ] Uji langsung: ganti password memutus sesi lain **di instance yang berbeda**,
      status yang sudah lewat waktunya tidak pernah ikut terbaca, dan mengganti
      foto benar-benar terlihat oleh orang lain tanpa menunggu cache

## Fase 11 — Kandidat berikutnya
- [ ] **Menemukan pesan**: cari, teruskan, dan pin — kelompok yang saling
      menguatkan seperti Fase 9. `pg_trgm` sudah terpasang sejak Fase 6, tapi
      untuk isi pesan `tsvector` hampir pasti pilihan yang lebih tepat daripada
      trigram, dan itu keputusan yang layak ditulis saat mengerjakannya.
- [ ] **Pesan suara**: lampiran audio dan permintaan sepotong sudah jalan sejak
      Fase 8; yang belum cuma UI perekamnya
- [ ] Partisi `messages` — tetap ditunda, dan pemicunya tetap belum ada; rencana
      lengkapnya sudah ditulis di [docs/scaling.md](docs/scaling.md)
- [ ] Pratinjau bingkai pertama untuk video — menuntut dekoder video di dalam
      proses ini, ketergantungan yang jauh lebih besar daripada seluruh Fase 8
- [ ] Beberapa ukuran turunan (`srcset`) — satu ukuran sudah menutup selisih
      seratus kali lipat; yang kedua hanya dua kali, dengan menggandakan jumlah
      objek di penyimpanan

## Sebelum aplikasi ini boleh dipakai orang
Bukan fase, melainkan daftar yang harus lunas kapan pun tujuannya berubah dari
"menambah fitur" jadi "menjalankannya untuk orang lain". Ditulis di sini supaya
tidak perlu ditemukan ulang nanti.

- [ ] Cara men-deploy: `Dockerfile`, CI, dan sesuatu yang menjalankan rolling
      deploy yang logikanya sudah ada di `cmd/server` sejak Fase 6
- [ ] TLS dan `SECURE_COOKIE=true` — bawaannya `false`, dan tanpa ini aplikasi
      tidak boleh menyentuh internet
- [ ] Prosedur cadangan untuk Postgres DAN SeaweedFS; keduanya memegang data
      yang tidak bisa dibuat ulang
- [ ] Reset password — **dijadwalkan di Fase 10**, bersama email yang memang jadi
      syaratnya; sampai itu selesai, orang yang lupa kehilangan akunnya selamanya
- [ ] Hapus akun, blokir pengguna, dan cara melaporkan penyalahgunaan

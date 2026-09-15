# Menjalankan chat ini untuk 800+ user

Catatan operasional Fase 6: apa yang berubah, apa yang dipantau, dan apa yang
sengaja **belum** dikerjakan beserta alasannya.

---

## Dua mode, satu kode

Aplikasi ini punya satu tombol yang menentukan bentuk deploymentnya:

| `REDIS_URL` | Siaran | Presence | Rate limit | Instance |
|---|---|---|---|---|
| kosong | memori proses | tabel koneksi | memori proses | tepat satu |
| diisi | Redis pub/sub | key ber-TTL | token bucket di Redis | berapa pun |

Handler HTTP dan WebSocket tidak tahu mana yang sedang dipakai — keduanya hanya
bicara ke `hub.Broadcaster` dan `ratelimit.Limiter`. Mode satu instance bukan
mode "kurang lengkap"; untuk pengembangan lokal justru itu yang paling jujur,
karena `go run ./cmd/server` cukup untuk menjalankan seluruh aplikasi.

### Kenapa satu channel Redis per user

Bukan satu channel global. Dengan channel global, setiap instance menerima dan
men-decode SEMUA event aplikasi lalu membuang yang bukan miliknya — biaya yang
tumbuh mengikuti jumlah instance. Dengan `chat:u:<uuid>`, Redis yang menyaring,
dan sebuah instance hanya menerima byte yang memang akan dia tulis ke socket.

Instance juga menerima kembali event yang dia publish sendiri. Itu disengaja:
hanya ada satu jalur pengiriman. Jalan pintas lokal berarti dua jalur yang
sama-sama harus benar dan tidak boleh mengirim dobel — biaya yang tidak sepadan
dengan hemat satu perjalanan ke Redis. Uji beban mengukur harganya: **p99 naik
dari 10,2 ms ke 12,3 ms**.

### Kenapa presence pakai sorted set, bukan set biasa

Anggotanya instance ID, skornya waktu kedaluwarsa. Instance yang mati mendadak
tidak sempat membersihkan dirinya; skor basi itulah yang membuatnya tersapu
sendiri pada operasi berikutnya. Dengan set biasa, satu instance yang crash
meninggalkan user "online selamanya" sampai ada yang membersihkan manual.

---

## Rolling deploy

Urutan yang dijalankan tiap instance saat menerima SIGTERM:

1. `/readyz` mulai menjawab **503**. Load balancer berhenti mengirim koneksi
   baru; yang sudah tersambung tidak diganggu sama sekali.
2. Jeda `DRAIN_DELAY` (5 detik). LB butuh waktu untuk benar-benar berhenti
   mengirim — menutup lebih awal justru memutus koneksi yang baru saja diantar.
3. `/ws` menolak koneksi baru dengan 503, jaga-jaga kalau LB lambat atau tidak
   ada. Tanpa ini, client yang baru diputus menyambung kembali ke instance yang
   sedang pamit, dan pengurasan berputar tanpa pernah selesai.
4. Tiap koneksi diberi tahu lewat event `server.shutdown`, lalu ditutup
   **tersebar** sepanjang `DRAIN_PERIOD` (20 detik), bukan serentak.
5. Baru HTTP dimatikan.

Langkah 4 tidak bisa diwakilkan ke `http.Server.Shutdown`: koneksi WebSocket
sudah di-hijack dari server HTTP, jadi `Shutdown` tidak melihatnya dan akan
selesai seolah semuanya beres sementara ribuan koneksi masih terbuka.

Client membedakan `server.shutdown` dari koneksi yang putus begitu saja:
yang pertama disambung lagi dalam 200–1200 ms acak, yang kedua mundur bertahap
sampai 15 detik. Keduanya diacak — server sudah menyebar penutupannya, dan
client yang membalas dengan jeda seragam akan merapikan sebaran itu kembali
jadi barisan.

Ukur dengan: `SHUTDOWN_TIMEOUT` harus lebih besar dari `DRAIN_DELAY +
DRAIN_PERIOD`; konfigurasi menolak jalan kalau tidak.

---

## Yang dipantau

`/metrics` dilayani di listener terpisah (`METRICS_ADDR`, default `:9091`).
Jangan diekspos ke internet.

Empat metrik yang paling cepat memberi tahu ada yang salah:

| Metrik | Artinya kalau naik |
|---|---|
| `chat_ws_slow_clients_dropped_total` | Siaran lebih cepat dari kemampuan client menyerap. Koneksi diputus dan client menyusul lewat resume — tapi kalau angkanya terus naik, buffer per koneksi (`sendBuffer`) terlalu kecil untuk beban ini. |
| `chat_ws_send_queue_depth` | Peringatan dini dari metrik di atas. Antrean yang biasanya nol lalu sering menyentuh belasan berarti batasnya sudah dekat. |
| `chat_db_pool_acquire_wait_seconds_total` | Kirim pesan mulai antre menunggu koneksi database. Naikkan `DB_MAX_CONNS`. Gejalanya sunyi: WebSocket tetap terbuka, presence tetap hijau, tapi semuanya melambat. |
| `chat_broadcast_duration_seconds{transport="redis"}` | Lag siaran lintas instance. Kalau ini naik sementara `transport="memory"` tidak, masalahnya di Redis, bukan di aplikasi. |

Label rute pada metrik HTTP sudah diciutkan (`/api/conversations/{id}/messages`),
jadi id percakapan tidak pernah menjadi label. Tanpa itu, satu deret waktu lahir
untuk setiap percakapan yang pernah dibuka.

---

## Uji beban

```bash
# Kuota auth dilonggarkan karena penyiapan membuat ribuan sesi dari satu IP.
RATE_AUTH_PER_MIN=100000 RATE_AUTH_BURST=2000 go run ./cmd/server

# Satu instance
go run ./cmd/loadtest -users 1000 -rate 50 -duration 60s

# Lintas instance: user disebar bergiliran, jadi pengirim dan penerima sebuah
# pesan sering berada di proses yang berbeda.
go run ./cmd/loadtest \
  -base http://127.0.0.1:8090,http://127.0.0.1:8091 \
  -metrics http://127.0.0.1:9091/metrics,http://127.0.0.1:9092/metrics \
  -users 1000 -rate 50 -duration 60s

# 1000 koneksi menganggur saja
go run ./cmd/loadtest -users 1000 -rate 0 -duration 5m
```

Alat ini ditulis sendiri, bukan memakai k6, karena angka yang paling menentukan
rasa sebuah aplikasi chat adalah **latensi fan-out**: jeda antara A menekan
kirim dan B melihat pesannya. Mengukurnya menuntut satu proses memegang kedua
sisi percakapan sekaligus — sementara perkakas load test umumnya mengisolasi
tiap virtual user justru supaya mereka tidak saling memengaruhi.

### Hasil, 5 September 2026

Satu mesin, Postgres + Redis di Docker. 1000 koneksi, grup 10 orang, 50 pesan/detik
selama 60 detik. Tiap koneksi juga menyusul ~120 pesan riwayat saat tersambung,
jadi ini sekaligus menguji reconnect massal — bukan cuma koneksi yang sudah mapan.

| | Satu instance | Dua instance (Redis) |
|---|---|---|
| Koneksi | 1000/1000, 0 putus | 1000/1000, 0 putus |
| Pengiriman | **29.990/29.990 (100%)** | **29.990/29.990 (100%)** |
| Latensi fan-out p50 | 7,2 ms | 7,9 ms |
| Latensi fan-out p95 | 9,1 ms | 9,7 ms |
| Latensi fan-out p99 | **10,0 ms** | **10,8 ms** |
| Latensi fan-out maks | 24,2 ms | 23,6 ms |
| POST /messages p99 | 10,0 ms | 10,8 ms |
| Waktu connect p99 | 3,5 ms | 4,4 ms |
| Client lambat diputus | 0 | 0 |

Harga menyeberangkan siaran lewat Redis: **0,8 ms pada p99.** Segitu yang dibayar
untuk bisa menambah instance.

### Verifikasi di browser

Selain uji beban, jalur yang berubah diuji di Brave dengan dua konteks browser
terpisah: daftar dua akun, buka DM, kirim pesan realtime, **matikan jaringan
salah satu client**, kirim tiga pesan selama dia terputus, lalu hidupkan lagi
jaringannya. Ketiga pesan muncul lewat `sync.batch`. 5/5 langkah lulus.

Yang diuji di sini dan tidak tertangkap uji beban: bahwa UI benar-benar
merender pesan susulan yang datang berkelompok, bukan cuma menerimanya.

### Tiga bug yang ditemukan OLEH uji ini

Ketiganya sudah diperbaiki. Ketiganya juga tidak akan muncul pada dua browser di
meja — hanya pada seribu koneksi yang menyusul bersamaan.

**1. Metrik backpressure ikut menghitung orang yang menutup tab.**
`Enqueue` mengembalikan `false` untuk dua sebab yang sangat berbeda: buffer penuh
dan koneksi sudah tutup. Setiap orang yang menutup tab saat sebuah siaran
kebetulan berjalan tercatat sebagai "client lambat" — 29 kejadian palsu dalam
satu kali uji. Alarm yang berbunyi saat tidak ada apa-apa adalah alarm yang
akhirnya dimatikan orang. Sekarang dipisah jadi `Delivered` / `Backpressure` /
`Gone`.

**2. Menutup koneksi tidak membangunkan pembacaan yang sedang memblokir.**
`Client.Close()` hanya menutup channel sinyal. `readLoop` sedang menggantung di
`conn.Read`, dan pembacaan itu hanya berakhir kalau lawan bicaranya yang menutup
— client yang diam (tab di latar belakang, HP terkunci) bertahan sampai
prosesnya dimatikan paksa. Akibatnya drain rolling deploy selalu kehabisan waktu
tunggu: 55 detik untuk pekerjaan 20 detik, dan tetap menyisakan koneksi.
Sekarang `Close()` ikut membatalkan konteks koneksi.

**3. Client yang tertinggal jauh diputus justru saat sedang menyusul.**
Ini yang paling berbahaya, dan yang paling lama tersembunyi.

Antrean kirim tiap koneksi dalamnya 64 frame — ukuran untuk meredam ledakan
siaran, bukan untuk menampung riwayat. Resume mengirim sampai 200 pesan susulan
**satu frame per pesan**, jadi antreannya luber, dan siaran pertama yang datang
saat itu memutus koneksinya karena dikira lambat. Client menyambung lagi,
menyusul lagi, diputus lagi.

Yang membuatnya berbahaya: kejadiannya justru saat semua orang menyusul
bersamaan — persis setelah rolling deploy, atau setelah jaringan pulih. Di uji
beban, 464 dari 1000 koneksi terjebak di lingkaran itu dan pengiriman anjlok ke
53,9%.

Dua perbaikan, dan keduanya perlu:

- Resume dikirim berkelompok (`sync.batch`, 50 pesan per frame). Riwayat sepenuh
  apa pun jadi muat dalam beberapa frame.
- Resume memakai `EnqueueWait` yang **menunggu**, bukan `Enqueue` yang membuang.
  Aturan "putus client lambat" ada untuk melindungi SIARAN dari satu orang yang
  lambat; menyusul adalah urusan koneksi itu sendiri, jadi tekanan baliknya
  ditanggung sendiri juga dan tidak merugikan siapa pun.

## Yang sengaja belum dikerjakan

### Partisi tabel `messages`

**Kapan mulai relevan:** puluhan juta baris. Di bawah itu, index
`(conversation_id, seq DESC)` sudah menjawab semua query panas dalam beberapa
pembacaan halaman, dan partisi cuma menambah kerumitan tanpa imbalan.

**Tanda harus dikerjakan:** `pg_relation_size('messages')` mendekati memori
mesin, autovacuum pada `messages` mulai memakan waktu berjam-jam, atau
`ANALYZE` mulai kelihatan di log lambat.

**Rencananya, kalau saatnya tiba:** partisi RANGE berdasarkan `created_at`,
bulanan — bukan HASH berdasarkan `conversation_id`. Alasannya:

- Beban baca chat sangat condong ke masa kini. Partisi per waktu membuat
  partisi terbaru cukup kecil untuk muat di cache, dan partisi lama nyaris tidak
  pernah disentuh.
- Menghapus riwayat lama jadi `DROP PARTITION` — instan, tanpa VACUUM.
- Partisi HASH menyebar data merata, yang justru membuang keunggulan itu:
  setiap partisi tetap berisi campuran pesan baru dan lama.

Yang perlu dipikirkan saat itu: `UNIQUE (conversation_id, seq)` harus menyertakan
kolom partisi, jadi berubah jadi `(conversation_id, seq, created_at)` — dan
`ON CONFLICT` untuk idempotensi ikut menyesuaikan.

### Push notification untuk user offline

Ditunda sebagai keputusan sadar. Ini fitur produk, bukan infrastruktur: butuh
pilihan Web Push (VAPID) versus FCM, service worker di frontend, UI izin
notifikasi, dan aturan kapan sebuah pesan pantas membangunkan orang. Paling enak
dikerjakan setelah lapisan di bawahnya stabil — yang sekarang sudah.

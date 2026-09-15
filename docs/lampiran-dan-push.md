# Lampiran dan push notification

Catatan Fase 7. Dua fitur yang dikerjakan bersamaan karena keduanya punya sifat
yang sama: berguna, terlihat sederhana dari luar, dan penuh keputusan yang baru
kelihatan akibatnya belakangan.

Keduanya bisa dimatikan lewat satu variabel lingkungan, dan aplikasi tetap utuh
sebagai chat tanpa keduanya. Itu bukan kehati-hatian berlebihan — itu yang
membuat `go run ./cmd/server` tetap cukup untuk mengembangkan sisanya.

---

## Bagian 1 — Lampiran

### Kenapa byte-nya tidak disimpan di Postgres

Kolom `attachments` sudah disiapkan sejak migrasi pertama, dan menaruh isinya
langsung di sana sebagai `bytea` adalah pilihan yang paling sedikit bagiannya.
Tetap tidak diambil, karena `messages` adalah tabel tulis terpanas di aplikasi
ini, dan Fase 6 baru saja menyetelnya untuk itu: fillfactor, autovacuum yang
lebih agresif, index yang menjawab tanpa menyentuh heap.

Satu foto sepuluh megabyte di tabel itu masuk ke WAL, ikut direplikasi, ikut
disalin setiap kali basis datanya dicadangkan, dan menggeser halaman yang tadinya
muat di cache. Semua penyetelan itu dibayar untuk baris yang panjangnya ratusan
byte, bukan jutaan.

### Kenapa lewat filer SeaweedFS, bukan API master+volume

SeaweedFS punya dua cara menulis. Cara aslinya: minta `fid` ke master, lalu
unggah ke volume server yang ditunjuk, lalu simpan `fid` itu. Cara kedua: bicara
ke **filer**, yang memberi namespace berupa path biasa dan mengurus sendiri
alokasi volume di belakangnya.

Yang dipakai di sini cara kedua. Alasannya bukan kemalasan: `storage_key` yang
berupa path membuat isi penyimpanan bisa dibaca manusia — `2026/09/<uuid>.png` —
dan seluruh percakapan dengan penyimpanan jadi HTTP biasa. Tidak ada SDK yang
perlu ditambahkan ke `go.mod`, dan `blob.Store` bisa ditukar ke S3 atau disk
biasa tanpa menyentuh satu pun handler.

Yang ditukar: satu lompatan jaringan di dalam klaster penyimpanan. Untuk lampiran
chat, itu tidak terukur.

### Kenapa isi lampiran melewati server Go, bukan diambil langsung

Ini keputusan terbesar di bagian ini, dan yang paling mahal kalau salah.

Penyimpanan objek tidak tahu apa-apa tentang keanggotaan percakapan. Satu-satunya
tempat yang bisa menjawab "boleh tidak orang ini membaca berkas ini" adalah
server yang menyimpan keanggotaannya. Maka tidak ada jalan menuju byte lampiran
selain `GET /api/attachments/{id}`, dan SeaweedFS tidak pernah terjangkau dari
internet sama sekali.

Izinnya dievaluasi di dalam query, bukan di handler:

```sql
WHERE a.id = $1
  AND (a.owner_id = $2
    OR (m.deleted_at IS NULL AND EXISTS (
          SELECT 1 FROM conversation_members cm
          WHERE cm.conversation_id = m.conversation_id AND cm.user_id = $2)))
```

Lampiran yang belum terpasang ke pesan mana pun hanya bisa dibaca pengunggahnya
— itu yang membuat pratinjau sebelum kirim jalan tanpa membuka berkas itu untuk
orang lain. Yang tidak berhak mendapat **404**, bukan 403: membedakan keduanya
berarti memberi tahu orang asing bahwa berkas itu ada.

Harganya: byte lampiran mengalir lewat proses Go, dua kali (masuk dan keluar).
Jalan keluarnya kalau itu jadi masalah ada dua — URL bertanda tangan berumur
pendek, atau `X-Accel-Redirect` ke reverse proxy di depan — dan keduanya menjaga
keputusan izin tetap di sini. Yang tidak akan pernah dilakukan: menaruh
penyimpanan langsung di internet dan berharap id yang panjang sudah cukup jadi
kunci.

### Kenapa unggah dipisah dari kirim pesan

Alurnya dua langkah: `POST /api/attachments` lebih dulu, lalu id-nya disebut di
`POST .../messages`.

Menggabungkannya jadi satu permintaan multipart terlihat lebih sederhana, tapi
menyatukan dua hal yang gagal dengan cara berbeda. Unggahan berkas sepuluh
megabyte butuh waktu dan bisa putus di tengah; pengiriman pesan harus tetap satu
tindakan cepat yang jawabannya pasti. Kalau digabung, jaringan yang putus pada
detik terakhir membatalkan teks yang sudah diketik orang.

Pemisahan itu melahirkan keadaan baru: berkas yang sudah ada isinya tapi belum
punya pesan. Itu sebabnya ada tabel `attachments` tersendiri walaupun kolom
jsonb-nya sudah ada — berkas dalam keadaan itu tetap butuh pemilik (untuk izin
baca) dan tetap butuh tercatat (untuk bisa dibuang).

### Tabelnya otoritas, jsonb-nya salinan baca

`attachments` adalah kebenaran: siapa pemiliknya, di kunci mana isinya, pesan
mana yang memakainya. `messages.attachments` adalah salinan yang ikut terbaca
saat mengambil riwayat.

Duplikasi itu disengaja. Menampilkan seratus pesan tidak boleh menuntut JOIN
tambahan per pesan, dan lampiran tidak pernah berubah setelah terpasang — jadi
salinannya tidak akan pernah basi. Menghapus pesan mengosongkan kolom jsonb-nya
sekaligus, sehingga client yang sedang offline ikut menyinkronkan hilangnya
lampiran itu.

### Yang dipaksakan saat lampiran dipasang ke pesan

Satu `UPDATE` memaksa tiga syarat sekaligus:

```sql
UPDATE attachments SET message_id = $1
WHERE id = ANY($2) AND owner_id = $3 AND message_id IS NULL
```

Miliknya sendiri, belum pernah dipakai pesan lain, dan memang ada. Jumlah baris
yang kembali lebih sedikit dari yang diminta berarti salah satunya gagal — dan
jawabannya sama untuk ketiganya: tolak. Semuanya di dalam transaksi pengiriman
pesan, supaya sebuah pesan tidak pernah terlihat tanpa lampiran yang
menyertainya, sekalipun untuk sepersekian detik.

Urutan yang dikembalikan `RETURNING` tidak dijamin, jadi hasilnya disusun ulang
mengikuti urutan yang dipilih pengirim di layarnya.

### Berkas aktif: kenapa hanya empat tipe yang disajikan inline

Lampiran disajikan dari origin yang SAMA dengan aplikasi. Itu berarti berkas
yang bisa dieksekusi browser — HTML, dan terutama SVG, yang boleh memuat
`<script>` — akan berjalan sebagai bagian dari aplikasi ini kalau dibuka inline.
Cookie sesi memang `httpOnly` dan tidak bisa dibaca script, tapi script yang
berjalan di origin ini tetap bisa MEMAKAI sesi itu lewat `fetch`.

Empat lapis, semuanya dipasang di satu fungsi supaya tidak ada jalur yang lupa
salah satunya:

1. **Tipe ditentukan dari isi, bukan dari yang diakui client.** 512 byte pertama
   dibaca dan dilewatkan ke `http.DetectContentType`. Header `Content-Type` pada
   unggahan adalah pernyataan pengunggah, dan pengunggah bisa berbohong.
2. **Hanya `image/png`, `image/jpeg`, `image/gif`, dan `image/webp` yang
   `inline`.** Selebihnya `Content-Disposition: attachment`, termasuk SVG.
3. **`X-Content-Type-Options: nosniff`**, supaya browser tidak menebak sendiri
   dan mengabaikan tipe yang kita sebutkan.
4. **`Content-Security-Policy: default-src 'none'; sandbox`**, sebagai lapis
   terakhir kalau sesuatu lolos sampai dirender.

Diuji: berkas HTML berisi `<script>alert(document.cookie)</script>` dan SVG
berisi `<script>` keduanya keluar sebagai `attachment`.

### Nama berkas tidak pernah ikut menentukan lokasi

`storage_key` disusun dari id dan tanggal: `2026/09/<uuid>.png`. Nama dari
komputer orang lain hanya dipinjam ekstensinya, dan itu pun sebagai cadangan
terakhir setelah tipe isinya.

Dengan skema ini tidak ada yang perlu "dibersihkan" — `../../../etc/passwd`,
`C:\Windows\...`, dan nama sepanjang lima ratus karakter semuanya menghasilkan
kunci yang bentuknya sama. Pembersihan nama yang dipakai sebagai path hanya
perlu lolos sekali untuk jadi masalah; tidak memakainya sama sekali tidak punya
kasus tepi.

Nama aslinya tetap disimpan — dia yang dikembalikan saat berkasnya diunduh, dan
dia ditulis dalam dua bentuk sekaligus (ASCII untuk client lama, RFC 5987 untuk
yang paham) supaya "Laporan Triwulan – Final.pdf" sampai dengan namanya utuh.

### Unggahan tidak pernah utuh di memori

`r.MultipartReader()`, bukan `r.ParseMultipartForm()`. Yang kedua menyalin
seluruh unggahan ke memori dan disk sementara lebih dulu, lalu menyerahkannya.
Untuk berkas yang cuma perlu diteruskan ke penyimpanan, singgah itu murni biaya
— dan seratus unggahan sepuluh megabyte bersamaan berarti satu gigabyte yang
tidak pernah direncanakan siapa pun.

Di sisi penyimpanan, `io.Pipe` melakukan hal yang sama ke arah sebaliknya: bagian
multipart ditulis di goroutine sendiri sementara `http.Client` membacanya sebagai
body.

Konsekuensinya satu: field form yang datang SETELAH berkas tidak akan pernah
terbaca tepat waktu. Karena itu ukuran gambar dikirim lewat query string
(`?w=320&h=200`), yang sudah lengkap sebelum byte pertama tiba.

Batas ukurannya dipasang pada body lewat `http.MaxBytesReader`, bukan diperiksa
setelah berkasnya masuk. Memeriksa belakangan berarti sudah terlanjur menerima.

### Lampiran yatim

Orang memilih foto, lalu berubah pikiran dan menutup tab. Itu normal, dan tiap
kejadiannya meninggalkan berkas yang tidak pernah dilihat siapa pun dan tidak
pernah bisa dihapus lewat UI mana pun — karena tidak ada UI yang bisa
menampilkannya.

Penyapu berjalan tiap 30 menit, mengambil paling banyak 200 baris per putaran.
Batas itu ada karena beberapa instance menjalankan penyapu yang sama, dan tiap
berkas yang dibuang adalah satu permintaan ke penyimpanan yang sedang melayani
orang membuka gambar.

Index-nya parsial:

```sql
CREATE INDEX attachments_orphan_idx ON attachments (created_at)
    WHERE message_id IS NULL;
```

Hanya memuat baris yang sedang menganggur, jadi ukurannya tetap kecil walau
tabelnya tumbuh — dan kembali mengecil begitu lampirannya terpakai.

Menghapus pesan mengembalikan lampirannya ke keadaan yatim (`message_id`
dilepas), sehingga penyapu yang sama ikut membuangnya. Tanpa itu, berkasnya
tertinggal selamanya: tidak ada layar yang bisa menampilkannya lagi, jadi tidak
ada jalan membuangnya lewat UI — dan penyapu hanya melihat baris yang
`message_id`-nya kosong. Efek sampingnya tepat: anggota lain langsung mendapat
404, karena lampiran yang belum terpasang hanya bisa dibaca pengunggahnya.

**Urutan yang penting, dan dipakai konsisten di dua tempat:**

- Saat mengunggah: tulis byte dulu, baru catat barisnya. Kalau mati di antaranya,
  yang tertinggal adalah berkas tanpa baris — sampah diam.
- Saat menyapu: hapus baris dulu, baru buang byte-nya. Sama alasannya.

Urutan sebaliknya, di kedua kasus, meninggalkan baris yang isinya tidak ada —
dan itu tampil di layar orang sebagai lampiran yang rusak.

---

## Bagian 2 — Push notification

### Aturannya, bukan mekanismenya

Fitur ini gampang dibuat menyebalkan, jadi aturannya dipasang di satu tempat dan
sengaja ketat:

1. **Hanya untuk yang benar-benar tidak terhubung.** Presence sudah tahu
   jawabannya lintas instance sejak Fase 6; orang yang tabnya terbuka di sebelah
   tidak perlu diberi tahu dua kali.
2. **Satu percakapan tidak boleh berbunyi berkali-kali.** Dua puluh pesan
   beruntun dari satu orang adalah SATU kabar.
3. **Pengiriman tidak pernah menahan pengirim pesan.**

### Peredam dering memakai token bucket yang sama dengan kuota

Kunci `push:<user>:<percakapan>`, aturan bawaan burst 2 dan 2 per menit — kira-
kira satu dering tiap 30 detik per percakapan, dengan kelonggaran dua di awal
supaya kabar pertama tidak pernah tertelan.

Memakai `ratelimit.Limiter` yang sudah ada bukan sekadar penghematan kode.
Versi Redis-nya membuat peredam itu berlaku LINTAS INSTANCE; dengan penghitung
per proses, dua instance akan membangunkan orang yang sama dua kali untuk
percakapan yang sama.

Diuji: lima pesan berturut-turut ke satu orang yang offline menghasilkan dua
kiriman dan tiga yang diredam (`chat_push_skipped_total{reason="debounce"} 3`).

### Antrean yang membuang, bukan menahan

Layanan push milik vendor browser bisa memakan ratusan milidetik, dan orang yang
menekan "kirim" tidak sedang menunggu itu. Delapan pekerja membaca dari antrean
sedalam 1024; kalau penuh, kabarnya **dibuang** dan dihitung di
`chat_push_dropped_total`.

Membuang terdengar salah sampai diingat apa yang dibuang: deringnya, bukan
pesannya. Pesannya sudah tersimpan, sudah disiarkan ke yang online, dan akan
terlihat begitu aplikasi dibuka.

Satu detail yang gampang terlewat: `select` dengan `default` melindungi dari
antrean PENUH, bukan dari antrean yang sudah DITUTUP. Mengirim ke channel
tertutup selalu panik, dan celahnya nyata — `Close` dipanggil saat instance
pamit, sedangkan permintaan HTTP yang sedang berjalan bisa saja baru sampai ke
baris `Enqueue`-nya saat itu juga. Panik di sana menjatuhkan seluruh proses di
tengah rolling deploy, persis saat paling banyak orang sedang dilayaninya. Jadi
ada `RWMutex`: banyak pengirim masuk bersamaan lewat `RLock`, dan hanya `Close`
yang mengambil kunci penuh.

Tiap kiriman juga punya anggaran waktunya sendiri, bukan berbagi satu anggaran
untuk seluruh kabar. Satu kabar bisa punya banyak penerima dan kirimannya
berjalan berurutan; dengan anggaran bersama, penerima terakhir gagal hanya karena
dia yang paling belakang antre.

### Langganan dicabut saat logout

Bukan sekadar kerapian. Di komputer yang dipakai bergantian, langganan yang
tertinggal membuat pratinjau pesan orang sebelumnya terus muncul di layar kunci
orang berikutnya — dan tidak ada apa pun di aplikasi yang akan mencabutnya nanti.
Urutannya: cabut dulu (yang menuntut sesi masih berlaku), baru logout.

### Langganan yang mati dibuang sendiri

Layanan push menjawab **404** atau **410** untuk langganan yang sudah tidak ada
— orangnya menghapus data situs, mencopot aplikasi, atau mencabut izin.
Mengabaikannya berarti mengirim ke alamat mati selamanya, jadi barisnya dihapus
saat itu juga.

`last_ok_at` hanya ditulis ulang kalau sudah lewat sejam. Yang ingin diketahui
adalah apakah langganan masih hidup HARI INI; menulis ulang barisnya di setiap
notifikasi membayar UPDATE untuk ketelitian sampai detik yang tidak pernah ada
yang membutuhkannya.

### Topic dan tag: satu percakapan, satu baris notifikasi

Dua mekanisme berbeda yang mengerjakan hal serupa di dua tempat:

- **`Topic`** (header RFC 8030, dikirim server) membuat layanan push MENGGANTI
  kabar lama yang belum sempat diantar dengan yang baru. Ponsel yang baru menyala
  setelah lama mati menampilkan kabar terakhir tiap percakapan, bukan
  menumpahkan seluruh riwayat dering.
- **`tag`** (di service worker) membuat notifikasi yang sudah tampil diganti,
  bukan ditumpuk.

Keduanya memakai id percakapan. `Topic` wajib base64url maksimal 32 karakter,
jadi tanda hubung UUID-nya dibuang — 32 karakter heksadesimal, pas.

### Kunci VAPID dibuat sekali, lalu tidak pernah diganti

```bash
cd server && go run ./cmd/vapid
```

Dipisah sebagai perintah tersendiri, bukan dibuat otomatis saat server start,
karena browser mengunci langganannya pada kunci publik yang dipakai saat
mendaftar. Kunci baru membuat SETIAP langganan yang sudah ada berhenti bekerja
diam-diam — tidak ada error, notifikasinya saja tidak pernah sampai lagi.
Server yang membuat kunci baru tiap restart adalah server yang notifikasinya
rusak setiap deploy.

Kunci publiknya diminta client ke `/api/push/config`, tidak ditulis di frontend.
Kunci adalah urusan deployment; kunci yang tertinggal di bundel frontend akan
jadi kunci yang salah begitu server diganti.

Config menolak start kalau hanya salah satu kunci terisi. Itu hampir selalu
berarti salah salin, dan akibatnya adalah fitur yang diam-diam mati padahal
terlihat dikonfigurasi.

### Endpoint langganan diperiksa, walaupun datang dari browser

```go
if !strings.HasPrefix(req.Endpoint, "https://") || len(req.Endpoint) > 1000 {
```

Endpoint itu akan jadi alamat yang DIHUBUNGI server ini nanti. Tanpa pembatasan
skema, sebuah "langganan" bisa dipakai menyuruh server memanggil alamat internal
— dan itu bukan lagi notifikasi, melainkan pemindai jaringan gratis.

Pencabutan dibatasi ke langganan milik pemanggil. Endpoint memang praktis tidak
bisa ditebak, tapi "praktis tidak bisa ditebak" adalah alasan yang berumur
pendek — satu kebocoran log sudah cukup untuk mematikan notifikasi orang lain.

### Service worker sengaja tidak menyentuh cache

Berkasnya hanya menangani `push` dan `notificationclick`. Chat realtime tidak
punya apa pun yang layak disajikan dari cache basi, dan service worker yang
menyajikan bundel lama adalah salah satu bug paling membingungkan yang bisa
dialami seseorang — halaman yang tidak mau berubah walaupun sudah di-reload
berkali-kali.

Mengklik notifikasi mengirim `postMessage` ke tab yang sudah ada, bukan
menavigasi. Aplikasi ini tidak punya rute per percakapan, dan navigasi akan
memuat ulang seluruh halaman beserta koneksi WebSocket-nya. Kalau tidak ada tab
sama sekali, barulah jendela baru dibuka dengan `?c=<id>`, dan parameter itu
dibuang dari URL setelah dipakai.

Izin notifikasi TIDAK pernah diminta otomatis saat halaman dibuka. Browser modern
menghukum situs yang melakukannya — Firefox menolaknya diam-diam, Chrome
menghitungnya sebagai gangguan. Semuanya berangkat dari satu tombol di sidebar,
dan tombol itu hanya muncul kalau browser mendukungnya DAN server menyalakannya.

---

## Metrik baru

```
chat_attachments_uploads_total{result}       ok | ditolak | gagal
chat_attachments_uploaded_bytes_total
chat_attachments_downloads_total{result}     ok | ditolak | hilang | gagal
chat_attachments_swept_total

chat_push_sent_total{result}                 ok | expired | error
chat_push_skipped_total{reason}              online | debounce
chat_push_dropped_total
chat_push_send_duration_seconds
```

Dua yang paling layak dipasangi alarm:

- `chat_attachments_downloads_total{result="hilang"}` naik berarti database dan
  penyimpanan sudah tidak sepakat — ada baris yang isinya tidak ada.
- `chat_push_dropped_total` naik berarti layanan push sedang lambat atau ada
  lonjakan yang jauh di atas perkiraan.

## Yang belum dikerjakan

- **Range request untuk lampiran.** Berkas dikirim utuh. Baru terasa kalau
  seseorang mengirim video panjang dan ingin melompat ke tengahnya.
- **Thumbnail.** Gambar dikirim dalam ukuran aslinya. Foto 12 megapiksel dari
  ponsel akan diunduh utuh untuk ditampilkan selebar 300 piksel.
- **Lampiran ikut terbuang saat percakapan dihapus.** `ON DELETE CASCADE`
  menghapus barisnya, tapi byte-nya tertinggal di penyimpanan — penyapu hanya
  tahu tentang baris. Belum ada jalur penghapusan percakapan di aplikasi ini,
  jadi belum ada kebocorannya; kalau fitur itu ditambahkan, ini harus ikut.

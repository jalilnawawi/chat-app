# Kelola akun & profil

Fase 10. Sampai Fase 9 sebuah akun hanya punya username, nama tampilan, dan
password. Tidak ada cara mengubah apa pun setelah mendaftar, tidak ada email,
dan tidak ada foto. Fase ini menutup itu.

Satu keputusan menaungi seluruhnya, dan kalau salah akan terasa di mana-mana.

---

## Status BUKAN presence

Presence — online/offline — sudah ada sejak Fase 6. Dia diturunkan dari koneksi
yang hidup, disimpan di Redis dengan TTL, dan sengaja **fana**: instance yang
mati tidak sempat membersihkan klaimnya, dan klaim itu tersapu sendiri dalam
satu TTL. Sifat itu yang membuatnya benar — presence memang tidak boleh
bertahan lebih lama dari koneksi yang menjadi sumbernya.

Status — "available", "busy", "sedang rapat sampai 13.00" — adalah pernyataan
yang dibuat orang **dengan sengaja**, dan dia harus bertahan melewati tutup
laptop, ganti perangkat, dan logout.

Menyatukan keduanya berarti status "busy sampai jam 1" yang baru saja seseorang
pasang lenyap begitu dia menutup tab. Jadi keduanya tinggal di tempat yang
berbeda:

| | presence | status |
|---|---|---|
| sumber kebenaran | koneksi yang hidup | baris di Postgres |
| tempat tinggal | Redis, ber-TTL | kolom di `users` |
| umur | selama koneksinya ada | sampai orangnya menggantinya |
| siapa yang mengubah | server, sendiri | orangnya, dengan sengaja |

Yang **sama** di antara keduanya: jalur penyebarannya. Status disiarkan ke
`ContactIDs` lewat `hub.Publish`, dan snapshot-nya dikirim saat koneksi dibuka —
persis seperti presence, lewat kode yang bentuknya sama persis. Tidak ada
mekanisme fan-out kedua yang harus ikut benar.

Client menampilkan **gabungan** keduanya sebagai satu titik, dengan aturan:
status yang dinyatakan orang selalu menang, presence mengisi sisanya. Itu bukan
pilihan sembarangan — seseorang yang memasang "sedang rapat" lalu tetap membuka
tabnya tidak sedang mengatakan "sapa saya". Kalau presence yang menang,
satu-satunya cara statusnya terlihat adalah dengan menutup aplikasinya.

### Durasi tidak dijaga timer

"10.00 – 13.00" disimpan sebagai `status_expires_at`, dan **tidak ada job yang
membersihkannya.** Pembacaan yang menyaring sendiri:

```sql
CASE WHEN status_expires_at IS NULL OR status_expires_at > now()
     THEN status ELSE 'available' END
```

Alasannya sama dengan typing indicator di Fase 3 yang sengaja tidak menyentuh
database: keadaan yang kedaluwarsa dengan sendirinya tidak butuh sesuatu yang
berjalan. Penyapu lintas instance justru menambah masalah — dia butuh
penguncian, dan tiap instance akan menyiarkan kabar kedaluwarsa yang sama.

Yang membuat ini aman adalah penyaringnya ditulis **satu kali**, di `userCols()`
di `internal/store/store.go`, dan setiap jalur baca memakai daftar kolom itu.
Tidak ada satu pun query yang menulis daftar kolom pengguna sendiri, jadi tidak
ada satu pun yang bisa lupa menyaringnya. Penyaring yang terlewat di satu jalur
tidak akan pernah mengeluh — dia cuma menampilkan "sedang rapat sampai 13.00"
pada pukul empat sore.

Waktunya disimpan sebagai `timestamptz`, dan **client yang merendernya ke jam
lokal**. "Sampai jam 13.00" di jam siapa adalah pertanyaan yang harus punya
jawaban sebelum baris pertama ditulis, dan jawabannya: jam orang yang
membacanya. Arah sebaliknya juga: pilihan cepat "sampai akhir hari" dihitung di
browser lalu dikirim sebagai instan absolut — hanya client yang tahu zona waktu
pemakainya, dan pukul 23.59 di Jakarta adalah tengah hari di tempat lain.

### `busy` meredam push

Inilah yang membuat status bukan sekadar hiasan: dia menyambung ke peredam
dering Fase 7. Di `push.Dispatcher.awake`, penerima yang sedang `busy` dilewati —
kecuali kalau namanya disebut.

Sebutan menembus **kedua** peredam, token bucket maupun status, dengan aturan
yang sama persis dengan keputusan Fase 9: dering biasa boleh diredam, panggilan
yang menyebut nama seseorang tidak. Orang yang memasang "sedang rapat" justru
yang paling butuh tahu kalau namanya dipanggil di tengah rapat itu.

Penyaringan presence diselesaikan **lebih dulu, sendiri**, sebelum status
ditanyakan sama sekali. Di percakapan dua orang yang keduanya sedang membuka
aplikasi — bentuk paling umum dari pesan yang dikirim — tidak ada seorang pun
yang tersisa setelah penyaringan itu, dan query status yang menyusul adalah
query yang jawabannya tidak akan pernah dibaca.

---

## Foto profil: alamatnya harus berubah setiap fotonya berubah

Fase 8 memasang `Cache-Control: private, max-age=31536000, immutable` pada semua
isi lampiran, dan itu benar untuk byte yang memang tidak pernah berubah.

**Avatar berubah.** Alamat tetap seperti `/api/users/{id}/avatar` berarti
browser menyimpan foto lama selama setahun dan tidak pernah lagi bertanya —
orang mengganti fotonya, dan tidak seorang pun melihatnya.

Jadi tiap unggahan menghasilkan id baru, dan alamatnya ikut berubah:
`/api/avatars/{avatar_id}`. Dengan begitu cache setahun kembali menjadi **benar**,
bukan menjadi jebakan. Byte di balik sebuah alamat memang tidak akan pernah
berubah, karena mengganti foto menghasilkan alamat yang lain sama sekali.

Konsekuensinya: alamat yang baru harus **sampai** ke orang lain. Event
`user.updated` disiarkan ke `ContactIDs` setiap kali nama atau foto berubah.
Tanpa itu, cache setahun jadi benar dan tidak berguna sekaligus — byte-nya tidak
basi, penunjuknya yang basi.

### Berkas aslinya dibuang

Yang tersimpan hanya hasil perkecilannya, paling besar 256 piksel pada sisi
terpanjang. Tidak ada yang butuh avatar dua belas megapiksel, dan menyimpannya
berarti membayar selamanya untuk sesuatu yang selalu ditampilkan selebar empat
puluh piksel.

Karena itu perkecilannya **wajib** di sini — berbeda dari turunan lampiran di
Fase 8, yang boleh tidak ada karena aslinya tetap tersimpan. Itu juga yang
membuat `imaging.Normalize` ada di samping `imaging.Make`: yang kedua menolak
bekerja untuk gambar yang sudah kecil, dan di jalur avatar penolakan itu berarti
tidak ada apa pun yang tersimpan. Penyandian ulang yang selalu terjadi itu
kebetulan juga membuang seluruh metadata yang menempel — termasuk koordinat GPS
di dalam EXIF sebuah foto, yang tidak pernah ada alasannya ikut terpasang
sebagai foto profil.

### Izin bacanya berbeda dari lampiran, dan itu ditulis eksplisit

Avatar boleh dilihat **siapa pun yang sudah login**. Orangnya memang sudah bisa
ditemukan lewat pencarian pengguna, lengkap dengan nama dan username-nya —
menyembunyikan fotonya di balik "harus satu percakapan dulu" tidak menutup apa
pun, dan justru membuat hasil pencarian tampil sebagai deretan huruf.

Ditulis eksplisit di `store.AvatarForRead` supaya tidak ada yang menyalin aturan
lampiran ke sini dan mengira itu kebetulan lebih aman. Yang tetap dijaga: tidak
ada jalan ke byte-nya selain lewat server ini, jadi sesi tetap syarat mutlak.

### Foto lama dititipkan ke penyapu, bukan dihapus di tempat

Penghapusan adalah satu permintaan HTTP ke penyimpanan yang bisa gagal sendiri,
dan kegagalan itu di tengah jalur "ganti foto" cuma menyisakan dua pilihan sama
buruknya: menggagalkan penggantian yang sebenarnya sudah berhasil, atau
menelannya diam-diam — dan dengan itu melupakan kuncinya selamanya, karena tidak
ada satu baris pun lagi yang menyebutnya.

Jadi kuncinya dicatat ke tabel `blob_garbage` di dalam transaksi yang sama, lalu
penyapu lampiran yatim yang sudah ada sejak Fase 7 yang membuangnya. Yang gagal
tetap tercatat dan dicoba lagi putaran berikutnya.

Barisnya dikunci (`SELECT ... FOR UPDATE`) selama penggantian. Tanpa itu, dua
unggahan yang datang hampir bersamaan dari dua tab akan sama-sama membaca kunci
lama, dan yang kalah balapan menitipkan kunci yang **sudah terpasang** sebagai
sampah — foto yang baru saja dipasang lalu dibuang penyapu beberapa menit
kemudian.

---

## Email & password

### Mengubah kredensial menuntut password saat ini

Bukan sekadar sesi yang masih hidup. Sesi bisa saja milik laptop yang ditinggal
terbuka di meja, dan orang yang lewat lalu mengganti password akan mengunci
pemiliknya keluar dari akunnya sendiri.

Berlaku untuk ganti password **dan** ganti email: alamat email adalah jalan
masuk kedua ke sebuah akun, jadi memasangnya adalah perubahan kredensial — bukan
sekadar mengisi kolom profil.

### Email yang belum terverifikasi tidak bisa memulihkan akun sama sekali

Kalau bisa, memulihkan akun orang lain cuma butuh mendaftar dengan alamat mereka
dan tidak pernah membuktikan apa pun. `UserByVerifiedEmail` adalah satu-satunya
jalan dari alamat ke pemiliknya, dan dia menuntut `email_verified_at IS NOT NULL`.

Dua aturan yang menjaganya tetap benar:

1. **Mengganti alamat menurunkan keadaan verifikasinya.** Tanpa itu, seseorang
   bisa memverifikasi alamatnya sendiri lalu menggantinya dengan alamat orang
   lain yang ikut terbawa "terverifikasi".
2. **Tautan yang dikirim ke alamat A tidak berlaku untuk alamat B.** Alamat
   tujuannya disalin ke dalam baris tokennya dan dicocokkan lagi saat tautannya
   diklik. Tanpa itu, membuktikan kepemilikan sebuah alamat berubah jadi
   membuktikan kepemilikan alamat apa pun yang diketik sesudahnya.

Jawaban `/api/auth/forgot-password` **selalu sama** — alamat yang tidak
terdaftar, yang terdaftar tapi belum terverifikasi, dan yang suratnya
benar-benar dikirim menghasilkan 200 yang identik. Membedakannya mengubah
endpoint ini jadi alat untuk menanyai server "apakah orang ini punya akun di
sini".

### Ganti password mencabut semua sesi lain — dan benar-benar memutus koneksinya

Ini bagian yang paling mudah dikerjakan setengah.

Menghapus baris di `sessions` saja **tidak cukup**. Koneksi WebSocket yang sudah
terlanjur terbuka dipegang di memori proses, dan sejak Fase 6 proses itu bisa
instance **lain**. Baris sesi yang hilang sementara koneksinya tetap hidup
berarti orang yang password-nya baru saja dicuri tetap terhubung, melihat setiap
pesan yang masuk, sampai dia sendiri yang memutuskan untuk reconnect.

Jadi `hub.Broadcaster` dapat dua method baru:

```go
RevokeSessionsExcept(userID uuid.UUID, keep []byte)  // ganti password
RevokeSession(userID uuid.UUID, hash []byte)         // cabut satu perangkat
```

Dua method, bukan satu dengan bendera. Penyaringnya memang berlawanan — yang
satu menyisakan satu sesi, yang satu menutup satu sesi — dan bendera yang salah
di sini berarti mengeluarkan orang dari semua perangkatnya saat dia cuma ingin
mengeluarkan satu.

Tiga hal yang menopangnya:

- **`hub.Sink` tahu hash sesinya.** `UserID` saja tidak cukup: mencabut satu
  perangkat tanpa itu berarti mengeluarkan orangnya dari semua perangkatnya
  sekaligus, termasuk yang sedang dia pakai menekan tombolnya.

- **Channel kendali terpisah di Redis** (`chat:ctl:<uuid>`, di samping
  `chat:u:<uuid>`). Keduanya bisa saja berbagi satu channel dan dibedakan dengan
  membaca isinya, tapi itu berarti setiap pesan chat yang lewat harus di-decode
  dulu hanya untuk memastikan dia bukan perintah — biaya yang dibayar jutaan
  kali demi sesuatu yang terjadi beberapa kali sehari. Dengan channel terpisah,
  Redis yang menyaring, dan pesan biasa tetap diteruskan ke socket sebagai byte
  mentah tanpa pernah disentuh.

- **`Sink.Kick`, bukan `Enqueue` lalu `Close`.** Yang kedua tampak sama tapi
  tidak: `Close` membatalkan konteks tulis seketika, dan pesan yang baru
  diantrekan kalah balapan dengan pembatalan itu lebih sering daripada tidak.
  Yang hilang justru satu-satunya keterangan yang dimiliki orang tentang kenapa
  aplikasinya tiba-tiba mengeluarkan dia.

Pemulihan password mencabut **semua** sesi tanpa kecuali — orang yang sampai ke
jalur itu tidak sedang memegang sesi mana pun yang layak dipercaya. Ganti
password menyisakan satu: yang sedang dipakai menekan tombolnya.

Ganti password juga membuang tautan pemulihan yang masih menggantung. Orang yang
baru saja berhasil mengganti password-nya sendiri sudah tidak butuh tautan
pemulihan, dan tautan yang masih berlaku di kotak masuk adalah jalan masuk kedua
yang tidak diminta siapa pun.

### SMTP mengikuti pola yang sama dengan Redis dan SeaweedFS

`SMTP_URL` kosong berarti fitur email mati dan aplikasinya tetap utuh: orang
masih bisa mendaftar, login, dan memakai seluruh chat. Yang hilang cuma
verifikasi alamat dan pemulihan password lewat tautan. `mail.Sender` nil adalah
bentuk "dimatikan", dan setiap method-nya aman dipanggil pada nilai nil — supaya
tidak ada pemanggil yang perlu menulis `if mailer != nil`, pemeriksaan yang
selalu ada satu yang terlupa.

Pengirimannya tidak pernah menahan permintaan HTTP: antrean dengan dua pekerja,
dan kiriman yang dibuang kalau antreannya penuh — bentuk yang sama dengan paket
`push`, karena masalahnya memang sama.

Isinya teks biasa. Tidak ada HTML, tidak ada gambar, tidak ada pelacak. Yang
perlu sampai cuma satu tautan, dan surat yang isinya satu tautan yang bisa
dibaca mata telanjang justru yang paling sulit dipalsukan bentuknya oleh orang
lain.

### Daftar sesi aktif

`sessions` dapat tiga kolom: `id` (nama publik), `user_agent` (keterangan
perangkat, disalin saat sesinya lahir), dan `last_seen_at`.

`id` ada supaya sebuah sesi punya nama yang boleh disebut di URL. Primary key
tabel itu adalah hash token-nya, dan hash itu memang tidak bisa dikembalikan
jadi token — tapi memakainya sebagai handle publik berarti daftar sesi seseorang
membocorkan bahan yang persis dipakai untuk mencari sesi di database.

`last_seen_at` dimajukan **paling sering sekali tiap lima menit**, lewat CTE di
dalam query pemeriksaan sesi. Dijalankan pada setiap permintaan tanpa syarat
itu, dia akan mengubah jalur terpanas aplikasi menjadi jalur tulis, dan
ketelitian sampai detik yang dibelinya tidak pernah ada yang membutuhkannya.
Pola yang sama dengan `MarkSubscriptionDelivered` di Fase 7.

---

## Yang berubah di bentuk data

`store.User` dipisah dari `store.Me`, dan itu bukan kerapian. `User` ikut di
hasil pencarian pengguna, di `peer` pada daftar percakapan, dan di setiap siaran
yang menyebut seseorang — tiga jalur yang tidak satu pun punya alasan membawa
alamat email siapa pun, dan tiga jalur yang akan diam-diam membawanya begitu
suatu hari ada yang menambahkan satu field ke struct yang salah.

`Me` adalah satu-satunya bentuk yang membawa email, dan satu-satunya yang pernah
dikirim ke pemiliknya saja. Karena itu `UserByUsername` mengembalikan `Me`:
pemanggilnya satu-satunya adalah jalur login, dan jawaban login adalah jawaban
kepada pemiliknya sendiri.

`Member` dapat `avatarUrl` tapi **tidak** status. Anggota sebuah percakapan
menurut definisi berbagi percakapan dengan pembacanya, jadi mereka sudah
termasuk kontak — dan status kontak sudah datang lewat snapshot saat koneksi
dibuka lalu tetap segar lewat siaran. Menyalinnya ke sini juga berarti dua
sumber untuk satu jawaban, dan yang satu ini membeku pada saat daftarnya
diambil.

---

## Verifikasi

- `go vet` + `go test -race ./...` bersih; 22 test store baru, 4 test hub untuk
  pencabutan sesi, dan 3 test push untuk peredam status.
- `tsc --noEmit` + `vite build` bersih.
- Uji HTTP langsung, 48/48: seluruh penolakan masukan, alamat yang sudah dipakai
  akun lain (409), token sekali pakai, jawaban pemulihan yang seragam, dan
  tautan pemulihan lama yang mati setelah password diganti.
- Uji avatar langsung, 20/20: foto 662 KB jadi 22 KB, alamatnya berubah tiap
  penggantian, alamat lama jadi 404, header cache dan keamanannya, dan izin baca
  yang memang lebih longgar dari lampiran.
- Uji lintas instance dengan Redis, 22/22: status yang disiarkan dari instance A
  sampai ke koneksi di instance B, dan **ganti password di instance A menutup
  koneksi di instance B** — beserta kebalikannya, mencabut satu perangkat tanpa
  menyentuh yang lain.
- Verifikasi browser (Brave, konteks terpisah, puppeteer-core): 36/36 lulus
  dalam tiga putaran.

Penyapu sampah penyimpanan diuji langsung terhadap SeaweedFS: enam kunci avatar
lama diambil dari `blob_garbage`, byte-nya ada sebelum disapu dan hilang
sesudahnya.

---

## Susulan: tombol yang pasti ditolak tidak pernah ditampilkan

Ditemukan dengan memakainya, bukan dengan membacanya: menekan "Ganti foto" di
server tanpa `SEAWEED_FILER_URL` menghasilkan

```json
{"error":"foto profil tidak aktif di server ini"}
```

Servernya benar — avatar menumpang `blob.Store` yang sama dengan lampiran, dan
tanpa penyimpanan tidak ada tempat menaruh byte-nya. Yang salah adalah
**menawarkan tombolnya sama sekali.**

Itu melanggar aturan yang project ini tulis sendiri di panel kelola grup:

> Tombol yang tidak boleh ditekan seseorang tidak ditampilkan kepadanya, bukan
> ditampilkan lalu ditolak server.

Tombol notifikasi di sidebar sudah menghormatinya sejak Fase 7 (`push.supported
&& push.available`). Tombol lampiran tidak — lubang yang sama, cuma lebih tua —
karena client tidak pernah menanyakan kemampuan server selain untuk push.

**`GET /api/config`** menutup keduanya sekaligus:

```json
{"attachments":true,"thumbnails":true,"avatars":true,"push":true,"mail":true,"vapidPublicKey":"..."}
```

Empat keputusan kecil di dalamnya:

- **Tanpa sesi.** Halaman masuk sudah membutuhkannya sebelum siapa pun login,
  untuk memutuskan apakah "Lupa password?" pantas ditawarkan. Isinya juga bukan
  rahasia baru: `/healthz` sudah melaporkan hal yang sama, di port yang sama,
  sejak Fase 7.

- **Satu sumber, dua pembaca.** `/healthz` dan `/api/config` sama-sama membaca
  `Server.features()`. Dua daftar yang disusun sendiri-sendiri adalah dua daftar
  yang suatu hari akan berbeda pendapat tentang apa yang sedang menyala. Kunci
  publik VAPID ditambahkan di atasnya, dan hanya di `/api/config` — `/healthz`
  tidak membawanya.

- **`avatars` bukan sekadar salinan `attachments`.** Dia menuntut penyimpanan
  **dan** pengolahan gambar, karena berkas aslinya dibuang setelah diperkecil —
  tanpa salah satunya tidak ada yang bisa disimpan sama sekali.

- **`/api/push/config` dilebur ke sini.** Endpoint terpisah untuk satu fitur
  berarti client harus tahu lebih dulu fitur mana yang punya endpoint sendiri,
  dan itu daftar yang tumbuh tiap fase.

Di client, `config` yang masih `null` berarti **mati**. Menampilkan tombol lalu
menariknya kembali sepersekian detik kemudian lebih buruk daripada
menampilkannya sedikit terlambat.

Yang ikut ditutup, dan ini bagian yang paling mudah terlewat: **ketiga jalan
masuk berkas**, bukan tombolnya saja. Menyeret berkas ke jendela dan menempel
tangkapan layar tidak pernah melewati tombol lampiran, dan keduanya akan
berakhir sebagai unggahan yang ditolak server tanpa pernah ada yang
menawarkannya. Ketiganya sekarang lewat satu gerbang di `ChatPanel.pick`.

Untuk email, seksinya tidak dihilangkan melainkan diganti: alamat yang sudah
tercatat tetap ditampilkan — dia milik orangnya — dengan satu baris yang
mengatakan verifikasi sedang tidak aktif. Menyembunyikan data seseorang dari
dirinya sendiri karena server sedang tidak bisa mengirim surat adalah jawaban
yang salah untuk masalah yang benar.

Diverifikasi dengan menjalankan **dua server berdampingan** — satu penuh, satu
telanjang seperti `go run ./cmd/server` tanpa satu pun variabel lingkungan —
lalu membandingkan apa yang terlihat di keduanya: 15/15. Yang diuji bukan
"tombolnya ada", melainkan "tombolnya mengikuti servernya", dan itu hanya bisa
dibuktikan dengan dua server sekaligus. Ditambah regresi 10/10 untuk jalur yang
bisa rusak karenanya: kiriman lampiran, unggahan avatar, tombol notifikasi yang
kunci VAPID-nya pindah alamat, dan verifikasi email.

---

## Tiga hal yang ditemukan OLEH menjalankannya

Ketiganya lolos `go vet`, lolos `go test`, dan lolos `tsc`.

1. **`/api/auth/login` dan `/register` mengembalikan bentuk yang salah.**
   Keduanya menjawab dengan `store.User`, sedangkan `/api/auth/me` menjawab
   dengan `store.Me` — dan client menyimpan keduanya di tempat yang sama. Yang
   baru saja masuk karenanya kehilangan keadaan verifikasi email-nya sampai
   halamannya dimuat ulang. Ketahuan dari satu pemeriksaan yang menanyakan
   `emailVerified` pada jawaban register.

2. **Pemulihan password memakai kuota yang salah.** Dia dipasang pada kuota
   `auth` per-IP bersama login dan register, padahal yang perlu dibatasi di sana
   bukan biaya argon2 melainkan kemampuan seseorang membanjiri kotak masuk orang
   lain dengan tautan yang tidak mereka minta. `rateLimitByIP` sekarang menerima
   aturannya sebagai parameter, dan pemulihan memakai kuota `email`.

3. **Query lawan bicara di `ListConversations` salah panjang.** `userCols()`
   dipasang di subquery `LEFT JOIN LATERAL`, tapi daftar kolom di SELECT luarnya
   masih menyebut empat kolom lama — dan ekspresi `CASE` tanpa alias tidak punya
   nama kolom sama sekali, jadi tidak bisa dirujuk dari luar. Ketahuan sebagai
   `number of field descriptions must equal number of destinations` di test
   store yang sudah ada sejak Fase 9, bukan di jalur yang baru ditulis.

---

## Yang sengaja TIDAK dikerjakan di fase ini

- **Hapus akun dan blokir pengguna.** Keduanya menyentuh riwayat orang lain —
  pesan yang sudah dikirim ada di percakapan yang bukan milik pengirimnya — dan
  jawaban atas "apa yang terjadi pada pesan lama" adalah keputusan tersendiri
  yang layak ditulis saat mengerjakannya. Tetap di daftar "sebelum aplikasi ini
  boleh dipakai orang".
- **Ganti username.** Username adalah cara orang lain menemukan seseorang, dan
  nama yang berpindah tangan berarti pesan lama yang menyebut seseorang suatu
  hari menunjuk orang yang berbeda. Nama tampilan tidak punya masalah itu: dia
  tidak pernah jadi kunci apa pun — bahkan sebutan sudah memakai id sejak Fase
  9, justru supaya siapa yang dipanggil tidak ditentukan oleh cara sebuah string
  ditulis.
- **Otentikasi dua faktor.** Dia menuntut jalur pemulihan keduanya sendiri —
  kode cadangan — dan jalur pemulihan yang setengah jadi lebih berbahaya
  daripada tidak ada 2FA sama sekali.

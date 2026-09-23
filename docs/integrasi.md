# Integrasi pihak ketiga: aplikasi, bot, dan mini-app

Dokumen ini **mencatat keputusan, bukan pekerjaan yang sudah selesai.** Sampai
Fase 15 tidak ada satu pun jalur integrasi di aplikasi ini: tidak ada bot, tidak
ada webhook, tidak ada token mesin, tidak ada identitas selain manusia yang login
lewat cookie. Yang ditulis di sini adalah bentuk yang akan dipakai kalau jalur itu
suatu hari dibuka, beserta alasan kenapa bentuk lain ditolak.

Alasan menulisnya sekarang, sebelum sebarisnya dikerjakan, adalah Prinsip Produk
nomor 5: *keputusan yang mahal ditambahkan belakangan dikerjakan di awal.* Satu
keputusan di bawah ini memang seperti itu, dan sisanya menggantung padanya.

| Bagian | Isi |
|---|---|
| Kasus | HRIS di dalam chat: sisa cuti, ajukan cuti, ajukan izin, profil |
| Yang dihapus self-hosting | tenant, app store, alur persetujuan OAuth |
| Seam A | identitas aplikasi — app adalah baris `users` |
| Seam B | masuk: app mengirim pesan, lewat endpoint yang sudah ada |
| Seam C | mini-app: halaman penuh di kolom utama, bukan percakapan |
| Seam D | keluar: app menyambung ke `/ws`, bukan webhook |
| Batas privasi | app hanya menerima yang ditujukan kepadanya |
| Ditunda | slash command, kartu interaktif, webhook, pesan ephemeral |

---

## Kasus yang dilayani

Rujukannya konkret: SeaTalk di Shopee, tempat HRIS perusahaan disambungkan ke
aplikasi chat yang sama yang dipakai untuk DM dan grup. Dari dalam chat orang bisa
melihat sisa cutinya, mengajukan cuti, mengajukan izin, dan membuka profil HRIS-nya
— tanpa membuka aplikasi kedua.

Yang penting dari contoh itu adalah **bentuk teknisnya**, dan bentuk itu bukan yang
biasanya dibayangkan orang saat mendengar "integrasi chat".

**HRIS-nya tidak dioperasikan lewat percakapan.** Dia bukan bot gaya Telegram,
tempat orang mengetik perintah kepada sebuah lawan bicara dan dijawab dengan pesan.
Yang ada adalah **satu halaman penuh milik HRIS, berdiri sejajar dengan percakapan
di dalam aplikasi chat**: orang mengklik HRIS di rel kiri, halamannya terbuka di
kolom utama, dengan daftar dan formulirnya sendiri. Kalimatnya: orang mengoperasikan
HRIS **melalui** aplikasi chat, bukan mengoperasikan HRIS **di dalam sebuah
percakapan**.

Perbedaan itu menentukan hampir seluruh dokumen ini, dan dia juga yang menjelaskan
kenapa Seam C — bukan Seam B — adalah bagian yang menjawab kasusnya. Aplikasi chat
di sini adalah **wadah**. Yang menggambar cuti dan menghitung sisanya tetap HRIS.
Chat menyediakan empat hal:

1. tempat berdiri di permukaan yang sama dengan percakapan — satu klik, bukan
   aplikasi kedua,
2. jawaban atas pertanyaan "ini siapa" tanpa login kedua,
3. jalur untuk memberi tahu orangnya saat ada yang berubah,
4. percakapan biasa, kalau memang ada yang perlu dibicarakan tentang perubahan itu.

Bot-nya tetap ada, tetapi dia bagian yang paling kecil: dia mengirim "pengajuan
cutimu disetujui" beserta tautan yang membuka halamannya. Dia bukan antarmukanya.

Keempatnya jauh lebih murah daripada membangun bahasa kartu sendiri, dan keempatnya
sudah menutup hampir seluruh kegunaan yang benar-benar dipakai orang setiap hari.
Kartu interaktif ditunda karena itu, bukan karena dia tidak bagus.

## Apa yang dihapus oleh self-hosting

Aplikasi ini di-host sendiri oleh perusahaan yang memakainya. Satu deployment sama
dengan satu perusahaan. Konsekuensinya jarang disadari, dan dia menghapus bagian
terbesar dari kerumitan yang biasa menempel pada "platform aplikasi":

- **Tidak ada tabel organisasi atau tenant.** Tidak ada yang perlu dipisahkan;
  seluruh basis datanya sudah milik satu perusahaan.
- **Tidak ada app store, tidak ada direktori, tidak ada penerbitan.** Aplikasi
  dipasang oleh admin perusahaan itu sendiri, untuk perusahaan itu sendiri.
- **Tidak ada alur persetujuan OAuth.** Layar "aplikasi X meminta izin Y" ada untuk
  melindungi pengguna dari aplikasi yang dipilih orang lain di internet. Di sini
  yang memasang dan yang dilindungi berada di satu perusahaan, dan izinnya
  diputuskan saat pemasangan, bukan saat pemakaian.
- **Tidak ada rotasi kunci publik, discovery document, atau JWKS.** Kedua sisi
  dikelola tim IT yang sama.

Catatan ini ditulis supaya tidak ada yang menambahkannya kembali "supaya standar".
Yang membuat Slack membutuhkan semua itu adalah multi-tenancy, dan aplikasi ini
tidak punya multi-tenancy.

---

## Seam A — identitas aplikasi

> **Sebuah aplikasi adalah sebuah baris di `users`.**

Ini keputusan intinya, dan satu-satunya yang mahal kalau ditunda.

Setiap aplikasi mendapat baris `users` dengan `users.kind = 'app'`, ditambah tabel
`apps` untuk hal-hal yang hanya dimiliki aplikasi: alamat mini-app, siapa yang
memasangnya, kapan dimatikan. Alasannya sama persis dengan alasan `direct` dan
`group` disatukan menjadi satu `conversation` sejak Fase 1: `messages.sender_id`
tetap `NOT NULL REFERENCES users(id)`, dan keanggotaan, alokasi `seq` yang gapless,
read receipt, pencarian, jendela riwayat, resume, soft delete, reaksi, dan sematan
semuanya ikut bekerja **tanpa satu baris kode baru.** Sebuah bot yang berbicara di
grup adalah anggota grup yang mengirim pesan, dan itu jalur yang sudah dilalui
ratusan test.

Dua bentuk lain ditolak:

- **`sender_id` dibuat nullable, aplikasinya ditunjuk kolom lain.** Ini memaksa
  setiap pembaca pesan memeriksa dua kemungkinan pengirim, selamanya. `scanMessage`
  dan `messageDest` sudah panjang, dan setiap jalur yang menampilkan nama pengirim
  — riwayat, kutipan balasan, hasil pencarian, isi notifikasi push — akan tumbuh
  satu cabang yang gampang lupa ditulis.
- **Tabel `bots` terpisah dengan foreign key sendiri di `messages`.** Menghasilkan
  dua kolom pengirim yang tidak boleh terisi keduanya dan tidak boleh kosong
  keduanya: sebuah invariant yang harus dijaga CHECK dan diingat di setiap tulisan.
  Tabel `conversations` sudah memikul satu invariant seperti itu untuk
  `type`/`direct_key`/`title`, dan satu sudah cukup.

Harga dari keputusan ini harus disebut jujur: begitu aplikasi adalah pengguna,
**setiap jalur yang menampilkan daftar pengguna harus ingat menyaringnya.** Tiga
yang sudah terlihat sekarang: `SearchUsers` (pencarian orang), pemilih anggota di
panel grup, dan `ContactIDs` yang menentukan siapa menerima siaran presence. Sebuah
aplikasi tidak "online" dan tidak "sedang mengetik"; dia tersedia atau tidak.

### Kredensial

Tabel `app_tokens` menyimpan **hash** token, bukan tokennya, memakai ulang
`auth.NewToken` dan `auth.HashToken` — jalur yang sama dengan `sessions` sejak
Fase 2 dan `email_tokens` sejak Fase 10. Tidak ada skema rahasia baru yang perlu
ditinjau: yang bocor dari basis data tetap tidak bisa dipakai masuk.

`requireAuth` ditambah satu cabang yang menerima `Authorization: Bearer`.

Ini perlu catatan, karena tanpanya orang berikutnya akan mengira ada kontradiksi.
Komentar di `middleware.go` menolak token dan memilih cookie httpOnly, dengan dua
alasan: script berbahaya tidak bisa membaca cookie httpOnly, dan WebSocket API di
browser tidak mengizinkan custom header sehingga token akan terpaksa dititipkan di
query string. **Kedua alasan itu tentang browser.** Sebuah aplikasi HRIS yang
berbicara dari servernya sendiri tidak punya XSS untuk dikhawatirkan dan bisa
menyetel header pada handshake WebSocket dengan bebas. Keputusan lama tidak
dibatalkan; dia memang tidak pernah berbicara tentang kasus ini.

### Scope

Token membawa daftar izin, dan izinnya dipisah lebih halus daripada yang terlihat
perlu:

| Scope | Artinya |
|---|---|
| `chat:write` | mengirim pesan ke percakapan yang diikutinya |
| `chat:read` | menerima **seluruh** isi percakapan yang diikutinya |
| `user:identity` | menukar tiket menjadi nama dan id pengguna |
| `user:email` | ikut menerima alamat email pada penukaran tiket |
| `commands` | mendaftarkan dan menerima perintah |

`user:email` sengaja dipisah dari `user:identity`. `store.User` tidak membawa email
dan `store.Me` membawanya, dan komentar di `models.go` menyebut pemisahan itu bukan
kerapian melainkan penjagaan: tiga jalur yang menyebut seseorang tidak punya alasan
membawa alamatnya. Pintu baru ke pihak ketiga adalah tempat paling mudah pemisahan
itu bocor, jadi dia dijaga oleh scope-nya sendiri.

`chat:read` berdiri sendiri karena dia yang mengubah sifat sebuah aplikasi — lihat
bagian batas privasi.

### Kuota

Satu kind baru di daftar aturan `config.go`, sejajar dengan `MessageRate` dan
kawan-kawan, dihitung per aplikasi. Sebuah aplikasi yang mengulang kesalahan tidak
boleh menghabiskan kuota orang, dan sebuah aplikasi yang rusak tidak boleh
membanjiri percakapan lebih cepat daripada yang bisa dibaca manusia.

---

## Seam B — masuk: aplikasi mengirim pesan

Nilai terbesar **per baris kode** — hampir seluruhnya sudah ada. Bukan bagian yang
terbesar; itu Seam C.

"Pengajuan cutimu disetujui" yang masuk ke DM adalah `POST /api/conversations/{id}/messages`
dengan token aplikasi. Endpoint-nya sudah ada, validasinya sudah ada, batas 4000
karakternya sudah ada. Yang kurang hanya cabang auth dan pemeriksaan scope
`chat:write`.

Satu hal yang sudah benar tanpa dikerjakan: id pesan dibuat di sisi pengirim
(`client_msg_id` sejak Fase 2), jadi HRIS yang mengulang kiriman karena jaringannya
putus mendapat pesan yang sama, bukan pesan kedua. Janji "tidak boleh ganda" berlaku
untuk aplikasi tanpa perlakuan khusus.

---

## Seam C — mini-app: halaman penuh milik aplikasinya

Ini bagian yang menjawab kasus di atas, dan **dia yang terbesar.** Kalau hanya satu
seam yang boleh dikerjakan, ini yang dikerjakan.

Keputusannya dua lapis, dan yang pertama lebih sering salah dibaca daripada yang
kedua.

**Halaman, bukan dialog.** Mini-app menempati **kolom utama** — tempat `ChatPanel`
berdiri sekarang — dan diluncurkan dari rel di `Sidebar`, sejajar dengan daftar
percakapan. Dia bukan panel kanan, dan dia tidak memakai `PanelShell`. `PanelShell`
adalah kolom 320–384px yang berubah menjadi `role="dialog"` di bawah 1024px; sebuah
formulir pengajuan cuti tidak muat di dalamnya, dan sebuah halaman yang dipakai lima
menit tidak boleh menjadi dialog yang harus ditutup lebih dulu sebelum hal lain bisa
dikerjakan. Aplikasi bukan tamu di atas percakapan; dia penghuni kolom yang sama.

**Iframe, bukan tab baru.** Alasannya pengalaman — berpindah tab berarti keluar dari
chat, dan seluruh gunanya justru supaya tidak perlu keluar.

### Satu kolom, dua penghuni

Yang menentukan isi kolom utama sekarang adalah `activeId`. Begitu ada mini-app,
kolom itu punya dua jenis penghuni, dan pilihannya sama dengan pilihan di Seam A:
satu nilai, bukan dua saklar.

```ts
type Aktif = { jenis: 'percakapan'; id: string } | { jenis: 'app'; id: string } | null;
```

Alasannya sudah tertulis di `App.tsx` untuk panel kanan: keadaan "dua terbuka
sekaligus" tidak perlu bisa diwakili, dan yang tidak bisa diwakili tidak perlu
dijaga supaya tidak terjadi. Dua saklar terpisah akan melahirkan pertanyaan "mana
yang menang" di setiap tempat yang membuka sesuatu — notifikasi yang diklik, `?c=`
pada URL, pesan dari service worker.

Aturan layar sempit ikut apa adanya: di bawah `lg`, sidebar dan kolom utama
bergantian, dan `hiddenOnMobile` hanya berganti dari `activeId !== null` menjadi
`aktif !== null`.

Konsekuensi di basis data: `apps` membawa **nama, ikon, dan alamat** mini-app. Rel
membutuhkan ketiganya, dan sebuah aplikasi tanpa ikon akan menjadi kotak kosong di
tempat yang paling terlihat di seluruh aplikasi.

### Harga sebuah iframe

Harganya nyata dan harus ditulis, karena dia yang akan mengejutkan orang yang
mengerjakannya:

- **`frame-ancestors` dipegang aplikasinya, bukan kita.** Halaman HRIS harus
  mengizinkan origin deployment ini di `Content-Security-Policy`-nya sendiri. Ini
  satu-satunya bagian dari seluruh dokumen ini yang tidak bisa dikerjakan dari sisi
  kita, dan karena itu dia harus masuk ke dokumen yang diberikan ke pihak ketiga,
  bukan hanya ke sini.
- **Jembatan `postMessage` yang berversi sejak hari pertama.** Bentuknya
  `{v: 1, type, ...}` dengan tipe `ready`, `resize`, `theme`, `close`, dan `notify`.
  `close` berarti "selesai, kembalikan aku ke percakapan terakhir" — bukan menutup
  sebuah dialog, karena tidak ada dialog.
  Origin diperiksa terhadap origin yang terdaftar di `apps`, di **kedua** arah, dan
  tidak pernah `'*'`. Versi ditaruh di awal karena protokol tanpa versi hanya bisa
  diubah dengan memutus aplikasi yang sudah jalan.
- **Tema diserahkan, tidak ditebak.** Kirim `terang` atau `gelap` saat `ready` dan
  setiap kali orangnya berganti tema, sumbernya `tema.ts`. Tanpa ini mini-app akan
  selalu terang di dalam chat yang gelap, dan itu terlihat rusak bukan terlihat
  berbeda.
- **`sandbox` pada iframe-nya.** `allow-scripts` bersama `allow-same-origin` hanya
  aman karena aplikasinya berada di origin lain; kalau suatu hari ada mini-app yang
  dilayani dari origin yang sama, kombinasi itu membatalkan sandbox-nya.

### Aksesibilitas

Karena mini-app bukan dialog, batas yang paling sering disebut orang tentang iframe
— "fokus yang sudah masuk ke dalam tidak bisa dikurung dari luar" — ikut hilang
bersama keputusan itu. Tidak ada yang perlu dikurung. Yang tersisa tiga hal, dan
ketiganya harus dikerjakan:

- **`<iframe title>` menyebut nama aplikasinya.** Tanpa itu pembaca layar hanya
  mengumumkan "frame".
- **Jalan keluar dari dalam.** Tab yang sudah masuk berjalan sampai halaman HRIS
  habis sebelum kembali ke aplikasi ini; itu perilaku iframe yang benar, bukan
  jebakan, tetapi orang yang salah masuk tidak bisa mengandalkan Shift+Tab lewat
  halaman yang bukan kita gambar. Rel kiri harus bisa dicapai lewat tautan lewati
  di awal halaman.
- **Escape tidak dipakai untuk menutup.** Halaman bukan dialog, dan Escape yang
  dicegat dari luar akan mencuri Escape milik dialog di dalam halaman HRIS.

Yang tidak bisa dikerjakan dari sisi kita: isi halaman HRIS-nya. Target aplikasi ini
WCAG 2.2 AA, dan sebuah iframe tidak menularkan kepatuhan. Syaratnya ikut di dokumen
untuk pihak ketiga, bersama `frame-ancestors`.

### Identitas: tiket sekali pakai

Orang mengklik "HRIS" di rel dan halamannya terbuka sudah dalam keadaan login.
Caranya:

```
POST /api/apps/{id}/ticket     (cookie sesi)   → { ticket, expiresIn: 60 }
POST /api/apps/ticket/redeem   (token app)     → { userId, username, displayName }
```

Tabel `app_tickets` menyimpan hash tiketnya, bukan tiketnya, dengan umur 60 detik
dan hangus begitu ditukar. Penukarannya lewat back-channel dari server HRIS,
sehingga yang pernah lewat URL hanyalah sebuah nilai sekali pakai yang berumur satu
menit — bukan identitas, bukan kredensial yang bisa diulang.

`redeem` mengembalikan `store.User`. Email hanya ikut kalau token aplikasinya punya
scope `user:email`.

OIDC penuh ditolak. Perbandingannya: satu tabel dan dua endpoint, melawan discovery
document, JWKS, rotasi kunci, alur consent, dan sebuah spesifikasi yang seluruh
kegunaannya adalah menyambungkan dua pihak yang **tidak** saling mengenal. Di sini
tim IT yang sama memasang kedua sisinya. Kalau suatu hari ada aplikasi pihak ketiga
sungguhan yang menuntut OIDC, tiket ini tetap bisa hidup di belakangnya.

---

## Seam D — keluar: aplikasi menerima kejadian

Aplikasi menyambung ke `/ws` yang sudah ada, sebagai klien, memakai token-nya.
Bukan webhook.

Yang menentukan pilihan ini bukan kesederhanaannya, melainkan sesuatu yang sudah
jadi: **resume.** Sejak Fase 3, klien yang tersambung mengirim `sync` berisi kursor
`seq` per percakapan, dan server mengirimkan kembali semua yang terlewat. Itu persis
masalah "aplikasi HRIS mati sepuluh menit, jangan sampai ada pengajuan yang hilang"
— masalah yang di tempat lain memaksa lahirnya tabel outbox, worker retry, dan
dead-letter queue. Di sini dia sudah diuji dan sudah dipakai setiap kali ada orang
menutup laptop.

Perlu diperiksa saat dikerjakan: `maxResume` 200 pesan dipilih untuk sebuah tab
browser, dan aplikasi yang mati berjam-jam mungkin menuntut batas yang lain.

Webhook ditunda sampai ada aplikasi yang benar-benar tidak bisa memelihara koneksi
keluar. Kalau hari itu tiba, dia butuh tabel outbox, worker dengan backoff, dan
tanda tangan HMAC atas timestamp dan badan pesan — dan ketiganya adalah pekerjaan
yang tidak perlu dikerjakan lebih awal.

---

## Batas privasi

> **Bawaannya, sebuah aplikasi hanya menerima yang ditujukan kepadanya.**

Yaitu: pesan di DM dengannya, dan pesan yang menyebut `@`-nya secara eksplisit.

`mentionsAll` **tidak** membangunkan aplikasi. "Semua orang" di sebuah grup berarti
semua orang, dan sebuah program bukan orang yang sedang diajak bicara.

Seluruh isi grup adalah scope `chat:read` yang terpisah dan harus diminta eksplisit.
Pemisahan ini yang membedakan "menambahkan bot ke grup" dari "memberikan salinan
seluruh percakapan kepada pihak ketiga", dan perbedaan itu tidak boleh bergantung
pada apakah orang yang memasangnya sedang memikirkannya.

Tempat penyaringannya sudah jelas. `sendAndPublish` memuat daftar anggota satu kali
dan memakainya untuk dua hal sekaligus: siaran WebSocket dan notifikasi push. Di
titik itu daftarnya dipecah dua — manusia menerima event utuh seperti sekarang,
aplikasi menerima hanya kalau pesannya ditujukan padanya. Push tidak pernah dikirim
ke pengguna `kind = 'app'`; aplikasi tidak punya browser dan tidak punya lonceng.

Satu hal yang sudah gratis dan layak disebut sebagai kebajikan, bukan kebetulan:
menambahkan aplikasi ke sebuah grup menghasilkan catatan sistem `member.added`
lewat `appendNotice`, jalur yang sama dengan menambahkan orang. Artinya kehadiran
sebuah aplikasi selalu terlihat di riwayat, tidak bisa disembunyikan, dan bisa
ditelusuri kapan dan oleh siapa. Sebuah bot yang diam-diam masuk ke grup adalah
hal yang tidak mungkin terjadi di sini tanpa seseorang menghapus fitur.

---

## Yang ditunda, dengan alasannya

**Slash command.** Tabel `app_commands` dan popup `/` di kolom tulis; preseden
strukturalnya `MentionList.tsx`, yang sudah menyelesaikan bagian sulitnya —
`aria-autocomplete`, `aria-activedescendant`, navigasi papan ketik, dan pemicu yang
hanya aktif di awal kata.

Yang dihindari di dalamnya: **pesan ephemeral**, yaitu balasan yang hanya terlihat
oleh yang memanggil. Setiap pesan di aplikasi ini memakan satu `seq` yang gapless
dan disiarkan ke seluruh anggota; "hanya terlihat satu orang" berarti sebuah syarat
tambahan di setiap kueri riwayat, setiap jendela `around`, setiap pencarian, dan
seluruh jalur resume. Itu harga yang terlalu mahal untuk kenyamanan. Versi pertama
cukup dengan perintah di DM bot, tempat semua pesan memang sudah hanya berdua.

**Kartu interaktif.** `messages.blocks jsonb` dengan renderer yang memakai daftar
izin ketat — tanpa HTML, tanpa `dangerouslySetInnerHTML`, yang sekarang nol di
seluruh `web/src` dan sebaiknya tetap nol.

Ditunda karena untuk kasus cuti, sebuah tombol yang isinya alamat sudah cukup, dan
mini-app sudah menutup sisanya. Bahasa kartu adalah pekerjaan sebesar satu fase
penuh yang harganya dibayar selamanya dalam bentuk permukaan yang harus dijaga.

**Webhook.** Lihat Seam D.

---

## Yang belum ada jawabannya

**Siapa yang boleh memasang aplikasi.** Belum ada peran admin di aplikasi ini sama
sekali; satu-satunya peran yang ada adalah `owner` dan `member` di dalam sebuah
grup. Dua kandidat, tanpa dipilih sekarang:

- `users.is_admin`, dengan halaman pemasangan di dalam aplikasi.
- Pendaftaran lewat subcommand di `cmd/`, sehingga memasang aplikasi menuntut akses
  ke servernya — cocok dengan posisi "tim IT perusahaan yang menjalankannya", dan
  tidak menambah permukaan apa pun ke aplikasi web.

Pertanyaan ini bersinggungan dengan pertanyaan terbuka yang sudah tercatat di
`PRODUCT.md`: model pendaftaran akun untuk perusahaan — undangan, SSO, atau daftar
bebas — juga belum diputuskan. Keduanya sebaiknya dijawab bersamaan, karena
keduanya menanyakan hal yang sama: siapa yang berwenang di sebuah deployment.

---

## Urutan

Integrasi datang **setelah** daftar "sebelum aplikasi ini boleh dipakai orang" di
`TASKLIST.md` selesai: cara men-deploy, TLS dengan `SECURE_COOKIE=true`, prosedur
cadangan untuk Postgres dan SeaweedFS, hapus akun, blokir pengguna, dan cara
melaporkan penyalahgunaan.

Alasannya sederhana. Membuka jalur pihak ketiga ke sebuah aplikasi yang belum boleh
menyentuh internet adalah urutan yang terbalik, dan tiap seam di dokumen ini
menambah permukaan yang harus dijaga oleh hal-hal di daftar itu.

---

## Daftar kerja

Sengaja tinggal di sini, bukan di `TASKLIST.md`. Ini bukan fase: dia tidak punya
tempat dalam urutan Fase 0–16, dan menaruhnya di sana akan membuatnya terbaca
seperti pekerjaan berikutnya, padahal dia berada di belakang seluruh daftar rilis.
Daftar ini dan keputusan di atasnya dibaca bersamaan, dan karena itu disimpan
bersamaan.

### Seam A — identitas aplikasi
- [ ] `users.kind = 'user'|'app'` — sebuah aplikasi adalah baris `users`, sehingga
      `sender_id` tetap `NOT NULL` dan keanggotaan, `seq`, read receipt, pencarian,
      dan resume ikut bekerja tanpa kode baru
- [ ] Tabel `apps` (alamat mini-app, pemasang, keadaan mati) dan `app_tokens`
      (hash token, memakai ulang `auth.NewToken`/`auth.HashToken`)
- [ ] Cabang `Authorization: Bearer` di `requireAuth`; cookie tetap satu-satunya
      jalur untuk browser
- [ ] Scope `chat:write`, `chat:read`, `user:identity`, `user:email`, `commands` —
      `user:email` dipisah karena `store.User` sengaja tidak membawa email
- [ ] Kuota per aplikasi di daftar aturan `config.go`
- [ ] Saring `kind='app'` dari `SearchUsers`, pemilih anggota grup, dan
      `ContactIDs`

### Seam B — masuk: aplikasi mengirim pesan
- [ ] `POST /api/conversations/{id}/messages` menerima token aplikasi + scope
      `chat:write`; jalur idempoten `client_msg_id` sudah menutup pengulangan

### Seam C — mini-app: halaman penuh milik aplikasinya
- [ ] Kolom utama menampung percakapan ATAU mini-app lewat satu nilai `aktif`,
      menggantikan `activeId`; rel aplikasi di `Sidebar`
- [ ] `apps` membawa nama, ikon, dan alamat mini-app — rel butuh ketiganya
- [ ] `POST /api/apps/{id}/ticket` dan `POST /api/apps/ticket/redeem` — tiket
      sekali pakai, hash disimpan, umur 60 detik, ditukar lewat back-channel
- [ ] Bingkai iframe di kolom utama — BUKAN `PanelShell` — dengan `sandbox` dan
      `<iframe title>`; tanpa `role="dialog"`, tanpa pengurungan fokus, tanpa
      Escape yang dicegat
- [ ] Tautan lewati ke rel, karena Shift+Tab dari dalam halaman pihak ketiga bukan
      milik kita
- [ ] Jembatan `postMessage` berversi (`ready`, `resize`, `theme`, `close`,
      `notify`) dengan pemeriksaan origin di kedua arah
- [ ] Dokumen syarat untuk pihak ketiga: `frame-ancestors` dipegang aplikasinya,
      dan isi halamannya harus memenuhi WCAG 2.2 AA sendiri

### Seam D — keluar: aplikasi menerima kejadian
- [ ] Aplikasi menyambung ke `/ws` sebagai klien; resume `sync` yang sudah ada
      sejak Fase 3 yang menjawab "aplikasi mati sepuluh menit"
- [ ] Periksa apakah `maxResume` 200 masih cocok untuk klien non-browser
- [ ] Pecah daftar anggota di `sendAndPublish`: aplikasi hanya menerima DM
      dengannya dan sebutan `@` eksplisit; `mentionsAll` tidak membangunkan
      aplikasi; push tidak pernah dikirim ke `kind='app'`

### Ditunda
- [ ] Slash command — dan pesan ephemeral dihindari karena `seq` gapless
- [ ] Kartu interaktif (`messages.blocks`) — tombol berisi alamat sudah cukup
- [ ] Webhook — hanya kalau ada aplikasi yang tidak bisa memelihara koneksi

### Prasyarat yang bukan milik dokumen ini
- [ ] Peran admin atau subcommand pendaftaran — belum diputuskan, lihat bagian
      "Yang belum ada jawabannya"
- [ ] Seluruh daftar "sebelum aplikasi ini boleh dipakai orang" di
      [../TASKLIST.md](../TASKLIST.md)

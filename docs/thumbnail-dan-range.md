# Turunan gambar dan permintaan sepotong

Catatan Fase 8. Dua fitur yang dikerjakan bersamaan karena keduanya menjawab
pertanyaan yang sama dari dua arah: **berapa banyak byte yang sebenarnya perlu
dikirim supaya orang melihat apa yang dia minta?**

Sebelum fase ini jawabannya selalu "semuanya". Foto dua belas megapiksel diunduh
utuh untuk ditampilkan selebar tiga ratus piksel. Video tiga menit diunduh dari
awal walau yang diklik menit terakhir. Keduanya benar, keduanya berfungsi, dan
keduanya membayar seratus kali lipat dari yang dibutuhkan.

Seperti Fase 6 dan 7, keduanya bisa dimatikan — turunan lewat
`THUMBNAIL_MAX_DIM=0`, dan permintaan sepotong memang tidak perlu dimatikan
karena dia hanya menjawab apa yang diminta client.

---

## Bagian 1 — Turunan gambar

### Kenapa dibuat saat unggah, bukan saat pertama kali diminta

Membuatnya saat pertama diminta ("lazy") terlihat lebih hemat: gambar yang tidak
pernah dilihat tidak pernah diperkecil. Tetap tidak diambil, dan alasannya bukan
soal kecepatan.

`messages.attachments` adalah **salinan baca** yang ditulis sekali saat lampiran
dipasang ke pesan (lihat [lampiran-dan-push.md](lampiran-dan-push.md)). Seluruh
alasan salinan itu boleh ada adalah karena lampiran **tidak pernah berubah
setelah terpasang** — jadi salinannya tidak akan pernah basi.

Turunan yang lahir belakangan melanggar tepat janji itu. Pesan yang sudah
terkirim harus diperbarui, salinan jsonb-nya ditulis ulang, dan client yang
sedang offline harus diberi tahu bahwa sebuah lampiran lama sekarang punya
alamat baru. Itu jalur sinkronisasi baru untuk sesuatu yang bukan peristiwa
dalam percakapan.

Dengan membuatnya saat unggah — sebelum `INSERT`, sebelum pesannya ada — turunan
sudah lengkap pada detik pertama lampiran itu tercatat. Salinan jsonb-nya benar
sejak awal dan tetap benar selamanya.

Harganya: unggahan jadi beberapa puluh milidetik lebih lama, dan gambar yang
tidak pernah dilihat siapa pun tetap dibuatkan turunan.

### Kenapa gambar disalin utuh di memori, padahal Fase 7 susah payah menghindarinya

Fase 7 memakai `MultipartReader` + `io.Pipe` supaya unggahan tidak pernah utuh di
memori. Fase ini menambahkan `io.TeeReader` yang justru menahan salinannya.

Itu bukan pembatalan keputusan sebelumnya, karena tidak ada cara lain:
memperkecil gambar menuntut gambarnya utuh. Tidak ada penapis yang bisa
mengecilkan sesuatu sambil hanya melihat sepotong kecilnya.

Yang dijaga adalah cakupannya. Salinan hanya diambil untuk **empat tipe gambar
yang memang disajikan inline**, dan hanya saat turunan menyala. Video, arsip, dan
PDF — justru yang paling besar — tetap mengalir lewat tanpa singgah sama sekali,
persis seperti sebelumnya. Dan salinannya tidak mungkin melewati
`MAX_UPLOAD_BYTES`, karena `MaxBytesReader` sudah menutup body-nya lebih dulu.

### Batasnya piksel, bukan byte

Ini penjagaan yang paling mudah dilupakan, dan batas ukuran unggahan tidak
menolongnya sama sekali.

Berkas PNG seratus kilobyte bisa berisi kanvas 40.000 x 40.000. Sah menurut
standar — header PNG hanya menyebut angka, dan isi yang seluruhnya satu warna
terkompresi jadi nyaris tidak ada. Begitu dibentangkan di memori untuk didekode,
angka itu jadi 1,6 miliar piksel, sekitar enam gigabyte. `MAX_UPLOAD_BYTES=10MB`
meloloskannya tanpa ragu, karena yang meledak bukan berkasnya melainkan hasil
dekodenya.

Karena itu `image.DecodeConfig` dipanggil **lebih dulu**, dan dia hanya membaca
header. Urutannya yang penting: memeriksa setelah mendekode berarti ledakannya
sudah terjadi.

Diuji sungguhan — berkas 33 byte berisi header 40.000 x 40.000 ditolak dengan
`dimensi gambar melewati batas yang wajar: 40000x40000`, dan lampirannya tetap
tersimpan serta tetap bisa diunduh. Hanya tanpa turunan.

### Dekode dibatasi jumlahnya, dan yang tidak kebagian TIDAK menunggu lama

Satu foto dua belas megapiksel jadi sekitar lima puluh megabyte piksel mentah
saat dibentangkan, dan angka itu tidak ada hubungannya dengan ukuran berkasnya.
Dua puluh unggahan yang kebetulan bersamaan adalah satu gigabyte yang tidak
pernah direncanakan siapa pun.

`THUMBNAIL_CONCURRENCY` (bawaan 4) membatasinya lewat semaphore. Yang membuatnya
bukan antrean: unggahan yang tidak kebagian giliran dalam tiga detik **melanjutkan
tanpa turunan**, bukan menunggu. Alasannya sama dengan antrean push di Fase 7
yang membuang saat penuh — turunannya boleh tidak ada, lampirannya tidak boleh
gagal.

Bawaannya sengaja kecil dan **tidak** mengikuti jumlah inti mesin: yang dibatasi
memori, bukan CPU, dan mesin berinti banyak justru yang paling mudah kehabisan
memori kalau batasnya ikut membesar.

### Format turunan ditentukan gambarnya, bukan format aslinya

JPEG tidak mengenal alpha sama sekali: logo atau tangkapan layar bertepi
transparan keluar dengan latar hitam pekat. PNG mengenalnya, tapi untuk foto
ukurannya bisa sepuluh kali lipat JPEG pada mutu yang sama — dan seluruh alasan
fitur ini ada adalah ukuran.

Jadi keduanya dipakai, dan yang memutuskan adalah `Opaque()` pada hasil
pengecilannya: padat jadi JPEG mutu 82, ada tembus pandang jadi PNG. Memindai
byte alpha pada gambar selebar 480 piksel biayanya tidak terukur.

Konsekuensinya satu, dan gampang terlewat: **tipe turunan tidak sama dengan tipe
aslinya**. WebP menghasilkan turunan PNG. Karena itu `thumb_mime` disimpan, bukan
ditebak dari ekstensi, dan nama tampilannya ikut disesuaikan — menamai berkas
"logo.webp" untuk byte yang sebenarnya PNG adalah berbohong kepada siapa pun yang
menyimpannya.

### CatmullRom, bukan bilinear

Penapis bilinear pada pengecilan besar hanya mencicipi sebagian kecil piksel
sumber, sehingga rambut, teks, dan garis halus pecah jadi bintik. Pengecilan
besar persis yang terjadi di sini: dari tiga ribu piksel ke empat ratus.

Harganya beberapa puluh milidetik per foto, dan itulah yang membuat batas dekode
bersamaan di atas ada.

### Orientasi EXIF: satu-satunya cacat yang langsung terlihat

Kamera ponsel hampir tidak pernah memutar piksel saat memotret. Dia menyimpan
gambarnya apa adanya — melintang, sebagaimana sensornya terpasang — lalu
menitipkan satu angka di metadata yang artinya "putar segini sebelum
ditampilkan". Browser modern menuruti angka itu secara bawaan.

Turunan yang dibuat tanpa memperhatikannya menghasilkan keadaan yang aneh:
berkas aslinya tampil **tegak** di layar, karena browser memutarnya, sedangkan
turunannya tampil **rebah** — dekoder Go menyerahkan piksel mentah, dan hasil
enkode ulang kita tidak lagi membawa metadata apa pun untuk diikuti browser.

Bukan turunan yang buram atau salah ukuran. Turunan yang MIRING, pada setiap foto
yang dikirim dari ponsel dalam orientasi potret — yaitu hampir semuanya.

Tidak dibutuhkan paket EXIF utuh untuk ini: yang dicari cuma satu tag, dan jalan
menuju tag itu pendek — telusuri segmen JPEG sampai APP1, baca urutan byte TIFF
("II" atau "MM", keduanya benar-benar dipakai di alam liar), lalu cari tag
`0x0112` di IFD0. Perputarannya dikerjakan **setelah** gambarnya diperkecil,
sehingga yang dipindahkan beberapa ratus ribu piksel, bukan dua belas juta.

Efek sampingnya berguna: `Measure` sekarang tahu ukuran gambar **sebagaimana akan
terlihat**, sehingga ruang yang dipesan di layar sebelum gambarnya termuat tidak
lagi tertukar sisi.

### Ukuran gambar tidak lagi dipercayakan ke client

Fase 7 mengambil `?w=` dan `?h=` dari client karena server tidak punya
salinannya untuk diperiksa. Sekarang punya — dan `image.DecodeConfig` selalu
dipanggil, bahkan untuk gambar yang **tidak** dibuatkan turunan karena sudah
kecil.

Angka dari client bisa salah tanpa niat jahat (browser lama), dan bisa salah
dengan niat jahat: satu gambar 1x1 yang mengaku 4000x3000 memesan satu layar
penuh di dalam percakapan orang.

Query string tetap dibaca sebagai cadangan, untuk saat turunan dimatikan.

---

## Bagian 2 — Permintaan sepotong

### Ukurannya datang dari database, bukan dari penyimpanan

Rentang yang diminta dinilai terhadap `attachments.size` — angka yang sudah ada
di baris yang sama dengan izin bacanya. Artinya rentang yang menunjuk ke luar
berkas dijawab **416 tanpa satu pun permintaan jaringan ke penyimpanan**.

Itu bukan penghematan sepele. Client yang meleset menghitung posisi adalah hal
biasa saat video di-seek berulang kali, dan tiap kesalahannya tidak perlu jadi
beban bagi penyimpanan yang sedang melayani orang lain membuka gambar.

### Yang menentukan status jawaban adalah penyimpanan, bukan permintaan kita

Ini yang paling mudah salah, dan akibatnya paling sunyi.

Header `Range` adalah **permintaan, bukan perintah**. Penyimpanan yang tidak
mendukungnya boleh — dan memang akan — menjawab berkas utuh dengan status 200.
Kalau server ini menandai jawaban itu sebagai 206 hanya karena dia *meminta*
sepotong, dia mengirim `Content-Range` yang berbohong; pemutar video yang
mempercayainya akan merakit berkas yang isinya tumpang tindih, dan hasilnya
video rusak tanpa satu pun error di mana pun.

Karena itu `blob.Object` membawa `Partial` yang hanya benar bila penyimpanan
menjawab 206, dan handler membaca **itu** — bukan apa yang dia minta. Diuji
dengan filer tiruan yang sengaja mengabaikan `Range`.

### Bentuk sufiks, dan kenapa dia bukan detail

`bytes=-500` berarti **lima ratus byte terakhir**, bukan mulai dari minus lima
ratus. Bentuk inilah yang dipakai pemutar video untuk membaca indeks MP4 yang
tersimpan di ujung berkas — dan salah membacanya berarti video yang diunggah
dari ponsel tidak pernah bisa diputar sama sekali, karena ponsel menulis
indeksnya di belakang.

### Yang bentuknya salah diabaikan, yang di luar ukuran ditolak

Dua hal yang terdengar mirip dan berlawanan akibatnya:

- **Bentuk tidak dikenali** (`bytes=abc-def`, `bytes=10-5`, beberapa rentang
  sekaligus, satuan selain byte) → **diabaikan**, kirim seluruhnya. Standar
  menuntut ini, dan menolaknya dengan 416 akan mematahkan client yang sebenarnya
  cuma mengirim sesuatu yang tidak kita kenali.
- **Menunjuk ke luar berkas** (`bytes=1000-` pada berkas 1000 byte) → **416**,
  dengan `Content-Range: bytes */<ukuran>` supaya percobaan berikutnya tidak
  meleset lagi. Menyamakannya dengan "abaikan" berarti client yang salah
  menghitung diam-diam menerima berkas utuh dan merakitnya di tempat yang salah.

Beberapa rentang sekaligus sengaja tidak dilayani: itu menuntut jawaban
`multipart/byteranges`, dan yang memintanya bukan pemutar video melainkan
pengunduh — yang sudah terjawab dengan benar oleh berkas utuh.

### Video dan rekaman suara sekarang disajikan inline

Fase 7 hanya mengizinkan empat tipe gambar disajikan `inline`; selebihnya dipaksa
terunduh. Menambah tipe ke daftar itu adalah keputusan keamanan, jadi alasannya
harus sama kuatnya: **tidak satu pun dari MP4, WebM, MP3, WAV, dan Ogg punya
kemampuan mengeksekusi apa pun.** Berbeda dari SVG dan HTML, yang justru
dirancang untuk membawa kode dan karena itu tetap di luar daftar.

Tanpa ini, membuka video di tab baru berarti mengunduh berkas setengah gigabyte
alih-alih memutarnya. Tiga lapis lain dari Fase 7 — tipe ditentukan dari isi,
`nosniff`, dan CSP — tetap berlaku persis sama.

Daftarnya **izin, bukan larangan**: `video/quicktime` ikut terunduh, karena tipe
baru tidak pernah boleh lolos tanpa disebut.

### `Accept-Ranges` dipasang untuk semua tipe

Tanpa header ini browser tidak akan pernah **mencoba** meminta sepotong: pemutar
video menganggap berkasnya tidak bisa dilompati dan mengunduhnya dari awal sampai
posisi yang diklik. Dipasang untuk semua tipe, bukan hanya video — pengunduh yang
putus di tengah memakai jalur yang sama untuk melanjutkan, bukan mengulang.

---

## Satu hal yang diperbaiki sambil lewat

Header penyajian dulu dipasang sebelum isinya diambil. Salah satunya
`Cache-Control: max-age=31536000, immutable`, dan itu benar untuk byte lampiran
yang memang tidak pernah berubah.

Menempel di jawaban 404 atau 416, dia berubah jadi kesalahan yang **menetap**:
browser menyimpannya selama setahun dan tidak pernah lagi bertanya, bahkan
setelah penyebabnya diperbaiki di sisi server. Sekarang header itu dipasang
setelah dipastikan ada yang benar-benar akan disajikan.

---

## Yang diukur

Foto uji 4000 x 3000 (12 megapiksel), JPEG mutu 90:

| | ukuran | catatan |
|---|---|---|
| berkas asli | 6.169.144 B | yang dulu diunduh setiap anggota percakapan |
| turunan | 13.023 B | 480 x 360, JPEG mutu 82 |

**474 kali lebih kecil**, dan itu per anggota per kali percakapannya dibuka.
PNG bertepi tembus pandang menghemat jauh lebih sedikit (3.929.600 → 590.255 B)
karena gambar ujinya derau murni yang tidak bisa dikompresi; gambar sungguhan
dengan bidang warna rata jauh lebih hemat.

Video uji 1280 x 720, tiga menit, 4.871.456 B, indeks di depan (`+faststart`).
Diukur dari byte yang **benar-benar lewat kabel** (`encodedDataLength`), bukan
dari `Content-Length` yang dijanjikan header — keduanya berbeda jauh justru di
sini, karena pemutar meminta `bytes=0-`, dijawab 206 dengan panjang seluruh
berkas, lalu memutus sambungan setelah dapat yang dia butuhkan:

- sebelum melompat: **13.454 byte** terunduh dari 4.871.456 byte
- melompat ke detik 170: satu permintaan baru, `bytes 4587520-4871455/4871456`

Yaitu: langsung dari tempat yang diklik.

---

## Yang tidak dikerjakan

**Partisi tabel `messages`** — kandidat ketiga Fase 8, tetap ditunda. Pemicunya
belum ada dan sudah ditulis lengkap di [scaling.md](scaling.md): puluhan juta
baris, `pg_relation_size('messages')` mendekati memori mesin, atau autovacuum
yang mulai memakan waktu berjam-jam. Mengerjakannya sekarang berarti menambah
kerumitan tanpa imbalan.

**Turunan untuk video** (bingkai pertama sebagai pratinjau) — menuntut dekoder
video di dalam proses ini, dan itu ketergantungan yang jauh lebih besar daripada
seluruh fase ini digabung.

**Beberapa ukuran turunan** (`srcset`) — satu ukuran sudah menutup selisih
seratus kali lipat; yang kedua menutup selisih dua kali lipat, dengan menggandakan
jumlah objek di penyimpanan.

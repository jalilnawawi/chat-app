# Pesan suara dan kolom tulis

Fase 13 menutup tiga hal di sekitar kolom tulis dan gelembung pesan:

| Fitur | Di mana |
|---|---|
| Rekam dan kirim pesan suara | tombol **Rekam** menggantikan **Kirim** selama kotak tulis kosong |
| Pemilih emoji | tombol di sisi kiri kotak tulis; papan yang sama dipakai untuk reaksi |
| Lampiran | tombol klip pindah ke dalam kotak tulis, sisi kanan |
| Tombol reaksi | di samping gelembung — kiri untuk pesan sendiri, kanan untuk pesan orang |

Server hanya berubah di satu tempat — jalur unggah lampiran — dan satu kolom
baru. Rekaman dikirim lewat jalur lampiran yang sudah ada sejak Fase 7:
unggah, id, lalu `attachmentIds` saat mengirim pesan.

Fase 15 menambah satu kolom lagi di jalur yang sama: bentuk gelombang. Lihat
[Bentuk gelombang](#bentuk-gelombang-attachmentspeaks) di bawah.

---

## Pesan suara

### Masalahnya ada di server, bukan di perekam

Rekaman `MediaRecorder` berwadah WebM (Chrome, Firefox), Ogg (Firefox lama),
atau MP4 (Safari). Server menentukan tipe dari **isi** berkas lewat
`http.DetectContentType`, dan beberapa byte pertama wadah-wadah itu hanya
menyebut wadahnya — tidak menyebut isinya suara atau gambar bergerak:

| Rekaman | Hasil sniffing | Akibat tanpa perbaikan |
|---|---|---|
| WebM/Opus dari Chrome | `video/webm` | tampil sebagai pemutar video hitam |
| Ogg/Opus | `application/ogg` | label "lampiran", bukan suara |
| M4A (brand `M4A `, tanpa `mp4*`) | `application/octet-stream` | dipaksa terunduh |

### `narrowContainer`: pengakuan hanya boleh mempersempit

Aturan "tipe dari isi, bukan dari pengakuan" tetap utuh. Pengakuan pengunggah
(`Content-Type` bagian multipart) hanya diterima untuk SATU hal: bahwa wadah
yang dikenali dari byte-nya berisi suara.

```
video/webm       + klaim audio/webm → audio/webm
video/mp4        + klaim audio/mp4  → audio/mp4
application/ogg  + klaim audio/ogg  → audio/ogg
octet-stream     + klaim audio/mp4  → audio/mp4   (hanya bila ada kotak ftyp)
lainnya                             → hasil sniffing, apa pun klaimnya
```

Tidak ada arah sebaliknya, tidak ada perpindahan antar-wadah, dan tidak ada
jalan dari tipe yang dipaksa terunduh ke tipe yang dirender — test-nya
memeriksa bahwa kelas penyajian tidak pernah naik. Label yang bohong paling
jauh membuat sebuah video tampil dengan pemutar suara.

Pengenal MP4 milik `net/http` hanya menerima berkas yang salah satu brand-nya
berawalan `mp4`. Rekaman M4A dari ffmpeg ber-brand `M4A isom iso2` dan lolos
sebagai octet-stream; kotak `ftyp` di awal berkas yang dipakai untuk
mengenalinya, dan hanya bila klaimnya `audio/mp4`.

### Durasi: `attachments.duration_ms`

`MediaRecorder` menulis sambil merekam dan tidak pernah kembali ke header, jadi
WebM hasilnya tidak menyebut durasi — `<audio>` melaporkan `Infinity`. Yang
tahu panjangnya adalah perekamnya; client mengirimnya sebagai `?d=<ms>`, lewat
query dengan alasan yang sama dengan `?w=&h=` (unggahan dialirkan, field form
yang datang setelah berkas tidak terbaca tepat waktu).

- Hanya disimpan untuk tipe `audio/*`, dan hanya dalam 1 ms–1 jam (CHECK yang
  sama di database). Di luar itu dibuang, bukan ditolak — durasi yang salah
  bukan alasan menggagalkan unggahan yang byte-nya sudah tersimpan.
- Murni tampilan. Pemutar memakai durasi dari berkasnya sendiri begitu itu
  angka yang wajar.
- Ikut ke salinan jsonb pesan, riwayat, dan salinan terusan. Kolom yang lupa
  disebut di salah satu daftar kolom tidak dilaporkan compiler, jadi test
  store-nya dibuktikan gagal lebih dulu dengan menghapus kolom itu dari
  `copyAttachments`.
- `durationMs` yang terisi berarti "direkam di dalam aplikasi". Sidebar, bilah
  sematan, dan push menyebutnya **Pesan suara**; berkas audio dari disk tetap
  **Rekaman suara**.

### Alur di client

1. Tombol **Rekam** ada hanya bila server menerima lampiran **dan** browser
   bisa merekam (`getUserMedia` hanya ada di HTTPS atau localhost — membuka
   aplikasi lewat IP LAN tanpa TLS menyembunyikannya).
2. Bilah perekam menggantikan kotak tulis: **buang**, pewaktu, batang tingkat
   suara dari `AnalyserNode`, **kirim**. Batangnya bukan hiasan: mikrofon yang
   dibisukan dari perangkat terlihat sebagai garis datar.
3. **Kirim** menghentikan perekam, menunggu potongan terakhirnya, lalu
   memasukkan berkasnya ke laci lampiran — jalur unggah, kemajuan, dan
   coba-lagi yang sama dengan berkas lain. Begitu siap, pesannya terkirim
   sendiri. Unggahan yang gagal tetap di laci dengan tombol coba lagi.
4. Opus 32 kbps, paling lama lima menit (±1,2 MB). Rekaman yang mencapai batas
   dikirim sendiri, bukan dibuang. Di bawah 0,7 detik ditolak dengan keterangan.
5. Rekaman dibuang dan mikrofon dilepas pada Escape, tombol buang, pindah
   percakapan, dan komponen yang hilang.

Pemutarnya bukan `<audio controls>`: yang bawaan menampilkan `0:00 / 0:00`
untuk rekaman tanpa durasi. Penggesernya `<input type="range">` biasa, jadi
jari, panah papan ketik, dan pembaca layar bekerja tanpa ARIA tambahan. Hanya
satu rekaman berbunyi pada satu waktu; kecepatan 1×/1,5×/2×.

### Bentuk gelombang: `attachments.peaks`

**Fase 15.** Durasi menjawab "berapa lama", dan hanya itu. Yang tidak dijawabnya
adalah di mana orangnya bicara, di mana dia diam, dan apakah rekaman lima belas
detik itu berisi kalimat atau berisi sunyi karena mikrofonnya dibisukan dari
perangkat. Menggeser penunjuk ke tengah rekaman tanpa bentuk adalah menebak.

Sumbernya sama dengan durasi, dan karena alasan yang sama: berkasnya tidak
menyebutnya. Membaca puncak dari sebuah WebM/Opus menuntut mendekode seluruh
Opus — dekoder audio di dalam proses server, untuk sesuatu yang murni navigasi.
Yang sudah memegang angkanya tanpa biaya tambahan adalah perekamnya:
`AnalyserNode` di client sudah menghitung tingkat suara tiap 100 ms sejak Fase
13, cuma dibuang setelah digambar di bilah perekam.

| | Durasi (Fase 13) | Gelombang (Fase 15) |
|---|---|---|
| Kolom | `duration_ms` integer | `peaks` text |
| Query unggahan | `?d=<ms>` | `?p=<40 karakter>` |
| Dijaga | CHECK 1 ms–1 jam | CHECK `^[A-Za-z0-9_-]{40}$` |
| Kosong berarti | bukan rekaman dari dalam aplikasi | pemutar memakai penggeser polos |

**Bentuknya 40 batang, satu karakter base64url per batang, bernilai 0..63.**
Empat puluh karena itu yang muat: batang 3 px dengan celah 1 px — sama dengan
bilah perekam — di lebar yang tersisa setelah tombol putar dan tombol kecepatan.
base64url, bukan base64 biasa, karena `+` dan `/` berubah arti di dalam query
string. Satu kolom `text`, bukan array `smallint`: yang dibaca client selalu
seluruhnya, tidak pernah satu elemen, dan 40 byte melewati salinan jsonb pesan
tanpa perlu diterjemahkan di kedua ujungnya.

Abjad dan jumlah batangnya disebut di satu tempat di client — `web/src/gelombang.ts`
— karena dua ujung harus setuju tanpa pernah saling melihat: perekam yang
menyusunnya dan pemutar yang menggambarnya.

#### Yang diambil per petak adalah puncaknya, bukan rata-ratanya

Rata-rata meratakan satu kalimat pendek di tengah keheningan sampai tidak
terlihat, dan justru itulah yang paling ingin dilihat orang sebelum menggeser.

Dan tidak dinormalkan ke batang tertinggi. Rekaman yang pelan memang terlihat
pelan, dan mikrofon yang dibisukan dari perangkatnya tetap terlihat sebagai
garis datar — sama seperti di bilah perekam, dan karena alasan yang sama.

#### Yang diperiksa server adalah bentuknya, bukan kebenarannya

Server tidak punya cara memeriksa apakah 40 angka itu benar-benar berasal dari
berkas yang diunggah bersamanya. Yang bisa dijaganya adalah 40 byte dari
himpunan karakter yang sempit, sehingga string apa pun yang menempel di kolom
ini aman digambar dan aman dikirim ulang. Yang salah bentuk **dibuang, bukan
ditolak** — persis seperti durasi. Gelombang yang salah bukan alasan
menggagalkan unggahan yang byte-nya sudah tersimpan.

Panjangnya persis 40, bukan paling banyak 40: gelombang separuh panjang akan
digambar sebagai rekaman yang berakhir di tengah.

#### Gelombang digambar DI BELAKANG penggeser yang sudah ada

Ini keputusan aksesibilitas, bukan keputusan tata letak. Penggeser posisi sudah
berupa `<input type="range">` sejak Fase 13, dan dari sana datang seret dengan
jari, panah kiri-kanan, Home/End, dan pengumuman posisi — empat-empatnya tanpa
satu baris ARIA. Menggambar gelombang sebagai tombol atau `div` berarti menulis
ulang keempatnya dengan tangan.

Jadi 40 batang duduk di lapisan `pointer-events: none` di belakang penggeser
yang menutupi kotak yang sama persis. Yang dilepas dari penggesernya hanya
jalur dan bulatannya (`.gelombang` di `index.css`); bulatannya diganti garis
tegak setinggi gelombang, karena penunjuk posisi pada bentuk gelombang adalah
garis, bukan kelereng yang menutupi tiga batang di bawahnya. Cincin fokus
aplikasi tetap berlaku apa adanya.

Batang yang sudah lewat memakai warna penuh, sisanya diredupkan: putih dan
putih 45% di gelembung sendiri, teal dan Garis Batas 55% di gelembung orang.
Batang terpendek 2 px — diam di tengah rekaman adalah garis tipis yang tetap
terlihat, bukan lubang.

---

## Kolom tulis dan emoji

Kotak tulis kini berisi tiga hal — emoji, teks, lampiran — dan satu tombol di
luarnya yang berganti antara **Rekam** dan **Kirim**. Dua tombol bulat sewarna
yang bersebelahan akan tertukar oleh ibu jari.

Emoji disisipkan di posisi kursor dan papannya dibiarkan terbuka. Di layar
sentuh kolom tulis tidak difokuskan ulang: fokus membuka papan ketik, dan papan
ketik menutupi papan emoji.

Daftarnya ditulis tangan di `web/src/emoji.ts` — enam kelompok, 514 emoji,
ditambah "Terakhir dipakai" di `localStorage`. Pustaka pemilih emoji membawa
ratusan kilobyte nama dan kata kunci untuk sesuatu yang dipakai orang dengan
menggulir. Seluruh daftar diperiksa terhadap `validReaction` di server, karena
papan yang sama dipakai untuk reaksi: emoji yang bisa dipilih tapi ditolak
saat ditekan adalah tombol yang berbohong.

## Tombol reaksi

Sebelumnya tombol reaksi duduk di baris di bawah gelembung, dan baris itu
harus selalu ada demi tombolnya. Kini dia di samping gelembung, sejajar
tengah, di ruang yang memang sudah kosong; baris di bawah hanya ada kalau ada
reaksi. Ruangnya ikut dipesan pada pesan yang masih terkirim, supaya kalimat
tidak dibungkus ulang saat pesannya terkonfirmasi.

Bilah cepat berisi delapan emoji plus **lainnya**, yang membuka papan lengkap.
Dari papan lengkap, emoji yang sudah kita berikan tidak dilepas — melepas punya
jalannya sendiri, chip di bawah gelembung. Pada layar sempit tombolnya 32 px
dan papan lengkapnya ditambatkan ke layar, bukan ke gelembung: sembilan tombol
36 px tidak muat di 360 px bila gelembungnya berdiri di samping foto pengirim.

---

## Verifikasi

- `go vet` + `go test -race ./...` bersih; test baru untuk `narrowContainer`
  (termasuk M4A), `duration`, dan durasi yang bertahan lewat kirim, riwayat,
  dan terusan.
- `tsc --noEmit` + `vite build` bersih.
- Uji HTTP langsung dengan WebM, Ogg, dan M4A asli dari ffmpeg: tipe dan
  durasi sesuai; tanpa klaim tetap hasil sniffing; durasi di luar jangkauan
  dibuang; WebM tersaji `inline` dan menjawab `Range` dengan 206.
- Browser (Brave headless, dua konteks, puppeteer-core, `MediaRecorder` asli
  dengan mikrofon tiruan 440 Hz): 45/45, termasuk layar 390 px bersentuhan.

Headless Brave melaporkan `hover: none`, jadi tombol reaksi yang tersembunyi
sampai kursor lewat tidak bisa diuji di sana — yang diuji adalah bahwa dia
selalu tampil pada perangkat tanpa penunjuk.

Fase 15 menambah: test parser `peaks` (panjang, himpunan karakter, dan bahwa
gelombang tidak menempel pada gambar atau video), test store yang dibuktikan
gagal lebih dulu dengan menghapus `peaks` dari `copyAttachments`, test CHECK
database untuk yang salah bentuk, dan delapan test `bun test` untuk
`encodeWaveform` / `decodeWaveform` (urutan, rekaman lebih pendek dari 40
petak, puncak yang bertahan, nilai di luar 0..1, yang salah bentuk). Itu test
pertama di sisi frontend; Bun sudah jadi syarat menjalankannya, jadi yang
bertambah cuma `@types/bun`. Di browser: 40 batang muncul hanya pada lampiran yang
`peaks`-nya sah, penggeser tetap bisa difokus dan digeser dengan panah, dan
tampilannya diperiksa terang, gelap, 1280 px, dan 360 px.

## Yang sengaja tidak dikerjakan

- ~~**Bentuk gelombang di pemutar.**~~ Dikerjakan di Fase 15, lewat jalan
  ketiga yang tidak terlihat waktu itu: bukan mendekode di penerima dan bukan
  menganalisis di server, melainkan menyimpan ringkasan yang SUDAH dihitung
  perekam. Lihat [Bentuk gelombang](#bentuk-gelombang-attachmentspeaks).
- **Tahan-untuk-merekam.** Tekan-sekali lebih mudah untuk semua umur dan
  bekerja sama di tetikus maupun layar sentuh.
- **Mendengar ulang sebelum mengirim.** Buang-lalu-rekam-lagi sudah menutup
  kebutuhan yang sama dengan satu langkah lebih sedikit.
- **Pencarian di papan emoji.** Tanpa nama dan kata kunci, dan itulah bagian
  pustaka yang paling berat.

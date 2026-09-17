# Menemukan pesan: cari, teruskan, sematkan

Sampai Fase 11 sebuah pesan hanya bisa ditemukan dengan **menggulir**. Pesan
minggu lalu di grup yang ramai berjarak ratusan pesan dari layar, dan tidak ada
cara menandai yang penting supaya tidak perlu dicari lagi.

Tiga fitur, diambil bersamaan karena saling menguatkan:

| Fitur | Siapa | Endpoint |
|---|---|---|
| Cari isi pesan | anggota | `GET /api/search?q=&conversationId=&cursor=` |
| Teruskan | anggota percakapan asal **dan** tujuan | `POST /api/conversations/{id}/messages` + `forwardFromId` |
| Sematkan / lepas | DM: keduanya · grup: pemilik | `PUT` / `DELETE /api/messages/{id}/pin` |
| Daftar sematan | anggota | `GET /api/conversations/{id}/pins` |
| Jendela riwayat | anggota | `GET /api/conversations/{id}/messages?around=` / `?after=` |

Ketiganya berbagi satu kebutuhan yang belum ada sebelumnya: **melompat ke pesan
yang jauh di belakang riwayat.** Hasil pencarian, sematan, dan catatan sematan
semuanya menunjuk pesan yang bisa saja berumur setahun.

Ketiganya juga punya gerbang kebocoran yang bentuknya sama dengan kutipan
balasan di Fase 9: sebuah id pesan yang menunjuk ke percakapan yang tidak
diikuti pemanggilnya. Sebagian besar test fase ini menanyakan hal yang sama dari
tiga arah.

---

## Cari

> **Pencarian tidak pernah boleh menemukan apa yang tidak akan ditampilkan
> riwayat.**

Izinnya dievaluasi di dalam kuerinya sendiri (join ke keanggotaan pembaca),
bukan disaring sesudahnya. Pesan yang dihapus dan catatan sistem tersaring oleh
syarat yang sama dengan index-nya. Orang yang dikeluarkan dari grup kehilangan
pencariannya bersamaan dengan riwayatnya.

Percakapan orang lain dijawab **hasil kosong**, bukan 404, bahkan saat id-nya
disebut langsung. Jawaban itu tidak bisa dibedakan dari percakapan yang memang
tidak memuat kata itu, jadi tidak ada yang bisa dipelajari darinya.

### tsvector, bukan trigram

`pg_trgm` sudah terpasang sejak Fase 6 untuk mencari nama pengguna, dan di sana
pilihannya tepat: teksnya pendek, dan orang mengetik potongan dari tengah
("santo" untuk "Budi Santoso"). Untuk isi pesan, keduanya diukur pada **sejuta
pesan** (kosakata sehari-hari berdistribusi miring, 4–16 kata per pesan):

| | tsvector (`simple`) | trigram |
|---|---|---|
| Ukuran index | **43 MB** | 207 MB — sebesar tabelnya sendiri |
| Waktu membangun | **8,6 detik** | 18,9 detik |
| Yang dicocokkan | kata | potongan huruf |

Soal "potongan huruf": `ILIKE '%api%'` menemukan **9.833** pesan pada data uji
ini, padahal kata "api" tidak pernah ditulis sekali pun. Semuanya adalah "sapi",
"kapital", dan sejenisnya. Trigram juga tidak bisa memakai index untuk kueri di
bawah tiga huruf.

### `simple`, bukan `indonesian`

Stemmer Indonesia di Postgres **diuji langsung sebelum dipilih**, dan hasilnya
menentukan:

| Kata | Akar menurut `indonesian_stem` |
|---|---|
| mengirimkan | `irim` |
| meeting | `eting` |
| berita | `ita` |
| perdana | `dana` |
| bertemu | `berte` |
| kirimin, ngirim | tidak disentuh |

Artinya mencari "kirim" **tidak** menemukan "mengirimkan" (akarnya jadi "irim"),
mencari "dana" menemukan "perdana", dan bahasa percakapan sehari-hari (bahasa
aplikasi ini) tidak disentuh sama sekali. Stemmer yang salah lebih buruk
daripada tidak ada stemmer: hasil yang hilang tanpa penjelasan membuat orang
berhenti percaya pada kotak pencarian.

`simple` cuma memecah kata dan mengecilkan huruf. Kekurangannya ditutup dengan
**pencocokan awalan**: "kirim" menemukan "kirimkan". Yang tidak tertutup,
"dikirim", adalah kekurangan yang bisa dijelaskan dalam satu kalimat. Kalimat
itu memang ditulis di panel pencarian, sebelum orang mengetik apa pun.

### Index ekspresi, bukan kolom tersimpan

```sql
CREATE INDEX messages_search_idx ON messages
    USING gin (to_tsvector('simple', body))
    WHERE kind = 'user' AND deleted_at IS NULL;
```

Hasil diurutkan dari yang **terbaru**, bukan menurut `ts_rank`. Di chat, yang
dicari hampir selalu "yang kemarin itu", bukan "yang paling mirip". Karena tidak
ada yang perlu membaca tsvector-nya kembali dari tabel, kolom tersimpan hanya
akan menambah ukuran setiap baris pesan.

Satu hal diperiksa sebelum index ini dipasang: apakah dia merusak **HOT update**
pada jalur tulis yang tidak menyentuh `body`. Diuji pada tabel dengan ruang
kosong di halamannya:

| UPDATE | tercatat | HOT |
|---|---|---|
| `attachments` (tanpa index pencarian) | 49 | 48 |
| `attachments` (dengan index pencarian) | +10 | **+10** |
| `body` (dengan index pencarian) | +10 | +0 |

Postgres membandingkan **nilai** kolom yang dirujuk index, bukan sekadar daftar
kolom yang disebut UPDATE. Jadi memasang lampiran ke pesan yang baru dikirim
tetap HOT. Yang kehilangan HOT hanya penyuntingan, dan penyuntingan memang harus
memperbarui index-nya. Jam reaksi sudah bukan HOT sejak Fase 9 karena index
parsialnya sendiri, dan index ini tidak mengubah apa pun di sana.

### Kueri diurai oleh pengurai yang SAMA dengan index

Kata-kata di kueri diurai Postgres (`to_tsvector('simple', $1)`), bukan dipecah
sendiri di Go. Pengurai Postgres menyimpan `13.00` sebagai satu kata, `e-mail`
sebagai tiga (`e-mail`, `e`, `mail`), dan alamat email utuh. Pengurai buatan
sendiri yang memecah di setiap tanda baca akan mencari `13` dan `00`, yang tidak
pernah ada di index.

Hasil penguraiannya lalu **di-cast** ke `tsquery`, tidak dilewatkan ke
`to_tsquery`. Yang kedua mengurai ulang setiap kata, dan `e-mail` pecah lagi jadi
awalan `e:*`, persis penelusuran satu huruf yang dicegah aturan di bawah. Tiap
kata dikutip, jadi `O'Brien`, `rapat:`, `a:*|!b`, dan `x & (y` semuanya jadi
pencarian biasa, bukan kesalahan sintaks. Semuanya diuji.

Kata-kata itu ikut dikembalikan ke client (`terms`), dan kata-kata itulah yang
disorot, bukan hasil penguraian di browser.

### Awalan hanya untuk kata tiga huruf ke atas

Awalan pendek menyuruh index menggabungkan daftar milik setiap kata yang
berawalan huruf itu. Pada sejuta pesan:

| Kueri | Cocok | Waktu |
|---|---|---|
| `'ra'` (utuh) | 0 | **0,1–0,4 ms** |
| `'ra':*` | 38.082 | 45–50 ms |
| `'r':*` | 176.943 | 94–108 ms |

Jadi kata di bawah tiga huruf dicocokkan utuh. Sorotan di client mengikuti
aturan yang sama: "ke" tidak tersorot di dalam "kemarin".

### Satu jebakan planner, dan jalan keluarnya

Ditulis sebagai satu kueri biasa, planner memilih *nested loop* yang memindai
index pencarian **sekali per percakapan** milik pembaca: 63 kali untuk satu kata,
**212 ms**. Kueri dua kata sempat tercatat 300 ms. Rencana khusus untuk kueri
itu sebenarnya cuma 15 ms; yang lambat adalah rencana *umum* yang dipilih
Postgres setelah lima eksekusi sebuah prepared statement. pgx memang menyimpan
prepared statement per koneksi.

Bentuk akhirnya memaksa index dipindai **sekali**, lalu keanggotaan disaring
lewat hash join:

```sql
WITH hit AS MATERIALIZED (
    SELECT m.id, m.conversation_id, m.created_at FROM messages m
    WHERE m.kind = 'user' AND m.deleted_at IS NULL
      AND to_tsvector('simple', m.body) @@ $2::tsquery   -- + percakapan, + cursor
), page AS (
    SELECT h.id, h.created_at FROM hit h
    JOIN conversation_members cm ON cm.conversation_id = h.conversation_id AND cm.user_id = $1
    ORDER BY h.created_at DESC, h.id DESC LIMIT $n
)
SELECT … FROM page JOIN messages … -- kolom berat hanya untuk satu halaman
```

Diukur tujuh kali berturut-turut (jadi rencana umumnya ikut terukur), pembaca
anggota 63 dari 2.000 grup, sejuta pesan:

| Kueri | Cocok (seluruh tabel) | Semua percakapan | Satu percakapan |
|---|---|---|---|
| kata jarang | 79 | 0,3–0,7 ms | 0,3–0,7 ms |
| `rapat` | 24.040 | 23–55 ms | 9–10 ms |
| `kirim` | 44.124 | 58–69 ms | 12–20 ms |
| `rapat besok` | 617 | 13–17 ms | 12–16 ms |
| `ra` (utuh) | 0 | 0,1–0,4 ms | 0,2–0,4 ms |

**Batas yang diketahui:** biaya pencarian lintas percakapan sebanding dengan
jumlah kecocokan di **seluruh tabel**, bukan di percakapan milik pembaca. Pada
sepuluh juta pesan, kata umum akan berada di kisaran setengah detik. Jalan
keluarnya sudah jelas dan sengaja ditunda: partisi `messages` (rencananya ada di
[scaling.md](scaling.md)), atau index GIN gabungan `(conversation_id, tsvector)`
lewat `btree_gin`. Sampai itu diperlukan, kuota `RATE_SEARCH` (20 beruntun, 60
per menit) yang menjaganya.

### Halaman berikutnya

Cursor-nya `(created_at, id)` pesan terakhir, dibungkus base64. Tujuannya bukan
menyembunyikan isinya, melainkan supaya client tidak tergoda menyusunnya
sendiri. Satu baris lebih dari yang diminta menjawab "masih ada lagi?" tanpa
`COUNT(*)`, yang pada kata umum berarti menghitung seluruh kecocokan hanya untuk
membuang angkanya.

---

## Jendela riwayat

Riwayat dimuat dari yang terbaru ke belakang, sehalaman demi sehalaman.
Sebelumnya, melompat ke kutipan menarik paling banyak lima halaman, dan pesan
yang lebih jauh dari itu tidak pernah sampai.

```
GET /api/conversations/{id}/messages?around=<seq>   → { messages, hasMore, hasNewer }
GET /api/conversations/{id}/messages?after=<seq>    → { messages, hasNewer }
```

Yang membuat `around` murah adalah janji dari migrasi pertama: **`seq` tanpa
lompatan.** Jendela di sekitar sebuah pesan bisa *dihitung*, karena seq-seq di
sekitarnya pasti ada. Jadi tidak butuh dua kueri ke dua arah, cukup satu
`BETWEEN`. Jendela yang menabrak ujung terbaru digeser ke belakang supaya tetap
penuh.

Di client, lompatan punya tiga jalan, dari yang termurah:

1. Pesannya sudah termuat: langsung gulir.
2. Pesannya **dekat** (≤ 150 pesan di belakang yang tertua): tarik halaman lama.
   Riwayat tetap utuh sampai ujung terbaru.
3. Pesannya **jauh**: muat jendela di sekitarnya, dan `hasNewer` mengingat
   bahwa ujungnya belum termuat.

Selama jendela lama terbuka, **pesan baru tidak ditempelkan ke daftar.**
Menempelkannya di bawah jendela berarti dua potong riwayat dengan lubang tak
terlihat di antaranya. Pesan baru ditampung (`stash`) dan digabungkan begitu
jendelanya menyambung kembali ke ujung, lewat "Muat pesan berikutnya" atau
"Ke pesan terbaru". Penampungan sudah berlaku **sejak permintaan jendela
berangkat**. Tanpa itu, pesan yang datang di antaranya ditempel ke riwayat
lama, lalu hilang tertimpa jendela begitu jawabannya tiba.

Sidebar tetap mengikuti pesan baru selama jendela terbuka. Menulis pesan dari
jendela lama mengembalikan layar ke ujung lebih dulu. Jendela lama tidak ikut
menyusul setelah reconnect (cursor-nya adalah ujung jendela, bukan ujung
riwayat) dan tidak menandai apa pun terbaca.

---

## Teruskan

> **Meneruskan adalah mengirim pesan.**

Jalurnya `POST /api/conversations/{id}/messages` yang sama persis, dengan id
dari client (jadi pengiriman ulang tidak menggandakan), siaran, dan notifikasi
push. Yang berbeda cuma dari mana isinya datang. Meneruskan ke lima percakapan
adalah lima pengiriman, dan satu yang gagal tidak membatalkan empat yang
berhasil. Dialognya melaporkan hasil per tujuan, dan hanya bila ada yang gagal.

Pesan terusan adalah ucapan utuh orang lain. Menggabungkannya dengan isi,
lampiran, kutipan, atau sebutan dijawab **400**. Komentar atasnya adalah pesan
berikutnya.

### Yang diteruskan adalah isinya, bukan asal-usulnya

Kolom `forwarded` cuma penanda: **tanpa** penunjuk ke pesan sumber, tanpa nama
penulis aslinya, tanpa nama percakapan asalnya. Pesan sumber berada di
percakapan yang belum tentu diikuti penerima. Menyebut siapa yang menulisnya dan
di grup mana berarti membuka sesuatu yang tidak pernah dibagikan penulisnya.
Yang memutuskan untuk membagikan cuma orang yang meneruskan, dan keputusannya
hanya berlaku untuk isinya.

Penunjuk ke pesan sumber, walau tidak pernah dikirim ke client, adalah jalur
kebocoran yang sama dengan yang ditutup pemeriksaan balasan di Fase 9. Lebih
aman tidak menyimpannya daripada menjaganya. Dialog terusan mengatakan ini di
muka ("tanpa nama penulis aslinya"), sebelum tombolnya ditekan.

### Disalin, berbeda dari kutipan

Kelihatannya bertentangan dengan aturan sejak Fase 7 ("yang boleh disalin adalah
yang tidak pernah berubah"), padahal justru aturan itu yang dipakai. Pesan
terusan adalah ucapan **baru** dari orang yang meneruskan, tentang apa yang dia
lihat saat itu. Kalau penulis aslinya menyunting kalimatnya besok, salinan di
percakapan yang tidak pernah dia datangi tidak boleh ikut berubah. Kutipan yang
dibaca ulang lintas percakapan juga berarti jalur baca ke percakapan yang tidak
diikuti penerimanya.

### Pengirim wajib bisa membaca sumbernya

`forwardSource` memeriksa keanggotaan di percakapan **asal**. Hasilnya 403 untuk
pesan yang tidak ada, sudah dihapus, catatan sistem, dan yang bukan haknya:
empat keadaan, satu jawaban. Keanggotaan di percakapan **tujuan** tetap
diperiksa jalur kirim biasa (404).

### Lampiran: barisnya disalin, byte-nya tidak

Baris lampiran baru punya pemilik baru dan pesan baru, jadi izin bacanya
mengikuti percakapan **tujuan**. Menyalin salinan jsonb-nya saja tidak cukup:
alamat lampiran sumber diperiksa terhadap keanggotaan percakapan sumber, dan
penerima akan mendapat 404 untuk setiap gambar. Byte-nya dipakai bersama lewat
`storage_key` yang sama. Menyalin byte berarti foto 10 MB yang diteruskan ke lima
grup memakan enam kali ruangnya.

Harganya: **penyapu tidak boleh lagi membuang byte hanya karena satu baris yang
menunjuknya sudah yatim.** `TakeOrphanAttachments` tetap menghapus barisnya,
tapi hanya melaporkan kunci yang tidak ditunjuk baris lain. Dua index baru
(`storage_key`, `thumb_key`) membuat pertanyaan itu murah.

Kenapa baris yang belum di-commit tidak jadi celah: salinan hanya bisa lahir
dari baris yang masih **terpasang** ke pesan yang belum dihapus, dan pesan itu
dikunci `FOR SHARE` selama salinannya dibuat. Baris terpasang itu sudah
di-commit dan terlihat oleh penyapu, jadi kuncinya tidak pernah terlihat
"tidak dipakai siapa pun" selama ada salinan yang sedang lahir darinya.
`TestPenyapuMenghormatiKunciBersama` mengujinya dua arah: sumber dihapus dan
byte-nya bertahan, lalu salinan terakhir dihapus dan byte-nya ikut dibuang.

### Urutan kunci: pesan dulu, percakapan kemudian

`DeleteMessage` memegang baris pesan, lalu meminta baris percakapannya lewat jam
reaksi. Rancangan pertama jalur terusan mengunci percakapan tujuan dulu, baru
pesan sumber. Kalau sumber dan tujuan adalah percakapan yang sama, itu dua
transaksi yang saling menunggu selamanya. Ketahuan saat menulis kodenya, sebelum
pernah terjadi. Aturannya sekarang satu untuk semua jalur baru: **pesan dulu,
percakapan kemudian.**

Pengiriman ulang terusan yang sudah tersimpan tidak menyentuh sumbernya sama
sekali. Sumber yang dihapus sesudahnya tidak boleh membuat retry yang sah
dijawab 403.

### Kuota sendiri

`RATE_FORWARD` (5 beruntun, 10 per menit) dihitung **di atas** kuota pesan
biasa. Meneruskan adalah cara termurah menyebarkan satu isi ke banyak ruang
sekaligus, dan itu jalan yang dipakai pesan berantai. Dialognya membatasi lima
tujuan, angka yang sama dengan kelonggarannya. Tombol yang membiarkan orang
memilih dua puluh lalu menolak lima belas adalah tombol yang berbohong.

---

## Sematkan

Tiga aturan, dan tidak satu pun baru:

1. **Pemilik mengelola.** Di grup, sematan adalah hak pemilik, sama dengan
   judul dan keanggotaan di Fase 9b. Menyematkan berarti mengatur apa yang
   dilihat semua orang setiap kali membuka grupnya. Di DM tidak ada pemilik,
   jadi keduanya boleh. Tombolnya tidak ditampilkan kepada yang tidak boleh.
2. **Setiap perubahan meninggalkan catatan.** Menyematkan dan melepas sama-sama
   menulis pesan sistem (`message.pinned` / `message.unpinned`).
3. **Pesan dulu, percakapan kemudian.** Urutan sebaliknya membuka dua celah:
   deadlock dengan penghapusan, dan sematan yang tercatat untuk pesan yang
   terhapus pada milidetik yang sama. Penghapusan tidak melihat baris sematan
   yang belum di-commit, dan tidak ada yang akan membersihkannya lagi.

### Sematan tidak butuh jam ketiga

Sematan adalah **keadaan**, dan tabel `pinned_messages` memegangnya.
**Kejadiannya** dicatat sebagai pesan sistem, dan catatan itulah satu-satunya
kabar yang dikirim: tidak ada event "sematan berubah". Client yang menerima
catatan sematan baru, baik lewat siaran maupun lewat susulan setelah reconnect,
membaca ulang daftar sematannya.

Bandingkan dengan reaksi di Fase 9, yang terpaksa punya jam sendiri karena tidak
ada yang layak dicatat di riwayat untuk setiap penekanan emoji.

Penghapusan pesan tersemat melepas sematannya **tanpa** catatan: penghapusannya
sendiri sudah terlihat di riwayat. Client yang terhubung membuangnya saat
menerima pesan terhapus. Yang offline membaca ulang daftar saat membuka
percakapan, dan daftar dibaca ulang **setiap kali** percakapan dibuka karena
daftarnya pendek dan kuerinya murah.

### Catatan membawa penunjuk, bukan cuplikan

`systemEvent` menyimpan `messageId` dan `messageSeq`, **tanpa** cuplikan isi.
Isi pesan bisa dihapus, dan cuplikan yang dibekukan di catatan itu akan terus
menampilkan kalimat yang sudah dihapus penulisnya, selamanya, di tengah riwayat
semua orang. `seq` boleh disalin karena tidak pernah berubah, dan dialah yang
membuat catatan "menyematkan sebuah pesan" bisa ditekan untuk melompat ke
pesannya walau jauh di belakang.

### Batas 20

Daftar sematan adalah daftar yang **dibaca**, bukan dicari. Lebih dari dua
puluh, dan dia berubah jadi riwayat kedua. Penolakannya berbunyi "paling banyak
20 pesan disematkan; lepas salah satu dulu", bukan "sudah ada atau bentrok";
lihat bagian berikut. Menyematkan ulang pesan yang sudah tersemat di percakapan
yang penuh tetap dijawab "tidak ada yang berubah", bukan "penuh".

---

## Satu perbaikan kecil di jalur kesalahan

`writeStoreError` sebelumnya menjawab **setiap** 409 dengan "sudah ada atau
bentrok", dan menjawab 400 dengan `err.Error()` apa adanya, sehingga orang
membaca "invalid: judul grup wajib diisi". Sekarang `storeDetail` mengambil
keterangan yang ditempelkan dengan sengaja (`%w: keterangan`) dan membuang
awalannya. Error yang membungkus dari arah lain, yang bisa saja membawa isi
kueri, tetap dijawab kalimat umum.

---

## Satu bug lama yang ditemukan di sepanjang jalan

**Menyunting pesan menghapus semua reaksinya dari layar orang lain.**

Siaran suntingan adalah satu payload untuk semua orang, jadi dia tidak pernah
membawa ringkasan reaksi, yang memuat "apakah aku ikut". Larik reaksinya selalu
kosong. `upsert` di client menimpa pesan lama dengan apa adanya, jadi reaksi
hilang sampai halaman dimuat ulang. Bug ini ada sejak Fase 9, dan ketemu karena
daftar sematan juga harus mengikuti suntingan dengan cara yang sama.

Dibuktikan sebelum diperbaiki, bukan cuma diduga: perbaikannya dimatikan
sementara, dan uji browser yang sama menghasilkan `false`. Dengan perbaikan
hasilnya `true`. `keepReactions` sekarang mempertahankan reaksi lama bila pesan
yang datang tidak membawa reaksi dan tidak dihapus.

---

## Verifikasi

- `go vet` + `go test -race ./...` bersih; 16 test store baru, plus test
  `storeDetail`.
- `tsc --noEmit` + `vite build` bersih.
- Tolok ukur sejuta pesan di database terpisah: ukuran dan waktu index,
  waktu kueri per bentuk dan per rencana, dan perilaku HOT update. Semua
  angka di atas berasal dari sana.
- Uji HTTP langsung, **50/50**: orang luar mendapat hasil kosong (bukan 404)
  bahkan dengan id percakapan yang disebut; tujuh bentuk kueri berisi sintaks
  tsquery tidak ada yang jadi 500; penerima terusan membuka lampirannya (200)
  tapi tidak lampiran aslinya (404); kiriman ulang terusan tidak menggandakan;
  terusan + isi atau + kutipan (400); orang luar meneruskan (403) dan meneruskan
  ke percakapan orang lain (404); kuota terusan dan kuota pencarian
  (429 + `Retry-After`); anggota biasa menyematkan (403), orang luar (404),
  catatan sistem (404); sematan ke-21 (409, dengan keterangan yang menyebut
  batasnya); pesan tersemat yang dihapus hilang dari daftar.
- Verifikasi browser (Brave, konteks terpisah, puppeteer-core), **33/33**: cari
  dengan awalan lalu melompat ke pesan ke-10 dari 400 (yang termuat cuma
  jendela di sekitarnya); pesan baru selama jendela terbuka tidak ditempel tapi
  sidebar tetap mengikutinya, lalu muncul saat kembali ke terbaru; reaksi
  bertahan setelah suntingan; bilah sematan dan catatannya realtime di anggota
  lain; lompat jauh lewat daftar sematan dan lewat catatan sematan; anggota
  biasa grup tidak ditawari tombol sematan; dialog terusan sampai ke penerima
  dengan penandanya, tanpa nama grup asal; cakupan pencarian dipindah tanpa
  mengetik ulang; di layar 390 px panel pencarian menutup sendiri setelah hasil
  ditekan, tanpa gulir mendatar; tanpa galat di konsol.

## Yang sengaja TIDAK dikerjakan

- **Peringkat kemiripan.** Lihat alasan "terbaru di atas". Bila suatu hari
  dibutuhkan, dia butuh kolom tsvector tersimpan, dan keputusan di migrasi
  0008 perlu ditinjau ulang.
- **Mencari isi lampiran** (nama berkas, teks di dalam PDF). Nama berkas mudah;
  isi berkas menuntut pengurai dokumen di dalam proses ini.
- **Membuat index pencarian tanpa mengunci tabel.** Migrator menjalankan tiap
  berkas di dalam transaksi, jadi `CREATE INDEX CONCURRENTLY` tidak bisa
  dipakai di sana. Pada tabel yang sudah besar, buat index-nya lebih dulu dengan
  `CREATE INDEX CONCURRENTLY messages_search_idx …` (definisi yang sama persis),
  lalu jalankan migrasinya. Migrasinya memakai `IF NOT EXISTS`, jadi index yang
  sudah ada dilewati. Tanpa langkah itu, penulisan pesan tertahan selama index
  dibangun: 8,6 detik pada sejuta pesan.
- **Menyebut penulis asli pada terusan.** Lihat "yang diteruskan adalah isinya".

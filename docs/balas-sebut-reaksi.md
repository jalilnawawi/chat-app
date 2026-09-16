# Membalas, menyebut, dan bereaksi

Fase 9. Tiga fitur yang diambil bersamaan karena bentuknya sama: **metadata
yang menempel pada SATU pesan tertentu.** Ketiganya butuh kolom atau tabel baru
yang menunjuk ke `messages`, ketiganya butuh event WS baru, dan ketiganya harus
ikut terbawa saat riwayat dibaca tanpa menambah satu query per pesan.

Mengerjakannya terpisah berarti menyelesaikan masalah "bagaimana metadata
per-pesan sampai ke client" tiga kali dengan tiga jawaban yang berbeda.

Referensinya SeaTalk, yang aplikasinya tertutup — yang bisa dilihat cuma daftar
fiturnya, bukan cara kerjanya.

---

## Satu pertanyaan yang memisahkan ketiganya

Ketiganya adalah metadata per-pesan, tapi cara menyimpannya berbeda, dan yang
menentukan bukan selera melainkan satu pertanyaan:

> **Apakah metadata ini bisa berubah setelah pesannya terkirim?**

| | Berubah? | Disimpan sebagai | Dibaca dengan |
|---|---|---|---|
| Balasan | isi yang dikutip **berubah** (diedit, dihapus) | penunjuk saja (`reply_to_id`) | self-join, satu per halaman |
| Sebutan | **tidak pernah** — ditentukan sekali saat kirim | larik di barisnya sendiri (`mentions`) | ikut terbaca, nol query tambahan |
| Reaksi | **berubah terus**, oleh banyak orang | tabel sendiri | satu query beragregasi per halaman |

Ini melanjutkan aturan yang sama dengan Fase 7, bukan melawannya. Di sana
lampiran BOLEH disalin ke kolom jsonb pesannya, dan alasannya persis sama:
lampiran tidak pernah berubah setelah terpasang. Isi pesan berubah, jadi
salinannya pasti basi.

---

## Balas / kutip

### Yang disimpan cuma penunjuknya

`messages.reply_to_id` menunjuk ke tabelnya sendiri. Isinya diambil lewat SATU
self-join per halaman riwayat:

```sql
SELECT m.…, rep.id, rep.seq, rep.sender_id, rep.body, rep.attachments, rep.deleted_at
FROM messages m
LEFT JOIN messages rep ON rep.id = m.reply_to_id
WHERE m.conversation_id = $1 …
```

Join-nya menempel pada primary key tabel yang sama, jadi Postgres menjawabnya
dengan satu lookup index per baris yang memang punya `reply_to_id`. Halaman
berisi seratus pesan yang tak satu pun membalas apa pun tidak membayar apa-apa.

Konsekuensinya — dan ketiganya adalah perilaku yang benar:

- pesan yang dikutip **diedit** → kutipan menampilkan versi terbarunya
- pesan yang dikutip **dihapus** → kutipan tampil sebagai "pesan dihapus",
  bukan menghilang dan meninggalkan balasan tanpa konteks
- tidak ada satu pun jalur sinkronisasi baru yang perlu ditulis

### Pemeriksaan yang bentuknya persis seperti fitur

**Pesan yang dibalas WAJIB berada di percakapan yang sama.**

Tanpa pemeriksaan ini, siapa pun bisa membuat DM dengan dirinya sendiri,
mengirim pesan yang mengutip id pesan dari percakapan yang tidak dia ikuti, dan
membaca isinya dari dalam gelembung kutipan. Bentuknya persis seperti fitur;
akibatnya adalah membaca percakapan orang lain satu pesan pada satu waktu.

Pemeriksaannya ada di `store.replyPreview`, di dalam transaksi pengiriman, dan
berbentuk satu klausa WHERE:

```sql
SELECT … FROM messages WHERE id = $1 AND conversation_id = $2
```

`conversation_id = $2` membuat jawabannya sama untuk pesan yang tidak ada dan
pesan yang tidak boleh dilihat: tidak ditemukan. Diuji di
`TestBalasLintasPercakapanDitolak`, dan diuji lagi lewat HTTP langsung.

### Satu hal yang hanya terlihat saat menulisnya

`EditMessage` dan `DeleteMessage` memakai `UPDATE … RETURNING`, dan **RETURNING
tidak bisa menjangkau tabel lain** — termasuk tabel yang sama lewat alias lain.
Jadi keduanya dibungkus CTE:

```sql
WITH upd AS (UPDATE messages SET … RETURNING *)
SELECT … FROM upd m LEFT JOIN messages rep ON rep.id = m.reply_to_id
```

Tanpa ini, gelembung kutipan lenyap begitu pengirimnya memperbaiki satu salah
ketik — dan lenyapnya hanya di layar, karena datanya baik-baik saja.

---

## Sebutan (mention)

### Client mengirim id, bukan server mengurai teks

`mentionedUserIds` dikirim eksplisit. Server tidak pernah mencari "@nama" di
dalam teks.

Alasan yang gampang disebut: nama tampilan boleh mengandung spasi, jadi
penguraian teks akan selalu punya kasus tepi. Alasan yang sebenarnya lebih
penting: **siapa yang dibangunkan tidak boleh ditentukan oleh cara sebuah string
kebetulan ditulis.** Orang yang menulis "kirim ke @budi ya" dalam kalimat biasa
tidak sedang memanggil Budi, dan orang yang mengganti nama tampilannya tidak
boleh membuat pesan lama berhenti memanggilnya.

Server tetap tidak mempercayai client soal SIAPA yang berhak dibangunkan. Tiap
id diperiksa keanggotaannya, dan pemeriksaannya menyatu dengan penulisannya:

```sql
UPDATE conversation_members SET mention_seq = GREATEST(mention_seq, $1)
WHERE conversation_id = $2 AND user_id = ANY($3)
```

Jumlah baris yang tersentuh lebih sedikit dari yang diminta berarti ada id yang
bukan anggota — dan pesannya dibatalkan seluruhnya. Memisahkan pemeriksaan dari
penulisan berarti ada celah di antara "sudah diperiksa" dan "sudah ditulis",
tepat pada pemeriksaan yang menentukan siapa boleh dibangunkan.

### Sebutan menembus peredam dering

Inilah yang membuat fitur ini layak digabung dengan push yang sudah ada, bukan
jadi hiasan tampilan.

Peredam dering Fase 7 ada karena dua puluh pesan beruntun adalah SATU kabar.
Tapi pesan yang menyebut nama seseorang bukan lagi bagian dari obrolan yang
mengalir — dia adalah panggilan, dan orang yang namanya dipanggil di tengah
percakapan ramai justru yang paling mungkin kehilangan kabarnya kalau
peredamnya ikut berlaku.

Yang TIDAK ditembus: pemeriksaan "sedang online". Orang yang tabnya terbuka di
sebelah sudah melihat sebutan itu di layarnya, dan membunyikan ponselnya juga
adalah memberi tahu dua kali.

Metriknya dipisah (`push_mention_bypass_total`, bukan label baru pada
`push_skipped_total`) supaya "peredamnya bekerja" dan "peredamnya ditembus"
tidak pernah terbaca sebagai satu angka.

### @semua

Disimpan sebagai kolom tersendiri (`mentions_all`), bukan diterjemahkan jadi
daftar berisi seluruh anggota. Keanggotaan grup berubah, dan pesan lama harus
tetap berarti "semua orang" — bukan "semua orang yang kebetulan ada di sana
waktu itu". Penerjemahannya terjadi saat kabar akan dikirim, bukan saat pesannya
disimpan.

Kuotanya sendiri, dihitung **per user per percakapan**: dua beruntun, lalu satu
tiap dua menit. Satu orang yang membangunkan dua ratus orang sekaligus adalah
hal yang harus dibatasi, bukan dilarang — rapat yang benar-benar mendesak memang
ada; yang tidak boleh ada adalah kemampuan mengulanginya tiap beberapa detik.
Token bucket yang sama dengan kuota lain, jadi batasnya tetap satu walau
koneksinya tersebar ke beberapa instance.

Di DM, `mentionsAll` diabaikan diam-diam: di sana dia cuma cara lain menembus
peredam dering tanpa menyebut siapa pun.

### Penanda yang bertahan

**"Ada yang menyebut kamu" berbeda dari "ada pesan baru", dan disimpan di kolom
yang berbeda.**

```
conversation_members.mention_seq      seq pesan TERAKHIR yang menyebut anggota ini
conversation_members.mention_ack_seq  sejauh mana sebutan itu sudah benar-benar dilihat
```

Badge menyala selama `mention_seq > mention_ack_seq`.

`last_read_seq` bergerak begitu ruangnya dibuka. Kalau sebutan ikut menumpang
di sana, membuka percakapan sebentar untuk melihat apa yang terjadi sudah cukup
untuk melupakan bahwa ada yang memanggil.

Jadi ada endpoint tersendiri, `POST /api/conversations/{id}/mentions/ack`, dan
client memanggilnya **bukan saat percakapannya dibuka**, melainkan saat pesan
yang memanggil namanya benar-benar masuk layar — lewat `IntersectionObserver`
pada gelembungnya, ambang 60%. Itu janji yang tidak bisa ditepati oleh markRead,
berapa pun cara memanggilnya diatur.

Hasil ack ini tidak disiarkan ke siapa pun. Sejauh mana seseorang sudah melihat
panggilan untuk dirinya sendiri bukan urusan anggota lain — berbeda dengan read
receipt, yang memang ditujukan untuk dilihat lawan bicara.

---

## Reaksi

### Bentuk kuncinya yang memaksakan aturannya

```sql
PRIMARY KEY (message_id, user_id, emoji)
```

Satu orang boleh memberi beberapa emoji berbeda pada satu pesan, tapi tidak bisa
memberi emoji yang sama dua kali. Menekan tombol yang sama dua kali karena
jaringan lambat menghasilkan keadaan yang sama, bukan hitungan ganda — dan yang
menjaminnya adalah database, bukan kode aplikasi yang harus diingat orang
berikutnya.

### Jam kedua

Ini persoalan yang tidak dimiliki balasan maupun sebutan: **reaksi mengubah
pesan LAMA.**

Seluruh mesin sinkronisasi aplikasi ini berdiri di atas satu pertanyaan sejak
Fase 3 — "pesan apa yang `seq`-nya lebih besar dari punyaku?" — dan reaksi tidak
muat di sana. Menekan emoji pada pesan kemarin tidak menggerakkan `seq` apa pun.
Orang yang menutup laptopnya dan membukanya lagi akan melihat riwayat yang
lengkap dengan reaksi yang hilang semua.

Jawabannya adalah penghitung kedua:

```
conversations.reaction_seq   dinaikkan setiap kali ada reaksi yang berubah
messages.reaction_seq        nilai penghitung itu saat reaksi pesan ini terakhir berubah
```

Menambah DAN mencabut sama-sama menaikkannya, sehingga pencabutan pun punya
jejak — tanpa perlu menyimpan baris nisan untuk reaksi yang sudah tidak ada.
Client menyimpan nilai terbesar yang pernah dilihatnya sebagai cursor kedua, dan
mengirimnya berdampingan dengan cursor pesan saat `sync`.

Yang menyusul adalah **keadaan, bukan kejadian**. Dua puluh orang yang menekan
lalu melepas emoji yang sama menghasilkan satu kiriman berisi jawaban akhir, dan
hasilnya benar tanpa client harus memutar ulang urutannya.

`LEFT JOIN`, bukan `JOIN`, di query susulannya: pesan yang SELURUH reaksinya
dicabut selagi client offline tetap harus ikut, justru karena jawabannya
sekarang kosong. Dengan INNER JOIN dia lenyap dari hasil dan client menyimpan
hitungan lama selamanya — tepat kasus yang membuat "state, bukan event" dipilih.

Index-nya parsial (`WHERE reaction_seq > 0`), pola yang sama dengan
`attachments_orphan_idx` di Fase 7: sebagian besar pesan tidak pernah menerima
satu reaksi pun.

### Siaran membawa selisih, susulan membawa ringkasan

Ringkasan reaksi memuat `mine` — "apakah AKU ikut" — dan itu satu-satunya bagian
sebuah pesan yang jawabannya berbeda untuk tiap pembaca.

`hub.Publish` mengirim SATU payload yang sama ke semua orang. Memasukkan
ringkasan ke dalamnya berarti mengirim jawaban milik orang lain ke setiap orang.
Jadi:

| Jalur | Tujuannya | Yang dikirim |
|---|---|---|
| `reaction.added` / `reaction.removed` | siaran, semua anggota | **selisih**: siapa, emoji apa, jam berapa |
| `reaction.batch` | satu koneksi (resume) | **ringkasan lengkap** beserta `mine` |
| riwayat REST | satu pembaca | ringkasan lengkap beserta `mine` |

### Satu perubahan, dua jalan pulang

Penekanan tombol sendiri diterapkan optimistik, lalu kabarnya kembali lewat DUA
jalan: jawaban HTTP dan siaran WebSocket. Tanpa penjagaan, hitungannya naik dua.

Dua gerbang di `store.applyReaction`:

1. **Jam yang tidak maju berarti sudah pernah terpasang.** `reactionSeq <=
   message.reactionSeq` → abaikan. Ini yang menangkap kabar yang sama datang dua
   kali.
2. **Perubahan sendiri yang keadaannya sudah benar.** Kalau `userId` adalah kita
   dan arahnya sudah sesuai dengan keadaan sekarang, yang dipasang cuma nomor
   jamnya.

Gerbang kedua juga menjawab kasus dua tab: tab yang belum tahu punya keadaan
yang BERBEDA dengan arah kabarnya, jadi dia menerapkannya seperti perubahan
orang lain.

### Validasi emoji

Kolom teks bebas yang ditampilkan di samping pesan orang lain adalah pesan kedua
yang menyamar jadi reaksi. Dua hal dipaksakan:

1. tiap rune harus termasuk daftar **IZIN** simbol emoji — daftar larangan pada
   Unicode selalu ketinggalan satu blok
2. hasilnya harus **satu grafem**: "👍👍👍" adalah tiga, dan tiga emoji dalam
   satu tombol adalah cara menyelundupkan panjang

Go tidak punya pemecah grafem UAX#29 di pustaka standarnya, dan menambah
dependensi untuk satu pemeriksaan masukan terasa mahal. Yang dipakai adalah
aturan yang menutup bentuk emoji nyata: rangkaian ZWJ (n hal terlihat dengan n-1
penyambung), pengubah warna kulit, penanda variasi, keycap, dan pasangan
bendera. Sengaja LEBIH KETAT dari UAX#29 — emoji yang sangat baru mungkin
ditolak, dan itu kegagalan yang benar arahnya.

Batas panjangnya ada di dua tempat, dan yang di aplikasi sengaja lebih ketat
dari yang di database (16 rune vs 24), supaya yang lolos di atas tidak pernah
ditolak lagi di bawah — dua lapis dengan batas berbeda adalah kegagalan yang
hanya muncul di produksi, sebagai kesalahan server, untuk masukan yang sudah
dinyatakan sah. Ada test yang menjaga hubungan itu.

### Reaksi TIDAK membangunkan notifikasi

Keputusan produk, ditulis di kepala `handlers_reactions.go` karena di situlah
tempat yang paling mungkin melanggarnya. Sebuah pesan ditujukan kepada
seseorang; sebuah emoji adalah tepukan di bahu. Membangunkan ponsel orang di
tengah malam untuk tepukan di bahu adalah cara tercepat membuat seluruh
notifikasi aplikasi ini dimatikan orang — termasuk yang benar-benar penting.

---

## Test untuk `internal/store`

Fase ini menambah SQL baru ke satu-satunya paket yang sampai Fase 8 tidak punya
satu test pun — dan itu justru lapisan tempat satu salah ketik berubah jadi
kehilangan data, bukan jadi error compile. `go vet` tidak membaca isi string
SQL.

Dua keputusan menentukan bentuk harness-nya:

1. **Database sungguhan, bukan tiruan.** Yang diuji adalah SQL: self-join,
   agregasi, `ON CONFLICT`, CHECK constraint, dan jumlah baris yang tersentuh
   sebuah UPDATE. Tiruan dari lapisan database hanya akan menguji tiruan itu
   sendiri.
2. **Dilewati, bukan gagal, bila databasenya tidak ada.** `go test ./...` harus
   tetap hijau di mesin yang belum menyalakan docker — kalau tidak, orang
   berhenti menjalankannya sama sekali, dan test yang tidak pernah dijalankan
   tidak melindungi apa pun.

Tiap kali dijalankan, seluruh skema dibuat baru di dalam schema Postgres
tersendiri (`test_store_<acak>`) lalu dibuang. Bukan transaksi yang di-rollback:
yang diuji termasuk kode yang membuka transaksinya sendiri, dan transaksi
bersarang bukan hal yang sama dengan transaksi. `public` tetap ikut di
`search_path`, bukan untuk tabel melainkan untuk extension — `gin_trgm_ops` di
migrasi 0002 tinggal di schema tempat `pg_trgm` dipasang.

20 test: jalur Fase 9, ditambah yang paling mudah rusak diam-diam dari fase
sebelumnya — idempotensi kirim pesan, pemasangan lampiran, dan izin baca
lampiran.

```
TEST_DATABASE_URL=postgres://chat:chat@localhost:5433/chatapp?sslmode=disable go test ./internal/store/
```

---

## Verifikasi

- `go vet` + `go test -race ./...` bersih
- `tsc --noEmit` + `vite build` bersih
- Uji HTTP langsung: kutipan lintas percakapan (403), sebutan orang luar (403),
  reaksi berupa kalimat (400), dua emoji sekaligus (400), reaksi orang luar
  (404), penekanan kembar (`changed:false`, jam tidak maju), kuota @semua
  (429 dengan `Retry-After: 120`), dan @semua di DM yang tersimpan sebagai
  `false`
- Uji WebSocket langsung: client yang cursor pesannya sudah lengkap tapi cursor
  reaksinya nol menerima `reaction.batch` berisi keadaan terkini — termasuk
  pencabutan yang terjadi selagi dia offline; client yang cursornya sudah
  mutakhir tidak menerima apa-apa
- Verifikasi browser (Brave, dua konteks terpisah, puppeteer-core): **36/36
  lulus** dalam dua putaran — balas, kutipan yang mengikuti edit, reaksi
  realtime dengan hitungan yang tidak pernah ganda, pemilih sebutan yang
  menyisipkan nama bersisipan spasi, penanda @ yang bertahan setelah ruangnya
  dibaca lalu padam setelah pesannya terlihat, dan lompat ke pesan yang dikutip
  yang halamannya belum termuat

---

## Tiga hal yang ditemukan OLEH menjalankannya di browser

Ketiganya lolos `tsc`, lolos `go test`, dan tidak satu pun bisa ditangkap test
mana pun yang tidak membuka halamannya.

### 1. Seluruh tombol aksi tidak bisa dijangkau dari ponsel

`group-hover:flex` di Tailwind v4 dibungkus `@media (hover: hover)`. Perangkat
tanpa penunjuk — setiap ponsel, dan juga browser headless — melaporkan
`hover: none`, sehingga aturannya tidak pernah berlaku.

Akibatnya bukan "tombolnya sulit ditemukan" melainkan **tidak ada sama sekali**:
balas, edit, hapus, dan tombol tambah reaksi. Sudah berlaku sejak Fase 4 untuk
edit dan hapus; Fase 9 menaruh dua fitur barunya di pola yang sama.

Diperbaiki dengan syarat kedua: `[@media(hover:none)]:flex` — di perangkat yang
memang tidak punya kursor, tombolnya selalu tampil, karena tidak ada isyarat
lain untuk memunculkannya.

Yang menemukannya: browser headless kebetulan berperilaku persis seperti ponsel.

### 2. Read receipt menanam larik kosong di daftar anggota

`applyRead` berbunyi `(s.members[id] ?? []).map(…)` lalu menulis hasilnya
kembali. Untuk percakapan yang daftar anggotanya belum pernah dimuat, itu
menanam **larik kosong** di sana — dan larik kosong tidak bisa dibedakan dari
"sudah dimuat, memang kosong".

Read receipt datang jauh lebih sering daripada orang membuka percakapan, jadi
keadaan itu nyaris permanen. Akibatnya semua yang bergantung pada daftar anggota
diam-diam berhenti bekerja: judul grup menulis "0 anggota", nama pengirim jadi
"Seseorang", dan — yang membuatnya ketahuan — sebutan tidak pernah tersorot.

Tidak terlihat di DM: di sana nama lawan bicara datang dari `peer` pada daftar
percakapan, bukan dari daftar anggota. Seluruh verifikasi browser Fase 6-8
memakai DM.

### 3. Daftar anggota menumpang syarat milik riwayat

`openConversation` memuat riwayat DAN anggota di balik satu syarat,
`!messages[id]`. Tapi `messages[id]` bisa terisi tanpa riwayat pernah dimuat:
satu pesan yang datang lewat WebSocket ke percakapan yang belum dibuka sudah
cukup.

Bentuk kegagalannya sama persis dengan nomor 2, dan itu yang membuatnya sempat
tersembunyi di baliknya — memperbaiki satu saja tidak cukup. Sekarang keduanya
punya syaratnya sendiri, dan penanda "riwayat sudah pernah dimuat" adalah
`hasMore[id]`, yang memang hanya pernah diisi oleh pemuatan riwayat.

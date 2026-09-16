# Kelola grup

Sampai Fase 9 sebuah grup hanya bisa **dibuat**. Setelah itu dia beku: tidak ada
cara menambah orang, mengeluarkan orang, mengganti judulnya, atau keluar
darinya. Dokumen ini menutup itu.

Lima tindakan, satu aturan yang menaungi semuanya:

> **Pemilik mengelola, anggota bisa keluar.**

| Tindakan | Siapa | Endpoint |
|---|---|---|
| Ganti judul | pemilik | `PATCH /api/conversations/{id}` |
| Tambah anggota | pemilik | `POST /api/conversations/{id}/members` |
| Keluarkan anggota | pemilik | `DELETE /api/conversations/{id}/members/{userId}` |
| Pindah kepemilikan | pemilik | `POST /api/conversations/{id}/owner` |
| Keluar dari grup | siapa saja | `POST /api/conversations/{id}/leave` |

---

## Keputusan utama: perubahan keanggotaan adalah PESAN

Godaannya adalah menyiarkan "daftar anggota berubah" lalu selesai. Itu membuat
perubahannya tidak punya jejak: orang yang membuka aplikasi besok pagi hanya
melihat bahwa jumlah anggotanya berbeda, tanpa tahu siapa yang mengeluarkan
siapa, atau kapan.

Sebuah percakapan **sudah** punya catatan berurutan yang tahan putus koneksi:
tabel `messages`, dengan `seq` yang jadi dasar pagination dan resume sejak Fase
1. Menumpang di sana berarti catatan keanggotaan ikut terbawa oleh SELURUH mesin
yang sudah ada — riwayat, cursor, susulan setelah reconnect, urutan yang
deterministik — tanpa satu baris pun jalur sinkronisasi baru.

Bandingkan dengan **reaksi** di Fase 9, yang tidak bisa menumpang dan karena itu
butuh jam kedua: reaksi mengubah pesan lama yang `seq`-nya sudah berhenti
bergerak. Catatan keanggotaan tidak begitu — dia memang kejadian baru, pada saat
ini, dan `seq` berikutnya adalah tempat yang tepat untuknya.

```sql
ALTER TABLE messages ADD COLUMN kind text NOT NULL DEFAULT 'user'
    CHECK (kind IN ('user', 'system'));
ALTER TABLE messages ADD COLUMN system_event jsonb;
```

### `kind` bukan kerapian tampilan

Tanpa kolom itu, catatan sistem ikut memenuhi syarat `sender_id = $aku` pada
jalur edit dan hapus — dan **pelakunya bisa menyunting catatan "Budi
mengeluarkan Ani" menjadi kalimat apa pun**, lalu kalimat itu tetap tampil
sebagai keterangan resmi dari sistem.

Jadi satu aturan, empat jalur:

> Catatan sistem bukan ucapan siapa pun: tidak bisa disunting, dihapus, dibalas,
> atau direaksi.

Masing-masing satu klausa `AND kind = 'user'`, dan semuanya diuji di
`TestCatatanSistemTidakBisaDisentuh`.

### Yang disimpan adalah kejadiannya, bukan kalimatnya

```json
{"type":"member.added","actor":{"id":"…","name":"Budi"},"targets":[{"id":"…","name":"Ani"}]}
```

Kalimatnya disusun di client (`systemText` di `MessageBubble.tsx`), sehingga
bahasanya bisa berubah — atau diterjemahkan — tanpa menulis ulang riwayat siapa
pun.

**Nama orangnya ikut disalin**, dan itu disengaja. Orang yang dikeluarkan tidak
lagi ada di daftar anggota, jadi client tidak punya tempat untuk mencari
namanya; catatan yang berbunyi "Budi mengeluarkan (tidak dikenal)" gagal justru
pada satu hal yang ingin diketahui orang.

Ini sejalan dengan aturan salinan yang sama sejak Fase 7: **yang boleh disalin
adalah yang tidak pernah berubah**, dan sebuah catatan sejarah memang dibekukan
pada saat kejadiannya. Bandingkan dengan kutipan balasan di Fase 9, yang justru
TIDAK disalin karena isi pesan berubah.

---

## DM tidak bisa dikelola

Setiap operasi menolak percakapan bertipe `direct`, dan ini bukan kerapian.

Menambahkan orang ketiga ke sebuah DM akan mempertahankan `direct_key` milik dua
orang pertama, sehingga percakapan itu tetap dianggap DM antara mereka berdua —
sekaligus berisi orang yang tidak pernah diajak oleh siapa pun. **Percakapan
pribadi yang diam-diam bertambah pendengarnya adalah bentuk kegagalan terburuk
yang bisa dimiliki aplikasi chat.**

Dijawab 404, bukan 403: percakapan ini memang tidak punya pengelolaan, dan
membedakan "bukan grup" dari "tidak ada" tidak memberi tahu apa pun yang berguna
kepada pemanggil yang sah. Diuji di `TestDMTidakBisaDikelola`, dan diuji lagi
lewat HTTP langsung.

---

## Pemilik yang keluar tidak ditahan

Kepemilikan berpindah sendiri ke **anggota yang paling lama bergabung**.

Alternatifnya adalah menuntut pemilik memindahkan kepemilikan lebih dulu, dan
itu menukar satu langkah tambahan dengan keadaan yang jauh lebih buruk: grup yang
pemiliknya berhenti memakai aplikasi ini adalah grup yang tidak bisa dikelola
siapa pun, selamanya, tanpa cara memperbaikinya dari dalam.

Anggota terlama, bukan yang pertama menurut abjad: yang paling lama di dalam
grup adalah yang paling mungkin mengenali isinya. Index pendampingnya
(`conversation_members_joined_idx`) ada supaya pencarian itu tidak memindai
seluruh keanggotaan grup besar.

Kalau tidak ada siapa-siapa lagi, grupnya memang habis — barisnya tetap ada
supaya riwayatnya tidak ikut terhapus, dan tidak ada seorang pun yang masih bisa
membukanya.

UI mengatakan ini **di muka**, bukan menjelaskannya setelah orangnya terlanjur
keluar: kejutan pada tindakan yang tidak bisa dibatalkan selalu terasa seperti
kesalahan aplikasi.

---

## Tiga kiriman untuk satu perubahan

`broadcastGroupChange` adalah satu tempat yang tahu siapa harus diberi tahu
tentang apa, sehingga menambah tindakan keenam tidak berarti menebak ulang
jawabannya.

1. **`conversation.new` ke yang baru bergabung** — dikirim lebih dulu, karena
   catatan sistem yang menyusul butuh percakapan untuk mendarat. Daftar
   penerimanya dipersempit ke orang yang benar-benar baru: bentuk percakapan
   berbeda untuk tiap orang (unread, penanda sebutan, lawan bicara), jadi tiap
   penerima butuh query sendiri — dan mengirimi seluruh grup dua ratus orang
   berarti dua ratus query untuk satu orang yang bergabung.
2. **`message.new` + `conversation.updated` ke anggota yang tersisa** — yang
   pertama adalah KEJADIAN yang masuk riwayat, yang kedua adalah KEADAAN
   sekarang. Daftar anggota hari ini tidak punya tempat di riwayat.
3. **`conversation.removed` ke yang baru saja pergi** — dia tidak ada di daftar
   penerima dua kiriman pertama dan tidak akan pernah menerimanya. Tanpa event
   ini, percakapan yang sudah bukan miliknya menggantung di sidebar sampai
   halamannya dimuat ulang, dan mengkliknya menghasilkan 404 yang tidak bisa
   dijelaskan.

Tidak satu pun memanggil `notifyOffline`. Perubahan keanggotaan bukan pesan yang
ditujukan kepada seseorang, dan membangunkan ponsel dua ratus orang karena judul
grup diganti adalah cara membuat notifikasi aplikasi ini dimatikan orang —
alasan yang sama persis dengan reaksi di Fase 9.

---

## Yang ditemukan oleh menulis test-nya

Dua celah nyata, keduanya lolos `go vet` dan `go build`.

### 1. Reaksi ke catatan sistem lolos

Aturan "catatan sistem tidak bisa disentuh" sudah ditulis untuk edit, hapus, dan
balas, tapi `changeReaction` terlewat. Akibatnya orang bisa menempelkan emoji
pada "Budi mengeluarkan Ani" — kecil, tapi persis jenis ketidakkonsistenan yang
membuat aturan berhenti dipercaya.

### 2. `SendMessage` tidak memeriksa keanggotaannya sendiri

Pemeriksaan "apakah pengirim anggota percakapan ini" hanya ada di handler HTTP
(`authorizedConversation`), sebagai query terpisah **sebelum** transaksi
pengiriman dibuka.

Itu menyisakan celah yang bentuknya persis sama dengan yang ditutup Fase 9 pada
validasi sebutan: pemeriksaan yang terpisah dari tindakannya punya jarak di
antara keduanya. Di sini jaraknya berarti orang yang baru saja dikeluarkan dari
grup masih bisa menyelipkan satu pesan, karena pengeluarannya terjadi persis
setelah handler memastikan dia anggota.

Sekarang keanggotaan diperiksa di dalam query yang sama yang mengunci baris
percakapan dan mengalokasikan `seq`:

```sql
SELECT c.last_seq, c.type
FROM conversations c
JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $2
WHERE c.id = $1
FOR UPDATE OF c
```

`JOIN`, bukan `EXISTS`: yang bukan anggota tidak menghasilkan baris sama sekali,
jadi jawabannya jatuh ke cabang yang sama dengan percakapan yang memang tidak
ada — dan itu memang jawaban yang benar untuknya.

---

## Satu hal yang ditemukan oleh menjalankannya lewat HTTP

Judul grup kosong dijawab **409 "sudah ada atau bentrok"**, karena
`NormalizeGroupTitle` memakai `ErrConflict` — satu-satunya sentinel yang
tersedia yang bukan soal izin atau keberadaan. Status itu mengirim orang mencari
bentrokan yang tidak pernah ada.

Ditambahkan `store.ErrInvalid` yang dipetakan ke **400**, dan pemetaannya hidup
di `writeStoreError` bersama yang lain — satu tempat, seperti sejak Fase 2.

---

## Kuota

`RATE_GROUP` (bawaan: 10 burst, 30/menit). Yang dibatasi bukan beban server
melainkan kemampuan satu orang memenuhi percakapan orang lain dengan catatan
yang tidak mereka minta: tiap tindakan menulis satu baris ke riwayat **semua**
anggota sekaligus menyiarkan dua event.

**Keluar dari grup sengaja tidak dibatasi.** Ini satu-satunya tindakan yang
dilakukan seseorang atas dirinya sendiri, dan menahan orang di dalam grup karena
dia terlalu sering menekan tombol adalah bentuk penolakan yang tidak pernah
pantas.

`maxGroupMembers = 200`, dan batas yang sama dipasang di jalur buat grup maupun
tambah anggota — dua pintu masuk ke keadaan yang sama dengan batas berbeda
berarti salah satunya bisa dipakai melewati yang lain.

---

## Verifikasi

- `go vet` + `go test -race ./...` bersih; 9 test store baru
- `tsc --noEmit` + `vite build` bersih
- Uji HTTP langsung: DM ditolak untuk keempat tindakan (404) dan tetap berisi
  dua orang, anggota biasa ditolak (403), orang luar ditolak (404), judul kosong
  (400), id yang bukan pengguna (404), mengeluarkan pemilik (409), keempat cara
  menyentuh catatan sistem ditolak, dan yang dikeluarkan kehilangan seluruh
  aksesnya
- Verifikasi browser (Brave, **tiga** konteks terpisah, puppeteer-core):
  **28/28 lulus** — panel yang isinya berbeda untuk pemilik dan anggota, judul
  yang berubah realtime di layar orang lain, anggota baru yang grupnya langsung
  muncul di sidebar, anggota yang dikeluarkan dan grupnya langsung hilang, serta
  kepemilikan yang berpindah saat pemilik keluar
- Test Fase 9 dijalankan ulang: 19/19 dan 17/17, tidak ada regresi

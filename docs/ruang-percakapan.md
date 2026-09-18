# Ruang percakapan

Fase 14 tidak menambah satu pun fitur baru. Yang dikerjakan adalah tempat yang
paling sering dilihat orang di aplikasi ini — satu percakapan yang terbuka —
dan tiga hal yang selama tiga belas fase sebelumnya dibiarkan: pesan yang bisa
hilang, tindakan yang tidak bisa ditarik kembali, dan riwayat yang hanya bisa
dipakai dengan kursor.

Titik berangkatnya lima putaran kritik desain atas `ChatPanel.tsx`
(`.impeccable/critique/2026-09-17T*`), skor 22 dari 40 di putaran pertama dan
28–30 di putaran terakhir, ditambah satu putaran atas `AccountPanel.tsx`
(skor 24 dari 40). Dokumen ini mencatat keputusannya, bukan riwayat kritiknya.

| Bagian | Isi |
|---|---|
| Keandalan | antrean kirim di perangkat, jeda urung untuk hapus dan sematan |
| Riwayat dan gulir | enam aturan gulir di satu tempat, pil pesan baru, lipatan pesan terhapus |
| Tindakan pesan | menu panah, reaksi cepat, popover di layar lebar dan lembar di layar sempit |
| Aksesibilitas | riwayat sebagai satu pemberhentian Tab, label baris lengkap, kontras |
| Tata ulang kode | `ChatPanel` 1066 baris dipecah; `AccountPanel` 666 baris jadi tiga panel |
| Server | `.env` dibaca sendiri, `DELETE /api/account/sessions` |

---

## Keandalan

### Antrean kirim disimpan di perangkat

"Pesan tidak boleh hilang" adalah janji pertama aplikasi ini, dan sampai fase
ini janji itu hanya berlaku selama tab-nya terbuka. Pesan yang sudah ditulis
tapi belum sampai ke server — jaringan putus, server sedang 5xx — hilang begitu
halamannya dimuat ulang.

Sekarang antreannya disimpan di `localStorage`, satu kunci per akun
(`antrean:<userId>`). Kecil, dan cukup untuk beberapa kalimat yang tertahan
jaringan. Empat keputusan di sekitarnya:

- **Per akun, dipulihkan saat orangnya dikenali.** Bukan saat modul dimuat:
  sebelum sesi dibaca, belum jelas antrean siapa yang boleh tampil di perangkat
  ini. Logout membuang kuncinya.
- **Yang dipulihkan selalu berstatus gagal.** Pengiriman yang sedang berjalan
  saat halaman ditutup tidak diketahui nasibnya, dan "gagal" adalah satu-satunya
  keadaan yang punya tombol.
- **Kirim ulang otomatis begitu tersambung**, lewat `retryStranded`. Aman karena
  id pesan dibuat di client: server mengembalikan pesan yang sama, bukan pesan
  kedua. Itu jalur idempoten yang sama dengan `client_msg_id` sejak Fase 2.
- **Hanya yang gagal tanpa sebab yang diulang sendiri.** Yang ditolak server
  dengan alasan jelas (kuota, lampiran tidak sah) dibiarkan: mengulanginya
  diam-diam cuma menghasilkan penolakan yang sama. Kegagalan 5xx sengaja
  dibiarkan tanpa sebab, supaya masuk ke jalur ulang.

Penyimpanan yang penuh atau diblokir tidak membuat apa pun gagal: antreannya
tetap hidup selama tab terbuka, yang hilang hanya ketahanannya terhadap muat
ulang.

### Pesan yang gagal punya tiga jalan keluar

Sebelumnya satu: coba lagi. Sekarang gelembung yang gagal menyebut **sebabnya**,
lalu menawarkan **"Tulis ulang"** (isinya kembali ke kolom tulis, pesannya
dibuang dari antrean) dan **"Kirim ulang"**. Rekaman suara yang terhenti karena
orangnya pindah ruang tidak ikut hilang — rekamannya disimpan di laci lampiran
percakapan asalnya.

### Hapus ditahan delapan detik

Hapus mengenai pesan orang lain juga: yang dihapus hilang dari layar semua
anggota. Sekarang gelembungnya langsung tampil sebagai "dihapus", tapi
permintaannya baru berangkat delapan detik kemudian, dan selama itu ada kabar
dengan tombol **"Urungkan"** serta hitung mundur yang terlihat.

Delapan detik, bukan lima: waktu untuk membaca kabarnya, memahaminya, lalu
menemukan tombolnya diukur dari pembaca yang paling lambat, bukan dari yang
paling cepat.

Sisanya menyusul dari situ:

- Hitung mundurnya **bisa dijeda** — kursor atau fokus yang berhenti di
  kabarnya menahan waktunya, karena orang yang sedang membaca tawarannya
  jelas belum selesai memutuskan.
- Beberapa hapus berurutan **digabung** jadi satu kabar, bukan setumpuk.
- Menutup tab **tetap mengirimnya** (`pagehide` + `fetch` `keepalive`),
  termasuk yang sedang dijeda. Kabarnya sudah mengatakan "menghapus"; menutup
  tab tidak boleh diam-diam membatalkan. Yang ingin membatalkan punya tombolnya.
- Hapus yang **ditolak server** tidak hilang sendiri: pesannya masih ada, dan
  orangnya perlu memutuskan.

### Sematan ditahan empat detik

Sematan juga disiarkan ke semua orang sebagai catatan di riwayat, dan satu Enter
yang meleset di menu tidak boleh langsung menulis catatan itu. Jedanya lebih
pendek dari hapus (4 detik) karena akibatnya bisa dibatalkan belakangan; yang
tidak bisa dibatalkan adalah bahwa semua orang sempat melihatnya.

Menyematkan sendiri tetap **tidak optimistik**: server bisa menolak karena batas
jumlah sematan, dan menampilkannya lebih dulu berarti menampilkan sesuatu yang
mungkin dicabut sedetik kemudian.

### Satu kabar pada satu waktu

`NoticeStack` menjaga paling banyak satu kabar di atas kolom tulis, dengan
urutan menurut apa yang paling butuh tindakan: hapus yang ditolak server, lalu
hapus yang masih menunggu, lalu kabar singkat ("Teks disalin."). Tanpa aturan
itu, kabar-kabar bertumpuk di atas laci lampiran dan kutipan balasan, dan kolom
tulis terdorong jauh ke bawah justru saat orang ingin menulis. Yang tidak tampil
tetap diumumkan ke pembaca layar.

Yang sengaja **tidak** di sana: bilah koneksi dan galat sematan menempel di
bawah kepala percakapan (keduanya tentang percakapan, bukan tentang apa yang
sedang ditulis), "sedang mengetik" tinggal di wilayah statusnya sendiri, dan
laci lampiran serta kutipan balasan adalah bagian kolom tulis.

---

## Riwayat dan gulir

Semua aturan gulir dikumpulkan di satu hook, `useHistoryScroll`. Sebelumnya
tersebar di beberapa `useEffect` yang saling menimpa, dan yang paling terlihat:
percakapan yang baru dibuka mendarat di pesan **terlama**.

1. Pindah percakapan: langsung ke pesan terbaru.
2. Isian pertama setelah membuka percakapan yang riwayatnya belum ada: selalu ke
   bawah. Saat itu posisi gulirnya masih nol, jadi "dekat bawah" selalu salah.
3. Pesan sendiri yang baru dikirim: selalu ke bawah, dari mana pun. Menekan
   Enter lalu tidak melihat apa pun adalah kabar yang hilang.
4. Pesan orang lain: ikut turun hanya kalau pembaca memang di dekat bawah.
   Selama dia membaca ke atas, yang datang dihitung untuk pil **"N pesan baru"**.
5. Tinggi atau isi area baca berubah (bilah yang datang belakangan, gambar yang
   selesai dimuat, papan ketik ponsel): tetap menempel di bawah bila tadinya di
   bawah.
6. Pesan yang baru saja gagal: yang memicu gulir adalah perubahan **status**,
   karena baris peringatannya setinggi tombol dan bisa jatuh di bawah tepi layar.

Tempelan ke bawah tidak lagi diputus oleh *scroll anchoring* peramban.

Riwayatnya sendiri disusun sekali di `chatHistory.ts`, bukan dihitung ulang di
dalam JSX: di mana hari berganti, pesan mana yang membuka rentetan (jeda
setengah jam memutus rentetan — dua pesan sejauh itu bukan satu tarikan napas),
dan deretan pesan terhapus mana yang **dilipat** jadi satu baris. Yang perlu
diketahui dari sederet pesan terhapus cuma bahwa ada yang dihapus, dan berapa.

Tata letaknya: kolom baca dibatasi **56rem** supaya barisnya tidak jadi selebar
monitor, penanda hari digambar sebagai garis cakrawala, dan percakapan kosong
membuka dengan salam menurut jam beserta ajakan yang menyebut nama lawan
bicaranya.

---

## Tindakan pesan

Tindakan pesan pindah ke **menu panah** di pojok gelembung, redup saat diam dan
muncul saat gelembungnya dihover atau barisnya difokus papan ketik. Di dalamnya
ada baris reaksi cepat, jadi bereaksi dan menyalin tidak tinggal di dua tempat
berbeda.

Bentuknya mengikuti layar: **popover** di layar lebar, **lembar** yang naik dari
bawah di layar sempit. Jalan masuknya tiga — tombol panah, klik kanan, dan tekan
lama. Di layar sentuh reaksi hanya lewat lembar, karena di sana tidak ada hover
yang bisa memunculkan bilah cepat, dan Enter di kolom tulis membuat baris baru,
bukan mengirim.

Ruang panah dan ruang tombol reaksi tetap dipesan walau keduanya belum terlihat,
supaya tata letak tidak bergeser saat kursor lewat.

---

## Aksesibilitas

### Riwayat sebagai satu pemberhentian Tab

Riwayat panjang berisi ratusan tombol: chip reaksi, lampiran, pemutar suara,
menu. Tanpa aturan, Tab dari kolom tulis melewati semuanya satu per satu.

`useRovingLog` membuat setiap baris (`[data-msg]`) bisa difokus, tapi hanya satu
yang ada di urutan Tab: yang terakhir dipilih, atau pesan terbaru. Panah
atas/bawah berpindah baris, Home/End ke ujung, Enter membuka tindakan baris itu.
Tombol di dalam baris baru ikut urutan Tab untuk baris yang terpilih, dan hanya
setelah fokus berada di baris itu — jadi Shift+Tab dari kolom tulis mendarat di
baris pesan, bukan di tombol reaksinya.

Aturannya ditegakkan di DOM, bukan lewat prop di tiap komponen. Satu komponen
yang lupa diberi prop sudah cukup untuk mengembalikan ratusan pemberhentian itu.

### Yang terdengar saat berpindah baris

`aria-label` pada sebuah artikel **menggantikan** isinya bagi pembaca layar,
jadi labelnya dibangun lengkap: siapa, kapan, apa, ditambah isi catatan sistem,
bahwa pesan itu menyebut namanya, bahwa ada lampiran di samping teksnya, dan
berapa reaksi yang sudah diberikan.

Sisanya: wilayah status yang wadahnya sudah terpasang sebelum teksnya datang
(wadah yang baru muncul bersama teksnya tidak dibacakan), Esc dan Shift+Tab
kembali ke riwayat, `↑` di kolom kosong menyunting pesan terakhir, dan daftar
sebutan sebagai listbox sungguhan.

### Warna dan target

- Teks di gelembung sendiri putih penuh di atas token `accent-deep` — ≥ 4,6:1.
- Cincin fokus dibuat terlihat di atas teal.
- Target sentuh 40–44px, patokan yang lalu ditulis ke DESIGN.md.
- Lencana "belum terverifikasi" memakai token bahaya, bukan mangga:
  `bg-call-soft`/`text-call-ink` turun ke 1,20:1 di tema gelap — praktis tidak
  terlihat, padahal itu satu-satunya penanda bahwa akun ini tidak punya jalan
  pulang. Mangga juga sudah punya arti sendiri: sebutan dan panggilan.
- Warna teks contoh diambil dari token redup; bawaan Tailwind jatuh ke 2,97:1 di
  tema terang.

---

## Tata ulang kode

`ChatPanel.tsx` masuk fase ini dengan 1066 baris dan keluar dengan 775, setelah
melepas dua belas bagian: `Composer`, `ChatHeader`, `ConversationStates`,
`NoticeStack`, `DeleteNotice`, `MentionList`, `RecordingBar`, `MessageMenu`,
`MessageParts`, `QuickReactions`, `ui/PillButton`, `ui/IconButton`. Tiga hook
(`useHistoryScroll`, `useRovingLog`, `useMediaQuery`) dan dua modul
(`format.ts`, `teks.ts`) menyusul.

`AccountPanel.tsx` (666 baris) dipecah jadi tiga panel dengan tiga pintu:

| Panel | Isi | Pintunya |
|---|---|---|
| Profil | foto, nama, status | avatar di sidebar |
| Akun | email, password, perangkat, "Keluar" | ikon kunci |
| Preferensi | tema, notifikasi push | ikon roda |

Satu panel memikul tiga urusan dengan frekuensi pemakaian yang sangat berbeda:
foto dan status berkali-kali sehari, tema sekali seumur pemakaian, email dan
password beberapa kali setahun. Keenam bagiannya digambar dengan chrome yang
identik, sehingga tidak satu pun batasnya membawa informasi. "Keluar" ikut
pindah dari pojok sidebar ke kaki panel Akun, tempat tindakan berakibat besar
lainnya berada, dan tombol notifikasi yang dulu jadi ikon lonceng tanpa label
akhirnya punya nama.

### `PanelShell`

Kerangka panel kanan yang sebelumnya disalin lima kali. Yang disalin bukan cuma
tampilannya: panel yang **menutupi seluruh layar** adalah dialog, dan tidak satu
pun dari mereka mengatakannya — fokus tertinggal di belakang panel, Escape tidak
menutup, Tab berjalan keluar ke percakapan yang sedang tertutup. Sekarang
`PanelShell` yang memegang `role="dialog"`, Escape, pemindahan dan pengembalian
fokus, serta pengurungan fokus. `GroupPanel` dan `SearchPanel` ikut memakainya,
jadi keduanya ikut sembuh.

Yang menentukan dia dialog atau bukan adalah **lebar**, bukan jenis panelnya: di
layar lebar dia kolom biasa di sebelah percakapan, dan mengurung fokus di kolom
yang tidak menutupi apa pun justru menjebak orangnya. Batasnya **lg (1024px)**,
bukan md: di 768px, sidebar 320 dan panel 320 menyisakan 128px untuk gelembung
pesan.

Judul bagian di dalam panel kini heading sungguhan, bukan paragraf tebal —
pembaca layar yang melompat antar-heading sebelumnya menemukan satu benda di
seluruh panel setelan.

### Perbaikan yang ikut, di kode yang memang sedang dipindah

- Lima kolom berpindah dari teks contoh ke `<label>` sungguhan, dengan
  `autocomplete` supaya pengelola password bisa mengisinya, dibungkus `<form>`
  supaya Enter mengirim.
- Kabar berhasil dan gagal memakai `role="status"`/`role="alert"`.
- Tombol "Cabut" terlihat saat difokus papan ketik, bukan hanya saat disentuh
  kursor. Aksi anggota di `GroupPanel` diperbaiki dengan cara yang sama.
- Mengganti status tidak lagi menghapus batas waktunya. Baris durasi berhenti
  berpura-pura menyimpan pilihan: yang tersimpan cuma satu instan, dan instan itu
  tidak pernah sama dengan tombol mana pun semenit kemudian.
- Daftar perangkat diangkat ke store dan dimuat ulang setelah ganti password,
  supaya tidak membantah kalimat yang baru saja diumumkannya.
- Empty state untuk daftar perangkat.

---

## Server

Dua perubahan, keduanya kecil.

**`.env` dibaca sendiri** (`internal/config/dotenv.go`): dari direktori kerja ke
atas sampai ketemu, tanpa pernah menimpa variabel lingkungan yang sudah ada.
Server biasa dijalankan dari `server/` sementara `.env` tinggal di akar repo, dan
tanpa ini lampiran serta email mati diam-diam di mesin pengembang.

**`DELETE /api/account/sessions`** mengeluarkan semua perangkat kecuali yang
sedang dipakai. Sampai sekarang jalan menuju keadaan itu cuma satu: mengganti
password, lalu masuk lagi di setiap perangkat yang masih dipegang. Sengaja tidak
menuntut password — yang paling buruk bisa dilakukan orang asing yang sudah
memegang sesi adalah mengeluarkan perangkat pemiliknya, dan pemiliknya tinggal
masuk lagi.

---

## Dokumen

Fase ini juga menulis dua dokumen yang sebelumnya tidak ada:
[PRODUCT.md](../PRODUCT.md) (untuk siapa aplikasi ini, dan apa yang dijanjikan)
dan [DESIGN.md](../DESIGN.md) — sistem desain "Pagi di Tepi Air", beserta
`.impeccable/design.json` yang dipakai putaran kritik berikutnya.

---

## Verifikasi

| Pemeriksaan | Hasil |
|---|---|
| `go vet ./...` | bersih |
| `go test -race ./...` | bersih (api, blob, config, hub, imaging, push, ratelimit, store) |
| `tsc --noEmit` | bersih |
| `vite build` | bersih — 353 kB, 109 kB gzip |
